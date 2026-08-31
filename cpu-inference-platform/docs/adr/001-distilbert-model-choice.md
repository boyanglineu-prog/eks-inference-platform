# ADR 001 — Model Choice: distilbert

## Status
Accepted

## Context
This project demonstrates AI infrastructure engineering, not ML research.
A model is needed purely as a realistic inference workload.

## Decision
Use `distilbert-base-uncased-finetuned-sst-2-english` for sentiment analysis.

## Reasons
- Lightweight (~268MB) — fits in t3.medium nodes without GPU
- Fast inference (~67ms p50) — produces meaningful latency benchmarks
- Well-known — recruiters and interviewers recognize it
- CPU-only — keeps infrastructure cost low during development

## Alternatives Considered
- **BERT-large** — too large for CPU inference, requires GPU nodes
- **GPT-2** — generative model, harder to benchmark consistently
- **Custom model** — out of scope, project focus is infrastructure

## Consequences
- Inference is CPU-bound — realistic load for autoscaling demonstration
- Model accuracy is not a concern — intentional tradeoff
