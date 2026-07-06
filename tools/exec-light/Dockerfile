# syntax=docker/dockerfile:1
# exec-light (~800MB): Python 3, Node.js 20, Bash, C (gcc), C++ (g++),
# x86-64 assembly (nasm + ld). Ships the shared executor-server + nsjail sandbox.

# --- Build stage: compile the executor server (Go stdlib only) --------------
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download || true
COPY executor-server.go ./
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w" -o /out/executor-server .

# --- nsjail stage: build the sandbox binary from source ---------------------
# The upstream ghcr.io/google/nsjail image is not publicly pullable, so build
# nsjail ourselves against a pinned release for a reproducible, verifiable binary.
FROM ubuntu:24.04 AS nsjail
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates build-essential bison flex \
    libprotobuf-dev protobuf-compiler libnl-route-3-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 --branch 3.4 https://github.com/google/nsjail.git /nsjail \
    && make -C /nsjail

# --- Runtime stage ----------------------------------------------------------
FROM ubuntu:24.04

# Disable interactive prompts during package installation.
ENV DEBIAN_FRONTEND=noninteractive

# Install base tools + runtimes. build-essential provides gcc/g++/ld/make.
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    python3 \
    nodejs \
    npm \
    bash \
    ca-certificates \
    curl \
    wget \
    nasm \
    libprotobuf-dev \
    libnl-route-3-200 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Create a non-root user used as the sandbox identity target.
RUN useradd -m -s /bin/bash sandbox

# Copy the nsjail binary built from source in the nsjail stage.
COPY --from=nsjail /nsjail/nsjail /usr/bin/nsjail

# Sandbox policy.
COPY nsjail.cfg /etc/nsjail/nsjail.cfg

# Executor server (compiled in the build stage).
COPY --from=build /out/executor-server /executor-server

EXPOSE 8765

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8765/health || exit 1

# nsjail requires elevated privileges (namespaces / pivot_root), so the server
# runs as root and drops into the sandbox user inside the jail.
ENTRYPOINT ["/executor-server"]
