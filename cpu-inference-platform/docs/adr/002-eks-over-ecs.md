# ADR 002 — Orchestration: EKS over ECS or Lambda

## Status
Accepted

## Context
Need a production-grade container orchestration platform on AWS to serve
the inference API reliably with autoscaling and observability.

## Decision
Use AWS EKS (Elastic Kubernetes Service).

## Reasons
- Kubernetes is the industry standard — directly matches job requirements
  at AWS, Google, Microsoft, CoreWeave, and Datadog
- HPA autoscaling is native — scales pods on CPU metrics automatically
- Helm ecosystem — rich tooling for deployment and monitoring
- Portable — same kubectl/Helm skills work on GKE, AKS, any k8s cluster
- Resume signal — EKS + Kubernetes appears in nearly every AI infra posting

## Alternatives Considered
- **ECS (Elastic Container Service)** — AWS-only, less portable,
  smaller ecosystem, not Kubernetes
- **AWS Lambda** — serverless, cold start latency unsuitable for
  ML inference, 15min timeout, no persistent model loading
- **EC2 directly** — no orchestration, manual scaling, no self-healing

## Consequences
- Higher complexity than ECS or Lambda
- Higher base cost (~$0.10/hr control plane)
- Steeper learning curve — worth it for resume signal
