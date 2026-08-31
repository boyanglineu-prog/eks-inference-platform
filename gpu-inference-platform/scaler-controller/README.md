# scaler-controller

A custom Kubernetes operator, written in Go, that autoscales a GPU-backed model-serving Deployment
based on live Prometheus queue-depth metrics — a simplified, hand-built version of what
[KEDA](https://keda.sh)/[HorizontalPodAutoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
do in production. Built as a companion project to
[eks-inference-platform](https://github.com/boyanglineu-prog/eks-inference-platform), to learn and
demonstrate the Kubernetes Operator pattern end to end: a custom CRD, a `controller-runtime`
reconciler, and real metrics-driven scaling decisions, deployed and validated against a real EKS
cluster with a real GPU workload ([vLLM](https://github.com/vllm-project/vllm) serving
`Qwen/Qwen2.5-1.5B-Instruct`).

*(Companion repo, currently local-only — not yet published as its own GitHub repository.)*

---

## What this demonstrates

| Component | Description |
|---|---|
| Custom Resource Definition | `ScalingPolicy` — declares a target Deployment, min/max replicas, and a queue-depth threshold |
| Reconciler | Fetches the target Deployment, queries live queue-depth **and/or** fleet-wide GPU utilization metrics, computes a desired replica count per metric using the same proportional formula real HPA uses, takes the max across metrics (same "most-constrained-metric wins" rule HPA applies with multiple metrics), patches the Deployment, reports status |
| MetricsProvider | An interface, dependency-injected — a fake implementation for unit tests, a real `PrometheusMetricsProvider` for production, so scaling logic is testable without a live cluster |
| Testing | Ginkgo/Gomega integration tests against `envtest` (a real local `etcd`/`kube-apiserver`, no full cluster needed) + plain Go unit tests with `httptest` for the Prometheus client |
| Real validation | Deployed to a real EKS cluster, watched it scale a real vLLM Deployment up and back down in response to real load |

---

## Architecture

```
ScalingPolicy (CRD instance)          — declarative config, inert data
  spec: { targetDeployment, minReplicas, maxReplicas, queueDepthThreshold }
  status: { conditions: [{ type: Available, status, reason, message }] }

Reconcile() loop, every 15s or on any ScalingPolicy change:
  1. fetch the ScalingPolicy
  2. fetch its target Deployment
  3. query MetricsProvider.GetQueueDepth() → real Prometheus query:
       sum(vllm:num_requests_waiting{service="<targetDeployment>"})
  4. desired = ceil(current × observedQueueDepth / queueDepthThreshold)
  5. clamp desired into [minReplicas, maxReplicas]
  6. patch the Deployment if desired != current
  7. write Available: True/False + a status message back onto the ScalingPolicy
```

Same watch → reconcile → patch pattern every Kubernetes controller uses internally (the built-in
Deployment controller, the real HPA controller) — this operator is a separate, independent process
using the same public API, not something baked into Kubernetes itself.

---

## Real results (2026-08-24, against a live EKS cluster)

2x `g4dn.xlarge` GPU nodes, vLLM serving `Qwen/Qwen2.5-1.5B-Instruct` (throttled to
`--max-num-seqs 4` to make queuing observable on a single small model without needing hundreds of
concurrent users), `ScalingPolicy{ minReplicas: 1, maxReplicas: 2, queueDepthThreshold: 5 }`.

| Scenario | Latency (300 max_tokens) |
|---|---|
| Baseline, idle, single request | 4.6s |
| Under a 50-request concurrent burst | first wave ~5.9s → tail 64.4s (~14x degradation) |

The controller detected the elevated queue depth, computed `desired: 2`, and genuinely patched the
Deployment — confirmed by a real second pod scheduling and running. Once the burst finished and
queue depth dropped back to 0, the next reconcile cycle scaled back down to 1 and terminated the
second pod. Full elastic loop, both directions, driven entirely by real metrics.

---

## Known limitations (honest scope, not hidden)

- The real load test result above (4.6s → 64.4s, real scale-up/down) was driven by
  **queue depth only** — `GPUUtilizationThreshold` was added afterward, built and unit-tested
  (`envtest`/`httptest`, no live cluster) but not yet exercised against a real cluster's actual
  DCGM label shape.
- GPU utilization is deliberately fleet-wide, not per-Deployment (DCGM has no concept of which
  Kubernetes workload caused the load) — a coarser signal than queue depth by design, not an
  oversight.
- `MetricsProvider`'s queue-depth query assumes the target Deployment's name matches its Service's
  `service` label (true by project convention here, not enforced by the API).
- No `livenessProbe`-style self-healing beyond what `controller-runtime`'s leader election already
  provides.
- Test coverage (~68-73% of `internal/controller`) doesn't exercise every error branch (e.g. a
  failed Prometheus query mid-reconcile).
- RBAC is currently a `ClusterRole` (cluster-wide) rather than scoped to a single namespace —
  tightening this to least-privilege is a planned follow-up, not yet done.

---

## Project structure

```
scaler-controller/
├── api/v1alpha1/
│   ├── scalingpolicy_types.go     # ScalingPolicySpec/Status schema
│   └── groupversion_info.go       # API group/version registration
├── internal/controller/
│   ├── scalingpolicy_controller.go   # Reconcile() — the actual scaling logic
│   ├── metrics_provider.go           # PrometheusMetricsProvider (real implementation)
│   ├── metrics_provider_test.go      # httptest-based unit test, no cluster needed
│   ├── scalingpolicy_controller_test.go  # Ginkgo/envtest integration tests
│   └── suite_test.go                 # envtest bootstrap (real local etcd/kube-apiserver)
├── cmd/main.go                    # Manager setup, flag parsing, reconciler wiring
├── config/                        # CRD/RBAC/manager manifests (kustomize)
├── Dockerfile                     # Multi-stage build, distroless nonroot final image
└── Makefile                       # make manifests/generate/test/docker-build/install/deploy
```

---

## Getting started

### Prerequisites
- Go 1.24+
- Docker
- `kubectl` + access to a Kubernetes cluster (or none, for local test-only development)

### Run tests locally (no cluster needed)

```bash
make test
```
Downloads a pinned `etcd`/`kube-apiserver` binary pair (`envtest`), runs Ginkgo integration tests
against a real local API server plus plain Go unit tests — genuine validation without touching AWS.

### Build and push the image

```bash
docker build --platform linux/amd64 -t <registry>/scaler-controller:latest .
docker push <registry>/scaler-controller:latest
```
`--platform linux/amd64` matters if building on Apple Silicon — EKS nodes are x86_64.

### Deploy to a real cluster

```bash
make install                                              # registers the CRD
make deploy IMG=<registry>/scaler-controller:latest        # RBAC + manager Deployment
```

### Create a ScalingPolicy instance

```yaml
apiVersion: autoscaling.example.com/v1alpha1
kind: ScalingPolicy
metadata:
  name: vllm-scaler
  namespace: default
spec:
  targetDeployment: vllm-inference
  minReplicas: 1
  maxReplicas: 2
  queueDepthThreshold: 5
```

```bash
kubectl apply -f vllm-scalingpolicy.yaml
kubectl get scalingpolicy vllm-scaler -o yaml   # check status.conditions
```

### Cleanup

```bash
make undeploy
make uninstall
```

---

## Tech stack

| Category | Tools |
|---|---|
| Language | Go 1.26 |
| Kubernetes tooling | kubebuilder, controller-runtime, client-go, controller-gen |
| Metrics | `github.com/prometheus/client_golang` (real Prometheus API client) |
| Testing | Ginkgo, Gomega, envtest, `net/http/httptest` |
| Container | Docker (multi-stage, distroless nonroot) |

---

## Why a custom operator instead of KEDA/HPA directly

KEDA and HPA already solve this problem in production. This project builds a deliberately
simplified version of the same pattern — CRD, reconciler, metrics-driven scaling — to learn the
Kubernetes Operator pattern hands-on rather than treat it as a black box, and to demonstrate that
understanding concretely: real code, real tests, real deployment, real load test results.

## License

Apache License 2.0
