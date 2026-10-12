# syntax=docker/dockerfile:1

# ---- web: the Next.js static export (`output: "export"`) → /web/out ----------------------
FROM --platform=$BUILDPLATFORM node:26-alpine AS web
WORKDIR /web
ENV NEXT_TELEMETRY_DISABLED=1
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml web/.npmrc ./
RUN npm install --global --no-fund --no-audit "$(node -p 'require("./package.json").packageManager')"
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build \
 && pnpm check-export \
 && pnpm precompress

# ---- build: the release binary (static: CGO_ENABLED=0, stripped, reproducible paths) ------
FROM --platform=$BUILDPLATFORM golang:1.27.0-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY migrations ./migrations
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /svc-registry ./cmd/svc-registry
RUN mkdir -p /out/data/uploads

# ---- the image: one process, `svc-registry serve` -----------------------------------------
FROM scratch
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="svc-registry" \
      org.opencontainers.image.source="https://github.com/hoknie/service-registry" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION"
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /svc-registry /svc-registry
COPY --from=web /web/out /web
COPY --from=build --chown=65534:65534 /out/data /data
USER 65534:65534
EXPOSE 8080
ENV HTTP_ADDR=0.0.0.0:8080 WEB_DIST_DIR=/web UPLOADS_DIR=/data/uploads
ENTRYPOINT ["/svc-registry"]
CMD ["serve"]
