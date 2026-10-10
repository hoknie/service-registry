# svc-registry — build, run and quality recipes.
#
# Thin aliases over go, pnpm and docker compose; the gates (lint, test, web-check) are the same
# commands CI runs (.github/workflows/ci.yml). The backend is Go (ADR-0023).
#
#   just                 # every recipe, by group
#   just dev             # dev Postgres (if not running) + migrate + web export (if stale) + go run serve
#   just test            # start + reset the test Postgres (:5434), run the Go suite
#   just lint            # gofmt + go vet + staticcheck + package layout
#   just web-check       # web: lint + typecheck + locale keys + build + export check
#
# `.env` is loaded by just (dotenv-load) for every recipe; the binary itself reads only the
# process environment. `cp .env.example .env` first. A dev database migrated before ADR-0031
# (no schema_migrations ledger) is reset with `just db-reset`.

set shell := ["bash", "-c"]
set dotenv-load := true

GO := env('GO', 'go')
PNPM := env('PNPM', 'pnpm')
# Pinned linter (ADR-0024); `go run` fetches it into the module cache once.
STATICCHECK := 'honnef.co/go/tools/cmd/staticcheck@v0.8.1'

[private]
default:
    @{{ just_executable() }} --list

# ---- build ------------------------------------------------------------------

# Build every package (compile check)
[group('build')]
build:
    {{ GO }} build ./...

# Release binary into bin/ (static: CGO_ENABLED=0, stripped, reproducible paths)
[group('build')]
release:
    mkdir -p bin
    CGO_ENABLED=0 {{ GO }} build -trimpath -ldflags='-s -w' -o bin/svc-registry ./cmd/svc-registry
    ls -l bin/svc-registry

# ---- run --------------------------------------------------------------------

# Local dev in one command: dev DB (if not running) + migrate + web export (if stale) + serve
[group('run')]
dev:
    scripts/dev.sh all

# Check that `just dev`'s preparation is idempotent (DB not restarted, export not rebuilt)
[group('run')]
dev-check:
    scripts/dev-check.sh

# Run the server: API on HTTP_ADDR, the web UI from the static export in WEB_DIST_DIR (no migrate)
[group('run')]
run:
    {{ GO }} run ./cmd/svc-registry serve

# Build the static web export once (web/out; served by `just dev` / `just run` without restart)
[group('run')]
web-build:
    cd web && {{ PNPM }} install --frozen-lockfile && {{ PNPM }} build

# Rebuild the web export whenever web/ sources change (polls 1 s; reload the page, no HMR)
[group('run')]
web-watch:
    PNPM={{ PNPM }} scripts/web-watch.sh

# ---- database ---------------------------------------------------------------

# Apply pending migrations (DATABASE_URL)
[group('database')]
db-migrate:
    {{ GO }} run ./cmd/svc-registry db:migrate

# Revert the last COUNT applied migrations: `just db-rollback [COUNT]`
[group('database')]
db-rollback COUNT='1':
    {{ GO }} run ./cmd/svc-registry db:rollback --count {{ COUNT }}

# Wipe the dev database (pgsql on :5440: drop + recreate schema public), then migrate
[group('database')]
db-reset:
    docker compose exec -T pgsql psql -U registry -d registry -v ON_ERROR_STOP=1 -q -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
    {{ GO }} run ./cmd/svc-registry db:migrate

# Create an empty reversible migration pair: `just new-migration create_projects`
[group('database')]
new-migration NAME:
    scripts/new-migration.sh {{ NAME }}

# ---- quality ----------------------------------------------------------------

# Start the throwaway test Postgres (pgsql-test on :5434)
[group('quality')]
test-db:
    docker compose up -d --wait pgsql-test
    docker compose up -d qdrant-test meilisearch-test

# Drop schemas left by interrupted test runs
[group('quality')]
test-db-reset: test-db
    docker compose exec -T pgsql-test psql -U postgres -d postgres -v ON_ERROR_STOP=1 -q < scripts/reset-test-db.sql

# Reset the test DB, then run the Go suite with the race detector (each DB test makes its own schema)
[group('quality')]
test: test-db-reset
    TEST_QDRANT_URL=localhost:6344 TEST_MEILISEARCH_URL=http://localhost:7710 {{ GO }} test -race -count=1 ./...

# Format the Go code
[group('quality')]
fmt:
    "$({{ GO }} env GOROOT)/bin/gofmt" -w cmd internal pkg migrations tests

# Check Go formatting (no writes)
[group('quality')]
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$("$({{ GO }} env GOROOT)/bin/gofmt" -l cmd internal pkg migrations tests)"
    if [[ -n "$unformatted" ]]; then echo "gofmt needed:"; echo "$unformatted"; exit 1; fi

# go vet + staticcheck
[group('quality')]
vet:
    {{ GO }} vet ./...
    {{ GO }} run {{ STATICCHECK }} ./...

# fmt-check + vet (go vet, staticcheck) + layout
[group('quality')]
lint: fmt-check vet layout

# Package dependency directions: pkg/ and platform/ import no features, features follow their graph
[group('quality')]
layout:
    GO={{ GO }} scripts/check-layout.sh

# Web gates: lockfile install, lint, typecheck, locale keys, static export build + export check
[group('quality')]
web-check:
    cd web && {{ PNPM }} install --frozen-lockfile && {{ PNPM }} lint && {{ PNPM }} typecheck && {{ PNPM }} test && {{ PNPM }} build && {{ PNPM }} check-export

# ---- docker -----------------------------------------------------------------

# Build and run the whole stack in containers (Postgres, migrate, the one image on :8080)
[group('docker')]
docker-up:
    docker compose up --build

[group('docker')]
docker-vdb ACTION='up' *ENGINES='all':
    #!/usr/bin/env bash
    set -euo pipefail
    services=()
    add() { case " ${services[*]-} " in *" $1 "*) ;; *) services+=("$1") ;; esac; }
    for e in {{ ENGINES }}; do
      case "$e" in
        all) add qdrant; add meilisearch ;;
        qdrant) add qdrant ;;
        meilisearch|meili) add meilisearch ;;
        *) echo "docker-vdb: unknown engine '$e' (qdrant, meilisearch, all)" >&2; exit 2 ;;
      esac
    done
    dc() { docker compose --profile search "$@"; }
    case "{{ ACTION }}" in
      up)
        dc up -d --wait "${services[@]}"
        echo
        for s in "${services[@]}"; do
          case "$s" in
            qdrant) echo "qdrant:      KNOWLEDGE_SEARCH_ENGINE=qdrant QDRANT_URL=localhost:${QDRANT_PORT:-6334}  (dashboard http://localhost:${QDRANT_HTTP_PORT:-6333}/dashboard)" ;;
            meilisearch) echo "meilisearch: KNOWLEDGE_SEARCH_ENGINE=meilisearch MEILISEARCH_URL=http://localhost:${MEILISEARCH_PORT:-7700}" ;;
          esac
        done
        echo "semantic/hybrid modes also need EMBEDDINGS_URL, EMBEDDINGS_MODEL, EMBEDDINGS_DIMENSIONS (see .env.example)" ;;
      down) dc stop "${services[@]}" && dc rm -f "${services[@]}" ;;
      restart) dc restart "${services[@]}" ;;
      logs) dc logs -f --tail 100 "${services[@]}" ;;
      ps) dc ps "${services[@]}" ;;
      reset)
        dc rm -sf "${services[@]}"
        for s in "${services[@]}"; do
          case "$s" in qdrant) v=qdrant-data ;; meilisearch) v=meili-data ;; esac
          docker volume ls -q --filter "label=com.docker.compose.project=svc-registry" \
            --filter "label=com.docker.compose.volume=$v" | xargs -r docker volume rm >/dev/null
        done
        dc up -d --wait "${services[@]}" ;;
      *) echo "docker-vdb: unknown action '{{ ACTION }}' (up, down, restart, logs, ps, reset)" >&2; exit 2 ;;
    esac
