#!/bin/bash
# deploy-gpu-stack.sh
# Recreates the base cluster + full GPU/vLLM/observability/controller stack,
# in order, from scratch. Run from ~/CS/EKS_Project/ (this script's own location),
# parallel to the cpu-inference-platform/, gpu-inference-platform/, and
# gpu-inference-platform/scaler-controller/ sibling folders it orchestrates.
#
# Assumes:
#   - gpu-inference-platform/scaler-controller has the controller image already built and
#     pushed to ECR (no code changes since 2026-08-23 -- if you changed Go code,
#     rebuild/push before running step 7).
#   - AWS account 454518197798, region us-west-2 (matches this project's existing
#     ECR repos / IAM policies).
#
# Cost note: 2x g4dn.xlarge GPU nodes ~ $1.06/hr on top of the base cluster's
# ~$0.18/hr, for as long as this stays up. Delete when done (see Cleanup in
# gpu-inference-platform/README.md).

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GPU_STACK_DIR="${SCRIPT_DIR}/gpu-inference-platform"
SCALER_CONTROLLER_DIR="${SCRIPT_DIR}/gpu-inference-platform/scaler-controller"
ECR_ACCOUNT="454518197798.dkr.ecr.us-west-2.amazonaws.com"

echo "=== 1. Base cluster (control plane + CPU nodegroup + ALB + monitoring + existing app) ==="
"${SCRIPT_DIR}/deploy.sh"

echo "=== 2. GPU nodegroup (2x g4dn.xlarge) ==="
eksctl create nodegroup \
  --cluster inference-platform \
  --region us-west-2 \
  --name gpu-nodes \
  --node-type g4dn.xlarge \
  --nodes 2 \
  --nodes-min 2 \
  --nodes-max 2

echo "=== 3-5. vLLM Deployment/Service + ServiceMonitor + DCGM exporter (one kustomization) ==="
kubectl apply -k "${GPU_STACK_DIR}/manifests"

echo "=== 6. Waiting for vLLM pod to become ready (model download + load, can take a few minutes) ==="
kubectl wait --for=condition=Ready pod -l app=vllm-inference --timeout=600s

echo "=== 7. Deploy the scaler-controller (CRD + RBAC + manager) ==="
echo "    (assumes image already built/pushed -- rebuild first if Go code changed since 2026-08-23)"
(cd "$SCALER_CONTROLLER_DIR" && make install)
(cd "$SCALER_CONTROLLER_DIR" && make deploy IMG="${ECR_ACCOUNT}/scaler-controller:latest")

echo "=== 8. Wait for controller manager to be ready ==="
kubectl wait --for=condition=Ready pod -n scaler-controller-system -l control-plane=controller-manager --timeout=120s

echo "=== 9. Create the ScalingPolicy instance (maxReplicas: 2, matching the 2 GPU nodes) ==="
kubectl apply -f "${GPU_STACK_DIR}/manifests/vllm-scalingpolicy.yaml"

echo ""
echo "=== Done. Verify with: ==="
echo "  kubectl get nodes"
echo "  kubectl get pods -A"
echo "  kubectl get scalingpolicy vllm-scaler -o yaml"
echo "  kubectl logs -n scaler-controller-system -l control-plane=controller-manager --tail=30"
echo ""
echo "Ready for the load test once vLLM and the controller both show healthy."
