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
needs **Postgres 15+** (the `vector` extension only for `KNOWLEDGE_SEARCH_ENGINE=pgvector`; `pg_trgm` from contrib is
recommended — it speeds up the catalog search, everything works without it) and
applies its migrations itself before every start.

### Upgrading from a version with inline tokens

Older versions could keep a forge, documentation source or cluster token inside the record itself.
Such records must be moved into named secrets before the upgrade completes: until then `db:migrate`
applies every migration before the ones that drop those columns, then stops with
`error: <N> record(s) still use inline credentials; run "svc-registry secrets:adopt-legacy", then "db:migrate" again`
(the service does not start). Run, with the same `DATABASE_URL` and `SECRETS_KEYS` as the service:

```sh
svc-registry secrets:adopt-legacy --dry-run   # the plan: record → secret (new|reused)
svc-registry secrets:adopt-legacy             # adopted <N> credential(s) into <M> secret(s)
svc-registry db:migrate
```

A connection gets a secret of its organization or folder, a source one of the project's parent, a
cluster a global one; equal values in one place share a secret. Rename the secrets later if you like.

`secrets:adopt-legacy` and this `db:migrate` check will be removed in **1.0.0**. An installation that still has
inline tokens must upgrade through a 0.x release first, run the command there, and only then move to 1.0.0.

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
Pages of the web interface carry their own `Content-Security-Policy` (inline scripts are allowed by
hash). Do not replace it at the proxy; a policy the proxy adds as a second header is applied by
browsers together with it, so it may only narrow what is allowed — `script-src 'unsafe-inline'` there
gains nothing, and a policy without the page hashes breaks the interface.

**MCP clients over OAuth.** `MCP_OAUTH=on` turns the registry into an OAuth 2.1 authorization server for
`/api/mcp` (`docs/mcp.md`); it needs `PUBLIC_URL`, and the proxy must pass `/.well-known/oauth-protected-resource…`,
`/.well-known/oauth-authorization-server…` and `/api/oauth/…` to the service unchanged.
`MCP_OAUTH_ACCESS_TTL_SECS` (300–86400, default 3600) and `MCP_OAUTH_REFRESH_DAYS` (1–365, default 30) set the
token lifetimes. Clients may identify themselves by an `https://` metadata document address only from the sites in
`MCP_OAUTH_CLIENT_METADATA_HOSTS` (empty — none; exactly `*` — any public host, logged as a warning;
`MCP_OAUTH_CLIENT_METADATA=off` turns documents off). The registry
fetches documents directly, not through `HTTPS_PROXY`, unless `MCP_OAUTH_CLIENT_METADATA_USE_PROXY=true` — then the
IP behind the proxy is not checked.

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

Open `svc-registry_<version>.macos.universal.pkg` with a double click. A release package is signed
with Developer ID and notarized by Apple, so Gatekeeper lets it open; check it with
`spctl --assess --type install -vv svc-registry_<version>.macos.universal.pkg` (`accepted`,
`source=Notarized Developer ID`).

A package that is **not signed** — the release notes say so when a release was built without the
signing keys, and every local build without them — is refused on a double click: use *Open* from
the context menu in Finder, or remove the quarantine mark first:
`xattr -d com.apple.quarantine svc-registry_<version>.macos.universal.pkg`.

The binary in `svc-registry_<version>.macos.universal.tar.gz` is the same signed file, but Apple does
not staple a ticket to an archive: Gatekeeper finds the ticket online on the first run. Offline, or
for an unsigned build, remove the mark from the unpacked folder:
`xattr -dr com.apple.quarantine svc-registry_<version>.macos.universal`.

### A signed build

`packaging/package-macos.sh` (and `just package-macos`) signs and notarizes when all of these are set,
and builds unsigned when none is; a partial set is an error that names what is missing:

| Variable | What |
|---|---|
| `MACOS_SIGN_APPLICATION` | the identity for the binary, `Developer ID Application: <name> (<team>)` |
| `MACOS_SIGN_INSTALLER` | the identity for the `.pkg`, `Developer ID Installer: <name> (<team>)` |
| `MACOS_TEAM_ID` | the team identifier, checked against both signatures |
| `MACOS_NOTARY_KEY` | path to the App Store Connect API key (`AuthKey_<id>.p8`) |
| `MACOS_NOTARY_KEY_ID` | the key's identifier |
| `MACOS_NOTARY_ISSUER_ID` | the issuer identifier of the key |
| `MACOS_KEYCHAIN` | optional: the keychain holding the identities |

`check` then also verifies the binary's signature (hardened runtime, the team), the `.pkg` signature,
the stapled ticket and the Gatekeeper assessment.

### Signing releases

The release workflow signs when the GitHub environment `release` holds the keys. Once, as the owner:

1. In Apple Developer create the certificates *Developer ID Application* and *Developer ID Installer*,
   and export both with their private keys into one `.p12` with a password.
2. In App Store Connect → Users and Access → Integrations create an API key with the *Developer* role;
   note its key ID and issuer ID and download the `.p8`.
3. In the repository settings create the environment `release` with a deployment rule for tags `v*`,
   and add to it the secrets `MACOS_CERTIFICATES_P12` (the `.p12` as base64: `base64 -i certs.p12`),
   `MACOS_CERTIFICATES_PASSWORD`, `MACOS_SIGN_APPLICATION`, `MACOS_SIGN_INSTALLER`,
   `MACOS_NOTARY_KEY_P8` (the contents of the `.p8`), `MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_ISSUER_ID`
   and the variable `MACOS_TEAM_ID`.

The next tag publishes a signed and notarized `.pkg`. Removing the secrets brings back unsigned
releases.

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
