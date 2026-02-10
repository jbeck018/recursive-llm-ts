# Contributing to recursive-llm-ts

Thank you for your interest in contributing! This guide will help you get started.

## Development Setup

### Prerequisites

- **Node.js 18+** (LTS recommended)
- **Go 1.25+** (for building the Go binary)
- **npm** (comes with Node.js)

### Quick Start

```bash
# Clone the repository
git clone https://github.com/jbeck018/recursive-llm-ts.git
cd recursive-llm-ts

# Install dependencies (also builds Go binary via postinstall)
npm install

# Run the test suite
npm test

# Type-check without building
npm run typecheck

# Build TypeScript to dist/
npm run build
```

### Project Structure

```
recursive-llm-ts/
├── src/                    # TypeScript source
│   ├── rlm.ts             # Main RLM class (completion, streaming, batch)
│   ├── errors.ts          # Structured error hierarchy
│   ├── streaming.ts       # Async iterable streaming support
│   ├── cache.ts           # Exact-match caching layer
│   ├── retry.ts           # Retry with backoff, provider fallback
│   ├── events.ts          # Typed event emitter
│   ├── config.ts          # Configuration validation
│   ├── bridge-interface.ts # Type definitions for IPC bridge
│   ├── bridge-factory.ts  # Runtime detection, bridge selection
│   ├── go-bridge.ts       # Go binary IPC (spawn, stdin/stdout JSON)
│   ├── file-storage.ts    # Local and S3 file context building
│   ├── coordinator.ts     # Agent coordinator for complex schemas
│   ├── structured-types.ts # TypeScript interfaces for structured output
│   └── index.ts           # Public API exports
├── test/                   # Test suite (Vitest)
│   ├── errors.test.ts     # Error hierarchy tests
│   ├── cache.test.ts      # Cache layer tests
│   ├── events.test.ts     # Event emitter tests
│   ├── retry.test.ts      # Retry/fallback tests
│   ├── streaming.test.ts  # Streaming tests
│   ├── config.test.ts     # Config validation tests
│   ├── rlm.test.ts        # Main RLM class tests
│   └── *.ts               # Legacy manual test files
├── go/                     # Go source
│   ├── cmd/rlm/           # CLI entry point
│   ├── rlm/               # Core RLM engine
│   └── go.mod             # Go module
├── docs/                   # Documentation
├── bin/                    # Built Go binary
└── dist/                   # Built TypeScript (generated)
```

## Testing

### Running Tests

```bash
# Run all tests
npm test

# Run tests in watch mode (re-runs on file changes)
npm run test:watch

# Run with coverage
npm run test:coverage

# Run a specific test file
npx vitest run test/errors.test.ts

# Run tests matching a pattern
npx vitest run -t "classifyError"
```

### Writing Tests

- All tests live in `test/` and use the `*.test.ts` naming convention
- We use [Vitest](https://vitest.dev/) as our test runner
- Tests should be fast (no real API calls) -- mock the bridge layer
- Each new module should have a corresponding test file

Example test:

```typescript
import { describe, it, expect } from 'vitest';
import { MyModule } from '../src/my-module';

describe('MyModule', () => {
  it('does the expected thing', () => {
    const result = new MyModule();
    expect(result.value).toBe(42);
  });
});
```

### Go Tests

```bash
cd go
go test ./rlm/... -v        # All tests
go test ./rlm -run TestFoo   # Single test
go test ./rlm/... -cover     # With coverage
```

## Making Changes

### TypeScript Changes

1. Edit files in `src/`
2. Run `npm run typecheck` to verify types
3. Run `npm test` to verify behavior
4. Export new public APIs from `src/index.ts`
5. Add JSDoc with `@param`, `@returns`, `@example` for public methods

### Go Changes

1. Edit files in `go/rlm/`
2. Run `go test ./rlm/... -v` to verify
3. Run `go build ./cmd/rlm` to verify compilation
4. Update `go/rlm/types.go` for any new config fields

### Adding a New Feature

1. Create a new file in `src/` (e.g., `src/my-feature.ts`)
2. Add comprehensive JSDoc documentation
3. Export from `src/index.ts`
4. Create `test/my-feature.test.ts` with tests
5. Update `README.md` with usage examples
6. If the feature requires Go changes, update both sides

## Pull Request Guidelines

- **Keep PRs focused** -- one feature or fix per PR
- **Include tests** -- new code should have corresponding test coverage
- **Update docs** -- if you change the public API, update README.md
- **Run the full test suite** before submitting
- **Use conventional commits** -- e.g., `feat:`, `fix:`, `docs:`, `test:`

### Commit Message Format

```
<type>: <short description>

<optional body>
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`

### PR Checklist

- [ ] Tests pass (`npm test`)
- [ ] TypeScript compiles (`npm run typecheck`)
- [ ] Go tests pass (`cd go && go test ./rlm/...`)
- [ ] JSDoc added for public APIs
- [ ] README.md updated if public API changed
- [ ] No console.log/debug statements left in production code

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a detailed technical overview.

## Code Style

- TypeScript: We follow standard TypeScript conventions
- Use `interface` for object shapes, `type` for unions/intersections
- Prefer explicit types over `any`
- Use async/await over raw Promises
- Keep files focused -- one module per file

## Questions?

- Open an issue at https://github.com/jbeck018/recursive-llm-ts/issues
- Check existing issues and discussions before creating new ones
