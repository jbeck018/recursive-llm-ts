# recursive-llm-ts Demo

Interactive demos showing file-based context processing with local files and S3 (via LocalStack).

## Prerequisites

- **Node.js 16+** and **Go 1.25+** (for building the Go binary)
- **Docker** (for LocalStack S3)
- **`OPENAI_API_KEY`** environment variable set

## Quick Start

```bash
# 1. Set your API key
export OPENAI_API_KEY="sk-..."

# 2. Start LocalStack (S3)
docker compose -f demo/docker-compose.yml up -d

# 3. Wait for the bucket to be seeded (takes ~5 seconds)
docker logs rlm-demo-localstack 2>&1 | tail -20

# 4. Run all demos
npx ts-node demo/run-demo.ts all
```

## Available Demos

### Local File Context
Process the seed data directly from the local filesystem:
```bash
npx ts-node demo/run-demo.ts local
```

### S3 File Context (LocalStack)
Process the same data from a LocalStack S3 bucket:
```bash
npx ts-node demo/run-demo.ts s3
```

### Structured Extraction from S3
Extract typed data from S3 files using a Zod schema:
```bash
npx ts-node demo/run-demo.ts structured
```

## Seed Data

The `seed-data/` directory contains synthetic data for a fictional "Acme Analytics Platform":

| Path | Format | Description |
|------|--------|-------------|
| `docs/architecture.md` | Markdown | System architecture overview |
| `docs/api-reference.md` | Markdown | REST API documentation |
| `docs/getting-started.md` | Markdown | Onboarding guide |
| `data/product-reviews.json` | JSON | 10 product reviews with ratings and tags |
| `data/quarterly-metrics.csv` | CSV | 6 months of revenue/customer metrics by region |
| `data/incident-reports.json` | JSON | 3 production incident post-mortems |
| `reports/q2-2024-report.md` | Markdown | Quarterly business report |
| `code/analytics-service.ts` | TypeScript | Event processing service code |

## LocalStack Details

- **Endpoint**: `http://localhost:4566`
- **Bucket**: `rlm-demo-docs`
- **Region**: `us-east-1`
- **Credentials**: `accessKeyId: 'test'`, `secretAccessKey: 'test'`

The bucket is automatically created and seeded when LocalStack starts via the `init-localstack.sh` script.

### Verify the bucket manually:
```bash
# List bucket contents
aws --endpoint-url=http://localhost:4566 s3 ls s3://rlm-demo-docs/ --recursive

# Read a specific file
aws --endpoint-url=http://localhost:4566 s3 cp s3://rlm-demo-docs/docs/architecture.md -
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | **Yes** | API key for OpenAI (or compatible provider) |
| `AWS_ACCESS_KEY_ID` | No | Not needed for LocalStack (uses `test`) |
| `AWS_SECRET_ACCESS_KEY` | No | Not needed for LocalStack (uses `test`) |

## Cleanup

```bash
docker compose -f demo/docker-compose.yml down
```
