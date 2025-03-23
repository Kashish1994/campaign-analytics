#!/bin/bash

# Function to check if a command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Check if required tools are installed
if ! command_exists go; then
  echo "Error: Go is not installed. Please install Go first."
  exit 1
fi

if ! command_exists docker-compose; then
  echo "Error: docker-compose is not installed. Please install Docker and docker-compose first."
  exit 1
fi

# Start infrastructure using docker-compose
echo "Starting infrastructure (PostgreSQL, ClickHouse, Redis, Kafka)..."
cd deployments/docker && docker-compose up -d postgres clickhouse redis kafka
cd ../..

# Wait for databases to be ready
echo "Waiting for databases to be ready..."
sleep 10

# Initialize database schema
echo "Initializing database schema..."
go run cmd/api/main.go --init-schema

# Run the API in the background
echo "Starting API service..."
go run cmd/api/main.go &
API_PID=$!

# Run the worker in the background
echo "Starting worker service..."
go run cmd/worker/main.go &
WORKER_PID=$!

# Function to cleanup on exit
cleanup() {
  echo "Shutting down services..."
  kill $API_PID
  kill $WORKER_PID
  cd deployments/docker && docker-compose down
  exit 0
}

# Setup signal handling
trap cleanup SIGINT SIGTERM

echo "Development environment is running..."
echo "- API service is available at http://localhost:8080"
echo "- PostgreSQL is available at localhost:5432"
echo "- ClickHouse is available at localhost:9000"
echo "- Redis is available at localhost:6379"
echo "- Kafka is available at localhost:9092"
echo "- Prometheus is available at http://localhost:9090"
echo ""
echo "Press Ctrl+C to stop all services."

# Wait for signal
wait
