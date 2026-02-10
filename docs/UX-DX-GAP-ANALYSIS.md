# UX/DX Gap Analysis & Feature Roadmap

> **Date:** 2026-02-10
> **Package:** recursive-llm-ts v4.4.1
> **Scope:** Developer experience, API ergonomics, missing features, industry alignment

---

## Executive Summary

This document identifies **32 UX/DX gaps** across 8 categories, benchmarked against industry leaders (Vercel AI SDK, Instructor, LangChain/LangGraph, LlamaIndex). The package has strong fundamentals -- solid type safety, excellent file storage, and good structured output -- but lags behind modern standards in **streaming**, **error taxonomy**, **observability completeness**, **caching**, and **developer tooling**.

The gaps are organized into three tiers:
- **P0 (Critical):** 7 items -- Features users expect from any modern LLM library
- **P1 (Important):** 12 items -- Competitive differentiation and production-readiness
- **P2 (Nice-to-have):** 13 items -- Polish, DX quality-of-life improvements

---

## Current Strengths (What Works Well)

| Area | Assessment |
|------|-----------|
| **Zod-first structured output** | First-class Zod integration with automatic JSON Schema conversion, parallel execution, instructor-style retry. Competitive with Instructor-JS. |
| **File storage** | 117-test coverage, S3/MinIO/LocalStack support, actionable error messages with `S3StorageError` codes. |
| **Go performance** | 50x faster startup, 3x less memory than Python bridge. Automatic binary resolution. |
| **Provider flexibility** | Any OpenAI-compatible API (Azure, Bedrock, local models) via LiteLLM passthrough. |
| **Type safety** | `structuredCompletion<T>` returns properly typed results. Strict TypeScript config. |
| **Cross-platform CI** | Matrix testing across 3 OS x 2 Node versions + 5 Go build targets. |

---

## P0: Critical Gaps

### 1. No Streaming Support

**Current state:** All methods are request/response only. No progressive output.

**Industry standard:** Every major LLM library provides streaming as a first-class primitive. Vercel AI SDK's `streamText()`, `streamObject()`, LangChain's `.stream()`, and Instructor's streaming mode are table stakes.

**Impact:** Users can't build chat UIs, show progress on long operations, or cancel mid-generation. Large document processing blocks with no feedback.

**Proposed solution:**
```typescript
// Text streaming
for await (const chunk of rlm.streamCompletion(query, context)) {
  process.stdout.write(chunk.text);
}

// Structured output streaming (partial objects)
for await (const partial of rlm.streamStructuredCompletion(query, context, schema)) {
  updateUI(partial.object); // Typed partial result
}

// AbortController support
const controller = new AbortController();
const stream = rlm.streamCompletion(query, context, { signal: controller.signal });
setTimeout(() => controller.abort(), 5000);
```

**Implementation notes:**
- Go binary needs stdout streaming mode (newline-delimited JSON chunks)
- TypeScript layer wraps as `AsyncIterable<StreamChunk>`
- Partial JSON parsing via incremental parser for structured streaming
- SSE transport layer for HTTP server integration

---

### 2. Broken Observability Features

**Current state:** Two critical features are non-functional:
- `go/rlm/observability.go:134-144` -- OTLP endpoint configuration is **ignored**; both branches export to stdout
- `go/rlm/observability.go:362-368` -- Langfuse integration is a **stub** that only debug-logs

**Industry standard:** OpenTelemetry GenAI Semantic Conventions are the emerging standard. Langfuse v3 is OTEL-native. Every production LLM system needs real trace export.

**Impact:** Users configure OTLP endpoints and Langfuse keys expecting traces to appear in their dashboards. Nothing happens. Silent failure erodes trust.

**Proposed fix:**
- Implement actual OTLP HTTP/gRPC exporter using `go.opentelemetry.io/otel/exporters/otlp/otlptracehttp`
- Implement Langfuse API integration (POST to `/api/public/ingestion`) or point OTLP exporter at Langfuse's OTLP endpoint
- Add GenAI semantic conventions to spans (model, token counts, cost)
- Add startup health check: verify endpoint reachability, log warning if unreachable
- Add `observability.validate()` method to test configuration

---

### 3. No Structured Error Taxonomy

**Current state:**
- TypeScript: Only `S3StorageError` is typed. All other errors are generic `Error`
- Go: Only 4 error types (`MaxIterationsError`, `MaxDepthError`, `REPLError`, `APIError`), none actionable
- Error messages lack remediation guidance (e.g., "Missing model" doesn't say what models exist)

**Industry standard:** Instructor, Vercel AI SDK, and LangChain all provide typed error hierarchies with error codes, retryable flags, and actionable messages.

**Impact:** Users can't write reliable error handling. `catch (e)` gives no type information. Programmatic retry decisions are impossible.

**Proposed error hierarchy:**
```typescript
// Base class
class RLMError extends Error {
  code: string;
  retryable: boolean;
  suggestion?: string;
}

// Specific types
class RLMValidationError extends RLMError { expected: JsonSchema; received: unknown; }
class RLMRateLimitError extends RLMError { retryAfter?: number; }
class RLMTimeoutError extends RLMError { elapsed: number; limit: number; }
class RLMProviderError extends RLMError { statusCode: number; provider: string; }
class RLMBinaryError extends RLMError { binaryPath: string; }
class RLMConfigError extends RLMError { field: string; value: unknown; }
class RLMSchemaError extends RLMError { path: string; constraint: string; }
```

---

### 4. No Test Runner or `npm test`

**Current state:** `package.json` has `"test": "echo \"Error: no test specified\" && exit 1"`. CI uses `continue-on-error: true` to skip TS test failures. Tests are run manually via `ts-node test/file.ts`.

**Industry standard:** Every published npm package has a working `npm test` command.

**Impact:** Contributors can't verify changes. CI doesn't catch TypeScript regressions. No coverage reporting for TypeScript (only Go has Codecov).

**Proposed fix:**
- Add Vitest as test runner (fast, TypeScript-native, ESM-compatible)
- Migrate existing manual test files to Vitest format
- Configure `npm test` to run full suite
- Add coverage reporting with threshold (target: 80%)
- Add TypeScript type-checking step to CI (`tsc --noEmit`)

---

### 5. Silent Configuration Failures

**Current state:**
- Go `ConfigFromMap` silently ignores unknown keys (typos like `max_detph` accepted)
- Negative values accepted without bounds checking
- `toInt("abc")` returns 0 without warning
- No validation that API key is set before first request
- `RLMConfig` has `[key: string]: any` index signature, defeating TypeScript safety

**Industry standard:** Zod-validated config with clear error messages at construction time.

**Impact:** Users spend hours debugging silent misconfiguration. A typo in config is indistinguishable from a working config.

**Proposed fix:**
- Remove `[key: string]: any` from `RLMConfig`; use explicit `litellm_params?: Record<string, unknown>`
- Add Go-side config validation with warnings for unknown keys
- Add bounds checking (positive integers for depth/iterations, valid URL for api_base)
- Add `rlm.validateConfig()` for eager validation
- Add TypeScript-side config validation using Zod (dogfooding the Zod dependency)

---

### 6. No Caching Layer

**Current state:** Zero caching. Every identical query+context pair makes a full API call.

**Industry standard:** Multi-tier caching is standard:
- Exact match (hash-based, sub-millisecond)
- Semantic cache (embedding similarity, 30-40% hit rate for repetitive workloads)
- Provider prefix caching (Anthropic: 90% cost reduction, OpenAI: 50% automatic)

**Impact:** Users pay full cost for repeated queries. No way to optimize spend for iterative development.

**Proposed solution:**
```typescript
const rlm = new RLM(model, {
  cache: {
    enabled: true,
    strategy: 'exact',     // 'exact' | 'semantic' | 'none'
    maxEntries: 1000,
    ttl: 3600,             // seconds
    storage: 'memory',     // 'memory' | 'redis' | 'file'
  }
});

// Cache-aware completion
const result = await rlm.completion(query, context);
console.log(result.cached); // true if served from cache
```

---

### 7. No Retry/Resilience Beyond Structured Validation

**Current state:** Structured output has retry with corrective prompting (good). But:
- Regular `completion()` has zero retry logic
- API failures (429, 503, network) cause immediate failure
- No exponential backoff
- No provider fallback
- No circuit breaker

**Industry standard:** Retries with exponential backoff + jitter for transient errors. Provider fallback chains. Circuit breakers for production systems.

**Proposed solution:**
```typescript
const rlm = new RLM(model, {
  retry: {
    maxRetries: 3,
    backoff: 'exponential',  // 1s, 2s, 4s with jitter
    retryableErrors: ['rate_limit', 'timeout', 'server_error'],
  },
  fallback: {
    models: ['gpt-4o', 'claude-sonnet-4-20250514', 'gemini-2.0-flash'],
    strategy: 'sequential', // try next on failure
  }
});
```

---

## P1: Important Gaps

### 8. No Event/Callback System

**Current state:** `getTraceEvents()` returns past events after completion. No real-time monitoring.

**Proposed:**
```typescript
rlm.on('llm_call', (event) => logToDatadog(event));
rlm.on('validation_retry', (event) => console.warn(event));
rlm.on('recursion_depth', (event) => trackDepth(event));
```

Use Node.js `EventEmitter` or typed event pattern. Enables monitoring dashboards, logging middleware, and progress tracking.

---

### 9. No Convenience Factory Methods

**Current state:** Constructor requires manual config assembly every time.

**Proposed:**
```typescript
// Quick setup from environment
const rlm = RLM.fromEnv('gpt-4o-mini');

// Debug mode preset
const rlm = RLM.withDebug('gpt-4o-mini');

// Provider-specific factories
const rlm = RLM.forAzure(deploymentName, { apiVersion: '2024-02-15' });
const rlm = RLM.forBedrock(region, model);
```

---

### 10. No Builder/Fluent API

**Current state:** Configuration is static at construction. Can't compose or modify.

**Proposed:**
```typescript
const rlm = RLM.builder('gpt-4o-mini')
  .maxDepth(10)
  .withMetaAgent()
  .withDebug()
  .withCache({ strategy: 'exact' })
  .build();
```

Enables IDE discoverability of all options and composable configuration.

---

### 11. No Batch Operations

**Current state:** One query at a time. No way to efficiently process multiple queries.

**Proposed:**
```typescript
const results = await rlm.batchCompletion([
  { query: 'Summarize chapter 1', context: ch1 },
  { query: 'Summarize chapter 2', context: ch2 },
], { concurrency: 3 });
```

Reuses binary process across calls, manages concurrency, aggregates stats.

---

### 12. Missing JSDoc Coverage

**Current state:** Most public methods lack `@param`, `@returns`, `@example` documentation. IDE autocomplete shows parameter names but no descriptions.

**Proposed:** Add comprehensive JSDoc to all public API surface:
```typescript
/**
 * Execute a completion against an LLM with recursive decomposition.
 *
 * @param query - The question or instruction for the LLM
 * @param context - The document or data to process (can be very large)
 * @returns The LLM response with execution statistics
 *
 * @example
 * ```typescript
 * const result = await rlm.completion('Summarize the key points', longDocument);
 * console.log(result.result);
 * console.log(`Used ${result.stats.llm_calls} LLM calls`);
 * ```
 */
```

---

### 13. No Pre-flight Validation

**Current state:** Errors surface at first API call. Binary existence, API key validity, S3 connectivity -- all deferred.

**Proposed:**
```typescript
// Check everything upfront
const issues = await rlm.validate();
// Returns: { valid: boolean, issues: ValidationIssue[] }
// e.g., [{ level: 'error', message: 'Go binary not found at ./bin/rlm-go' }]
```

---

### 14. No ESLint/Prettier Configuration

**Current state:** No code formatting or linting tools. No `.eslintrc`, `.prettierrc`, `.editorconfig`, or pre-commit hooks.

**Proposed:**
- Add ESLint with strict TypeScript rules
- Add Prettier for consistent formatting
- Add `.editorconfig` for cross-editor consistency
- Add `husky` + `lint-staged` for pre-commit enforcement
- Add to CI pipeline

---

### 15. Meta-Agent Behavior is Opaque

**Current state:**
- Query optimization happens silently (Go `rlm.go:82-88` swallows errors)
- Users can't see original vs optimized query
- `needsOptimization` uses hardcoded heuristics (50-char threshold) with no transparency
- No per-call override

**Proposed:**
- Return `optimized_query` in response stats
- Add `meta_agent.verbose` option to log optimization reasoning
- Allow per-call `skipMetaAgent: true` option
- Surface optimization decision in trace events

---

### 16. Coordinator is Under-Utilized

**Current state:** `RLMAgentCoordinator.processComplex()` is a thin wrapper over `structuredCompletion()`. Schema decomposition types exist in `structured-types.ts` but are never exposed.

**Proposed:**
- Expose `decomposeSchema()` for inspection before execution
- Add multi-query coordination with dependency tracking
- Add task-level event callbacks
- Consider deprecating in favor of enriching RLM class directly

---

### 17. No Result Formatting Helpers

**Current state:** Users manually format stats, trace events, and results.

**Proposed:**
```typescript
console.log(result.prettyStats());
// LLM Calls: 3 | Iterations: 12 | Depth: 2 | Tokens: 4,521

console.log(result.toJSON());       // Serializable output
console.log(result.toMarkdown());   // Markdown-formatted result
```

---

### 18. `RLMResult.result` Type is `string | any`

**Current state:** `bridge-interface.ts:9` defines `result: string | any`, which defeats TypeScript type safety.

**Fix:** Change to `result: string` for regular completions. `structuredCompletion<T>` already returns `StructuredRLMResult<T>` with proper typing.

---

### 19. No CONTRIBUTING.md or Onboarding Docs

**Current state:** No contribution guide, no architecture overview for external contributors, no quick-start separate from the full README.

**Proposed:**
- `CONTRIBUTING.md` with development setup, testing, PR guidelines
- `docs/ARCHITECTURE.md` (move content from CLAUDE.md internal notes)
- `docs/QUICKSTART.md` for 5-minute getting-started guide

---

## P2: Nice-to-Have Improvements

### 20. No `using` / Disposable Pattern
Support `Symbol.asyncDispose` for Node 22+ automatic cleanup:
```typescript
await using rlm = new RLM(model);
// Automatically cleaned up at block exit
```

### 21. No Dependency Injection
Can't inject custom bridges, file storage providers, or validators for testing.

### 22. No Schema Inspection API
Can't see how a Zod schema will be decomposed before execution.

### 23. No Partial Result Recovery
If 4/5 fields validate but 1 fails, entire result is discarded. Could return partial results with error annotations.

### 24. No File Context Caching
Rebuilding file context is expensive for large directories. TTL-based caching would help.

### 25. No Multi-Source File Loading
Can't combine local + S3 sources in a single query without manual context building.

### 26. No Performance Benchmarks in CI
Go benchmarks exist but aren't tracked for regressions. No TypeScript performance testing.

### 27. No Security Scanning in CI
No SAST, no dependency audit (`npm audit`), no SBOM generation.

### 28. REPL Function Discovery
Users of the Go REPL have no way to discover available functions (15+ built-ins like `json`, `Counter`, `range`).

### 29. No Migration Guides
No documentation for upgrading between major versions (e.g., 3.x to 4.x).

### 30. No Deprecation Warning System
No way to warn users about upcoming breaking changes.

### 31. No Request/Response Logging
Debug mode logs operations but doesn't capture raw API requests/responses for debugging.

### 32. FileStorageConfig Discriminated Union
`type: 'local' | 's3'` should be a discriminated union so S3-specific fields aren't available for local configs.

---

## Implementation Roadmap

### Phase 1: Foundation (Fix What's Broken)
| Item | Effort | Impact |
|------|--------|--------|
| #2 Fix observability (OTLP + Langfuse) | Medium | High -- currently broken |
| #3 Error taxonomy | Medium | High -- enables reliable error handling |
| #4 Test runner + `npm test` | Low | High -- unblocks contributors |
| #5 Config validation | Medium | High -- prevents silent misconfiguration |
| #18 Fix `string \| any` type | Trivial | Medium -- type safety |
| #14 ESLint/Prettier | Low | Medium -- code quality |

### Phase 2: Core Features (Industry Parity)
| Item | Effort | Impact |
|------|--------|--------|
| #1 Streaming support | High | Critical -- table stakes for LLM libraries |
| #6 Caching layer | Medium | High -- cost reduction |
| #7 Retry/resilience | Medium | High -- production readiness |
| #8 Event system | Medium | High -- monitoring/debugging |
| #15 Meta-agent transparency | Low | Medium -- trust/debuggability |

### Phase 3: DX Polish (Competitive Edge)
| Item | Effort | Impact |
|------|--------|--------|
| #9 Factory methods | Low | Medium -- onboarding speed |
| #10 Builder API | Medium | Medium -- discoverability |
| #11 Batch operations | Medium | Medium -- throughput |
| #12 JSDoc coverage | Low | Medium -- IDE experience |
| #13 Pre-flight validation | Low | Medium -- early error detection |
| #17 Result formatters | Low | Low -- convenience |

### Phase 4: Ecosystem (Growth)
| Item | Effort | Impact |
|------|--------|--------|
| #19 Contribution docs | Low | Medium -- community growth |
| #16 Coordinator enrichment | Medium | Medium -- advanced use cases |
| #20 Disposable pattern | Trivial | Low -- modern Node.js |
| #23 Partial results | Medium | Medium -- resilience |
| #27 Security scanning | Low | Medium -- supply chain safety |

---

## Competitive Positioning Matrix

| Feature | recursive-llm-ts | Vercel AI SDK | Instructor | LangChain |
|---------|------------------|---------------|------------|-----------|
| Structured output (Zod) | Yes | Yes | Yes | Partial |
| Streaming | **No** | Yes | Yes | Yes |
| Schema validation + retry | Yes | Yes | Yes | No |
| File/S3 context loading | **Yes (unique)** | No | No | Via loaders |
| Recursive decomposition | **Yes (unique)** | No | No | Via chains |
| OTEL tracing | Broken | No | No | Yes |
| Caching | **No** | No | Yes | Yes |
| Error taxonomy | Partial | Yes | Yes | Partial |
| Multi-provider fallback | **No** | Yes | Yes | Yes |
| Event system | **No** | Via hooks | Via hooks | Yes |
| Type safety | Good | Excellent | Good | Poor (TS) |
| Go performance backend | **Yes (unique)** | N/A | N/A | N/A |

### Unique Differentiators to Lean Into
1. **Recursive decomposition** -- No competitor offers this. Unbounded context processing is a genuine moat.
2. **Go performance backend** -- 50x faster startup, 3x less memory. Real advantage for serverless/edge.
3. **File/S3 context loading** -- Built-in file ingestion with S3 support. Others require external loaders.
4. **REPL execution** -- JavaScript code execution within the LLM loop. Enables computational reasoning.

### Where to Catch Up
1. **Streaming** -- Non-negotiable for modern LLM libraries
2. **Observability** -- Fix what's claimed to work
3. **Error handling** -- Match Instructor's actionable error patterns
4. **Caching** -- At minimum, exact-match caching for development workflows

---

## Appendix: Industry Research Sources

- Vercel AI SDK 6 (20M+ monthly downloads) -- agent abstraction, streaming objects, framework hooks
- Instructor (Python/TS/Go) -- schema-first extraction, corrective retry, minimal abstraction
- LangChain/LangGraph -- graph-based orchestration (45% never deploy, "Abstraction Tax")
- OpenTelemetry GenAI Semantic Conventions -- emerging standard for LLM tracing
- Langfuse v3 -- OTEL-native, MIT license, self-hostable
- Redis semantic caching -- 30-40% hit rate for repetitive LLM workloads
- SSE streaming -- dominant transport for LLM responses in 2025-2026
