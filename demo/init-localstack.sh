#!/bin/bash
# init-localstack.sh
# Auto-seeds the LocalStack S3 bucket with demo data on startup.
# This script runs automatically when the LocalStack container starts.

set -euo pipefail

BUCKET="rlm-demo-docs"
REGION="us-east-1"

echo "============================================"
echo "  RLM Demo: Seeding LocalStack S3 bucket"
echo "============================================"

echo ""
echo "Creating bucket: s3://${BUCKET}"
awslocal s3 mb "s3://${BUCKET}" --region "${REGION}" 2>/dev/null || true

echo ""
echo "Uploading seed data..."
awslocal s3 cp /seed-data/ "s3://${BUCKET}/" --recursive --quiet

echo ""
echo "Bucket contents:"
awslocal s3 ls "s3://${BUCKET}/" --recursive --human-readable

echo ""
echo "============================================"
echo "  Demo bucket ready!"
echo ""
echo "  Endpoint: http://localhost:4566"
echo "  Bucket:   ${BUCKET}"
echo "  Region:   ${REGION}"
echo ""
echo "  Use in your code:"
echo "    {"
echo "      type: 's3',"
echo "      path: '${BUCKET}',"
echo "      endpoint: 'http://localhost:4566',"
echo "      region: '${REGION}',"
echo "      credentials: {"
echo "        accessKeyId: 'test',"
echo "        secretAccessKey: 'test',"
echo "      },"
echo "    }"
echo "============================================"
