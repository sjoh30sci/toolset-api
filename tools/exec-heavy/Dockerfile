# syntax=docker/dockerfile:1
# exec-heavy (~2GB): .NET 8 SDK, Java 21 (OpenJDK), Rust (rustc/cargo), plus
# the shared executor-server + nsjail sandbox. Runtimes are installed onto a
# single Ubuntu base (rather than clobbering full-image layers) so that /usr,
# /etc, and library paths remain coherent.

# --- Build stage: compile the executor server (Go stdlib only) --------------
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download || true
COPY executor-server.go ./
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w" -o /out/executor-server .

# --- nsjail stage: build the sandbox binary from source ---------------------
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

ENV DEBIAN_FRONTEND=noninteractive

# Base packages + toolchains available from the Ubuntu repositories:
#   - openjdk-21-jdk : Java 21 (javac/java)
#   - dotnet-sdk-8.0 : .NET 8 (dotnet/csc), available in Ubuntu 24.04 repos
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    build-essential \
    openjdk-21-jdk-headless \
    dotnet-sdk-8.0 \
    libprotobuf-dev \
    libnl-route-3-200 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Rust toolchain via rustup (pinned to stable, minimal profile).
ENV RUSTUP_HOME=/opt/rust/rustup \
    CARGO_HOME=/opt/rust/cargo \
    PATH=/opt/rust/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --profile minimal --default-toolchain stable \
    && chmod -R a+rx /opt/rust

# Create the sandbox identity target.
RUN useradd -m -s /bin/bash sandbox

# nsjail binary built from source in the nsjail stage.
COPY --from=nsjail /nsjail/nsjail /usr/bin/nsjail

# Sandbox policy.
COPY nsjail.cfg /etc/nsjail/nsjail.cfg

# Executor server.
COPY --from=build /out/executor-server /executor-server

EXPOSE 8765

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8765/health || exit 1

ENTRYPOINT ["/executor-server"]
