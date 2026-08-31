# app/main.py

from fastapi import FastAPI
from pydantic import BaseModel
from prometheus_client import generate_latest, CONTENT_TYPE_LATEST # type: ignore
from fastapi.responses import Response
from app.model import model
from app.metrics import REQUEST_COUNT, REQUEST_LATENCY

app = FastAPI()

class TextInput(BaseModel):
    text: str

@app.get("/health")
def health():
    return {"status": "ok"}

@app.post("/predict")
def predict(input: TextInput):
    result = model.predict(input.text)
    REQUEST_COUNT.labels(label = result["label"]).inc()
    REQUEST_LATENCY.observe(result["latency_ms"])
    return result

@app.get("/metrics")
def metrics():
    return Response(generate_latest(), media_type = CONTENT_TYPE_LATEST)
    

