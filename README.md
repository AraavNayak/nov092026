# GPU Fleet Health Controller

A Kubernetes controller that detects degraded GPU nodes during large
training runs and removes them from service before they stall or crash the
whole job.

The controller watches [NVIDIA DCGM](https://developer.nvidia.com/dcgm)
metrics (scraped by Prometheus) for each GPU node, and when a node crosses a
degradation threshold (GPU over-temp, uncorrectable ECC errors, etc.) it
cordons and taints the node so the scheduler moves workloads to healthy
nodes. Three Kubernetes Jobs provide active health checks (GPU stress,
memory, inter-node NCCL bandwidth) to complement DCGM's passive metrics.

## Repo layout

```
cmd/controller/        main.go entrypoint for the controller binary
internal/controller/   NodeHealthReconciler (cordon/uncordon on DCGM thresholds)
internal/metrics/      Prometheus client used to query DCGM metrics
jobs/                  Kubernetes Job manifests for the three health checks
deploy/prometheus/     helm values for kube-prometheus-stack
deploy/grafana/        GPU fleet dashboard
deploy/controller/     RBAC + Deployment for the controller itself
hack/                  kind config and install scripts
```

## Week 1: local cluster + monitoring stack

Prereqs: `go`, `kind`, `kubectl`, `helm`, `docker` (all installed via
`brew install go kind kubernetes-cli helm` + a static kubectl download if
brew's build breaks on your platform).

```bash
# 1. Bring up a local 3-node kind cluster
kind create cluster --config hack/kind-config.yaml

# 2. Install Prometheus + Grafana + Alertmanager
./hack/install-monitoring.sh

# 3. Load the GPU fleet Grafana dashboard
./hack/install-dashboard.sh

# 4. View it
kubectl -n monitoring port-forward svc/kube-prometheus-stack-grafana 3000:80
# open http://localhost:3000 (admin/admin)
```

kind nodes have no real GPUs, so step 5 below only applies once you rent
actual GPU nodes (see Week 1 plan: 2-3 small GPU nodes, a few hours at a
time).

```bash
# 5. On a cluster with real NVIDIA GPU nodes: install the GPU Operator (DCGM exporter included)
./hack/install-gpu-stack.sh
```

### Note on Apple Silicon / arm64

`hack/install-monitoring.sh` pins `kube-prometheus-stack` to chart version
`92.22.0`. Newer chart versions ship `prometheus-operator:v0.95.1`, which
quay.io currently only publishes as `amd64` — it `ImagePullBackOff`s on
arm64 kind nodes. `v0.94.2` (used by 92.22.0) is multi-arch.

## Running the health-check Jobs

```bash
kubectl apply -f jobs/namespace.yaml

# GPU stress test (DCGM long diagnostic) and memory test target one node at a time:
NODE_NAME=<node> envsubst < jobs/gpu-stress-job.yaml | kubectl apply -f -
NODE_NAME=<node> envsubst < jobs/memory-test-job.yaml | kubectl apply -f -

# NCCL all-reduce bandwidth test runs across two nodes automatically:
kubectl apply -f jobs/nccl-bandwidth-job.yaml
```

## Running the controller locally against the cluster

```bash
go build -o bin/controller ./cmd/controller
kubectl -n monitoring port-forward svc/kube-prometheus-stack-prometheus 9090:9090 &
./bin/controller --prometheus-url=http://localhost:9090
```

Or deploy it in-cluster once an image is built and pushed:

```bash
kubectl apply -f deploy/controller/rbac.yaml
kubectl apply -f deploy/controller/deployment.yaml
```

## Target result

"Detects a degraded GPU node in X seconds and restores training throughput
within Y minutes; without it, the job slows by Z%." X/Y/Z get measured once
real GPU node runs are available — the local kind cluster validates wiring
only, not real degradation signals.
