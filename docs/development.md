# Development

Requirements: Go (the toolchain `go1.26.8` pinned in `go.mod` is fetched automatically by
`GOTOOLCHAIN=auto`), Node 24, pnpm 12, Docker and [`just`](https://github.com/casey/just).
`just` alone lists every recipe; it loads `.env` (`cp .env.example .env`), the binary itself reads
only the process environment.

## Running locally

```sh
cp .env.example .env
just dev
```

`just dev`:

- starts the dev Postgres (Docker, host port 5440) only if it is not running and waits until it
  is ready;
- applies migrations;
- builds the web export (`web/out`) if it is missing or older than the sources;
- runs the server on <http://localhost:8080> with `SESSION_COOKIE_SECURE=false`.

On an empty database it creates the first superadmin **`admin@example.com` / `change-me-please`**
(override with `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD` in `.env`; change the password
after the first sign-in).

**Working on the UI:** run `just web-watch` in a second terminal. It rebuilds the export on every
change and the running server picks it up without a restart — reload the page, there is no hot
reload.

**Another user from the command line** (the password comes from `USER_PASSWORD` or stdin, never
from arguments):

```sh
printf '%s\n' 'a long password' | go run ./cmd/svc-registry user:create --email you@example.com --name You --superadmin
```

**The whole stack in containers:** `just docker-up` (one image: the binary plus the web export).

**Uploaded files** (link icons) go to `UPLOADS_DIR` — `env/uploads` in the repository when you run
`just dev` (ignored by git), a volume at `/data/uploads` in Docker; `UPLOADS_DIR=off` turns uploads off.
Files uploaded before the move to `env/` lie in `data/uploads`: `mv data/uploads/* env/uploads/`, or
keep `UPLOADS_DIR=data/uploads` in `.env`.

## The local environment: `env/`

Everything only the local environment needs lives in `env/`:

| Path | What |
|---|---|
| `env/justice/*.just` | the recipes, by group; the root `justfile` only imports them |
| `env/compose.yml` | the dev and test containers (project `svc-registry`) |
| `env/Dockerfile` (+ `env/Dockerfile.dockerignore`) | the one image; the build context is the repository root |
| `env/scripts/` | `dev.sh`, `dev-check.sh`, `web-watch.sh`, `new-migration.sh`, `reset-test-db.sql` |
| `env/uploads/`, `env/docsroot/` | local data, ignored by git (`docsroot` — a place for `KNOWLEDGE_LOCAL_ROOTS` sources) |

`just` exports `COMPOSE_FILE=env/compose.yml`, so `docker compose …` inside recipes needs no `-f`.
Outside `just` run `docker compose -f env/compose.yml …` (or export `COMPOSE_FILE` yourself); the
project name stays `svc-registry`, so containers and volumes started before the move are reused.
The image builds with `docker build -f env/Dockerfile .` and needs BuildKit (Docker 23+), which
reads `env/Dockerfile.dockerignore`.

## Database

| Command | What it does |
|---|---|
| `just db-migrate` | apply new migrations (`serve` never migrates) |
| `just db-rollback [COUNT]` | roll back the last migrations |
| `just db-reset` | drop the schema and migrate again; also fixes a dev database migrated before golang-migrate (no `schema_migrations` table) |
| `just new-migration <name>` | create an `.up.sql` / `.down.sql` pair |

## Search engines for development

`just docker-vdb` starts Qdrant and Meilisearch (compose profile `search`) and prints the variables
to set:

```sh
just docker-vdb                    # up both
just docker-vdb up qdrant          # one engine
just docker-vdb down|restart|logs|ps|reset [qdrant|meilisearch|all]
```

Ports: `QDRANT_PORT` (gRPC, 6334), `QDRANT_HTTP_PORT` (6333, dashboard), `MEILISEARCH_PORT` (7700);
`reset` also drops their data. Configuring the engines: [search-engines.md](search-engines.md).

## Where code lives

- `env/` — the local environment (see above); `packaging/` — packages, CHANGELOG and release scripts.
- `internal/feature/<feature>/` — the domain of a feature (entities, rules, errors, ports);
  `service/` — its `Service` with the use-cases; `internal/repository/` — its SQL (Go's `internal/`
  keeps it inside the feature); other subfolders — clients of external systems.
- `internal/platform/` — database pool and transactions, configuration, errors, outgoing HTTP, crypto.
- `internal/presentation/` — `http/` (router, handlers, request and response DTOs, middleware, MCP,
  web UI files), `console/` (CLI commands), `jobs/` (background jobs).
- `internal/app/` — builds the application from all of the above.

Features import each other only along the graph in `ARCHITECTURE.md` §2; `just layout` checks it.

## Quality gates

The same commands run in CI on every push to `main` and every pull request
(`.github/workflows/ci.yml`):

```sh
just lint        # gofmt + go vet + staticcheck + package layout check
just test        # go test -race ./... with a throwaway Postgres (:5434), Qdrant and Meilisearch
just web-check   # web: lint, typecheck, locale keys and contrast tests, static export build + check
just vuln        # govulncheck: known vulnerabilities reachable from the code, and in Go itself
just sast        # gosec; an exception is `#nosec G<n> -- <reason>` on the line
just web-audit   # pnpm audit of the web's production dependencies, high and above
just ci-lint     # actionlint (workflows), shellcheck (scripts), hadolint (env/Dockerfile)
just security    # vuln + sast + web-audit + ci-lint
```

CI also runs CodeQL (Go and TypeScript, `.github/workflows/codeql.yml`), gitleaks over the history,
dependency review on pull requests, and builds the Linux amd64 archive, deb and rpm and checks them
(`package-check`). Dependabot proposes updates of Go modules, npm, Actions and base images weekly.

One Go test: `go test ./tests -run TestName`.

## Packages

```sh
just web-dist              # the export with .gz siblings, as the packages and the image ship it
just package               # static Linux binaries (amd64, arm64) and their archives in dist/release
just package-deb           # the deb (in debian:12), TARGET=amd64|arm64
just package-rpm           # the rpm (in almalinux:9), TARGET=amd64|arm64
just package-check deb     # install it on a clean system next to Postgres, sign in, reinstall, remove
just package-check rpm
just package-macos         # macOS only: universal binary, archive, .pkg, checked
```

Everything but `package-macos` needs only Go and Docker. What the packages install: [install.md](install.md).

## Releasing

The version lives in `internal/app/version.go`; a release is a tag `vX.Y.Z` (or `vX.Y.Z-rc.N`)
naming exactly that version. Subjects follow [Conventional Commits](https://www.conventionalcommits.org)
(`feat:`, `fix:`, `docs:`, `refactor:`, `perf:`, `build:`, `ci:`, `test:`, `type!:` for breaking).

```sh
just next-version          # what the next version would be (auto | patch | minor | major)
just release               # bump internal/app/version.go and write its CHANGELOG.md section
git commit -am "chore(release): v$(just version)"
git tag "v$(just version)" && git push origin main "v$(just version)"
```

`just changelog` rebuilds `CHANGELOG.md` from the whole history. The tag starts
`.github/workflows/release.yml`: the gates, the packages for Linux (amd64, arm64) and macOS, the
deb and rpm checks, the image `ghcr.io/hoknie/service-registry` (`linux/amd64`, `linux/arm64`) and
the GitHub Release with `SHA256SUMS` and the version's CHANGELOG section. A tag with a suffix is a
pre-release and does not move `latest`. After the first release make the GHCR package public once
(GitHub → Packages → service-registry → Package settings).

## Where to read more

- `ARCHITECTURE.md` — code layout and rules (Russian).
- `CLAUDE.md` — a concise guide for AI coding agents.
- `openspec/specs/` — behavior specs; `openspec/decisions/` — architecture decision records.
