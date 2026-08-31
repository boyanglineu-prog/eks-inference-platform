package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrometheusMetricsProvider_GetQueueDepth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{"app": "vllm-inference"},
						"value":  []interface{}{1700000000, "42"},
					},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode fake response: %v", err)
		}
	}))
	defer server.Close()

	provider, err := NewPrometheusMetricsProvider(server.URL)
	if err != nil {
		t.Fatalf("NewPrometheusMetricsProvider() error = %v", err)
	}

	got, err := provider.GetQueueDepth(context.Background(), "vllm-inference")
	if err != nil {
		t.Fatalf("GetQueueDepth() error = %v", err)
	}

	if got != 42 {
		t.Errorf("GetQueueDepth() = %d, want 42", got)
	}
}

func TestPrometheusMetricsProvider_GetGPUUtilization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{},
						"value":  []interface{}{1700000000, "73.5"},
					},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode fake response: %v", err)
		}
	}))
	defer server.Close()

	provider, err := NewPrometheusMetricsProvider(server.URL)
	if err != nil {
		t.Fatalf("NewPrometheusMetricsProvider() error = %v", err)
	}

	got, err := provider.GetGPUUtilization(context.Background())
	if err != nil {
		t.Fatalf("GetGPUUtilization() error = %v", err)
	}

	if got != 73.5 {
		t.Errorf("GetGPUUtilization() = %v, want 73.5", got)
	}
}
