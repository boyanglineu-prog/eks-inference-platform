# app/model.py

from transformers import pipeline # type: ignore
import time

class SentimentModel:
    def __init__(self):
        self.model = pipeline("sentiment-analysis", model = "distilbert/distilbert-base-uncased-finetuned-sst-2-english")

    def predict(self, text: str) -> dict:
        start = time.time()
        result = self.model(text)[0]
        latency_ms = round((time.time() - start) * 1000, 2)

        return{
                "label" : result["label"],
                "score" : round(result["score"], 4),
                "latency_ms" : latency_ms
                }

model = SentimentModel()
