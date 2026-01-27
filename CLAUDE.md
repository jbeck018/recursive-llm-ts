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

# Go tests
cd go
go test ./rlm/... -v                    # All tests verbose
go test ./rlm -run TestParser -v        # Single test
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
    ↓ calls OpenAI-compatible API
    ↓ executes JS code in REPL (goja)
    ↓ recursive decomposition
JSON response via stdout
    ↑
TypeScript (parses result)
```

### Key Files

**TypeScript:**
- `src/rlm.ts` - Main RLM class, Zod→JSON Schema conversion (450+ lines of schema logic)
- `src/go-bridge.ts` - Spawns Go binary, handles stdin/stdout JSON IPC
- `src/bridge-factory.ts` - Runtime detection, bridge selection (Go preferred, Python fallback)

**Go:**
- `go/cmd/rlm/main.go` - CLI entry, JSON I/O handler
- `go/rlm/rlm.go` - Core RLM engine, LLM orchestration
- `go/rlm/structured.go` - Structured output with schema validation, parallel execution
- `go/rlm/repl.go` - JavaScript REPL executor using goja engine

### Binary Resolution

The Go bridge looks for the binary in this order:
1. `RLMConfig.go_binary_path` parameter
2. `RLM_GO_BINARY` environment variable
3. `./bin/rlm-go` (npm package location)
4. `./go/rlm-go` (development location)

### Structured Output Flow

When `structuredCompletion()` is called:
1. TypeScript converts Zod schema → JSON Schema
2. Go decomposes complex schemas into subtasks
3. Parallel execution via goroutines (if enabled)
4. Validation with retry logic (instructor-style feedback)
5. Results merged and returned as typed object

## Go Module

The Go code is a public package importable by other Go projects:

```go
import "github.com/jbeck018/recursive-llm-ts/go/rlm"

engine := rlm.New("gpt-4o-mini", rlm.Config{APIKey: "..."})
result, stats, err := engine.Completion(query, context)
```

## Environment

Required: `OPENAI_API_KEY` (or pass via config)

Optional: `AZURE_API_KEY`, `RLM_GO_BINARY`

## CI/CD

- `.github/workflows/ci.yml` - Matrix test (Ubuntu/macOS/Windows × Node 18/20)
- `.github/workflows/go-release.yml` - Cross-platform binary builds on tag push
- `.github/workflows/publish.yml` - NPM publish on release
