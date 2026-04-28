# syntax=docker/dockerfile:1
# Multi-stage build for free5GC control plane NFs.
# Clones the free5gc monorepo at the pinned release tag and initialises only
# the submodule for the requested NF — keeps each parallel CI job independent.
#
# Build arg NF selects which network function to compile.
# Usage: docker build --build-arg NF=amf -t free5gc-amf .

ARG NF
ARG FREE5GC_VERSION=v4.2.0
ARG GO_VERSION=1.25.5

# ── Stage 1: builder ──────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-bookworm AS builder

ARG NF
ARG FREE5GC_VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Clone monorepo at pinned release — shallow for speed.
# Then initialise only the one submodule needed for this build job.
# This ensures the NF source is at exactly the commit the monorepo pins,
# not just the latest tag on the individual NF repo.
RUN git clone --depth 1 --branch ${FREE5GC_VERSION} \
    https://github.com/free5gc/free5gc.git /src/free5gc && \
    cd /src/free5gc && \
    git submodule update --init --depth 1 NFs/${NF}

WORKDIR /src/free5gc/NFs/${NF}

# Download deps then build. Entry point is cmd/main.go across all CP NFs
# (verified on amf, smf, pcf at their pinned commits).
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/${NF} ./cmd/main.go

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

ARG NF

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates iproute2 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/${NF} /free5gc/${NF}

WORKDIR /free5gc

# Port 8000  — SBI (service-based interface)
# Port 9089  — Prometheus metrics (activated via metrics.enable: true in config)
EXPOSE 8000 9089

ENTRYPOINT ["/bin/sh", "-c", "/free5gc/${NF} -c /free5gc/config/${NF}cfg.yaml"]
