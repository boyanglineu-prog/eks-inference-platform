# Benchmark Results

**Infrastructure:** AWS EKS, 2x t3.medium nodes, 2 pods  
**Model:** distilbert-base-uncased-finetuned-sst-2-english  
**Tool:** Locust  
**Date:** May 2026

---

## Summary

| Users | Req/sec | p50 | p95 | p99 | Failures |
|-------|---------|-----|-----|-----|----------|
| 10    | ~6      | 67ms | 130ms | 180ms | 0% |
| 20    | ~12     | 66ms | 140ms | 180ms | 0% |
| 50    | ~28     | 130ms | 420ms | 580ms | 0% |

---

## Test 1 — Light Load (10 users)

```
Concurrent users   10
Spawn rate         1/sec

/predict   p50: 67ms   p95: 130ms   p99: 180ms   p100: 200ms
/health    p50: 20ms   p95: 28ms    p99: 100ms   p100: 100ms
Failures   0%
```

---

## Test 2 — Medium Load (20 users)

```
Concurrent users   20
Spawn rate         2/sec

/predict   p50: 66ms   p95: 140ms   p99: 180ms   p100: 280ms
/health    p50: 19ms   p95: 28ms    p99: 49ms    p100: 61ms
Failures   0%
```

---

## Test 3 — Heavy Load (50 users)

```
Concurrent users   50
Spawn rate         5/sec

/predict   p50: 130ms   p95: 420ms   p99: 580ms   p100: 750ms
/health    p50: 21ms    p95: 75ms    p99: 120ms   p100: 160ms
Failures   0%
```

---

## Observations

- Service handles 10-20 concurrent users with stable sub-200ms p99 latency
- At 50 users p99 degrades to 580ms — CPU saturation on 2 pods
- HPA configured to scale 2→10 pods at 70% CPU — would reduce latency under sustained heavy load
- /health endpoint stays fast across all load levels — Kubernetes liveness probe reliable

---

## Resume Bullets (use Test 2 — clean baseline)

- Deployed ML inference service on AWS EKS serving **12 req/sec** at **p99 latency 180ms** across 2 pods with **0% error rate**
- Load tested with Locust across 20 concurrent users; validated stable throughput at p50 67ms
- Configured HPA autoscaling 2→10 pods triggered by CPU utilization threshold
