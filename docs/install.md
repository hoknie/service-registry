# Installing

Every release on [GitHub](https://github.com/hoknie/service-registry/releases) carries the same
version in several forms, plus `SHA256SUMS` over them (`sha256sum -c SHA256SUMS`):

| File | For |
|---|---|
| `svc-registry_<version>.debian.amd64.deb`, `….debian.arm64.deb` | Debian 12+, Ubuntu 22.04+ |
| `svc-registry_<version>.el.x86_64.rpm`, `….el.aarch64.rpm` | RHEL, AlmaLinux, Rocky 9+, Fedora |
| `svc-registry_<version>.linux.x86_64.tar.gz`, `….linux.aarch64.tar.gz` | any Linux, by hand |
| `svc-registry_<version>.macos.universal.pkg` | macOS 11+, Apple Silicon and Intel |
| `svc-registry_<version>.macos.universal.tar.gz` | macOS, by hand |
| `ghcr.io/hoknie/service-registry:<version>` | containers (`linux/amd64`, `linux/arm64`) |

Every form is one static binary plus the web interface; none of them bundles Postgres. The service
needs **Postgres 15+** (the `vector` extension only for `KNOWLEDGE_SEARCH_ENGINE=pgvector`) and
applies its migrations itself before every start.

## deb and rpm

```sh
sudo apt install ./svc-registry_<version>.debian.amd64.deb      # or
sudo dnf install ./svc-registry_<version>.el.x86_64.rpm
```

The package installs:

| Path | What |
|---|---|
| `/usr/bin/svc-registry` | the binary |
| `/usr/share/svc-registry/web` | the interface (`WEB_DIST_DIR`) |
| `/usr/share/svc-registry/svc-registry.env.example` | the settings template |
| `/usr/lib/systemd/system/svc-registry.service` | the unit: `db:migrate`, then `serve`, as the user `svc-registry` |
| `/etc/svc-registry/svc-registry.env` | the settings (`0640 root:svc-registry`), written once on the first installation |
| `/etc/svc-registry/initial-password` | the first superadmin's password (`0600 root`) |
| `/var/lib/svc-registry/uploads` | uploaded files (`UPLOADS_DIR`) |
| `/usr/share/doc/svc-registry` | this documentation and `env.example`, every variable with its default |

The first installation generates `SECRETS_KEYS` and the first superadmin
(`admin@example.com` and a random password), enables the service and tries to start it. Until
`DATABASE_URL` is set the service does not start and says so in the journal
(`journalctl -u svc-registry`). Then:

```sh
sudoedit /etc/svc-registry/svc-registry.env        # DATABASE_URL=postgres://…
sudo systemctl restart svc-registry
sudo cat /etc/svc-registry/initial-password
```

Open <http://127.0.0.1:8080> and sign in as `admin@example.com`. After the first sign-in remove the
`BOOTSTRAP_ADMIN_*` lines from `svc-registry.env` and `initial-password` — the bootstrap only works
on a database without users anyway.

**Behind a proxy.** The service listens on `127.0.0.1:8080` (`HTTP_ADDR`) and sets session cookies
with `Secure`: browsers accept them over plain HTTP only on `127.0.0.1` and `localhost`. Serve it to
others through a reverse proxy with TLS (Caddy, nginx) and set `PUBLIC_URL` to the address users
see; use `SESSION_COOKIE_SECURE=false` only for plain HTTP on purpose.

**Updates.** Installing a newer package keeps `svc-registry.env` as it is, restarts the service and
so applies the new migrations. **Removal** (`apt remove`, `dnf remove`) stops and disables the
service and leaves `/etc/svc-registry` and `/var/lib/svc-registry`; delete them by hand to purge.

Local changes to the unit go through `systemctl edit svc-registry` (for example `After=` of a local
Postgres unit with another name).

## Archives

```sh
tar -xzf svc-registry_<version>.linux.x86_64.tar.gz
cd svc-registry_<version>.linux.x86_64
cp svc-registry.env.example svc-registry.env        # set DATABASE_URL, WEB_DIST_DIR=web, UPLOADS_DIR, …
set -a; . ./svc-registry.env; set +a
./svc-registry db:migrate && ./svc-registry serve
```

The archive holds the binary, the interface in `web/`, the settings template, the unit file
(Linux) or the LaunchAgent helpers (macOS), `README.md`, `docs/` and `LICENSE`.

## macOS

Open `svc-registry_<version>.macos.universal.pkg`. The package is **not signed**: macOS refuses to
open it with a double click — use *Open* from the context menu in Finder, or remove the quarantine
mark first: `xattr -d com.apple.quarantine svc-registry_<version>.macos.universal.pkg`.

The installer puts the binary in `/usr/local/bin`, the interface in
`/usr/local/share/svc-registry/web`, the helpers in `/usr/local/libexec/svc-registry`, and sets the
service up for the user signed in:

| Path | What |
|---|---|
| `~/.config/svc-registry/svc-registry.env` | the settings, written once |
| `~/.config/svc-registry/initial-password` | the first superadmin's password |
| `~/Library/Application Support/svc-registry/uploads` | uploaded files |
| `~/Library/LaunchAgents/io.github.hoknie.svc-registry.plist` | runs `db:migrate`, then `serve` at login |
| `~/Library/Logs/svc-registry.log` | the log |

Postgres comes from elsewhere, for example `brew install postgresql@17`. Set `DATABASE_URL`, then
restart the agent:

```sh
launchctl kickstart -k gui/$(id -u)/io.github.hoknie.svc-registry
```

While `DATABASE_URL` is empty the agent writes one line to the log and stays stopped. Another user of
the Mac runs `/usr/local/libexec/svc-registry/setup` for themselves. To remove the service:
`sudo /usr/local/libexec/svc-registry/uninstall` (add `--purge` to delete the settings and data too).

## Container image

```sh
docker run -d -p 8080:8080 -e DATABASE_URL=postgres://… -e SECRETS_KEYS=k1:… \
  -v registry-uploads:/data/uploads ghcr.io/hoknie/service-registry:<version> serve
docker run --rm -e DATABASE_URL=postgres://… ghcr.io/hoknie/service-registry:<version> db:migrate
```

The image does not migrate by itself: run `db:migrate` before `serve` (as `env/compose.yml` does with
its `migrate` service). Tags: `<version>`, and for releases without a suffix also `<major>.<minor>`
and `latest`.
