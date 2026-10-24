# Installing

svc-registry is one program plus its web interface. It needs an external **Postgres 15+** and nothing else.

Every release on [GitHub](https://github.com/hoknie/service-registry/releases) comes in several forms, with
`SHA256SUMS` to check them (`sha256sum -c SHA256SUMS`):

| File | For |
|---|---|
| `svc-registry_<version>.debian.amd64.deb`, `….debian.arm64.deb` | Debian 12+, Ubuntu 22.04+ |
| `svc-registry_<version>.el.x86_64.rpm`, `….el.aarch64.rpm` | RHEL, AlmaLinux, Rocky 9+, Fedora |
| `svc-registry_<version>.linux.x86_64.tar.gz`, `….linux.aarch64.tar.gz` | any Linux, by hand |
| `svc-registry_<version>.macos.universal.pkg` | macOS 11+, Apple Silicon and Intel |
| `svc-registry_<version>.macos.universal.tar.gz` | macOS, by hand |
| `ghcr.io/hoknie/service-registry:<version>` | containers (`linux/amd64`, `linux/arm64`) |

About Postgres:
- `pg_trgm` from contrib is recommended: it speeds up catalog search, and everything works without it.
- The `vector` extension is needed only for search by meaning with `pgvector` ([search-engines.md](search-engines.md)).
- The server's own tables are created and updated by `svc-registry db:migrate`. The packages run it before every
  start; with the archive and the image you run it yourself.

## deb and rpm

```sh
sudo apt install ./svc-registry_<version>.debian.amd64.deb      # or
sudo dnf install ./svc-registry_<version>.el.x86_64.rpm
```

The first installation:
- generates `SECRETS_KEYS` and the first superadmin, `admin@example.com` with a random password;
- enables the service and tries to start it.

The service waits until you set `DATABASE_URL`, and says so in `journalctl -u svc-registry`. Then:

```sh
sudoedit /etc/svc-registry/svc-registry.env        # DATABASE_URL=postgres://…
sudo systemctl restart svc-registry
sudo cat /etc/svc-registry/initial-password
```

Open <http://127.0.0.1:8080> and sign in as `admin@example.com`. After the first sign-in delete `initial-password`
and the `BOOTSTRAP_ADMIN_*` lines from `svc-registry.env`. They only work on a database without users anyway.

| Path | What |
|---|---|
| `/usr/bin/svc-registry` | the program |
| `/usr/share/svc-registry/web` | the web interface (`WEB_DIST_DIR`) |
| `/etc/svc-registry/svc-registry.env` | the settings (`0640 root:svc-registry`), written once at the first installation |
| `/etc/svc-registry/initial-password` | the first superadmin's password (`0600 root`) |
| `/var/lib/svc-registry/uploads` | uploaded files (`UPLOADS_DIR`) |
| `/usr/lib/systemd/system/svc-registry.service` | the service: `db:migrate`, then `serve`, as the user `svc-registry` |
| `/usr/share/doc/svc-registry` | this documentation and `env.example` with every variable |

- **Updates.** Installing a newer package keeps `svc-registry.env` and restarts the service, which applies the new
  migrations.
- **Removal.** `apt remove` / `dnf remove` stops the service and leaves `/etc/svc-registry` and
  `/var/lib/svc-registry`; delete them by hand to purge.
- **Local changes to the service** go through `systemctl edit svc-registry`, for example `After=` for a local Postgres
  unit with another name.

## Archives

```sh
tar -xzf svc-registry_<version>.linux.x86_64.tar.gz
cd svc-registry_<version>.linux.x86_64
cp svc-registry.env.example svc-registry.env        # set DATABASE_URL, WEB_DIST_DIR=web, UPLOADS_DIR, …
set -a; . ./svc-registry.env; set +a
./svc-registry db:migrate && ./svc-registry serve
```

The archive holds the program, the interface in `web/`, the settings template, the service file (Linux) or the
LaunchAgent helpers (macOS), `README.md`, `docs/` and `LICENSE`.

## macOS

Open `svc-registry_<version>.macos.universal.pkg` with a double click. A release package is signed with Developer ID
and notarized by Apple, so Gatekeeper lets it open. To check it:
`spctl --assess --type install -vv svc-registry_<version>.macos.universal.pkg` should say `accepted` and
`source=Notarized Developer ID`.

A package that is **not signed** is refused on a double click. The release notes say so when a release was built
without the signing keys, and every local build without them is unsigned. Either use *Open* from the context menu in
Finder, or remove the quarantine mark first:

```sh
xattr -d com.apple.quarantine svc-registry_<version>.macos.universal.pkg
```

The program in `svc-registry_<version>.macos.universal.tar.gz` is the same signed file. Apple cannot attach the
notarization ticket to an archive, so Gatekeeper looks it up online on the first run. Offline, or for an unsigned
build, remove the mark from the unpacked folder:
`xattr -dr com.apple.quarantine svc-registry_<version>.macos.universal`.

The installer puts the program in `/usr/local/bin`, the interface in `/usr/local/share/svc-registry/web`, the helpers
in `/usr/local/libexec/svc-registry`, and sets up the service for the user who is signed in:

| Path | What |
|---|---|
| `~/.config/svc-registry/svc-registry.env` | the settings, written once |
| `~/.config/svc-registry/initial-password` | the first superadmin's password |
| `~/Library/Application Support/svc-registry/uploads` | uploaded files |
| `~/Library/LaunchAgents/io.github.hoknie.svc-registry.plist` | runs `db:migrate`, then `serve`, at login |
| `~/Library/Logs/svc-registry.log` | the log |

Postgres comes from elsewhere, for example `brew install postgresql@17`. Set `DATABASE_URL`, then restart the agent:

```sh
launchctl kickstart -k gui/$(id -u)/io.github.hoknie.svc-registry
```

- **Before `DATABASE_URL` is set** the agent writes one line to the log and stays stopped.
- **Another user of the Mac** runs `/usr/local/libexec/svc-registry/setup` for themselves.
- **Removing:** `sudo /usr/local/libexec/svc-registry/uninstall`; add `--purge` to delete the settings and data too.

## Container image

```sh
docker run --rm -e DATABASE_URL=postgres://… ghcr.io/hoknie/service-registry:<version> db:migrate
docker run -d -p 8080:8080 -e DATABASE_URL=postgres://… -e SECRETS_KEYS=k1:… \
  -v registry-uploads:/data/uploads ghcr.io/hoknie/service-registry:<version> serve
```

The image does not migrate by itself: run `db:migrate` before `serve`, as the `migrate` service of
`env/compose.yml` does. Tags: `<version>`, and for releases without a suffix also `<major>.<minor>` and `latest`.

## Main settings

The server reads its settings from environment variables. The packages keep them in `svc-registry.env`. Every
variable with its default is in [`.env.example`](../.env.example).

| Variable | Default | Meaning |
|---|---|---|
| `DATABASE_URL` | — | the Postgres connection, `postgres://…`; nothing works without it |
| `SECRETS_KEYS` | — | keys that encrypt stored secrets ([secrets.md](secrets.md#encryption-keys)) |
| `PUBLIC_URL` | — | the address people use, e.g. `https://registry.example.com`; needed for webhooks, sign-in providers, MCP over OAuth and links in notifications |
| `HTTP_ADDR` | `0.0.0.0:8080` (packages: `127.0.0.1:8080`) | where the server listens |
| `WEB_DIST_DIR` | `web/out` | the folder with the web interface |
| `UPLOADS_DIR` | `data/uploads` | uploaded link icons; `off` turns uploads off |
| `BACKGROUND_JOBS_ENABLED` | `true` | `false` — this server only answers requests; run the background jobs on another one |
| `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD` | — | the first superadmin, created only on a database without users |
| `SESSION_COOKIE_SECURE` | `true` | `false` only for plain HTTP on purpose |
| `SESSION_IDLE_TIMEOUT_SECS`, `SESSION_ABSOLUTE_TIMEOUT_SECS` | 12 h, 7 days | when a browser session ends |
| `LOG_LEVEL` | `info` | `off`, `error`, `warn`, `info`, `debug` or `trace`; also per component, e.g. `info,pgx=warn` |
| `HTTPS_PROXY`, `NO_PROXY`, `HTTP_CA_FILE` | — | outgoing proxy and extra CA certificates |

## Behind a reverse proxy

Serve the registry to other people through a reverse proxy with TLS (Caddy, nginx). Set `PUBLIC_URL` to the address
they see.

- **Cookies.** Session cookies are `Secure`, and browsers accept them over plain HTTP only on `127.0.0.1` and
  `localhost`.
- **Content-Security-Policy.** Pages of the interface carry their own policy, which allows their inline scripts by
  hash. Do not replace it at the proxy. A second policy added by the proxy can only narrow what is allowed: one
  without the page hashes breaks the interface.
- **MCP over OAuth** ([mcp.md](mcp.md#connecting-without-a-token-oauth)). The proxy must pass these paths to the
  server unchanged:
  - `/.well-known/oauth-protected-resource…`
  - `/.well-known/oauth-authorization-server…`
  - `/api/oauth/…`

## Upgrading from a version with inline tokens

Older versions could keep a forge, documentation source or cluster token inside the record itself. Such tokens must
be moved into named [secrets](secrets.md) before the upgrade completes. Until then `db:migrate` applies everything up
to the step that drops them, then stops with

```
error: <N> record(s) still use inline credentials; run "svc-registry secrets:adopt-legacy", then "db:migrate" again
```

and the service does not start. Run, with the same `DATABASE_URL` and `SECRETS_KEYS` as the service:

```sh
svc-registry secrets:adopt-legacy --dry-run   # the plan: record → secret (new or reused)
svc-registry secrets:adopt-legacy             # adopted <N> credential(s) into <M> secret(s)
svc-registry db:migrate
```

- **Where the secrets go.** A connection gets a secret of its organization or folder, a source one of the project's
  parent, a cluster a global one. Equal values in one place share a secret. Rename them later if you like.
- **1.0.0.** `secrets:adopt-legacy` and this check will be removed there. An installation that still has inline
  tokens must upgrade through a 0.x release first, run the command there, and only then move to 1.0.0.
