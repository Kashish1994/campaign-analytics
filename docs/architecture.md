# Campaign Analytics System Architecture

## Overview

This document outlines the architecture for Zocket's campaign analytics system, designed to process and aggregate ad performance data from multiple sources (Meta, Google, LinkedIn, TikTok) in real-time. The system provides insights via a RESTful API with low latency and scales dynamically as data volume grows.

## System Components

### 1. Data Ingestion Layer

The data ingestion layer is responsible for collecting data from various ad platforms in real-time. It includes:

- **Platform Adapters**: Specific implementations for each platform (Meta, Google, LinkedIn, TikTok) to handle their API peculiarities.
- **Webhook Receivers**: HTTP endpoints that receive webhook events from platforms.
- **Kafka Message Queue**: Acts as a buffer for incoming data, ensuring fault tolerance and scalability.
- **Idempotency Processor**: Ensures duplicate events aren't processed multiple times.

### 2. Data Processing & Aggregation Layer

This layer processes the raw data and transforms it into meaningful analytics:

- **Stream Processors**: Consumes messages from Kafka and processes them in real-time.
- **Aggregation Engine**: Computes metrics like CTR, ROAS, CPA, etc.
- **Time-Series Database**: Stores pre-aggregated metrics for efficient querying.
- **Batch Processing**: Handles historical data reprocessing and correction.

### 3. API & Presentation Layer

Exposes the processed data to clients:

- **RESTful API**: Provides endpoints for campaign insights and analytics.
- **Authentication & Authorization**: Secures access to the API.
- **Caching**: Improves response times for frequently accessed data.
- **Rate Limiting**: Protects the system from excessive requests.

## Technology Stack

### Infrastructure
- **Go**: Main programming language
- **Docker & Kubernetes**: Containerization and orchestration
- **Prometheus & Grafana**: Monitoring and alerting
- **ELK Stack**: Logging and tracing

### Data Storage & Messaging
- **Kafka**: Message queue for event streaming
- **ClickHouse**: Time-series database for analytics data
- **Redis**: Caching and rate limiting
- **PostgreSQL**: Metadata and user information

### API & Middleware
- **Gin**: HTTP web framework
- **go-redis**: Redis client
- **kafka-go**: Kafka client
- **sqlx**: Database client
- **JWT**: Authentication

## Data Flow

1. **Ingestion**:
   - Ad platform sends data via API or webhook
   - Data is validated and normalized
   - Events are published to Kafka with deduplication keys

2. **Processing**:
   - Stream processors consume events from Kafka
   - Raw events are transformed into aggregated metrics
   - Metrics are stored in ClickHouse for fast retrieval
   - Cache is updated for hot queries

3. **Serving**:
   - API receives client requests
   - Authentication and authorization are verified
   - Cached results are returned if available
   - Otherwise, queries are executed against the database
   - Results are formatted and returned to the client

## Scalability Considerations

- **Horizontal Scaling**: All components can be scaled independently
- **Partitioning**: Kafka topics are partitioned by campaign ID
- **Sharding**: ClickHouse tables are sharded by time and campaign ID
- **Caching Strategy**: Multi-level caching with time-based invalidation
- **Auto-scaling**: Kubernetes HPA based on CPU/memory metrics

## Fault Tolerance & Reliability

- **At-least-once Delivery**: Kafka ensures messages aren't lost
- **Idempotent Processing**: Prevents duplicate processing
- **Circuit Breakers**: Protect downstream services from cascading failures
- **Retry Mechanisms**: Automatically retry failed operations
- **Dead-letter Queues**: Capture and analyze failed messages

## Performance Optimization

- **Pre-aggregation**: Compute and store common aggregations
- **Materialized Views**: Optimize for common query patterns
- **Indexing Strategy**: Efficient indexes on frequently queried fields
- **Query Optimization**: Use of efficient query patterns
- **Connection Pooling**: Reuse database connections

## Security Considerations

- **JWT Authentication**: Secure API access
- **Role-based Access Control**: Limit access based on user roles
- **Rate Limiting**: Prevent abuse
- **Encryption**: Secure data in transit and at rest
- **Audit Logging**: Track all access and changes

## Deployment Strategy

- **Continuous Integration/Continuous Deployment**: Automated testing and deployment
- **Blue-Green Deployments**: Zero-downtime updates
- **Canary Releases**: Gradual rollout of new features
- **Infrastructure as Code**: Managed via Terraform or similar tools
- **Environment Parity**: Development, staging, and production environments are as similar as possible

## Monitoring & Observability

- **Health Checks**: Regular system status verification
- **Metrics Collection**: Key performance indicators
- **Distributed Tracing**: Track requests across services
- **Log Aggregation**: Centralized logging
- **Alerting**: Notify on-call personnel of issues

## Trade-offs and Considerations

- **Consistency vs. Availability**: System prioritizes availability over strong consistency
- **Processing Latency vs. Throughput**: Optimized for high throughput with acceptable latency
- **Storage Cost vs. Query Performance**: Some data duplication for query efficiency
- **Development Speed vs. System Complexity**: Modular design balances functionality and maintainability
