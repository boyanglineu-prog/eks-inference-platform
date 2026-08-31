# ADR 004 — Deployment: Helm over raw kubectl

## Status
Accepted

## Context
Need a way to manage Kubernetes manifests for the inference service,
including deployment, service, ingress, HPA, and ServiceMonitor.

## Decision
Use Helm charts for all Kubernetes resource management.

## Reasons
- Single source of truth — values.yaml centralizes all configuration
- One command deployment — helm install/upgrade deploys all resources
- Templating — {{ .Values.xxx }} eliminates hardcoded duplication
- Version tracking — helm list shows revision history
- Rollback — helm rollback reverts to previous working state
- Ecosystem — third-party apps (ALB controller, Prometheus) distributed
  as Helm charts, consistent tooling across all installs

## Alternatives Considered
- **Raw kubectl apply** — no templating, values hardcoded in every file,
  must apply each file separately, no rollback mechanism
- **Kustomize** — overlay-based, no templating syntax, less ecosystem
  support than Helm for third-party charts
- **Terraform** — better for infrastructure provisioning, not designed
  for Kubernetes app deployment

## Consequences
- Additional abstraction layer — must understand both Helm and Kubernetes
- Helm templates are not valid YAML — YAML formatters break {{ }} syntax
- Small overhead — helm template renders before applying to cluster
- All acceptable tradeoffs given Helm's industry adoption
