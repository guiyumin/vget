# Sharing a library without a public IP (tailcat)

`vget serve` shares a directory as a WebDAV library. By default it also opens a
**tailcat address**: an encrypted WireGuard tunnel that works across NATs with
no public IP, domain, certificate, port forwarding or account. The other side
only needs the address (and the library's user/password).

The protocol on the wire is still WebDAV + Basic Auth. tailcat only decides how
the two machines connect; swap it for a LAN address, Tailscale, or an HTTPS
reverse proxy (`--listen`) and vget behaves exactly the same.

## Commands

| Command | What it does |
|---|---|
| `vget serve [dir]` | Long-running library. Read-only by default, `--rw` to allow writes, `--inbox <dir>` for a write-only drop box at `/inbox`. Requires `--user/--pass` (or `serve.user` in config) unless `--no-auth`. |
| `vget send <file>...` | One-time read-only share of exactly these files. No password by default; `--once` exits after everything was downloaded. |
| `vget recv [dir]` | One-time write-only inbox. Senders can add files but not list, read or overwrite. |
| `vget <tc-address>` | Download what `vget send` offers. One file downloads right away; several open a picker (`--all` takes everything). Interrupted downloads resume. |
| `vget push <file>... <tc-address \| remote:/dir/>` | Upload to a `recv` inbox, or into a library served with `--rw` / `--inbox`. |
| `vget config webdav add <name> <tc-address> --user u --pass p` | Save a shared library as a remote. Afterwards `vget ls name:/`, `vget name:/path/file` and the TUI browser work as with any WebDAV server. |
| `vget config webdav ping <name>` | Relay round trip and whether the connection is direct. `--until-direct` waits for NAT traversal. |
| `vget identity` | This machine's server address (masked; `--full`) and client key. `--rotate` issues a new address. |
| `vget derp test <host[:port]>` | Check a self-hosted relay and print its certificate fingerprint. |

Flags for `serve` can also be set once in `config.yml`:

```yaml
serve:
  dir: ~/library
  user: alice
  password: secret       # re-read on every request: change it without restarting
  tailcat: true
  listen: ""             # optional plain HTTP, e.g. ":7777"
  rw: false
  inbox: ""
  derp:                  # self-hosted relay (see below); empty = public relays
    host: 1.2.3.4
    port: 8443
    stun_port: 3478
    cert_sha256: ""
  allow: []              # client keys allowed to connect, from `vget identity`
```

## Security model

- The address embeds the tunnel's pre-shared key. **Treat it like a password**: anyone who has it can reach the WebDAV layer. `vget config show` masks it.
- On top of the tunnel, a library still requires Basic Auth. Accounts can be changed without changing the address. `send`/`recv` skip the password by default because their address is one-time.
- `--allow nodekey:...` pins which client keys may even open the tunnel. Each machine's key is shown by `vget identity`.
- The persistent identity lives in `~/.config/vget/tailcat/server.json` (mode 0600). `vget identity --rotate` replaces it; every remote pointing at it must then be re-added. `--ephemeral` serves with a throwaway identity.
- Sharing with someone you do not trust: read-only, with an account, ideally with `--allow`.

## The relay

Two machines find each other through a DERP relay: the client announces itself
there, both sides exchange their UDP endpoints through it, then attempt a
direct connection. When NAT traversal succeeds the relay only carries
keep-alives; when it fails, all traffic goes through the relay. The relay
never sees plaintext. It is required even on the same LAN (there is no other
rendezvous); for an offline LAN use `--listen` with the machine's IP.

By default vget uses tailcat's public relays (New York, San Francisco,
Frankfurt, Tokyo), picked by latency once when the identity is created. They
are rate-limited, carry no SLA, and are all outside China.

### Running your own relay

Any VPS works. A bare IP with a self-signed certificate is enough: no domain,
no port 443, no ICP filing.

```bash
# on the VPS
go install tailscale.com/cmd/derper@latest
derper -a :8443 -certmode manual -hostname 1.2.3.4 -http-port -1 -stun-port 3478
# open 8443/tcp and 3478/udp in the firewall; the self-signed cert is generated on first start

# on the machine that will serve
vget derp test 1.2.3.4:8443            # prints "sha256: ab12..." for the self-signed cert
vget serve ~/library --user u --pass p --derp 1.2.3.4:8443 --derp-cert-sha256 ab12...
```

The relay is written into the identity and into the address, so clients need
no configuration and never contact the public relays. Later `vget serve` runs
remember it; `--derp` only needs repeating to change it.

## Keeping it running

`vget serve` runs in the foreground. Under systemd:

```ini
[Unit]
Description=vget library
After=network-online.target

[Service]
ExecStart=/usr/local/bin/vget serve /srv/library
Restart=always
User=vget

[Install]
WantedBy=multi-user.target
```

with `dir`, `user` and `password` in that user's `~/.config/vget/config.yml`.
On macOS, a launchd agent with the same command works; the identity file is
per user, so run it as the user whose `vget identity` you shared.

## Troubleshooting

- **`ping` says `via relay` and never `direct`**: both sides are behind NATs that
  cannot be traversed (carrier-grade NAT, symmetric NAT). Transfers still work
  through the relay, only slower. IPv6 on both sides usually fixes it.
- **`no reply from the relay` when serving**: the relay host is unreachable
  (DNS, firewall, or a public relay blocked on your network). Run your own
  relay, or serve on the LAN with `--listen`.
- **`invalid tailcat address`**: the address was mangled when pasted, or the
  other side runs a different vget version. Both sides should `vget update`.
- **`the server identity is already in use`**: another `vget serve` is running
  with the same identity. Stop it, or use `--ephemeral`.
- **Interrupted download**: run the same `vget` command again; the `.part`
  file next to the target is resumed.
