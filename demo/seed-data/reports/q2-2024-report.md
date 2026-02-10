# Acme Analytics - Q2 2024 Quarterly Report

## Executive Summary

Q2 2024 was a strong quarter with continued growth across all key metrics. Total ARR reached $42.8M (up 28% YoY), driven by enterprise expansion and strong net revenue retention.

## Financial Highlights

| Metric | Q2 2024 | Q1 2024 | YoY Change |
|--------|---------|---------|------------|
| ARR | $42.8M | $38.5M | +28% |
| New ARR | $4.3M | $3.8M | +32% |
| Net Revenue Retention | 118% | 115% | +5pp |
| Gross Margin | 76.2% | 75.1% | +2.1pp |
| Customers > $100K ARR | 47 | 41 | +42% |
| Total Customers | 1,247 | 1,156 | +31% |

## Product Highlights

### Event Ingestion Service
- Peak throughput reached 4.2M events/minute during a customer's Black Friday equivalent event
- P99 latency improved from 15ms to 12ms after Kafka broker upgrade to io2 volumes
- New batch ingestion endpoint launched, supporting up to 1,000 events per request
- Go SDK reached feature parity with TypeScript and Python SDKs

### Dashboard & Query Engine
- Query performance improved 2.3x for time-range queries spanning multiple storage tiers
- Launched 4 new visualization types: Sankey diagrams, treemaps, radar charts, and geographic heatmaps
- WebSocket real-time updates now refresh in < 1.5 seconds (down from 3 seconds)
- Role-based access control (RBAC) now supports custom roles with granular permissions

### Platform Reliability
- Achieved 99.97% availability (target: 99.95%)
- 3 incidents in the quarter: 1 critical, 1 high, 1 medium
- Mean time to recovery (MTTR) improved from 142 minutes to 100 minutes
- Zero data loss events across all incidents

## Customer Wins

- **TechFlow Inc.** expanded from Team to Enterprise plan (+$180K ARR)
- **FinTrack Pro** signed 3-year commitment at $500K/year
- **Streamline SaaS** case study published, showcasing 18% activation rate improvement
- **ScaleUp Technologies** renewed with 40% expansion after Black Friday performance

## Engineering Investments

- Hired 12 engineers (4 backend, 3 platform, 3 frontend, 2 data)
- Began development on dbt integration for metrics-as-code
- Kubernetes cluster upgraded to 1.29 with improved pod autoscaling
- Terraform modules refactored for multi-region deployment readiness

## Risks & Challenges

1. **EU Data Residency**: Increasing regulatory pressure. Plan to launch Frankfurt region in Q3.
2. **ClickHouse Scaling**: Current cluster approaching 80% capacity. Horizontal scaling planned for August.
3. **Competitive Pressure**: Amplitude launched similar funnel features. Need to accelerate cohort analysis roadmap.
4. **Hiring**: Platform engineering positions taking 45+ days to fill.

## Q3 2024 Priorities

1. Launch EU-West (Frankfurt) data region
2. Ship dbt integration beta for metrics-as-code
3. Implement query priority queuing (post-incident action item)
4. Expand to 4 new enterprise logos in financial services vertical
5. Release mobile SDK v2 with offline event batching
