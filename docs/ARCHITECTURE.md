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
- Specific error types: `RLMValidationError`, `RLMRateLimitError`, `RLMTimeoutError`, `RLMProviderError`, `RLMBinaryError`, `RLMConfigError`, `RLMSchemaError`, `RLMAbortError`
- `classifyError()` auto-classifies raw errors into typed hierarchy

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

## Go Binary Architecture

### Key Components
- **`rlm.go`** -- Core engine, LLM orchestration, observer integration
- **`structured.go`** -- Schema decomposition, parallel execution, validation retry
- **`schema.go`** -- JSON Schema validation using Google jsonschema-go
- **`meta_agent.go`** -- Query optimization heuristics
- **`observability.go`** -- OTEL tracing, Langfuse integration, debug logging
- **`repl.go`** -- JavaScript execution using goja engine

### Binary Resolution Order
1. `RLMConfig.go_binary_path`
2. `RLM_GO_BINARY` environment variable
3. `./bin/rlm-go` (npm package location)
4. `./go/rlm-go` (development location)
