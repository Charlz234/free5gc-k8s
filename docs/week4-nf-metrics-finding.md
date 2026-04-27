# NF Prometheus Metrics — Investigation Finding

## Status: Deferred to Week 6

## What was attempted
- Verified port 9089 (metrics) and 8080 (SBI) on all NF pods
- All connections refused — no HTTP server bound on either port
- ServiceMonitors are correctly structured (app label selector, port: metrics, path: /metrics)

## Root cause
`free5gc/*:v4.2.0` DockerHub images do not start the Prometheus metrics HTTP server.
Metrics support was introduced in v4.1.0 but is only active in images built from
`free5gc-compose` (build-from-source). The standalone DockerHub images ship the binary
without the metrics server initialised.

## Alternatives evaluated
- **towards5gs-helm images**: Pinned to v3.3.0 — incompatible with v4.2.0 configs
- **Build from free5gc-compose source**: Requires Go module egress (blocked on GCP VPC)
- **Custom sidecar exporter**: Overkill for this phase

## Resolution
NF-level metrics deferred to Week 6 — custom Go Prometheus exporter targeting the
free5gc SBI/PFCP interfaces directly. This is a stronger portfolio deliverable than
consuming upstream images.

## Current observability coverage
- Cluster-level: node-exporter, kube-state-metrics (pod health, restarts, CPU/mem)
- Grafana: kubernetes-overview + node-exporter dashboards live at grafana.niv-dev.xyz
- NF-level: pending Week 6
EOF