# syntax=docker/dockerfile:1

# --- Build stage ------------------------------------------------------------
# CGO is required for github.com/mattn/go-sqlite3, so we use a build image
# that has a C toolchain and link statically for a tiny final image.
FROM golang:1.22-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download || true

# Copy source and build.
COPY . .

ARG VERSION=dev
ENV CGO_ENABLED=1
RUN go build \
      -trimpath \
      -tags "sqlite_omit_load_extension" \
      -ldflags "-s -w -X main.Version=${VERSION} -linkmode external -extldflags '-static'" \
      -o /out/gateway .

# --- Runtime stage ----------------------------------------------------------
# Alpine base is required because go-sqlite3 links against musl (CGO). The final
# image is ~15MB: alpine (~7MB) + the stripped static-musl binary.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S toolset && adduser -S -G toolset toolset \
    && mkdir -p /data /logs && chown -R toolset:toolset /data /logs

WORKDIR /app

COPY --from=build /out/gateway /app/gateway
COPY migrations /app/migrations

ENV TOOLSET_PORT=8080 \
    TOOLSET_DB_PATH=/data/toolset.db

EXPOSE 8080
USER toolset:toolset

ENTRYPOINT ["/app/gateway"]
