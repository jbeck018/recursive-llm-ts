# Getting Started with Acme Analytics

Welcome to Acme Analytics! This guide walks you through setting up your first dashboard, ingesting events, and running your first query.

## Quick Start

### 1. Create an Account

Sign up at [app.acme-analytics.com](https://app.acme-analytics.com). You'll receive a verification email within 30 seconds.

### 2. Create a Project

After signing in, click **New Project** and provide:
- **Project name**: A descriptive name for your application
- **Environment**: `production`, `staging`, or `development`
- **Data region**: US East, US West, EU West, or Asia Pacific

### 3. Get Your API Key

Navigate to **Settings > API Keys** and click **Generate Key**. You'll get:
- **API Key**: `ak_live_xxxxxxxx` (for event ingestion)
- **Client ID / Secret**: For OAuth token generation (dashboard API access)

> **Important**: Store your API key securely. It cannot be displayed again after creation.

### 4. Install the SDK

Choose your platform:

```bash
# JavaScript/TypeScript
npm install @acme/analytics-sdk

# Python
pip install acme-analytics

# Go
go get github.com/acme/analytics-go
```

### 5. Send Your First Event

```typescript
import { AcmeAnalytics } from '@acme/analytics-sdk';

const analytics = new AcmeAnalytics({
  apiKey: process.env.ACME_API_KEY,
});

await analytics.track({
  event: 'page_view',
  userId: 'user_123',
  properties: {
    page: '/home',
    referrer: 'https://google.com',
  },
});
```

### 6. View in Dashboard

Events appear in your dashboard within 2 seconds. Navigate to **Live View** to see events streaming in real-time.

## Key Concepts

### Events
An event represents a single user action. Events have:
- **event_type**: What happened (e.g., `page_view`, `purchase`, `sign_up`)
- **user_id**: Who did it
- **timestamp**: When it happened
- **properties**: Additional context (any JSON object)

### Metrics
Metrics are aggregated calculations over events:
- **Count**: Total number of events
- **Unique Count**: Distinct users/sessions
- **Sum/Average**: Over numeric properties
- **Percentiles**: P50, P90, P95, P99 of numeric properties

### Funnels
Funnels track user progression through a sequence of steps. Define the steps, and Acme computes conversion rates at each stage.

### Cohorts
Group users by shared characteristics (signup date, plan type, geography) and compare behavior across groups.

## Best Practices

1. **Use consistent event names**: `page_view` not `PageView` or `page-view`
2. **Include user_id on every event**: Required for sessionization and funnel analysis
3. **Keep properties flat**: Avoid deeply nested objects for better query performance
4. **Use ISO 8601 timestamps**: Always include timezone (`Z` for UTC)
5. **Batch events**: Send events in batches of 50-100 for optimal throughput
6. **Handle failures**: Implement retry logic with exponential backoff

## Next Steps

- [API Reference](./api-reference.md) - Complete endpoint documentation
- [Architecture Overview](./architecture.md) - How the platform works
- [Dashboard Guide](#) - Build your first dashboard
- [Alerting Setup](#) - Configure monitoring and alerts
