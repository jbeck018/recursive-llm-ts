# Quick Start Guide

Get up and running with recursive-llm-ts in under 5 minutes.

## 1. Install

```bash
npm install recursive-llm-ts
```

## 2. Set Your API Key

```bash
export OPENAI_API_KEY='sk-...'
```

## 3. Basic Completion

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
});

const result = await rlm.completion(
  'What are the key points in this document?',
  longDocument
);

console.log(result.result);
console.log(result.stats); // { llm_calls, iterations, depth }
```

## 4. Structured Output (with Zod)

Extract typed data from any context:

```typescript
import { z } from 'zod';

const schema = z.object({
  summary: z.string(),
  sentiment: z.enum(['positive', 'negative', 'neutral']),
  keyTopics: z.array(z.string()),
  confidence: z.number().min(0).max(1),
});

const result = await rlm.structuredCompletion(
  'Analyze this customer feedback',
  feedback,
  schema
);

// Fully typed!
console.log(result.result.summary);    // string
console.log(result.result.sentiment);  // 'positive' | 'negative' | 'neutral'
console.log(result.result.keyTopics);  // string[]
```

## 5. Enable Caching

Avoid redundant API calls during development:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  cache: { enabled: true, ttl: 3600 },
});

const r1 = await rlm.completion('Summarize', doc); // API call
const r2 = await rlm.completion('Summarize', doc); // Cache hit!
console.log(r2.cached); // true
```

## 6. Retry with Resilience

Handle transient failures automatically:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  retry: {
    maxRetries: 3,
    backoff: 'exponential',
  },
});
```

## 7. Streaming

Get progressive output:

```typescript
const stream = rlm.streamCompletion('Summarize', doc);

for await (const chunk of stream) {
  if (chunk.type === 'text') {
    process.stdout.write(chunk.text);
  }
}

// Or collect all text at once:
const text = await rlm.streamCompletion('Summarize', doc).toText();
```

## 8. Monitor with Events

Track what's happening:

```typescript
rlm.on('llm_call', (e) => console.log(`Calling ${e.model}...`));
rlm.on('cache', (e) => console.log(`Cache ${e.action}`));
rlm.on('error', (e) => console.error('Error:', e.error.message));
```

## 9. Process Files

Load files directly as context:

```typescript
const result = await rlm.completionFromFiles(
  'Summarize this codebase',
  {
    type: 'local',
    path: './src',
    extensions: ['.ts'],
    excludePatterns: ['*.test.ts'],
  }
);
```

## 10. Use the Builder

For complex configuration:

```typescript
const rlm = RLM.builder('gpt-4o-mini')
  .apiKey(process.env.OPENAI_API_KEY!)
  .maxDepth(10)
  .withMetaAgent()
  .withCache({ strategy: 'exact' })
  .withRetry({ maxRetries: 3 })
  .withDebug()
  .build();
```

## Next Steps

- [Full API Reference](../README.md#api)
- [Architecture Overview](ARCHITECTURE.md)
- [Contributing Guide](../CONTRIBUTING.md)
