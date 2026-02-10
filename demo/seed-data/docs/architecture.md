# Acme Analytics Platform - Architecture Overview

## System Design

The Acme Analytics Platform is a distributed data processing system designed to handle real-time event ingestion, batch analytics, and interactive dashboards for enterprise customers.

### Core Components

#### 1. Event Ingestion Service
The ingestion layer receives events from client SDKs via HTTP/2 and gRPC endpoints. Events are validated against per-customer schemas, enriched with geolocation and device metadata, and published to Apache Kafka topics partitioned by customer ID.

- **Throughput**: 2.4M events/second at peak
- **Latency**: P99 ingestion-to-Kafka < 12ms
- **Validation**: JSON Schema validation with customer-specific rulesets
- **Rate limiting**: Token bucket per API key, 10K events/sec default

#### 2. Stream Processing Layer
Apache Flink jobs consume from Kafka and perform:
- **Real-time aggregations**: 1-minute, 5-minute, and 1-hour tumbling windows
- **Sessionization**: Group events into user sessions with 30-minute inactivity timeout
- **Anomaly detection**: Z-score based alerting on metric deviations > 3 sigma
- **Funnel computation**: Pre-computed conversion funnels updated every minute

The Flink cluster runs on Kubernetes with auto-scaling based on consumer lag.

#### 3. Storage Layer
Three storage tiers optimize for different query patterns:

| Tier | Technology | Use Case | Retention |
|------|-----------|----------|-----------|
| Hot | ClickHouse | Last 24h, real-time dashboards | 7 days |
| Warm | Apache Druid | Last 90 days, interactive queries | 90 days |
| Cold | Parquet on S3 | Historical, batch analytics | 7 years |

#### 4. Query Engine
A federated query engine routes queries to the appropriate storage tier:
- Sub-second queries hit ClickHouse directly
- Time-range queries spanning tiers use a scatter-gather pattern
- Complex analytical queries are compiled to Trino SQL and executed on the warm tier
- Export queries stream directly from S3 Parquet files

#### 5. Dashboard Service
React-based dashboard application with:
- WebSocket-powered real-time updates (< 2 second data freshness)
- Drag-and-drop chart builder with 15 visualization types
- Alerting rules with PagerDuty, Slack, and webhook integrations
- Role-based access control (RBAC) with SSO via SAML 2.0 and OIDC

### Infrastructure

All services run on AWS EKS (Kubernetes 1.29) across three availability zones in us-east-1. Infrastructure is managed with Terraform and deployed via ArgoCD GitOps workflows.

**Key infrastructure decisions:**
- Kafka on Amazon MSK (managed) to reduce operational burden
- ClickHouse on dedicated i3en instances for NVMe storage performance
- S3 Intelligent-Tiering for cold storage cost optimization
- CloudFront CDN for dashboard static assets

### API Design

The public API follows REST conventions with GraphQL available for complex queries:

```
POST /v2/events          # Ingest events (batch)
POST /v2/events/single   # Ingest single event
GET  /v2/query           # Execute analytics query
POST /v2/query/async     # Async query (returns job ID)
GET  /v2/dashboards      # List dashboards
POST /v2/alerts          # Create alert rule
GET  /v2/funnels/:id     # Get funnel results
```

Authentication uses short-lived JWT tokens issued via OAuth 2.0 client credentials flow. API keys are used for server-to-server event ingestion.

### Observability Stack

- **Metrics**: Prometheus + Grafana (custom dashboards for each service)
- **Logging**: Structured JSON logs → Fluent Bit → OpenSearch
- **Tracing**: OpenTelemetry → Jaeger (sampled at 1% in production)
- **Alerting**: Prometheus AlertManager → PagerDuty rotation
- **SLOs**: 99.95% availability, P99 query latency < 500ms

### Deployment Pipeline

1. Developer pushes to feature branch
2. CI runs: lint, unit tests, integration tests (LocalStack for AWS mocks)
3. PR review with mandatory approval from code owners
4. Merge to main triggers staging deployment via ArgoCD
5. Canary deployment: 5% traffic for 30 minutes with automated rollback
6. Full production rollout across all three AZs
7. Post-deploy smoke tests validate critical paths
