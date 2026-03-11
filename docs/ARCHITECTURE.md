# Architecture

## Overview

recursive-llm-ts is a TypeScript/Go hybrid package implementing Recursive Language Models (RLM). The TypeScript layer provides the public API; the Go binary handles LLM orchestration, REPL execution, and recursive decomposition.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    TypeScript Layer                       │
│                                                          │
│  RLM Class ─── Events ─── Cache ─── Retry ─── Stream   │
│       │                                                  │
│  Builder ─── Config Validator ─── Error Hierarchy        │
│       │                                                  │
│  File Storage (Local/S3) ─── Zod→JSON Schema             │
│       │                                                  │
│  Bridge Factory (auto-detect Go/Python)                  │
└──────────────────────┬──────────────────────────────────┘
                       │ JSON via stdin/stdout
┌──────────────────────▼──────────────────────────────────┐
│                      Go Binary                           │
│                                                          │
│  RLM Engine ─── Meta-Agent ─── Observability             │
│       │                                                  │
│  Structured Output ─── Schema Validator ─── REPL (goja)  │
│       │                                                  │
│  OpenAI-Compatible API Client                            │
└─────────────────────────────────────────────────────────┘
```

## IPC Bridge Pattern

The TypeScript and Go layers communicate via JSON over stdin/stdout:

1. TypeScript spawns the Go binary as a child process
2. Sends a JSON request via stdin (model, query, context, config)
3. Go processes the request (may make multiple LLM calls, REPL executions)
4. Returns a JSON response via stdout (result, stats, trace_events)

This architecture provides:
- **Process isolation** -- Go binary crashes don't crash Node.js
- **No FFI** -- Pure JSON IPC, works on any platform
- **Binary distribution** -- Pre-built binaries for common platforms

## Module Responsibilities

### Core (`src/rlm.ts`)
- Main `RLM` class with all public API methods
- `RLMBuilder` for fluent configuration
- `RLMResultFormatter` for output formatting
- Static factory methods (`fromEnv`, `withDebug`, `forAzure`)
- Zod→JSON Schema conversion (supports 30+ Zod types)

### Errors (`src/errors.ts`)
- `RLMError` base class with `code`, `retryable`, `suggestion`
- Specific error types: `RLMValidationError`, `RLMRateLimitError`, `RLMTimeoutError`, `RLMProviderError`, `RLMBinaryError`, `RLMConfigError`, `RLMSchemaError`, `RLMAbortError`, `RLMContextOverflowError`
- `classifyError()` auto-classifies raw errors into typed hierarchy (detects OpenAI, Azure, vLLM overflow patterns)

### Streaming (`src/streaming.ts`)
- `RLMStream<T>` -- async iterable stream of typed chunks
- `toText()` / `toObject()` convenience collectors
- AbortController support
- Simulated streaming (chunked output) until Go binary supports native streaming

### Cache (`src/cache.ts`)
- `RLMCache` -- exact-match caching with SHA-256 key generation
- `MemoryCache` -- in-memory LRU with TTL eviction
- `FileCache` -- file-system persistent cache
- Cache statistics (hits, misses, hit rate, evictions)

### Retry (`src/retry.ts`)
- `withRetry()` -- exponential/linear/fixed backoff with jitter
- `withFallback()` -- sequential model fallback chains
- Respects `retryAfter` from rate limit errors
- AbortController-aware

### Events (`src/events.ts`)
- `RLMEventEmitter` -- typed event emitter (not Node.js EventEmitter)
- Events: `llm_call`, `llm_response`, `validation_retry`, `recursion`, `meta_agent`, `error`, `completion_start`, `completion_end`, `cache`, `retry`
- Listener isolation (errors in listeners don't propagate)

### Config (`src/config.ts`)
- `validateConfig()` -- returns issues (error/warning/info levels)
- `assertValidConfig()` -- throws on errors, logs warnings
- Detects unknown keys (likely typos), validates bounds, checks URLs

### File Storage (`src/file-storage.ts`)
- `FileContextBuilder` -- builds LLM context from file trees
- `LocalFileStorage` -- recursive local directory traversal
- `S3FileStorage` -- S3-compatible object storage (AWS, MinIO, LocalStack)
- Filtering by extension, glob pattern, size limits

## Data Flow

### Completion Flow
```
RLM.completion(query, context)
  → Check cache (hit? return cached)
  → Emit 'completion_start' event
  → withRetry(async () => {
      → Ensure bridge (spawn Go binary if needed)
      → Emit 'llm_call' event
      → bridge.completion(model, query, context, config)
        → Go: meta-agent optimizes query (if enabled)
        → Go: recursive decomposition + LLM calls
        → Go: REPL execution (if code in response)
        → Go: collect trace events
        → Go: return JSON {result, stats, trace_events}
      → Emit 'llm_response' event
    })
  → Store in cache
  → Emit 'completion_end' event
  → Return result with {cached: false, model}
```

### Structured Output Flow
```
RLM.structuredCompletion(query, context, zodSchema)
  → Convert Zod schema → JSON Schema
  → Send to Go with structured config
  → Go: validate schema
  → Go: decompose complex schemas into subtasks
  → Go: parallel execution via goroutines
  → Go: validate output against schema
  → Go: retry with corrective prompting on validation failure
  → TypeScript: Zod.parse() for type-safe validation
  → Return typed result
```

### Context Overflow Recovery Flow
```
RLM engine attempts LLM call
  → API returns context_length_exceeded (400/413)
  → classifyError() detects overflow pattern
  → RLMContextOverflowError created (modelLimit, requestTokens)
  → ContextReducer.ReduceForCompletion(query, context, limit)
    → Strategy dispatch:
      mapreduce: chunks → parallel LLM summarize → merge
      truncate:  drop tokens from end → findBreakPoint()
      chunked:   chunks → sequential LLM extract → join
      tfidf:     SplitSentences → ComputeTFIDF → top-k selection
      textrank:  BuildSimilarityGraph → PageRank → top-k selection
      refine:    chunks → sequential LLM refine with accumulation
  → Retry with reduced context (up to max_reduction_attempts)
```

### Token Tracking Flow
```
CallChatCompletion() parses API response "usage" field
  → Returns ChatCompletionResult { Content, Usage *TokenUsage }
  → RLM.callLLM() accumulates into r.stats.{TotalTokens, PromptTokens, CompletionTokens}
  → Observer.LLMCall() records actual token count (not hardcoded 0)
  → All paths accumulate: completion, structured, meta-agent, overflow reduction
  → Stats returned via JSON IPC → TypeScript RLMStats.total_tokens
```

See [Token Tracking and Efficiency Guide](TOKEN_TRACKING_AND_EFFICIENCY.md) for benchmarks, test details, and strategy recommendations.

## Go Binary Architecture

### Key Components
- **`rlm.go`** -- Core engine, LLM orchestration, observer integration
- **`structured.go`** -- Schema decomposition, parallel execution, validation retry
- **`schema.go`** -- JSON Schema validation using Google jsonschema-go
- **`meta_agent.go`** -- Query optimization heuristics
- **`observability.go`** -- OTEL tracing, Langfuse integration, debug logging
- **`repl.go`** -- JavaScript execution using goja engine
- **`context_overflow.go`** -- Context overflow detection, error classification, 6 reduction strategies, token estimation
- **`openai.go`** -- OpenAI-compatible API client, `ChatCompletionResult` with `TokenUsage` parsing
- **`tfidf.go`** -- TF-IDF extractive compression: sentence splitting, tokenization, stop-word filtering, IDF scoring
- **`textrank.go`** -- TextRank graph-based ranking: cosine similarity graph, PageRank iteration
- **`token_tracking_test.go`** -- 22 tests proving token tracking and context reduction efficiency

### Binary Resolution Order
1. `RLMConfig.go_binary_path`
2. `RLM_GO_BINARY` environment variable
3. `./bin/rlm-go` (npm package location)
4. `./go/rlm-go` (development location)
