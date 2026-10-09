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

**Uploaded files** (link icons) go to `UPLOADS_DIR` — `data/uploads` in the repository when you run
`just dev` (ignored by git), a volume at `/data/uploads` in Docker; `UPLOADS_DIR=off` turns uploads off.

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

## Quality gates

The same commands run in CI:

```sh
just lint        # gofmt + go vet + staticcheck + package layout check
just test        # go test -race ./... with a throwaway Postgres (:5434), Qdrant and Meilisearch
just web-check   # web: lint, typecheck, locale keys and contrast tests, static export build + check
```

One Go test: `go test ./tests -run TestName`.

## Where to read more

- `ARCHITECTURE.md` — code layout and rules (Russian).
- `CLAUDE.md` — a concise guide for AI coding agents.
- `openspec/specs/` — behavior specs; `openspec/decisions/` — architecture decision records.
