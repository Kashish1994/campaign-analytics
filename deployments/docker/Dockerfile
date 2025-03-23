# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk --no-cache add ca-certificates tzdata git

# Copy go.mod and go.sum files
COPY go.mod ./
COPY go.sum ./

# Download dependencies (may be cached)
RUN go mod download

# Copy the source code
COPY . .

# Build the API service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/api ./cmd/api
# Build the worker service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/worker ./cmd/worker

# Final stage for API
FROM alpine:3.18 AS api

# Add certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/api .
# Copy configuration
COPY config/config.yaml ./config/

# Set environment variables
ENV GIN_MODE=release

# Expose the API port
EXPOSE 8080

# Run the API service
ENTRYPOINT ["./api"]

# Final stage for Worker
FROM alpine:3.18 AS worker

# Add certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/worker .
# Copy configuration
COPY config/config.yaml ./config/

# Run the worker service
ENTRYPOINT ["./worker"]
