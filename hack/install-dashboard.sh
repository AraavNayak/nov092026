#!/usr/bin/env bash
# Loads deploy/grafana/dashboards/gpu-fleet.json into Grafana via the
# kube-prometheus-stack dashboard sidecar (watches ConfigMaps labeled
# grafana_dashboard=1 in the monitoring namespace).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

kubectl -n monitoring create configmap gpu-fleet-dashboard \
  --from-file=gpu-fleet.json="${ROOT_DIR}/deploy/grafana/dashboards/gpu-fleet.json" \
  --dry-run=client -o yaml | \
  kubectl label -f - --local -o yaml grafana_dashboard=1 | \
  kubectl apply -f -

echo "Dashboard ConfigMap applied. Grafana sidecar will pick it up within ~30s."
