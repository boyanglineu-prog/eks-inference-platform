# gpu-inference-platform

A GPU-backed generative model serving stack — [vLLM](https://github.com/vllm-project/vllm) serving
`Qwen/Qwen2.5-1.5B-Instruct` on an EKS GPU nodegroup, observed with Prometheus/Grafana/DCGM, and
autoscaled by a custom-built Go Kubernetes operator. Built as an extension to
[`eks-inference-platform`](../cpu-inference-platform) (the CPU/distilbert service), added
alongside it rather than replacing it — same cluster, two independent model-serving paths.

*(Local companion project — not yet a standalone public GitHub repo.)*

**Stack:** vLLM · AWS EKS (GPU nodegroup) · NVIDIA DCGM · Prometheus · Grafana · Go (custom operator, companion project)

---

## What this demonstrates

| Component | Description |
|---|---|
| GPU Generative Serving | vLLM serving `Qwen/Qwen2.5-1.5B-Instruct` on a `g4dn.xlarge` GPU nodegroup, OpenAI-compatible API |
| GPU Observability | NVIDIA DCGM exporter — GPU utilization, VRAM usage, alongside app-level queue-depth metrics |
| Custom Autoscaling Controller | [`scaler-controller`](./scaler-controller) (companion project, nested in this folder) — CRD + reconciler, autoscales this Deployment off real Prometheus metrics |
| Real load test | Verified real scale-up and scale-down driven by real load, not a simulated/mocked result |

---

## Architecture

```
# GPU generative inference path
Client → vLLM Service (ClusterIP) → vLLM Pod (GPU nodegroup) → Qwen2.5-1.5B-Instruct
       → OpenAI-compatible /v1/chat/completions response

# Observability pipeline
vLLM /metrics  ──┐
DCGM /metrics ───┴─→ Prometheus (ServiceMonitor-driven scraping, no manual config)
                     → Grafana dashboards (GPU util/VRAM, queue depth, pod count)

# Custom autoscaling loop (scaler-controller, companion repo)
ScalingPolicy CRD (declares target Deployment, min/max replicas, queue-depth/GPU-util thresholds)
  → Go reconciler (its own Pod) watches it + polls every 15s
  → queries Prometheus for real vLLM queue depth (and/or fleet GPU utilization)
  → computes desired replicas (same proportional formula real HPA uses, max across metrics)
  → patches the vLLM Deployment
  → Kubernetes' own built-in Deployment controller creates/removes the actual Pods
```

---

## GPU Nodegroup

Added on top of the already-running base cluster (from the top-level `deploy.sh`) —
separate, additive step, doesn't touch the existing CPU nodegroup:

```bash
eksctl create nodegroup \
  --cluster inference-platform \
  --region us-west-2 \
  --name gpu-nodes \
  --node-type g4dn.xlarge \
  --nodes 1 \
  --nodes-min 1 \
  --nodes-max 1
```

`--node-ami-family` deliberately omitted — eksctl auto-detects `g4dn.xlarge` is a GPU instance type
and automatically selects the EKS-Optimized Accelerated AMI, and auto-installs the NVIDIA device
plugin DaemonSet — no manual device plugin step needed.

Verified with a real smoke-test pod (not just checking `Allocatable` metadata) — a real `nvidia-smi`
table came back: Tesla T4, driver 580.178.04, CUDA 13.0, 15360MiB VRAM. Confirms the GPU is
genuinely schedulable and usable inside a container, not just present in node metadata.

**Cost:** `g4dn.xlarge` is ~$0.53/hr on-demand (vs ~$0.04/hr per t3.medium). `eksctl delete cluster`
deletes *all* nodegroups automatically — no separate teardown needed for `gpu-nodes`.

---

## vLLM Deployment

`manifests/vllm-deployment.yaml` — Deployment + Service, deployed alongside the existing distilbert/FastAPI
service, not replacing it:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-inference
spec:
  replicas: 1
  selector:
    matchLabels: { app: vllm-inference }
  template:
    metadata:
      labels: { app: vllm-inference }
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
          args: ["--model", "Qwen/Qwen2.5-1.5B-Instruct"]
          ports: [{ containerPort: 8000 }]
          resources:
            limits: { nvidia.com/gpu: 1 }
            requests: { nvidia.com/gpu: 1 }
          readinessProbe:
            httpGet: { path: /health, port: 8000 }
            initialDelaySeconds: 120
            periodSeconds: 15
            failureThreshold: 20
---
apiVersion: v1
kind: Service
metadata:
  name: vllm-inference
spec:
  selector: { app: vllm-inference }
  ports: [{ name: http, port: 80, targetPort: 8000 }]
  type: ClusterIP
```

No `livenessProbe` — deliberate. Model download + CUDA graph capture takes a few minutes on first
start; a liveness probe risks killing the pod mid-load in a restart loop.

**Verified end-to-end:** `GET /v1/models` confirmed the model loaded; `POST /v1/chat/completions`
returned real generated text (correct factual answer, real token counts) — proving actual GPU
inference, not a cached/static response.

---

## DCGM Exporter (GPU observability)

`manifests/dcgm-exporter.yaml` — DaemonSet + Service + ServiceMonitor, same pattern as vLLM's. Deliberately
**no `resources.limits.nvidia.com/gpu`** — the GPU node's single GPU is already claimed by vLLM's
own resource request; DCGM instead runs `privileged: true` for direct host-level GPU visibility
(monitoring the whole node's GPU(s), not "consuming" one via the device-plugin resource system).

Verified via `curl .../metrics`: `DCGM_FI_DEV_GPU_UTIL` and `DCGM_FI_DEV_FB_USED` both returned real
numbers, matching vLLM's own startup log (weights + KV cache ≈ 13.1GB) — two independent tools
agreeing on real state, not placeholder data.

---

## Custom Autoscaling Controller

A hand-built Kubernetes operator ([`scaler-controller`](./scaler-controller), companion
project) — a `ScalingPolicy` CRD plus a `controller-runtime` reconciler, written in Go — autoscaling
the vLLM Deployment based on live Prometheus metrics. Supports two independent scaling signals,
following the same "most-constrained-metric-wins" rule real HPA uses with multiple metrics:
**queue depth** (per-Deployment, reported by vLLM itself) and **fleet-wide GPU utilization**
(per-device, via DCGM — deliberately not per-Deployment, since a physical GPU has no concept of
which Kubernetes workload caused its load). A deliberately simplified version of what
[KEDA](https://keda.sh)/HPA do in production, built to learn and demonstrate the Operator pattern
hands-on rather than treat it as a black box.

**Real load test result (2026-08-24)** — queue-depth-driven scaling, verified end to end on a live
cluster (2x GPU nodes, vLLM throttled to `--max-num-seqs 4` to make queuing observable on a single
small model). GPU-utilization-based scaling is built and unit-tested but not yet exercised against a
live cluster:

| Scenario | Latency (300 max_tokens) |
|---|---|
| Baseline, idle, single request | 4.6s |
| Under a 50-request concurrent burst | first wave ~5.9s → tail 64.4s (~14x degradation) |

Full data (all 50 requests, wave-by-wave) in `memo.md`. The controller detected the elevated queue
depth, computed a real scale-up decision, and genuinely patched the Deployment — confirmed by a real
second pod scheduling and running. Once load cleared, the next reconcile cycle scaled back down and
terminated it — the full elastic loop, both directions, driven entirely by real metrics.

---

## Project structure

```
gpu-inference-platform/
├── manifests/
│   ├── kustomization.yaml     # ties the three standing-infra manifests below into one unit
│   ├── vllm-deployment.yaml   # vLLM Deployment + Service
│   ├── vllm-servicemonitor.yaml  # Prometheus scrape config for vLLM's /metrics
│   ├── dcgm-exporter.yaml     # GPU utilization/VRAM exporter (DaemonSet + Service + ServiceMonitor)
│   ├── vllm-scalingpolicy.yaml   # Real ScalingPolicy instance — applied separately, not part of
│   │                             # the kustomization (different lifecycle: needs the controller
│   │                             # to already exist to mean anything)
│   └── gpu-smoke-test.yaml    # One-off nvidia-smi debug pod, not part of the standing stack
├── memo.md                   # Full command/debugging history for this stack
└── scaler-controller/        # Custom Go Kubernetes operator (companion project, own README/memo)
    ├── api/v1alpha1/          # ScalingPolicy CRD schema
    ├── internal/controller/   # Reconciler + MetricsProvider
    ├── cmd/main.go
    ├── README.md
    └── memo.md                # Controller's own dev history (Go/client-go/kubebuilder learning)
```

The custom operator's source (`scaler-controller/`) is nested directly in this folder — see
[Custom Autoscaling Controller](#custom-autoscaling-controller) above. The CPU/distilbert service
(`cpu-inference-platform/`) is a separate, sibling project.

---

## Getting started

### Deploy the full stack (base cluster + GPU + vLLM + DCGM + controller)

From `~/CS/EKS_Project/` (one level up, parallel to this folder):

```bash
chmod +x deploy-gpu-stack.sh
./deploy-gpu-stack.sh
```
This calls the top-level `deploy.sh` first (base cluster), then adds everything in this
folder plus the controller. Brings up 2x `g4dn.xlarge` GPU nodes (~$1.06/hr on top of the base
cluster) — always delete the cluster when done (see Cleanup).

### Test the vLLM endpoint directly

```bash
kubectl port-forward svc/vllm-inference 8000:80

curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "Qwen/Qwen2.5-1.5B-Instruct", "messages": [{"role": "user", "content": "Say hello."}]}'
```

### Access observability

```bash
kubectl port-forward -n monitoring svc/monitoring-kube-prometheus-prometheus 9090:9090 &
kubectl port-forward -n monitoring svc/monitoring-grafana 3000:80 &
kubectl --namespace monitoring get secrets monitoring-grafana \
  -o jsonpath="{.data.admin-password}" | base64 -d ; echo
```

### Run a load test

```bash
for i in $(seq 1 50); do
  curl -s -X POST http://localhost:8000/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model": "Qwen/Qwen2.5-1.5B-Instruct", "messages": [{"role": "user", "content": "..."}], "max_tokens": 300}' \
    -o /dev/null -w "%{time_total}\n" &
done
wait
```

### Cleanup

```bash
# ⚠️ full stack with 2x GPU nodes: ~$29/day. Always delete when done.
eksctl delete cluster --name inference-platform --region us-west-2
```

---

## Target companies

| Company | Role | Key matched keywords |
|---|---|---|
| CoreWeave | SWE GPU Infra | Kubernetes, GPU scheduling, vLLM, DCGM, Prometheus, cloud-native |
| AWS / Amazon | SDE1 AI Infra | EKS, GPU nodegroups, Kubernetes, custom controllers |
| Google | L3 SWE Infra | Distributed systems, Kubernetes, custom controllers, observability |
| Datadog | SWE Infra | Prometheus, Grafana, observability, Go |

---

## References

- [vLLM documentation](https://docs.vllm.ai)
- [NVIDIA DCGM Exporter](https://github.com/NVIDIA/dcgm-exporter)
- [KEDA](https://keda.sh)
- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts)
- [eksctl documentation](https://eksctl.io)

---

## License

MIT
