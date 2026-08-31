# docker/Dockerfile

# Stage 1: builder
FROM python:3.13-slim AS builder
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Stage 2: runtime
FROM python:3.13-slim AS runtime
WORKDIR /app
COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY app/ ./app/
EXPOSE 8000

# once the container is deployed, uvicorn is runned automatically 
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
