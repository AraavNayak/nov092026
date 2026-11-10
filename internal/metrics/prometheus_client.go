// Package metrics queries DCGM GPU metrics from Prometheus on behalf of the
// node-health reconciler.
package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// NodeGPUStats summarizes the DCGM signals the reconciler checks per node.
type NodeGPUStats struct {
	MaxTempCelsius   float64
	ECCDoubleBitErrs float64
	MaxUtilPercent   float64
}

type Client struct {
	api promv1.API
}

func NewClient(prometheusURL string) (*Client, error) {
	c, err := api.NewClient(api.Config{Address: prometheusURL})
	if err != nil {
		return nil, fmt.Errorf("creating prometheus client: %w", err)
	}
	return &Client{api: promv1.NewAPI(c)}, nil
}

// NodeStats returns the current DCGM signals for the given Kubernetes node
// name, as reported by the DCGM exporter's "Hostname" label.
func (c *Client) NodeStats(ctx context.Context, nodeName string) (NodeGPUStats, error) {
	temp, err := c.scalarQuery(ctx, fmt.Sprintf(`max(DCGM_FI_DEV_GPU_TEMP{Hostname="%s"})`, nodeName))
	if err != nil {
		return NodeGPUStats{}, err
	}
	ecc, err := c.scalarQuery(ctx, fmt.Sprintf(`max(DCGM_FI_DEV_ECC_DBE_VOL_TOTAL{Hostname="%s"})`, nodeName))
	if err != nil {
		return NodeGPUStats{}, err
	}
	util, err := c.scalarQuery(ctx, fmt.Sprintf(`max(DCGM_FI_DEV_GPU_UTIL{Hostname="%s"})`, nodeName))
	if err != nil {
		return NodeGPUStats{}, err
	}
	return NodeGPUStats{MaxTempCelsius: temp, ECCDoubleBitErrs: ecc, MaxUtilPercent: util}, nil
}

func (c *Client) scalarQuery(ctx context.Context, query string) (float64, error) {
	result, _, _, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("querying prometheus (%s): %w", query, err)
	}
	vec, ok := result.(model.Vector)
	if !ok || len(vec) == 0 {
		return 0, nil
	}
	return float64(vec[0].Value), nil
}
