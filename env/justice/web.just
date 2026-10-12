# Build the static web export once (web/out; served by `just dev` / `just run` without restart)
[group('web')]
web-build:
    cd web && {{ PNPM }} install --frozen-lockfile && {{ PNPM }} build

# Build the web export and write .gz siblings of its text assets, as the image and the packages ship it
[group('web')]
web-dist: web-build
    cd web && {{ PNPM }} precompress

# Web gates: lockfile install, lint, typecheck, locale keys, static export build + export check
[group('web')]
web-check:
    cd web && {{ PNPM }} install --frozen-lockfile && {{ PNPM }} lint && {{ PNPM }} typecheck && {{ PNPM }} test && {{ PNPM }} build && {{ PNPM }} check-export
