<p align="center">
  <img src="docs/demo.gif" alt="proxray in action" width="900" />
</p>

# proxray

**English** | [Русский](README.ru.md)

A terminal VPN client compatible with [HAPP](https://happ.su) subscription
profiles. It fetches a subscription, parses its share links
(VLESS / VMess / Trojan / Shadowsocks), and connects through an embedded
[xray-core](https://github.com/XTLS/Xray-core) — as a local proxy, a system
proxy, or a full system-wide TUN tunnel.

Single self-contained binary: xray-core and tun2socks are embedded, no external
binaries required.

## Features

- **HAPP-compatible** subscriptions in both formats a panel may serve: a base64
  list of share links, or a JSON document of ready-made xray configs. The
  metadata headers HAPP clients understand are read too:
  `subscription-userinfo` (traffic/expiry), `profile-title`,
  `profile-update-interval`, `support-url`. Requests are sent with
  `User-Agent: Happ/1.0` so panels return the format HAPP expects (overridable
  with `--ua`).
- **Device limit (HWID)**: panels that count devices — Remnawave's *HWID Device
  Limit* and compatible ones — answer `404` to clients that do not identify
  themselves. proxray sends `x-hwid` plus `x-device-os` / `x-ver-os` /
  `x-device-model`, deriving a stable id from the machine so re-adding a
  subscription does not eat another device slot. See `proxray hwid`.
- **JSON subscriptions run as the panel wrote them**: routing rules, balancers
  and observatory-based auto-switching are kept verbatim, and only the listen
  ports and log level are ours. Since the panel already decides what goes
  direct, `--bypass` is not applied to such entries.
- **Protocols**: VLESS (incl. Reality / XTLS Vision), VMess, Trojan,
  Shadowsocks, Hysteria2. **Transports**: TCP, WebSocket, gRPC, HTTP/2.
  **Security**: TLS, Reality.
- **Three ways to route traffic**:
  - `connect` — local SOCKS5 + HTTP proxy on `127.0.0.1` (no root);
  - `connect --system-proxy` — sets the macOS system SOCKS + HTTP/HTTPS proxy
    (needs `sudo`), so browsers and most apps go through it without touching the
    routing table — coexists with another active VPN;
  - `connect --mode tun` — full system-wide VPN via a utun device (needs `sudo`).

> **Note**: xray-core cannot dial TUIC, nor Hysteria2 with obfuscation. Such
> servers are still parsed and listed (marked `unsupported`), but you cannot
> connect to them through xray (a sing-box based core would be required).

## How it works

```
subscription URL
      │  profile.Fetch (User-Agent: Happ/1.0, x-hwid: device id)
      ▼
base64 list of links ──► link.Parse ──► []link.Server
                                            │ xray.BuildConfig
      JSON configs ─────► rawconf.Parse ────┤ rawconf.Render
                                            ▼
                                    xray-core config (JSON)
                                            │ xray.Start (embedded core)
              ┌─────────────────────────────┼─────────────────────────────┐
              ▼                             ▼                             ▼
      proxy: SOCKS5/HTTP          --system-proxy: networksetup     tun: tun2socks
      on 127.0.0.1                sets system SOCKS/HTTP            + route table
      (no root)                   (sudo)                           (sudo, utun)
```

## Install

### mise (recommended)

Prebuilt binaries are published to GitHub Releases. Install with
[mise](https://mise.jdx.dev) — no Go toolchain required:

```sh
mise use -g "github:aimuzov/proxray@latest"
```

or pin it in `mise.toml`:

```toml
[tools]
"github:aimuzov/proxray" = "latest"
```

The `ubi` backend works the same way against the same releases, if you prefer it:
`ubi:aimuzov/proxray`.

> For frequent installs, set `MISE_GITHUB_TOKEN` (or `GITHUB_TOKEN`) to avoid
> GitHub API rate limits.

### Manual download

Download the archive for your OS/arch from the
[Releases](https://github.com/aimuzov/proxray/releases) page, extract it, and
put the `proxray` binary on your `PATH`.

### From source

```sh
git clone https://github.com/aimuzov/proxray
cd proxray
go build -o proxray .   # requires Go 1.26+
```

The resulting `proxray` binary is self-contained.

> **`go install github.com/aimuzov/proxray@latest` does not work.** The build
> relies on a `replace` directive in `go.mod` (to reconcile xray-core and
> tun2socks on gvisor), and `go install pkg@version` ignores `replace`. Use a
> prebuilt binary or clone and build.

## Usage

### Subscriptions

```sh
proxray sub add https://panel.example/sub/TOKEN --name myvpn   # add (becomes active)
proxray sub list                                               # list subscriptions
proxray sub update [name]                                      # re-fetch (default: active)
proxray sub use <name>                                         # set the active subscription
proxray sub rm <name>                                          # remove
```

### Device limit (HWID)

Panels with a device limit count devices by the `x-hwid` header. proxray derives
one from the machine on first use and stores it, so it stays the same across
updates and re-adds:

```sh
proxray hwid                     # show the id and the device headers that go with it
proxray hwid set <id>            # use a specific id (10-64 chars of a-zA-Z0-9=-)
proxray hwid reset               # forget the stored id and derive it again
proxray hwid reset --random      # look like a different device on the same machine
proxray sub add <url> --no-hwid  # do not identify this machine at all
```

`sub add` and `sub update` print the id they sent. Most panels only read the
header and answer nothing, so `-v` spells out both halves of the exchange:

```
DEBU subscription request  host=panel.example user-agent=Happ/1.0 x-hwid=64ed… x-device-os=macOS …
DEBU subscription response status=200 hwid-headers="none (panel ignores the device id)"
```

The request path is never logged — it carries the subscription token.

`sub list` shows traffic and expiry from the subscription headers:

```
ACTIVE  NAME    TITLE       SERVERS  TRAFFIC          EXPIRES
*       myvpn   My VPN      12       12.4 GB / 200 GB  2026-09-01
```

### Servers

```sh
proxray list           # servers in the active subscription
proxray list --sub x   # servers in a specific subscription
```

```
#  PROTOCOL   ADDRESS         TAG               NOTE
1  vless      4 servers       🇪🇺 Auto           Picks the fastest server
2  vless      de.example:443  🇩🇪 Germany
3  hysteria   hy.example:443  Fast HY2
```

Entries of a JSON subscription show the panel's own description, and an entry
that pools several servers behind a balancer shows their count instead of a
single address. The `NOTE` column is omitted when the subscription has no
descriptions.

### Connecting

`connect` runs in the foreground until interrupted with `Ctrl+C`. The `selector`
picks a server: empty = first, a number = 1-based index from `proxray list`, or a
case-insensitive substring of the server tag.

```sh
proxray connect                 # first server, proxy mode
proxray connect 2               # server #2
proxray connect germany         # first server whose tag matches "germany"

sudo proxray connect 1 --system-proxy   # browsers/apps via system proxy (no routing changes)
sudo proxray connect 1 --mode tun       # full system-wide VPN
```

In plain proxy mode, point apps at `socks5://127.0.0.1:10808` (Firefox: enable
"Proxy DNS when using SOCKS v5").

### `connect` flags

| Flag             | Default | Description                                         |
| ---------------- | ------- | --------------------------------------------------- |
| `-m, --mode`     | `proxy` | `proxy` or `tun`                                    |
| `--socks`        | `10808` | local SOCKS5 port                                   |
| `--http`         | `10809` | local HTTP proxy port (proxy mode)                  |
| `--system-proxy` | `false` | set the macOS system proxy (proxy mode, needs sudo) |
| `--sub`          | active  | subscription name                                   |
| `--bypass`       | `ru`    | route a region's traffic direct: `ru` or `off`      |

### Three ways to route traffic, compared

- **`connect` (proxy)** — only apps explicitly pointed at
  `socks5://127.0.0.1:10808` (e.g. Firefox with remote DNS). No root.
- **`connect --system-proxy`** — sets, on every enabled network service, the
  system SOCKS (`--socks` port) and HTTP/HTTPS (`--http` port) proxies, so
  Safari/Chrome and apps that ignore SOCKS go through the proxy. Does **not**
  touch the routing table, so it **coexists with another active VPN**. Needs
  `sudo`; the previous proxy settings are restored on exit. If a session was
  killed (`kill -9`) and the proxy stuck, reset it with
  `sudo proxray system-proxy off`.
- **`connect --mode tun`** — a full system VPN via a utun device; captures all
  traffic. Needs `sudo`. If another VPN is active at the same time, disconnect it
  first so the tunnels don't fight over routes/DNS.

### Bypassing Russian traffic

By default proxray routes Russian domains and IP ranges (`geosite:category-ru`,
`geoip:ru`) straight out, outside the tunnel, so sites that block foreign VPNs
(e.g. `ozon.ru`) keep working. The first connect downloads the `geoip.dat` and
`geosite.dat` databases from
[Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)
into `<config>/geo/` and refreshes them once a day. Bypass is effective in proxy
and `--system-proxy` modes. In tun mode it is **not supported yet** (the direct
outbound's sockets still loop back through utun), so `connect --mode tun` forces
bypass off with a warning and routes everything through the tunnel.

```sh
proxray connect --bypass off    # route everything through the tunnel (one run)
proxray connect --bypass ru     # force RU bypass (one run)

proxray route                   # show the active subscription's bypass setting
proxray route set off           # persist: send all traffic through the tunnel
proxray route set ru            # persist: bypass Russian traffic (default)
proxray route update            # force-refresh the geo databases
```

The `--bypass` flag overrides the stored setting for a single run; `route set`
changes the stored default per subscription. If the databases can't be
downloaded while bypass is enabled, connect fails with an explanatory error
rather than silently sending RU traffic through the tunnel — retry with a
network connection or use `--bypass off`.

### Other commands

```sh
proxray config [selector]       # print the generated xray-core config (debug)
proxray system-proxy off        # emergency reset of the system proxy (sudo)
```

## Configuration & storage

State (subscriptions and cached links) is stored as `state.json` in the
per-user config directory (`~/Library/Application Support/proxray` on macOS),
overridable with the global `--home` flag.

## TUN mode details (macOS)

1. the server address is resolved to IP(s), and a host route to each is pinned to
   its current next hop (a physical gateway, or an already-active VPN interface),
   so the proxy's own connection to the server does not loop back into the tunnel;
2. a `utun` device is created and tun2socks forwards its traffic to the local
   SOCKS proxy served by xray;
3. the default route is overridden with two `/1` routes scoped to the utun device
   (the real default route is left intact for a clean teardown);
4. global IPv6 is routed into `lo0` (blocked), so IPv6-capable sites
   (Google, YouTube) don't leak outside the tunnel — apps fall back to IPv4
   through the tunnel; link-local IPv6 keeps working via its more-specific route;
5. on `Ctrl+C` all routes are removed in reverse order.

## Limitations

- **TUIC**, and **Hysteria2 with obfuscation**, cannot be dialed by xray-core
  (they are parsed and listed as `unsupported`).
- **`--bypass` does not apply to JSON subscription entries**: their routing comes
  from the panel's own rules.
- **TUN and `--system-proxy` are macOS-only** in this version.
- **IPv6 is blocked in TUN mode** (the proxy path is IPv4); IPv6-only
  destinations become unreachable while connected.
- `connect` runs in the **foreground**; there is no background daemon yet.
- A `kill -9` skips cleanup: a system proxy stays set (`sudo proxray system-proxy
off`) and TUN IPv6-block routes remain (`sudo route -n delete -inet6 -net
::/1; sudo route -n delete -inet6 -net 8000::/1`). A normal `Ctrl+C` cleans up.

## Project layout

| Package             | Responsibility                                              |
| ------------------- | ----------------------------------------------------------- |
| `internal/link`     | parse share links (vless/vmess/trojan/ss/hysteria2)         |
| `internal/profile`  | fetch a subscription, decode its body + headers             |
| `internal/rawconf`  | JSON subscriptions: inspect and adjust ready-made configs   |
| `internal/xray`     | build xray-core config from a server, run the embedded core |
| `internal/tunnel`   | TUN mode: tun2socks + macOS route management                |
| `internal/sysproxy` | macOS system proxy via networksetup                         |
| `internal/store`    | persist subscriptions and cached links                      |
| `internal/cli`      | cobra commands                                              |

## Development

```sh
go test ./...        # unit tests + a real end-to-end proxy test
go vet ./...
```

The xray integration test starts a real Shadowsocks server and a client built
from a `link.Server`, then verifies an HTTP request routed through the client's
SOCKS inbound reaches a target through the proxy.

> xray-core and tun2socks require different `gvisor.dev/gvisor` versions; a
> `replace` directive in `go.mod` pins gvisor to the version both build against.
> Don't drop it — see the comment there.

### Recording the demo

The gif at the top is scripted, so it can be redone on any machine that has
[vhs](https://github.com/charmbracelet/vhs) and `ttyd`:

```sh
go build -o proxray .
cd docs && vhs demo.tape
```

It runs against a local fake panel (`docs/demo/panel.go` serves an invented
subscription on `127.0.0.1:8099`), in the throwaway fish profile under
`docs/demo-profile`, with `HOME` pointed at `docs/demo-home`. The commands on
screen are the real ones; the subscription behind them belongs to nobody, so
neither the recorder's URL nor their servers end up in the gif. The first
recording downloads the geo databases the RU bypass needs; later ones reuse the
cache.

### Releasing

Releases are built by [GoReleaser](https://goreleaser.com) in CI on a tag push:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `release` workflow (`.github/workflows/release.yml`) builds darwin/linux
binaries for amd64/arm64 and uploads them to GitHub Releases. Building there
honors the `go.mod` `replace` directive (proxray is the main module). Dry-run
locally with `goreleaser release --clean --snapshot`.
