# eks-inference-platform

A production-grade ML inference service deployed on AWS EKS with Kubernetes autoscaling, Prometheus observability, Locust load testing, and GitHub Actions CI/CD. Built as a portfolio project targeting AI infrastructure engineering roles.

**Stack:** Python · FastAPI · AWS EKS · Kubernetes · Docker · Prometheus · Grafana · Helm · GitHub Actions

---

## What this project demonstrates

An end-to-end inference serving platform that mirrors production AI infrastructure patterns at AWS, Google, and Microsoft. The project emphasizes operational concerns — reliability, observability, and automation — over model complexity. The model (distilbert) is intentionally lightweight; the infrastructure is the point.

| Component | Description |
|---|---|
| Inference API | FastAPI endpoint serving HuggingFace distilbert with Prometheus metrics |
| EKS Cluster | Kubernetes on AWS with Helm charts, ALB ingress, and HPA autoscaling |
| Observability | Prometheus + Grafana tracking latency, throughput, and pod count |
| CI/CD | GitHub Actions — git push triggers build, ECR push, and Helm rollout |
| Load Testing | Locust benchmarks with real p50/p95/p99 latency numbers |

---

## Architecture

```
# Request flow
Client → AWS ALB (Application Load Balancer)
       → Kubernetes Ingress
       → FastAPI Pod (x N replicas, HPA-managed)
       → HuggingFace distilbert model
       → JSON response { label, score, latency_ms }

# Autoscaling
Prometheus scrapes /metrics from FastAPI pods
→ HPA watches CPU utilization (threshold: 70%)
→ Scales pods: min 2 → max 10

# Observability pipeline
FastAPI app → prometheus_client (custom Counter + Histogram)
           → Prometheus scrape every 15s via ServiceMonitor
           → Grafana dashboard (request rate, p99 latency, pod count)

# CI/CD pipeline
git push → GitHub Actions runner
         → docker build --platform linux/amd64
         → docker push → AWS ECR
         → helm upgrade → EKS rolling deployment

# AWS services
ECR   → container registry
EKS   → managed Kubernetes control plane
ALB   → application load balancer
IAM   → node role policies for ECR + ALB access
```

---

## Related projects (not part of this repo)

This CPU/distilbert service is one piece of a larger platform built as a learning/portfolio effort.
Two companion pieces extend it, developed and kept separate deliberately (different toolchains,
independent lifecycles — see each for details):

- **`gpu-inference-platform`** (sibling folder, local) — a GPU-backed generative model (vLLM)
  serving alongside this CPU service, with DCGM GPU observability and a real load-tested result.
- **`scaler-controller`** (`gpu-inference-platform/scaler-controller/`, companion project, not yet public) —
  a custom Go Kubernetes operator (CRD + `controller-runtime` reconciler) that autoscales the
  GPU/vLLM deployment based on live Prometheus metrics.

---

## Tech stack

| Category | Tools |
|---|---|
| Languages | Python 3.13, Bash, YAML |
| ML Serving | FastAPI, PyTorch, transformers, pydantic, uvicorn |
| Containerization | Docker (multi-stage), docker-compose |
| Orchestration | Kubernetes, Helm, kubectl, eksctl, HPA |
| AWS Services | EKS, ECR, ALB, IAM |
| Observability | Prometheus, Grafana, prometheus_client, ServiceMonitor |
| Load Testing | Locust |
| CI/CD | GitHub Actions |

---

## Project structure

```
eks-inference-platform/
├── app/
│   ├── main.py               # FastAPI app — /predict /health /metrics
│   ├── model.py              # HuggingFace model loader (singleton pattern)
│   └── metrics.py            # Prometheus Counter + Histogram instrumentation
├── docker/
│   └── Dockerfile            # Multi-stage build (builder + slim runtime)
├── helm/
│   └── inference-platform/
│       ├── Chart.yaml
│       ├── values.yaml       # single source of truth for all config
│       └── templates/
│           ├── deployment.yml
│           ├── service.yml
│           ├── ingress.yml
│           ├── hpa.yml
│           └── servicemonitor.yaml
├── monitoring/
│   └── grafana/              # Dashboard JSON (importable)
├── .github/
│   └── workflows/
│       └── deploy.yml        # GitHub Actions CI/CD pipeline
├── tests/
│   └── locustfile.py         # Locust load test
├── benchmarks/
│   └── results.md            # p50/p95/p99 latency report
├── docs/
│   └── adr/                  # Architecture decision records
│       ├── 001-distilbert-model-choice.md
│       ├── 002-eks-over-ecs.md
│       ├── 003-prometheus-grafana-over-datadog.md
│       └── 004-helm-over-raw-kubectl.md
├── docker-compose.yml        # Local development
├── deploy.sh                 # Full cluster deploy script
└── requirements.txt
```

---

## Benchmark results

2 pods, 2x t3.medium nodes, distilbert CPU inference.

| Users | req/sec | p50 | p95 | p99 | Failures |
|-------|---------|-----|-----|-----|----------|
| 10 | ~6 | 67ms | 130ms | 180ms | 0% |
| 20 | ~12 | 66ms | 140ms | 180ms | 0% |
| 50 | ~28 | 130ms | 420ms | 580ms | 0% |

HPA validated: scaled 2 → 10 pods under 100 concurrent users (187% CPU utilization trigger).

---

## Getting started

### Prerequisites

```bash
brew install awscli kubectl helm eksctl watch
aws configure
```

### Run locally

```bash
git clone https://github.com/boyanglineu-prog/eks-inference-platform
cd eks-inference-platform
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

```bash
curl http://localhost:8000/health
curl -X POST http://localhost:8000/predict \
  -H "Content-Type: application/json" \
  -d '{"text": "This inference service is working."}'
# {"label": "POSITIVE", "score": 0.9998, "latency_ms": 38}
```

### Run with Docker

```bash
docker compose up
```

### Deploy to EKS

```bash
# one-time setup — build and push image to ECR
aws ecr create-repository --repository-name inference-platform --region us-west-2
docker build --platform linux/amd64 -t inference-platform -f docker/Dockerfile .
aws ecr get-login-password --region us-west-2 | docker login --username AWS \
  --password-stdin <account_id>.dkr.ecr.us-west-2.amazonaws.com
docker tag inference-platform:latest \
  <account_id>.dkr.ecr.us-west-2.amazonaws.com/inference-platform:latest
docker push \
  <account_id>.dkr.ecr.us-west-2.amazonaws.com/inference-platform:latest

# deploy — handles cluster + IAM + Helm installs in correct order
# deploy.sh lives one level up (~/CS/EKS_Project/deploy.sh), parallel to this repo,
# since it's an orchestration script, not part of the app itself
cd ..
./deploy.sh
```

Note: to bring up the full platform including the GPU/vLLM path and the custom autoscaling
controller, see `gpu-inference-platform`'s README and the `deploy-gpu-stack.sh` orchestrator script
one level up (in `~/CS/EKS_Project/`), which calls this repo's `deploy.sh` as its first step.

### Access observability

```bash
# Prometheus — http://localhost:9090
kubectl port-forward -n monitoring \
  svc/monitoring-kube-prometheus-prometheus 9090:9090 &

# Grafana — http://localhost:3000
kubectl port-forward -n monitoring svc/monitoring-grafana 3000:80 &

# get Grafana admin password
kubectl --namespace monitoring get secrets monitoring-grafana \
  -o jsonpath="{.data.admin-password}" | base64 -d ; echo
```

### Run load test

```bash
locust -f tests/locustfile.py \
  --host=http://<ALB-endpoint> \
  --headless -u 20 -r 2 --run-time 60s
```

### Cleanup

```bash
# ⚠️ ~$4.40/day while running — always delete when done
eksctl delete cluster --name inference-platform --region us-west-2
```

---

## CI/CD

Every push to `main` triggers GitHub Actions:

```
git push
  → build AMD64 image
  → push to ECR
  → helm upgrade → EKS rolling deploy (zero downtime)
```

AWS credentials stored as GitHub repository secrets (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`).

---

## Target companies

| Company | Role | Key matched keywords |
|---|---|---|
| AWS / Amazon | SDE1 AI Infra | EKS, ECR, IAM, Kubernetes, Docker |
| Microsoft Azure | SWE AI Infra | Kubernetes, Helm, Prometheus, Grafana, CI/CD |
| Google | L3 SWE Infra | Distributed systems, Kubernetes, observability |
| CoreWeave | SWE GPU Infra | Kubernetes, Docker, Prometheus, cloud-native |
| Datadog | SWE I Infra | Prometheus, Grafana, observability, Python |
| Snowflake | SWE IC1 Cloud | AWS, Kubernetes, Docker, cloud-native |

---

## Connection to systems background

Pod scheduling and resource limits connect to process scheduling and memory management from OS kernel work (CS162). Service networking and ingress configuration build on TCP/IP and socket programming foundations (CS168). FastAPI async workers mirror POSIX thread and IPC patterns.

---

## References

- [AWS EKS documentation](https://docs.aws.amazon.com/eks/)
- [HuggingFace transformers](https://huggingface.co/docs/transformers)
- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts)
- [FastAPI documentation](https://fastapi.tiangolo.com)
- [Locust load testing](https://locust.io)
- [eksctl documentation](https://eksctl.io)
- [AWS ALB ingress controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller)

---

## License

MIT
