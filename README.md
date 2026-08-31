# EKS Inference Platform

A production-style ML inference platform on AWS EKS, built as a portfolio project targeting AI
infrastructure engineering roles. Two independent model-serving paths on one cluster — a CPU
classifier and a GPU-backed generative model — plus a hand-built Kubernetes operator that
autoscales the GPU workload based on live metrics.

**Stack:** Python · Go · AWS EKS · Kubernetes · vLLM · Docker · Prometheus · Grafana · DCGM · Helm · GitHub Actions

---

## What's here

| Project | What it is |
|---|---|
| [`cpu-inference-platform/`](./cpu-inference-platform) | FastAPI + HuggingFace distilbert, Kubernetes HPA, Prometheus/Grafana, Locust load testing, GitHub Actions CI/CD |
| [`gpu-inference-platform/`](./gpu-inference-platform) | vLLM serving a generative model on a GPU nodegroup, NVIDIA DCGM observability |
| [`gpu-inference-platform/scaler-controller/`](./gpu-inference-platform/scaler-controller) | A custom Go Kubernetes operator (CRD + `controller-runtime` reconciler) that autoscales the GPU workload off live Prometheus queue-depth and GPU-utilization metrics — a simplified, hand-built version of what KEDA/HPA do in production |
| [`deploy.sh`](./deploy.sh) | Brings up the base cluster + CPU stack alone |
| [`deploy-gpu-stack.sh`](./deploy-gpu-stack.sh) | Brings up the full stack — base cluster, GPU nodegroup, vLLM, DCGM, and the controller |

Each sub-project has its own detailed README covering architecture, setup, and results — this file
is just the map.

---

## Real, verified result

The custom controller's core claim — that it can detect load and actually scale a real GPU
workload — was validated end to end on a live cluster, not simulated:

| Scenario | Latency (300 max_tokens) |
|---|---|
| Baseline, idle, single request | 4.6s |
| Under a 50-request concurrent burst | first wave ~5.9s → tail 64.4s (~14x degradation) |

The controller detected the elevated queue depth, computed a real scale-up decision, and genuinely
patched the Deployment — confirmed by a real second pod scheduling and running. Once load cleared,
the next reconcile cycle scaled back down and terminated it. Full detail in
[`gpu-inference-platform/README.md`](./gpu-inference-platform/README.md#custom-autoscaling-controller).

---

## Architecture, at a glance

```
Client → AWS ALB/Ingress → FastAPI Pod (CPU, HPA-managed) → distilbert
Client → vLLM Service (ClusterIP) → vLLM Pod (GPU nodegroup) → generative model

Prometheus scrapes both apps + DCGM (GPU utilization/VRAM) via ServiceMonitors
  → Grafana dashboards

ScalingPolicy CRD → Go reconciler (its own Pod, watches + polls every 15s)
  → queries Prometheus for queue depth / GPU utilization
  → patches the vLLM Deployment's replica count
  → Kubernetes' own built-in Deployment controller creates/removes the actual Pods
```

---

## Getting started

```bash
git clone https://github.com/boyanglineu-prog/eks-inference-platform
cd eks-inference-platform

# CPU stack only
./deploy.sh

# Full stack — CPU + GPU + vLLM + DCGM + custom controller
./deploy-gpu-stack.sh
```

See each sub-project's README for local development, testing, and cleanup instructions.

---

## License

MIT
