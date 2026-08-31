#!/bin/bash
# shebang — tells OS to use bash
# Run from ~/CS/EKS_Project/ (this script's own location) — deploys the base
# cluster + CPU/distilbert app, whose source/Helm chart live in the sibling
# cpu-inference-platform/ folder.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 1. create cluster if it doesn't exist (15-20 min)
if ! eksctl get cluster --name inference-platform --region us-west-2 2>/dev/null; then
    eksctl create cluster \
        --name inference-platform \
        --region us-west-2 \
        --nodegroup-name standard \
        --node-type t3.medium \
        --nodes 2
fi

# 2. get node role name — changes every time cluster is recreated
ROLE=$(aws eks describe-nodegroup \
    --cluster-name inference-platform \
    --nodegroup-name standard \
    --region us-west-2 \
    --query 'nodegroup.nodeRole' \
    --output text | awk -F'/' '{print $2}')

# 3. attach IAM policies to node role
aws iam attach-role-policy \
    --role-name $ROLE \
    --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
aws iam attach-role-policy \
    --role-name $ROLE \
    --policy-arn arn:aws:iam::454518197798:policy/AWSLoadBalancerControllerIAMPolicy

# 4. refresh helm chart index
helm repo update

# 5. install ALB controller first — app ingress depends on it
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
    --namespace kube-system \
    --set clusterName=inference-platform \
    --set serviceAccount.create=true

# 6. install monitoring — app servicemonitor depends on Prometheus CRDs
helm install monitoring prometheus-community/kube-prometheus-stack \
    --namespace monitoring \
    --create-namespace

# 7. wait for ALB and Prometheus to be ready
echo "Waiting for ALB and Prometheus to be ready..."
sleep 60

# 8. install app last — depends on ALB + Prometheus CRDs
helm install inference-platform "${SCRIPT_DIR}/cpu-inference-platform/helm/inference-platform"

echo "done — check: kubectl get pods -A"
