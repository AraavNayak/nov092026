package main

import (
	"flag"
	"os"
	"time"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/araavnayak/gpu-fleet-controller/internal/controller"
	"github.com/araavnayak/gpu-fleet-controller/internal/metrics"
)

func main() {
	var prometheusURL string
	var requeueInterval time.Duration
	var metricsAddr string
	flag.StringVar(&prometheusURL, "prometheus-url", "http://kube-prometheus-stack-prometheus.monitoring:9090", "Prometheus base URL to query DCGM metrics from")
	flag.DurationVar(&requeueInterval, "requeue-interval", 30*time.Second, "how often to re-check each GPU node's health")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "address the controller's own metrics endpoint binds to")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  clientgoscheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: metricsAddr},
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	promClient, err := metrics.NewClient(prometheusURL)
	if err != nil {
		ctrl.Log.Error(err, "unable to create prometheus client")
		os.Exit(1)
	}

	reconciler := &controller.NodeHealthReconciler{
		Client:          mgr.GetClient(),
		Metrics:         promClient,
		RequeueInterval: requeueInterval,
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to set up NodeHealthReconciler")
		os.Exit(1)
	}

	ctrl.Log.Info("starting gpu-fleet-controller", "prometheusURL", prometheusURL, "requeueInterval", requeueInterval)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
