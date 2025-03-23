#!/bin/bash

# Ensure the script is executable
chmod +x ./scripts/build.sh

# Function to check if a command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Check if required tools are installed
if ! command_exists go; then
  echo "Error: Go is not installed. Please install Go first."
  exit 1
fi

# Set build variables
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X github.com/zocket/campaign-analytics/internal/version.Version=${VERSION} -X github.com/zocket/campaign-analytics/internal/version.Commit=${COMMIT} -X github.com/zocket/campaign-analytics/internal/version.BuildTime=${BUILD_TIME}"

# Create bin directory if it doesn't exist
mkdir -p bin

echo "Building API service..."
CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o bin/api ./cmd/api

echo "Building Worker service..."
CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o bin/worker ./cmd/worker

echo "Build completed successfully."
echo "Binaries are available in the 'bin' directory:"
echo "  - bin/api"
echo "  - bin/worker"
