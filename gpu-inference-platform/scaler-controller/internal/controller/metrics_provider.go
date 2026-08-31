package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusMetricsProvider queries a live Prometheus server for vLLM queue-depth
// metrics (vllm:num_requests_waiting), filtered by the "service" label Prometheus
// attaches via ServiceMonitor scraping -- verified against a real vLLM deployment
// on 2026-08-23.

type PrometheusMetricsProvider struct {
	api promv1.API
}

func NewPrometheusMetricsProvider(address string) (*PrometheusMetricsProvider, error) {
	client, err := api.NewClient(api.Config{Address: address})
	if err != nil {
		return nil, fmt.Errorf("creating prometheus clint: %w", err)
	}
	return &PrometheusMetricsProvider{api: promv1.NewAPI(client)}, nil
}

func (p *PrometheusMetricsProvider) GetQueueDepth(ctx context.Context, targetDeployment string) (int32, error) {
	// "service" is a label Prometheus auto-attaches during scraping (via the vLLM
	// ServiceMonitor's target discovery), not something vLLM itself exposes --
	// verified against a real cluster on 2026-08-23. Assumes the Service name
	// matches the target Deployment's name (true in this project's convention).
	query := fmt.Sprintf("sum(vllm:num_requests_waiting{service=%q})", targetDeployment)

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("querying prometheus: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no data returned for query %q", query)
	}

	return int32(vector[0].Value), nil
}

// GetGPUUtilization returns fleet-average GPU utilization (0-100) across every
// GPU DCGM is exporting. Deliberately not scoped to a single Deployment -- DCGM
// reports per physical device, with no concept of which Kubernetes workload is
// responsible for the load, unlike vllm:num_requests_waiting which vLLM reports
// per-instance itself.
func (p *PrometheusMetricsProvider) GetGPUUtilization(ctx context.Context) (float64, error) {
	query := "avg(DCGM_FI_DEV_GPU_UTIL)"

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("querying prometheus: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		return 0, fmt.Errorf("no data returned for query %q", query)
	}

	return float64(vector[0].Value), nil
}
