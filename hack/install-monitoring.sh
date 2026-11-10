#!/usr/bin/env bash
# Installs Prometheus + Grafana + Alertmanager (kube-prometheus-stack) into
# the "monitoring" namespace. Safe to re-run (helm upgrade --install).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null
helm repo update >/dev/null

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

# Pinned: chart versions >=93.0.0 ship prometheus-operator v0.95.1, which quay.io
# only publishes as amd64 -- breaks on arm64 kind nodes (Apple Silicon).
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --version 92.22.0 \
  -f "${ROOT_DIR}/deploy/prometheus/values.yaml" \
  --wait --timeout 5m

echo
echo "Grafana:    kubectl -n monitoring port-forward svc/kube-prometheus-stack-grafana 3000:80"
echo "Prometheus: kubectl -n monitoring port-forward svc/kube-prometheus-stack-prometheus 9090:9090"
echo "Grafana admin user/pass: admin/admin (see deploy/prometheus/values.yaml)"
