#!/usr/bin/env bash
# Local development in one command (`just dev` = `scripts/dev.sh all`; spec `platform/build`).
# Every step is idempotent and can be run on its own:
#   db       start the dev Postgres (`pgsql` in docker-compose.yml, host :5440) only if it is not
#            running yet, then wait until it is healthy (up to 60 s)
#   migrate  apply pending migrations (`go run`)
#   web      build the static web export (web/out) only if it is missing or web/ sources are
#            newer than it
#   serve    run the server: `go run ./cmd/svc-registry serve` with dev defaults
#   all      db → migrate → web → serve
# Defaults never override the environment or `.env` (just loads it), except
# SESSION_COOKIE_SECURE=false: `just dev` is plain http://localhost by definition.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO="${GO:-go}"
PNPM="${PNPM:-pnpm}"
export DATABASE_URL="${DATABASE_URL:-postgres://registry:registry@127.0.0.1:${PGSQL_PORT:-5440}/registry}"
export WEB_DIST_DIR="${WEB_DIST_DIR:-web/out}"

log() { printf '\033[1mdev:\033[0m %s\n' "$*" >&2; }

# The dev Postgres container id ("" if it does not exist).
pgsql_id() { docker compose ps -q pgsql 2>/dev/null || true; }

cmd_db() {
  if docker compose ps --status running --services 2>/dev/null | grep -qx pgsql; then
    log "pgsql is already running — not touching it"
  else
    log "starting pgsql (docker compose up -d pgsql)"
    docker compose up -d pgsql
  fi
  local id status
  for _ in $(seq 1 60); do
    id="$(pgsql_id)"
    status="$(docker inspect -f '{{.State.Health.Status}}' "$id" 2>/dev/null || echo unknown)"
    if [[ "$status" == healthy ]]; then
      log "pgsql is healthy (${id:0:12})"
      return 0
    fi
    sleep 1
  done
  log "pgsql did not become healthy within 60 s (status: ${status:-unknown})"
  return 1
}

cmd_migrate() {
  log "applying migrations"
  if ! "$GO" run ./cmd/svc-registry db:migrate; then
    log "migration failed; a dev database migrated before (no schema_migrations ledger). Needs \`just db-reset\`"
    return 1
  fi
}

# Prints the first web source newer than the export (nothing if the export is up to date).
web_stale_reason() {
  local marker="web/out/404.html"
  if [[ ! -f "$marker" ]]; then
    echo "no $marker"
    return
  fi
  local newer
  newer="$(find web/src web/public web/next.config.ts web/package.json web/pnpm-lock.yaml \
    web/tsconfig.json -newer "$marker" -type f -print -quit 2>/dev/null || true)"
  [[ -n "$newer" ]] && echo "$newer is newer than the export"
  return 0
}

cmd_web() {
  local reason
  reason="$(web_stale_reason)"
  if [[ -z "$reason" ]]; then
    log "web export is up to date (web/out)"
    return 0
  fi
  log "building the web export: $reason"
  if [[ ! -d web/node_modules ]]; then
    (cd web && "$PNPM" install --frozen-lockfile)
  fi
  (cd web && "$PNPM" build)
}

cmd_serve() {
  export SESSION_COOKIE_SECURE=false
  # First superadmin
  if [[ -z "${BOOTSTRAP_ADMIN_EMAIL:-}" && -z "${BOOTSTRAP_ADMIN_PASSWORD:-}" ]]; then
    export BOOTSTRAP_ADMIN_EMAIL=admin@example.com
    export BOOTSTRAP_ADMIN_PASSWORD=change-me-please
  fi
  local addr="${HTTP_ADDR:-0.0.0.0:8080}"
  log "serving http://localhost:${addr##*:} (go run; WEB_DIST_DIR=$WEB_DIST_DIR)"
  exec "$GO" run ./cmd/svc-registry serve
}

case "${1:-all}" in
  db) cmd_db ;;
  migrate) cmd_migrate ;;
  web) cmd_web ;;
  serve) cmd_serve ;;
  all)
    cmd_db
    cmd_migrate
    cmd_web
    cmd_serve
    ;;
  *)
    echo "usage: scripts/dev.sh [db|migrate|web|serve|all]" >&2
    exit 2
    ;;
esac
