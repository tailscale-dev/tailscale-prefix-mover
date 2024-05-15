package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	randv2 "math/rand/v2" // TODO: necessary?
	"net/netip"
	"os"
	"strings"

	"github.com/tailscale/tailscale-client-go/tailscale"
	"go4.org/netipx"
)

var (
	fromPrefixes    stringSlice
	maxRetries      = flag.Int("max-retries", 10, "max times to retry if random new IP is already in use")
	continueOnError = flag.Bool("continue-on-error", false, "continue reassigning devices if an error for any device is encountered")
	silent          = flag.Bool("silent", false, "do not output any messages")
)

type stringSlice []string

func (i *stringSlice) String() string {
	return fmt.Sprintf("%s", *i)
}

func (i *stringSlice) Set(value string) error {
	values := strings.Split(value, ",")
	for _, v := range values {
		// TODO: parse with netip.MustParsePrefix(strings.TrimSpace(s)) here?
		*i = append(*i, v)
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: tailscale-prefix-mover [flags]\n")
	flag.PrintDefaults()
}

func checkArgs() error {
	// if *inParentFile == "" {
	// 	return errors.New("missing argument -f - a parent file must be provided")
	// }
	// if *inChildDir == "" {
	// 	return errors.New("missing argument -d - a directory of child files to process must be provided")
	// }
	// if len(allowedAclSections) == 0 {
	// 	return errors.New("missing argument -allow - a list of acl sections to allow from children must be provided - e.g. -allow=acls,ssh")
	// }
	return nil
}

func main() {
	flag.Var(&fromPrefixes, "prefixes", "prefixes to evacuate")
	flag.Parse()
	argsErr := checkArgs()
	if argsErr != nil {
		fmt.Fprintf(os.Stderr, "%s\n", argsErr)
		usage()
		os.Exit(1)
	}

	apiKey := os.Getenv("TAILSCALE_API_KEY")
	tailnet := os.Getenv("TAILSCALE_TAILNET")

	tailscaleClient, err := tailscale.NewClient(apiKey, tailnet)
	if err != nil {
		log.Fatalln(err)
	}

	availablePrefixes, err := availablePrefixes(fromPrefixes)
	if err != nil {
		log.Fatalln(err)
	}

	logVerbose("Moving devices from %s to %s\n", fromPrefixes, availablePrefixes.Prefixes())

	ctx := context.Background() // TODO: probably not right?

	devices, err := tailscaleClient.Devices(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	errCount := 0
	for _, prefix := range fromPrefixes {
		for _, device := range devices {
			pfx, err := netip.ParsePrefix(prefix) // TODO: move ParsePrefix to arg parsing
			if err != nil {
				log.Fatalln(err)
			}

			v4Address, err := netip.ParseAddr(device.Addresses[0])
			if err != nil {
				log.Fatalln(err)
			}

			if pfx.Contains(v4Address) {
				err = reassignDeviceAddress(ctx, tailscaleClient, device, availablePrefixes.Prefixes())
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
		prefix := availablePrefixes[rand.Intn(len(availablePrefixes))]
		newAddress := randV4(prefix)

		logVerbose("Setting v4 address [%-15s] to [nodeid:%-18s / name:%s]... ", newAddress, device.ID, device.Name)
		err := tailscaleClient.SetDeviceIPv4Address(ctx, device.ID, newAddress.String())
		if err != nil && err.Error() == "address already in use (500)" {
			logVerbose("[%s] - retrying...\n", err)
			continue
		} else if err != nil {
			return err
		}
		logVerbose("done.\n")
		return nil
	}
	return errors.New(fmt.Sprintf("Unable to set new address after [%v] tries", *maxRetries)) // TODO: falls through to this too easily, need to move this
}

func availablePrefixes(prefixes []string) (*netipx.IPSet, error) {
	var b netipx.IPSetBuilder
	b.AddPrefix(netip.MustParsePrefix("100.64.0.0/10"))

	for _, p := range prefixes {
		b.RemovePrefix(netip.MustParsePrefix(p)) // TODO: handle unparsable prefixes?
	}

	s, err := b.IPSet()
	if err != nil {
		return nil, err
	}
	return s, nil
}

// TODO: simplify this?
func randV4(maskedPfx netip.Prefix) netip.Addr {
	bits := 32 - maskedPfx.Bits()
	randBits := randv2.Uint32N(1 << uint(bits))

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
