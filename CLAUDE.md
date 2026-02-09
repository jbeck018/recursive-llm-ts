# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# TypeScript
npm run build          # Compile src/ to dist/
npm install            # Install deps + auto-build Go binary via postinstall

# Go binary (manual)
cd go && go build -ldflags="-s -w" -o ../bin/rlm-go ./cmd/rlm

# Release
./scripts/release.sh   # Interactive version bump, build, tag, push
```

## Testing

```bash
# TypeScript tests (no npm test configured - run directly)
ts-node test/test.ts                    # Main integration test
ts-node test/test-go.ts                 # Go bridge tests
ts-node test/structured-test.ts         # Structured output tests
ts-node test/zod-schema-converter-test.ts  # Schema conversion tests
ts-node test/new-features-test.ts       # Meta-agent, observability, config tests
ts-node test/file-storage-test.ts       # File storage tests (local + S3, 117 tests)

# Go tests
cd go
go test ./rlm/... -v                    # All tests verbose
go test ./rlm -run TestParser -v        # Single test
go test ./rlm -run TestSchema -v        # Schema validation tests
go test ./rlm -run TestObserver -v      # Observability tests
go test ./rlm -run TestMetaAgent -v     # Meta-agent tests
go test ./rlm -run TestExtract -v       # Balanced JSON extraction tests
go test ./rlm/... -cover                # With coverage
go test ./rlm/... -bench=. -benchmem    # Benchmarks

# Go integration tests (require built binary)
./go/test_simple.sh
./go/integration_test.sh
```

## Linting

```bash
cd go && golangci-lint run ./...        # Go linting (CI uses this)
```

## Architecture

This is a TypeScript/Go hybrid package implementing Recursive Language Models (RLM). The TypeScript layer provides the public API; the Go binary does the heavy lifting.

### IPC Bridge Pattern

```
TypeScript (RLM class)
    ↓ spawn binary, JSON via stdin
Go Binary (bin/rlm-go)
    ↓ meta-agent optimizes query (optional)
    ↓ calls OpenAI-compatible API
    ↓ executes JS code in REPL (goja)
    ↓ recursive decomposition
    ↓ observability: OTEL traces, Langfuse events, debug logs
JSON response via stdout (+ trace_events)
    ↑
TypeScript (parses result, exposes trace events)
```

### Key Files

**TypeScript:**
- `src/rlm.ts` - Main RLM class, Zod->JSON Schema conversion, trace event access, file-based completions
- `src/bridge-interface.ts` - Config types (RLMConfig, MetaAgentConfig, ObservabilityConfig, TraceEvent, FileStorageConfig)
- `src/file-storage.ts` - File storage providers (LocalFileStorage, S3FileStorage), FileContextBuilder, S3StorageError
- `src/go-bridge.ts` - Spawns Go binary, handles stdin/stdout JSON IPC
- `src/bridge-factory.ts` - Runtime detection, bridge selection (Go preferred, Python fallback)
- `src/structured-types.ts` - TypeScript interfaces for structured output (SubTask, CoordinatorConfig, SchemaDecomposition)

**Go:**
- `go/cmd/rlm/main.go` - CLI entry, JSON I/O handler
- `go/rlm/rlm.go` - Core RLM engine, LLM orchestration, observer/meta-agent integration
- `go/rlm/structured.go` - Structured output with schema validation, parallel execution
- `go/rlm/schema.go` - Google jsonschema-go bridge, SchemaValidator, schema inference
- `go/rlm/meta_agent.go` - Meta-agent for query optimization
- `go/rlm/observability.go` - OTEL tracing, Langfuse integration, debug logging
- `go/rlm/repl.go` - JavaScript REPL executor using goja engine
- `go/rlm/types.go` - Type definitions (Config, MetaAgentConfig, ObservabilityConfig)

### Binary Resolution

The Go bridge looks for the binary in this order:
1. `RLMConfig.go_binary_path` parameter
2. `RLM_GO_BINARY` environment variable
3. `./bin/rlm-go` (npm package location)
4. `./go/rlm-go` (development location)

### Structured Output Flow

When `structuredCompletion()` is called:
1. TypeScript converts Zod schema -> JSON Schema
2. Meta-agent optimizes query for extraction (if enabled)
3. Go validates schema with Google jsonschema-go
4. Go decomposes complex schemas into subtasks
5. Parallel execution via goroutines (if enabled)
6. Validation with retry logic (instructor-style feedback)
7. Results merged and returned as typed object
8. Trace events returned alongside results (if observability enabled)

### Meta-Agent Flow

When meta-agent is enabled:
1. Query is analyzed for specificity
2. If vague/short, meta-agent rewrites it
3. For structured queries, schema fields are referenced explicitly
4. Optimized query is passed to the RLM engine
5. Falls back to original query on error

### File Storage Flow

When file-based completions are used:
1. FileContextBuilder creates appropriate provider (LocalFileStorage or S3FileStorage)
2. Provider lists all files recursively (traverses nested directories)
3. Filters applied: extensions, include/exclude globs, max file size, max total size, max files
4. Files read and formatted with clear delimiters for LLM consumption
5. Per-file errors caught gracefully (skipped with reason, other files still included)
6. Built context string passed to normal completion/structuredCompletion flow

S3 credential resolution order:
1. Explicit `config.credentials`
2. Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
3. AWS SDK default credential chain (IAM role, `~/.aws/credentials`, ECS task role, etc.)

S3-compatible services supported: AWS S3, MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2

### Observability Flow

When observability is configured:
1. Observer is created with debug/OTEL/Langfuse config
2. Root trace span is created per completion call
3. Child spans track: LLM calls, REPL execution, meta-agent, validation
4. Events are collected and returned in response JSON
5. OTEL spans are exported to configured endpoint
6. Debug mode logs all operations to stderr/stdout/file

## Go Module

The Go code is a public package importable by other Go projects:

```go
import "github.com/jbeck018/recursive-llm-ts/go/rlm"

engine := rlm.New("gpt-4o-mini", rlm.Config{
    APIKey: "...",
    MetaAgent: &rlm.MetaAgentConfig{Enabled: true},
    Observability: &rlm.ObservabilityConfig{Debug: true},
})
result, stats, err := engine.Completion(query, context)
```

## Environment

Required: `OPENAI_API_KEY` (or pass via config)

Optional: `AZURE_API_KEY`, `RLM_GO_BINARY`, `RLM_DEBUG`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_HOST`

S3 File Storage: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`, `AWS_DEFAULT_REGION`

## CI/CD

- `.github/workflows/ci.yml` - Go tests + linting, then Node.js matrix (Ubuntu/macOS/Windows x Node 18/20)
- `.github/workflows/go-release.yml` - Cross-platform binary builds on tag push
- `.github/workflows/publish.yml` - NPM publish on release
