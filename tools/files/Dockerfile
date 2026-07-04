# syntax=docker/dockerfile:1

# --- Build stage ------------------------------------------------------------
# The file-server uses only the Go standard library, so a static CGO-free
# binary builds cleanly and runs on a minimal Alpine runtime.
FROM golang:1.22-alpine AS build

WORKDIR /src

# Cache module metadata first (stdlib-only, so this is effectively a no-op but
# keeps the layer cache warm if deps are added later).
COPY go.mod ./
RUN go mod download || true

COPY . .

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w" -o /out/file-server .

# --- Runtime stage ----------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl \
    && addgroup -S toolset && adduser -S -G toolset toolset \
    && mkdir -p /data/files && chown -R toolset:toolset /data

WORKDIR /app
COPY --from=build /out/file-server /app/file-server

ENV FILES_PORT=8765 \
    FILES_SANDBOX_ROOT=/data/files

# Internal port only; do not publish to the host.
EXPOSE 8765

USER toolset:toolset

HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=5s \
    CMD curl -fsS http://localhost:8765/health >/dev/null || exit 1

ENTRYPOINT ["/app/file-server"]
