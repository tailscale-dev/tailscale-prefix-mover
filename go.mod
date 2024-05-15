module github.com/clstokes/tailscale-prefix-mover

go 1.22.0

toolchain go1.22.3

// replace github.com/tailscale/tailscale-client-go => /Users/cameron/cc/tailscale/tailscale-client-go-set-device-ivp4-address // TODO: remove

require (
	github.com/tailscale/tailscale-client-go v1.17.1-0.20240515175515-5de5ead197a1
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba
)

require (
	github.com/tailscale/hujson v0.0.0-20220506213045-af5ed07155e5 // indirect
	golang.org/x/oauth2 v0.19.0 // indirect
)
