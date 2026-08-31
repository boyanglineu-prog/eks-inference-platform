# ADR 003 — Observability: Prometheus + Grafana over Datadog

## Status
Accepted

## Context
Need an observability stack to track request rate, latency percentiles,
and pod resource utilization for the inference service.

## Decision
Use Prometheus + Grafana via kube-prometheus-stack Helm chart.

## Reasons
- Open source — no per-seat or per-host licensing cost
- Native Kubernetes integration — ServiceMonitor CRD designed for k8s
- Industry standard — Prometheus is the default monitoring solution
  for Kubernetes workloads
- Resume signal — Prometheus + Grafana appears in AI infra job postings
  at Datadog, AWS, and Microsoft
- Self-hosted — data stays within the cluster, no external dependency

## Alternatives Considered
- **Datadog** — excellent product but expensive ($15-23/host/month),
  overkill for a portfolio project, requires agent installation
- **AWS CloudWatch** — AWS-native but limited Kubernetes integration,
  weaker visualization than Grafana
- **New Relic** — similar cost concerns as Datadog

## Consequences
- Must manage Prometheus storage and retention manually
- No built-in alerting as sophisticated as Datadog
- Grafana dashboard requires manual configuration
- All acceptable tradeoffs for a portfolio project
