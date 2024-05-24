# tailscale-prefix-mover

[![status: experimental](https://img.shields.io/badge/status-experimental-blue)](https://tailscale.com/kb/1167/release-stages/#experimental)

Provide a set of prefixes within `100.60.0.0/10` and this tool will find devices within those prefixes and reassign devices to other space within the CGNAT prefix.

See [Visual Subnet Calculator](https://www.davidc.net/sites/default/subnets/subnets.html?network=100.64.0.0&mask=10&division=1.0) for an easy subnet calculator.

## Usage

```shell
go run github.com/tailscale-dev/tailscale-prefix-mover -help
```

### Example

Pass `-apply` to make changes.

```shell
export TAILSCALE_TAILNET=...
export TAILSCALE_API_KEY=...

go run github.com/tailscale-dev/tailscale-prefix-mover -from-prefixes=100.72.0.0/13,100.96.0.0/11
Moving devices from [100.72.0.0/13 100.96.0.0/11] to [100.64.0.0/13 100.80.0.0/12]
Setting v4 address [w.x.y.z  ] to [nodeid:1234567890   / name:device123.example.ts.net]... done.
Setting v4 address [w.x.y.z  ] to [nodeid:9876543210   / name:device987.example.ts.net]... done.
Pass -apply to make changes.
Done.
```

### Example with -to-prefixes

Pass `-apply` to make changes.

```shell
export TAILSCALE_TAILNET=...
export TAILSCALE_API_KEY=...

go run github.com/tailscale-dev/tailscale-prefix-mover -from-prefixes=100.72.0.0/13,100.96.0.0/11 -to-prefixes=100.64.0.0/24
Moving devices from [100.72.0.0/13 100.96.0.0/11] to [100.64.0.0/24]
Setting v4 address [w.x.y.z  ] to [nodeid:1234567890   / name:device123.example.ts.net]... done.
Setting v4 address [w.x.y.z  ] to [nodeid:9876543210   / name:device987.example.ts.net]... done.
Pass -apply to make changes.
Done.
```
