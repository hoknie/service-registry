# svc-registry — build, run, quality, packaging and release recipes.
#
# Thin aliases over go, pnpm, docker compose and the scripts in env/scripts and packaging/; the
# gates (lint, test, web-check, security) are the same commands CI runs (.github/workflows/ci.yml).
# Recipes live in env/justice/*.just; the local environment (compose, Dockerfile, dev scripts, data)
# lives in env/ (ADR-0068).
#
#   just                 # every recipe, by group
#   just dev             # dev Postgres (if not running) + migrate + web export (if stale) + go run serve
#   just test            # start + reset the test Postgres (:5434), run the Go suite
#   just lint            # gofmt + go vet + staticcheck + package layout
#   just web-check       # web: lint + typecheck + locale keys + build + export check
#   just security        # govulncheck + gosec + pnpm audit + actionlint/shellcheck/hadolint
#
# `.env` is loaded by just (dotenv-load) for every recipe; the binary itself reads only the
# process environment. `cp .env.example .env` first.

set shell := ["bash", "-c"]
set dotenv-load := true

GO := env('GO', 'go')
PNPM := env('PNPM', 'pnpm')
STATICCHECK := 'honnef.co/go/tools/cmd/staticcheck@v0.8.2'

export COMPOSE_FILE := justfile_directory() / "env/compose.yml"

[private]
default:
    @{{ just_executable() }} --list

import 'env/justice/build.just'
import 'env/justice/run.just'
import 'env/justice/database.just'
import 'env/justice/quality.just'
import 'env/justice/web.just'
import 'env/justice/security.just'
import 'env/justice/docker.just'
import 'env/justice/package.just'
import 'env/justice/release.just'
