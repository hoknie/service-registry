# syntax=docker/dockerfile:1

# ---- web: the Next.js static export (`output: "export"`) → /web/out ----------------------
FROM node:24-alpine AS web
WORKDIR /web
RUN corepack enable
ENV NEXT_TELEMETRY_DISABLED=1
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build \
 && pnpm check-export \
 && find out -type f -size +1k \
      \( -name '*.html' -o -name '*.js' -o -name '*.css' -o -name '*.txt' -o -name '*.json' \
         -o -name '*.svg' -o -name '*.map' \) \
      -exec sh -c 'for f; do gzip -9 -c "$f" > "$f.gz"; done' sh {} +

# ---- build: the release binary (static: CGO_ENABLED=0, stripped, reproducible paths) ------
FROM golang:1.26.8-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /svc-registry ./cmd/svc-registry
RUN mkdir -p /out/data/uploads

# ---- the image: one process, `svc-registry serve` -----------------------------------------
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /svc-registry /svc-registry
COPY --from=web /web/out /web
COPY --from=build --chown=65534:65534 /out/data /data
USER 65534:65534
EXPOSE 8080
ENV HTTP_ADDR=0.0.0.0:8080 WEB_DIST_DIR=/web UPLOADS_DIR=/data/uploads
ENTRYPOINT ["/svc-registry"]
CMD ["serve"]
