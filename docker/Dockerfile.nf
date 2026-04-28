# syntax=docker/dockerfile:1
# Multi-stage build for free5GC control plane NFs.
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
    git ca-certificates make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Clone only the NF submodule repo at the pinned tag.
# Each NF lives at github.com/free5gc/{nf} and is tagged independently.
# We do a shallow clone for speed; depth=1 is sufficient for a CI build.
RUN git clone --depth 1 --branch ${FREE5GC_VERSION} \
    https://github.com/free5gc/${NF}.git /src/${NF}

WORKDIR /src/${NF}

# Download deps and build. The binary name matches the NF name in all cases.
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/${NF} ./cmd/${NF}/main.go

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
