# tests/locustfile.py

from locust import HttpUser, task, between

class InferenceUser(HttpUser):
    wait_time = between(1, 2)  # wait 1-2 seconds between requests

    @task(3)
    def predict(self):
        self.client.post("/predict",
            json={"text": "this product is absolutely amazing"},
            headers={"Content-Type": "application/json"}
        )

    @task(1)
    def health(self):
        self.client.get("/health")
