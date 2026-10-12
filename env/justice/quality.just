# Start the throwaway test Postgres (pgsql-test on :5434) and the test search engines
[group('quality')]
test-db:
    docker compose up -d --wait pgsql-test
    docker compose up -d qdrant-test meilisearch-test

# Drop schemas left by interrupted test runs
[group('quality')]
test-db-reset: test-db
    docker compose exec -T pgsql-test psql -U postgres -d postgres -v ON_ERROR_STOP=1 -q < env/scripts/reset-test-db.sql

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
