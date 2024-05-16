package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/netip"
	"os"
	"strings"

	"github.com/tailscale/tailscale-client-go/tailscale"
	"go4.org/netipx"
)

var (
	fromPrefixes    prefixSlice
	toPrefixes      prefixSlice
	maxRetries      = flag.Int("max-retries", 5, "max times to retry if random new IP is already in use")
	continueOnError = flag.Bool("continue-on-error", false, "continue reassigning devices if an error for any device is encountered")
	silent          = flag.Bool("silent", false, "do not output any messages")

	cgnatPfx = netip.MustParsePrefix("100.64.0.0/10")
)

type prefixSlice []netip.Prefix

func (i *prefixSlice) String() string {
	return fmt.Sprintf("%s", *i)
}

func (i *prefixSlice) Set(value string) error {
	values := strings.Split(value, ",")
	for _, v := range values {
		parsedPrefix, err := netip.ParsePrefix(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		if !cgnatPfx.Overlaps(parsedPrefix) {
			return errors.New(fmt.Sprintf("prefix [%s] is not within [%s]", v, cgnatPfx))
		}
		*i = append(*i, parsedPrefix)
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: tailscale-prefix-mover [flags]\n")
	flag.PrintDefaults()
}

func checkArgs() error {
	if fromPrefixes == nil || len(fromPrefixes) == 0 {
		return errors.New("missing required flag -from-prefixes")
	}
	return nil
}

func main() {
	flag.Var(&fromPrefixes, "from-prefixes", fmt.Sprintf("prefixes to move devices FROM - must be within %s", cgnatPfx))
	flag.Var(&toPrefixes, "to-prefixes", fmt.Sprintf("prefixes to move devices to - must be within %s", cgnatPfx))
	flag.Parse()

	err := checkArgs()
	if err != nil {
		logStderr("%s\n", err)
		usage()
		os.Exit(1)
	}

	apiKey := os.Getenv("TAILSCALE_API_KEY")
	tailnet := os.Getenv("TAILSCALE_TAILNET")

	tailscaleClient, err := tailscale.NewClient(apiKey, tailnet)
	if err != nil {
		log.Fatalln(err)
	}

	availablePrefixes := toPrefixes
	if availablePrefixes == nil {
		availablePrefixes, err = calculateAvailablePrefixes(fromPrefixes)
		if err != nil {
			log.Fatalln(err)
		}
	}

	logVerbose("Moving devices from %s to %s\n", fromPrefixes, availablePrefixes)

	ctx := context.Background()
	devices, err := tailscaleClient.Devices(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	errCount := 0
	for _, fromPrefix := range fromPrefixes {
		for _, device := range devices {
			v4Address, err := netip.ParseAddr(device.Addresses[0])
			if err != nil {
				log.Fatalln(err)
			}

			if fromPrefix.Contains(v4Address) {
				err = reassignDeviceAddress(ctx, tailscaleClient, device, availablePrefixes)
				if err != nil {
					errCount++
					logStderr("error setting address for device [nodeid:%-16s / name:%s] - [%s]\n", device.ID, device.Name, err)
					if *continueOnError {
						logVerbose(" Continuing...\n")
						continue
					} else {
						logVerbose(" Stopping.\n")
						break // unnecessary because log.Fatal will exit, but seems good to have here anyway
					}
				}
			}
		}
	}
	if errCount > 0 {
		logVerbose(("Done.\n"))
		os.Exit(1)
	} else {
		logVerbose(("Done.\n"))
	}
}

func reassignDeviceAddress(ctx context.Context, tailscaleClient *tailscale.Client, device tailscale.Device, availablePrefixes []netip.Prefix) error {
	for i := 0; i < *maxRetries; i++ {
		prefix := availablePrefixes[rand.IntN(len(availablePrefixes))]
		newAddress := randV4(prefix)

		logVerbose("Setting v4 address [%-15s] to [nodeid:%-18s / name:%s]... ", newAddress, device.ID, device.Name)
		err := tailscaleClient.SetDeviceIPv4Address(ctx, device.ID, newAddress.String())
		if err != nil && err.Error() == "address already in use (500)" {
			logVerbose("[%s] - retrying...\n", err)
			continue
		} else if err != nil {
			return err
		} else {
			logVerbose("done.\n")
			return nil
		}
	}
	return errors.New(fmt.Sprintf("Unable to set new address after [%v] tries", *maxRetries))
}

func calculateAvailablePrefixes(prefixes []netip.Prefix) ([]netip.Prefix, error) {
	var b netipx.IPSetBuilder
	b.AddPrefix(cgnatPfx)

	for _, p := range prefixes {
		b.RemovePrefix(p)
	}

	s, err := b.IPSet()
	if err != nil {
		return nil, err
	}
	return s.Prefixes(), nil
}

// TODO: simplify this?
func randV4(maskedPfx netip.Prefix) netip.Addr {
	bits := 32 - maskedPfx.Bits()
	randBits := rand.Uint32N(1 << uint(bits))

	ip4 := maskedPfx.Addr().As4()
	pn := binary.BigEndian.Uint32(ip4[:])
	binary.BigEndian.PutUint32(ip4[:], randBits|pn)
	return netip.AddrFrom4(ip4)
}

func logVerbose(message string, a ...any) {
	if !*silent {
		logStderr(fmt.Sprintf(message, a...))
	}
}

func logStderr(message string, a ...any) {
	os.Stderr.WriteString(fmt.Sprintf(message, a...))
}
