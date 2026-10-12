# Local dev in one command: dev DB (if not running) + migrate + web export (if stale) + serve
[group('run')]
dev:
    env/scripts/dev.sh all

# Check that `just dev`'s preparation is idempotent (DB not restarted, export not rebuilt)
[group('run')]
dev-check:
    env/scripts/dev-check.sh

# Run the server: API on HTTP_ADDR, the web UI from the static export in WEB_DIST_DIR (no migrate)
[group('run')]
run:
    {{ GO }} run ./cmd/svc-registry serve

# Rebuild the web export whenever web/ sources change (polls 1 s; reload the page, no HMR)
[group('run')]
web-watch:
    PNPM={{ PNPM }} env/scripts/web-watch.sh
