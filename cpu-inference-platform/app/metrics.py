# app/metrics.py

from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST # type: ignore

# counts total requests
REQUEST_COUNT = Counter("inference_requests_total", "Total number of inference requests", ["label"])

# track latency distribution
REQUEST_LATENCY = Histogram("inference_latency_ms", "Inference latency in milliseconds", buckets = [10, 25, 50, 100, 250, 500, 1000])
