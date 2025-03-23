# Campaign Analytics System

A scalable, fault-tolerant real-time campaign analytics system that processes and aggregates ad performance data from multiple ad platforms (Meta, Google, LinkedIn, TikTok).

## Overview

This service provides a RESTful API for fetching real-time analytics about ad campaign performance. It handles data ingestion from multiple ad platforms, efficiently processes and aggregates metrics, and serves insights with low latency.

## Architecture

The system is composed of two main services:

1. **API Service**: Handles HTTP requests and provides the RESTful API
2. **Worker Service**: Processes events from Kafka and manages data aggregation

Supported by the following infrastructure:

- **ClickHouse**: Time-series database for analytics data
- **PostgreSQL**: Relational database for user and campaign metadata
- **Kafka**: Message queue for event streaming
- **Redis**: Cache for API responses and deduplication

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose (for local development)

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/zocket/campaign-analytics.git
   cd campaign-analytics
   ```

2. Start the services with Docker Compose:
   ```bash
   docker-compose -f deployments/docker/docker-compose.yml up -d
   ```

3. Initialize the database schemas:
   ```bash
   go run cmd/api/main.go --init-schema
   ```

4. Run the services:
   ```bash
   # API service
   go run cmd/api/main.go

   # Worker service (in another terminal)
   go run cmd/worker/main.go
   ```

5. Or use the development script:
   ```bash
   ./scripts/run-dev.sh
   ```

### Building the Application

```bash
./scripts/build.sh
```

The binaries will be available in the `bin` directory.

## API Endpoints

### Authentication

- `POST /api/v1/auth/register`: Register a new user
- `POST /api/v1/auth/login`: Login and get JWT token

### Campaigns

- `GET /api/v1/campaigns`: List campaigns
- `POST /api/v1/campaigns`: Create a new campaign
- `GET /api/v1/campaigns/:id`: Get campaign details
- `PUT /api/v1/campaigns/:id`: Update a campaign

### Analytics

- `GET /api/v1/campaigns/:id/insights`: Get campaign insights with support for:
  - Date range filtering
  - Platform filtering
  - Region filtering
  - Granularity specification (daily, weekly, monthly)

- `POST /api/v1/campaigns/:id/fetch-data`: Trigger data fetch from ad platforms
- `POST /api/v1/campaigns/:id/reaggregate`: Trigger re-aggregation of metrics

### System

- `GET /health`: Health check endpoint
- `GET /metrics`: Prometheus metrics endpoint

## Configuration

Configuration can be provided via:

1. Configuration file (`config/config.yaml`)
2. Environment variables (prefixed with `CA_`)

Key configuration options:

```yaml
# Server settings
server:
  port: 8080

# Database settings
postgres:
  host: localhost
  port: 5432
  database: campaign_analytics
  username: postgres
  password: postgres

clickhouse:
  host: localhost
  port: 9000
  database: campaign_analytics

# Redis settings
redis:
  addr: localhost:6379

# Kafka settings
kafka:
  brokers:
    - localhost:9092

# Authentication
jwt:
  key: your-secret-key-here
```

## Deployment

### Docker

```bash
# Build Docker images
docker build -f deployments/docker/Dockerfile -t campaign-analytics-api --target api .
docker build -f deployments/docker/Dockerfile -t campaign-analytics-worker --target worker .

# Run services
docker run -p 8080:8080 campaign-analytics-api
docker run campaign-analytics-worker
```

### Kubernetes

Kubernetes manifests are available in the `deployments/kubernetes` directory.

```bash
kubectl apply -f deployments/kubernetes/
```

## Monitoring

The system exposes Prometheus metrics at the `/metrics` endpoint. Docker Compose setup includes Prometheus for local monitoring.

## Performance & Scalability

- Horizontal scaling of API and worker services
- Kafka partitioning for parallel processing
- ClickHouse for efficient time-series storage and querying
- Multi-level caching strategy with Redis
