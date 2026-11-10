// Package controller reconciles Kubernetes Nodes against live DCGM GPU
// metrics and cordons any node that crosses a degradation threshold so the
// scheduler stops placing new work there.
package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/araavnayak/gpu-fleet-controller/internal/metrics"
)

const (
	// Thresholds are conservative Week-1 defaults; tune once real GPU node
	// data from the rented cluster is available.
	maxGPUTempCelsius    = 85.0
	maxECCDoubleBitErrs  = 0
	degradedNodeTaintKey = "gpu-fleet-health/degraded"
)

// NodeHealthReconciler watches Nodes that run GPU workloads and cordons any
// node whose DCGM metrics (temperature, ECC errors) cross a degradation
// threshold, so the scheduler stops placing new pods there.
type NodeHealthReconciler struct {
	client.Client
	Metrics *metrics.Client
	// RequeueInterval controls how often a healthy node is re-checked.
	RequeueInterval time.Duration
}

func (r *NodeHealthReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var node corev1.Node
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, ok := node.Labels["nvidia.com/gpu.present"]; !ok {
		// Not a GPU node; nothing for this reconciler to do.
		return ctrl.Result{}, nil
	}

	stats, err := r.Metrics.NodeStats(ctx, node.Name)
	if err != nil {
		logger.Error(err, "failed to fetch DCGM stats", "node", node.Name)
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}

	degraded := stats.MaxTempCelsius > maxGPUTempCelsius || stats.ECCDoubleBitErrs > maxECCDoubleBitErrs

	switch {
	case degraded && !node.Spec.Unschedulable:
		logger.Info("degraded node detected, cordoning", "node", node.Name,
			"tempC", stats.MaxTempCelsius, "eccDoubleBitErrs", stats.ECCDoubleBitErrs)
		if err := r.cordon(ctx, &node); err != nil {
			return ctrl.Result{}, err
		}
	case !degraded && node.Spec.Unschedulable && hasDegradedTaint(&node):
		logger.Info("node recovered, uncordoning", "node", node.Name)
		if err := r.uncordon(ctx, &node); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

func (r *NodeHealthReconciler) cordon(ctx context.Context, node *corev1.Node) error {
	patch := client.MergeFrom(node.DeepCopy())
	node.Spec.Unschedulable = true
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    degradedNodeTaintKey,
		Value:  "true",
		Effect: corev1.TaintEffectNoSchedule,
	})
	return r.Patch(ctx, node, patch)
}

func (r *NodeHealthReconciler) uncordon(ctx context.Context, node *corev1.Node) error {
	patch := client.MergeFrom(node.DeepCopy())
	node.Spec.Unschedulable = false
	taints := make([]corev1.Taint, 0, len(node.Spec.Taints))
	for _, t := range node.Spec.Taints {
		if t.Key != degradedNodeTaintKey {
			taints = append(taints, t)
		}
	}
	node.Spec.Taints = taints
	return r.Patch(ctx, node, patch)
}

func hasDegradedTaint(node *corev1.Node) bool {
	for _, t := range node.Spec.Taints {
		if t.Key == degradedNodeTaintKey {
			return true
		}
	}
	return false
}

func (r *NodeHealthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
