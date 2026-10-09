#!/usr/bin/env bash
# `just lint`: the dependency directions — pkg/ never imports internal/; domains never
# import transport, use-cases, storage or infrastructure, Fiber or pgx; use-cases never import
# Fiber or the HTTP transport.
set -euo pipefail
GO="${GO:-go}"
mod="svc-registry"
fail=0

check() {
  local pkgs="$1" pattern="$2" rule="$3"
  local bad
  bad="$("$GO" list -f '{{.ImportPath}}: {{join .Imports " "}}' $pkgs | tr ' ' '\n' | awk -v p="$pattern" '/:$/ {cur=$0; next} $0 ~ p {print cur " " $0}')"
  if [[ -n "$bad" ]]; then
    echo "layout: $rule"
    echo "$bad"
    fail=1
  fi
}

domains="./internal/access/... ./internal/catalog/... ./internal/deploy/... ./internal/forge/... ./internal/ingest/... ./internal/knowledge/... ./internal/links/..."
check "./pkg/..." "^$mod/internal/" "pkg/ must not import internal/"
check "$domains" "^($mod/internal/(service|httpapi|webui|cli|app|postgres|auth|forgeclient|k8s|docsource|linkcheck|outbound|embeddings|search|config|apperr)(/|$)|github.com/gofiber/|github.com/jackc/pgx)" \
  "domains must not import use-cases, transport, storage, infrastructure, Fiber or pgx"
check "./internal/service/..." "^($mod/internal/(httpapi|webui|cli|app)(/|$)|github.com/gofiber/)" "service must not import the transport or Fiber"
exit "$fail"
