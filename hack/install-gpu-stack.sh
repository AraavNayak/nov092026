#!/usr/bin/env bash
# Installs the NVIDIA GPU Operator (which bundles the DCGM exporter) into the
# "gpu-operator" namespace. Only run this against a cluster with real NVIDIA
# GPU nodes -- it requires nodes with NVIDIA drivers/toolkit installable
# (bare metal or a cloud GPU instance type), not a local kind cluster.
set -euo pipefail

helm repo add nvidia https://helm.ngc.nvidia.com/nvidia >/dev/null
helm repo update >/dev/null

kubectl create namespace gpu-operator --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install gpu-operator nvidia/gpu-operator \
  --namespace gpu-operator \
  --set dcgmExporter.serviceMonitor.enabled=true \
  --set dcgmExporter.serviceMonitor.additionalLabels.release=kube-prometheus-stack \
  --wait --timeout 10m

echo
echo "DCGM metrics will appear in Prometheus once GPU nodes are Ready."
echo "Verify with: kubectl -n gpu-operator get pods -l app=nvidia-dcgm-exporter"
