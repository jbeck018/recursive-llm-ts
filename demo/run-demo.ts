/**
 * recursive-llm-ts Demo Runner
 *
 * Demonstrates file-based completions with both local and S3 (LocalStack) storage.
 *
 * Prerequisites:
 *   1. Set OPENAI_API_KEY in your environment
 *   2. Start LocalStack: docker compose -f demo/docker-compose.yml up -d
 *
 * Usage:
 *   npx ts-node demo/run-demo.ts [example]
 *
 * Examples:
 *   npx ts-node demo/run-demo.ts local       # Local file context
 *   npx ts-node demo/run-demo.ts s3           # S3 (LocalStack) file context
 *   npx ts-node demo/run-demo.ts structured   # Structured extraction from S3
 *   npx ts-node demo/run-demo.ts all          # Run all demos
 */

import { RLM } from '../src';
import { z } from 'zod';
import * as path from 'path';

// ─── Configuration ───────────────────────────────────────────────────────────

const OPENAI_API_KEY = process.env.OPENAI_API_KEY;
if (!OPENAI_API_KEY) {
  console.error('Error: OPENAI_API_KEY environment variable is required.');
  console.error('Set it with: export OPENAI_API_KEY="sk-..."');
  process.exit(1);
}

const LOCALSTACK_ENDPOINT = 'http://localhost:4566';
const DEMO_BUCKET = 'rlm-demo-docs';
const SEED_DATA_DIR = path.join(__dirname, 'seed-data');

// ─── Demo: Local File Context ────────────────────────────────────────────────

async function demoLocalFiles() {
  console.log('\n' + '='.repeat(60));
  console.log('  Demo: Local File Context');
  console.log('='.repeat(60) + '\n');

  const rlm = new RLM('gpt-4o-mini', {
    api_key: OPENAI_API_KEY,
    max_iterations: 15,
  });

  try {
    console.log(`Reading files from: ${SEED_DATA_DIR}`);
    console.log('');

    const result = await rlm.completionFromFiles(
      'Analyze these documents and provide: (1) a summary of what this company does, ' +
      '(2) their key technical architecture decisions, and (3) any notable incidents or challenges.',
      {
        type: 'local',
        path: SEED_DATA_DIR,
        extensions: ['.md', '.json', '.ts', '.csv'],
        excludePatterns: ['*.test.*'],
        maxFileSize: 200_000,
      }
    );

    console.log('Result:');
    console.log('-'.repeat(40));
    console.log(result.result);
    console.log('-'.repeat(40));
    console.log(`\nStats: ${result.stats.llm_calls} LLM calls, ${result.stats.iterations} iterations`);

    if (result.fileStorage) {
      console.log(`Files included: ${result.fileStorage.files.length}`);
      for (const f of result.fileStorage.files) {
        console.log(`  - ${f.relativePath} (${(f.size / 1024).toFixed(1)}KB)`);
      }
      if (result.fileStorage.skipped.length > 0) {
        console.log(`Skipped: ${result.fileStorage.skipped.length}`);
      }
    }
  } finally {
    await rlm.cleanup();
  }
}

// ─── Demo: S3 File Context (LocalStack) ──────────────────────────────────────

async function demoS3Files() {
  console.log('\n' + '='.repeat(60));
  console.log('  Demo: S3 File Context (LocalStack)');
  console.log('='.repeat(60) + '\n');

  const rlm = new RLM('gpt-4o-mini', {
    api_key: OPENAI_API_KEY,
    max_iterations: 15,
  });

  try {
    console.log(`Connecting to LocalStack: ${LOCALSTACK_ENDPOINT}`);
    console.log(`Bucket: ${DEMO_BUCKET}`);
    console.log('');

    const result = await rlm.completionFromFiles(
      'What are the main products and services described in these documents? ' +
      'What is the overall business performance based on the metrics?',
      {
        type: 's3',
        path: DEMO_BUCKET,
        endpoint: LOCALSTACK_ENDPOINT,
        region: 'us-east-1',
        credentials: {
          accessKeyId: 'test',
          secretAccessKey: 'test',
        },
        extensions: ['.md', '.json', '.csv'],
      }
    );

    console.log('Result:');
    console.log('-'.repeat(40));
    console.log(result.result);
    console.log('-'.repeat(40));
    console.log(`\nStats: ${result.stats.llm_calls} LLM calls, ${result.stats.iterations} iterations`);

    if (result.fileStorage) {
      console.log(`Files from S3: ${result.fileStorage.files.length}`);
    }
  } finally {
    await rlm.cleanup();
  }
}

// ─── Demo: Structured Extraction from S3 ─────────────────────────────────────

async function demoStructuredS3() {
  console.log('\n' + '='.repeat(60));
  console.log('  Demo: Structured Extraction from S3');
  console.log('='.repeat(60) + '\n');

  const rlm = new RLM('gpt-4o-mini', {
    api_key: OPENAI_API_KEY,
  });

  const reviewAnalysisSchema = z.object({
    overallSentiment: z.number().min(1).max(5)
      .describe('Average sentiment across all reviews (1=very negative, 5=very positive)'),
    totalReviews: z.number(),
    topStrengths: z.array(z.string())
      .describe('Top 3 most praised aspects of the product'),
    topWeaknesses: z.array(z.string())
      .describe('Top 3 most criticized aspects'),
    productBreakdown: z.array(z.object({
      product: z.string(),
      avgRating: z.number(),
      reviewCount: z.number(),
      summary: z.string(),
    })),
    recommendations: z.array(z.string())
      .describe('3-5 actionable recommendations based on the reviews'),
  });

  try {
    console.log('Extracting structured review analysis from S3...');
    console.log('');

    const result = await rlm.structuredCompletionFromFiles(
      'Analyze all product reviews and extract structured insights',
      {
        type: 's3',
        path: DEMO_BUCKET,
        prefix: 'data/',
        endpoint: LOCALSTACK_ENDPOINT,
        region: 'us-east-1',
        credentials: {
          accessKeyId: 'test',
          secretAccessKey: 'test',
        },
        extensions: ['.json'],
      },
      reviewAnalysisSchema,
      { parallelExecution: true }
    );

    console.log('Structured Result:');
    console.log('-'.repeat(40));
    console.log(JSON.stringify(result.result, null, 2));
    console.log('-'.repeat(40));
    console.log(`\nStats: ${result.stats.llm_calls} LLM calls`);
  } finally {
    await rlm.cleanup();
  }
}

// ─── Main ────────────────────────────────────────────────────────────────────

async function main() {
  const example = process.argv[2] || 'all';

  console.log('recursive-llm-ts Demo');
  console.log(`OPENAI_API_KEY: ${OPENAI_API_KEY!.substring(0, 8)}...`);

  switch (example) {
    case 'local':
      await demoLocalFiles();
      break;
    case 's3':
      await demoS3Files();
      break;
    case 'structured':
      await demoStructuredS3();
      break;
    case 'all':
      await demoLocalFiles();
      await demoS3Files();
      await demoStructuredS3();
      break;
    default:
      console.error(`Unknown example: ${example}`);
      console.error('Available: local, s3, structured, all');
      process.exit(1);
  }

  console.log('\nAll demos complete!');
}

main().catch(err => {
  console.error('Demo failed:', err);
  process.exit(1);
});
