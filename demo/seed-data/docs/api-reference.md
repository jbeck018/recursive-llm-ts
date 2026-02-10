# Acme Analytics Platform - API Reference

## Authentication

All API requests require authentication. Two methods are supported:

### Bearer Token (Dashboards, Queries)
```
Authorization: Bearer <jwt_token>
```
Tokens are obtained via the OAuth 2.0 client credentials flow at `POST /oauth/token`.

### API Key (Event Ingestion)
```
X-API-Key: ak_live_xxxxxxxxxxxxxxxx
```
API keys are created in the dashboard under Settings > API Keys.

---

## Endpoints

### POST /v2/events

Ingest a batch of events.

**Request Body:**
```json
{
  "events": [
    {
      "event_type": "page_view",
      "timestamp": "2024-01-15T10:30:00Z",
      "user_id": "usr_abc123",
      "properties": {
        "page": "/pricing",
        "referrer": "https://google.com",
        "duration_ms": 4500
      }
    }
  ]
}
```

**Response (200):**
```json
{
  "accepted": 1,
  "rejected": 0,
  "errors": []
}
```

**Rate Limits:**
- Default: 10,000 events/second per API key
- Enterprise: 100,000 events/second per API key
- Burst: 2x sustained rate for 10 seconds

### GET /v2/query

Execute a synchronous analytics query.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `metric` | string | Yes | Metric to query (e.g., `page_views`, `unique_users`) |
| `start` | ISO 8601 | Yes | Query start time |
| `end` | ISO 8601 | Yes | Query end time |
| `interval` | string | No | Aggregation interval (`1m`, `5m`, `1h`, `1d`) |
| `filters` | JSON | No | Filter conditions |
| `group_by` | string[] | No | Dimensions to group by |

**Response (200):**
```json
{
  "data": [
    {
      "timestamp": "2024-01-15T10:00:00Z",
      "page_views": 15234,
      "unique_users": 8901
    }
  ],
  "meta": {
    "query_time_ms": 45,
    "rows_scanned": 1250000,
    "cache_hit": true
  }
}
```

### POST /v2/query/async

Execute an asynchronous query for large result sets.

**Request Body:**
```json
{
  "query": "SELECT user_id, count(*) as events FROM events WHERE timestamp > '2024-01-01' GROUP BY user_id ORDER BY events DESC LIMIT 10000",
  "format": "csv",
  "notify_webhook": "https://api.example.com/webhook/query-complete"
}
```

**Response (202):**
```json
{
  "job_id": "job_xxxxxxxx",
  "status": "queued",
  "estimated_completion": "2024-01-15T10:35:00Z"
}
```

### GET /v2/funnels/:id

Get pre-computed funnel results.

**Response (200):**
```json
{
  "funnel_id": "fnl_signup_flow",
  "steps": [
    { "name": "Landing Page", "users": 50000, "conversion_rate": 1.0 },
    { "name": "Sign Up Form", "users": 12500, "conversion_rate": 0.25 },
    { "name": "Email Verified", "users": 8750, "conversion_rate": 0.70 },
    { "name": "First Event", "users": 6125, "conversion_rate": 0.70 }
  ],
  "overall_conversion": 0.1225,
  "period": "2024-01-01 to 2024-01-31"
}
```

### POST /v2/alerts

Create an alerting rule.

**Request Body:**
```json
{
  "name": "High Error Rate",
  "metric": "error_count",
  "condition": {
    "operator": "greater_than",
    "threshold": 100,
    "window": "5m"
  },
  "channels": ["pagerduty:team-backend", "slack:#alerts"],
  "severity": "critical"
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_EVENT` | 400 | Event failed schema validation |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `QUERY_TIMEOUT` | 408 | Query exceeded 30s timeout |
| `INSUFFICIENT_PERMISSIONS` | 403 | Missing required RBAC role |
| `RESOURCE_NOT_FOUND` | 404 | Requested resource does not exist |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

## SDKs

Official SDKs available for:
- **JavaScript/TypeScript**: `npm install @acme/analytics-sdk`
- **Python**: `pip install acme-analytics`
- **Go**: `go get github.com/acme/analytics-go`
- **Ruby**: `gem install acme-analytics`
- **Java**: Maven artifact `com.acme:analytics-sdk`
