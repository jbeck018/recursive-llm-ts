# CI/CD Migration Guide: Python to Go Binary

This guide explains how to update your CI/CD pipelines to build and use the Go binary instead of Python dependencies.

## Overview

The RLM library has migrated from Python (with RestrictedPython) to a Go binary for better performance, smaller footprint, and easier distribution.

### Before (Python)
- Required Python 3.9+ runtime
- Dependencies: `litellm`, `RestrictedPython`, etc.
- ~150MB memory footprint
- ~500ms startup time

### After (Go)
- Single binary (~15MB)
- No runtime dependencies
- ~50MB memory footprint
- ~10ms startup time

## NPM Package Changes

### package.json Updates

#### Remove Python Dependencies

```json
{
  "dependencies": {
    // REMOVE these Python bridge dependencies
    // "bunpy": "^0.1.0",
    // "pythonia": "^1.2.6"
  }
}
```

#### Update Scripts

```json
{
  "scripts": {
    "build": "tsc",
    "postinstall": "node scripts/build-go-binary.js",  // Builds Go binary
    "prepublishOnly": "npm run build",
    "test": "ts-node test/test-go.ts"  // Use Go-based tests
  }
}
```

#### Update Files

```json
{
  "files": [
    "dist",
    "go",                              // ADD: Go source code
    "scripts/build-go-binary.js",      // ADD: Build script
    // REMOVE: "recursive-llm/src"
    // REMOVE: "recursive-llm/pyproject.toml"
    // REMOVE: "scripts/install-python-deps.js"
  ]
}
```

## Build Script

### scripts/build-go-binary.js

```javascript
#!/usr/bin/env node
const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const goDir = path.join(__dirname, '..', 'go');
const binDir = path.join(__dirname, '..', 'bin');
const binaryName = process.platform === 'win32' ? 'rlm.exe' : 'rlm';
const binaryPath = path.join(binDir, binaryName);

// Create bin directory
if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

// Check if Go is installed
try {
  execSync('go version', { stdio: 'ignore' });
} catch {
  console.error('❌ Go is not installed');
  console.error('   Download from: https://golang.org/dl/');
  process.exit(1);
}

// Build the binary
console.log('🔨 Building Go binary...');
try {
  execSync(
    `go build -ldflags="-s -w" -o "${binaryPath}" ./cmd/rlm`,
    {
      cwd: goDir,
      stdio: 'inherit'
    }
  );
  console.log(`✅ Built ${binaryPath}`);
} catch (error) {
  console.error('❌ Failed to build Go binary');
  process.exit(1);
}
```

## GitHub Actions

### .github/workflows/test.yml

```yaml
name: Test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        node-version: [18.x, 20.x]
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: ${{ matrix.node-version }}
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install Node dependencies
        run: npm ci
      
      - name: Build TypeScript
        run: npm run build
      
      - name: Run Go tests
        run: |
          cd go
          go test ./internal/rlm/... -v
      
      - name: Run Go benchmarks
        run: |
          cd go
          go test ./internal/rlm/... -bench=. -benchtime=100ms
      
      # Integration tests require API key
      - name: Run integration tests
        if: env.OPENAI_API_KEY != ''
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          cd go
          ./integration_test.sh
```

### .github/workflows/publish.yml

```yaml
name: Publish

on:
  release:
    types: [created]

jobs:
  build-binaries:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            target: linux-amd64
          - os: macos-latest
            target: darwin-amd64
          - os: macos-latest
            target: darwin-arm64
          - os: windows-latest
            target: windows-amd64
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build binary
        run: |
          cd go
          go build -ldflags="-s -w" -o rlm-${{ matrix.target }} ./cmd/rlm
      
      - name: Upload binary
        uses: actions/upload-artifact@v3
        with:
          name: rlm-${{ matrix.target }}
          path: go/rlm-${{ matrix.target }}*
  
  publish-npm:
    needs: build-binaries
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '20.x'
          registry-url: 'https://registry.npmjs.org'
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Download binaries
        uses: actions/download-artifact@v3
      
      - name: Organize binaries
        run: |
          mkdir -p bin
          # Copy binaries to bin directory with standard names
          # Implement platform-specific logic
      
      - name: Install dependencies
        run: npm ci
      
      - name: Build
        run: npm run build
      
      - name: Publish to NPM
        run: npm publish
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

## Docker

### Dockerfile (Multi-stage)

```dockerfile
# Stage 1: Build Go binary
FROM golang:1.21-alpine AS go-builder

WORKDIR /build

# Copy Go source
COPY go/ ./go/

# Build binary
RUN cd go && \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o rlm ./cmd/rlm

# Stage 2: Build TypeScript
FROM node:20-alpine AS ts-builder

WORKDIR /build

# Copy package files
COPY package*.json ./
RUN npm ci

# Copy source
COPY src/ ./src/
COPY tsconfig.json ./

# Build TypeScript
RUN npm run build

# Stage 3: Runtime
FROM node:20-alpine

WORKDIR /app

# Copy built artifacts
COPY --from=go-builder /build/go/rlm /app/bin/rlm
COPY --from=ts-builder /build/dist /app/dist
COPY --from=ts-builder /build/node_modules /app/node_modules
COPY package.json /app/

# Make binary executable
RUN chmod +x /app/bin/rlm

ENV RLM_GO_BINARY=/app/bin/rlm

CMD ["node", "dist/index.js"]
```

## GitLab CI

### .gitlab-ci.yml

```yaml
stages:
  - test
  - build
  - publish

variables:
  GO_VERSION: "1.21"
  NODE_VERSION: "20"

# Test stage
test:go:
  stage: test
  image: golang:${GO_VERSION}
  script:
    - cd go
    - go test ./internal/rlm/... -v
    - go test ./internal/rlm/... -bench=. -benchtime=100ms

test:typescript:
  stage: test
  image: node:${NODE_VERSION}
  before_script:
    - apt-get update && apt-get install -y golang-${GO_VERSION}
    - npm ci
  script:
    - npm run build
    - cd go && go build -o rlm ./cmd/rlm

# Build binaries
build:linux:
  stage: build
  image: golang:${GO_VERSION}
  script:
    - cd go
    - CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o rlm-linux-amd64 ./cmd/rlm
  artifacts:
    paths:
      - go/rlm-linux-amd64

build:darwin:
  stage: build
  image: golang:${GO_VERSION}
  script:
    - cd go
    - CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o rlm-darwin-amd64 ./cmd/rlm
    - CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o rlm-darwin-arm64 ./cmd/rlm
  artifacts:
    paths:
      - go/rlm-darwin-*

# Publish
publish:npm:
  stage: publish
  image: node:${NODE_VERSION}
  only:
    - tags
  before_script:
    - apt-get update && apt-get install -y golang-${GO_VERSION}
    - npm ci
  script:
    - npm run build
    - npm publish
  environment:
    name: npm
```

## Local Development

### Pre-commit Hook

`.git/hooks/pre-commit`:

```bash
#!/bin/bash
set -e

echo "Running pre-commit checks..."

# Run Go tests
cd go
echo "🧪 Testing Go code..."
go test ./internal/rlm/... -v

# Run Go formatting
echo "🎨 Formatting Go code..."
go fmt ./...

# Build binary
echo "🔨 Building Go binary..."
go build -o rlm ./cmd/rlm

cd ..

# Run TypeScript build
echo "🔨 Building TypeScript..."
npm run build

echo "✅ All checks passed!"
```

Make executable: `chmod +x .git/hooks/pre-commit`

## Deployment

### Binary Distribution

#### Option 1: Include in NPM Package

Binaries bundled in `bin/` directory for each platform.

#### Option 2: Download on Postinstall

```javascript
// scripts/build-go-binary.js
async function downloadBinary() {
  const platform = process.platform;
  const arch = process.arch;
  const version = require('../package.json').version;
  
  const url = `https://github.com/yourorg/recursive-llm-ts/releases/download/v${version}/rlm-${platform}-${arch}`;
  
  // Download and extract
  // ...
}
```

### Environment Variables

```bash
# Override binary location
export RLM_GO_BINARY=/path/to/custom/rlm

# API keys (unchanged)
export OPENAI_API_KEY=sk-...
```

## Verification

### Checklist

- [ ] Go 1.21+ installed on CI/CD runners
- [ ] Build script creates binary successfully
- [ ] Unit tests pass (Go and TypeScript)
- [ ] Integration tests pass with real API
- [ ] Binary is executable on all platforms
- [ ] NPM package includes binary or downloads it
- [ ] Environment variables work correctly
- [ ] Docker images build successfully
- [ ] Documentation updated

### Test Commands

```bash
# Build binary
cd go && go build -o rlm ./cmd/rlm

# Run Go tests
cd go && go test ./internal/rlm/... -v

# Run benchmarks
cd go && go test ./internal/rlm/... -bench=.

# Run integration tests (requires API key)
cd go && ./integration_test.sh

# Test TypeScript integration
npm run build
ts-node test/test-go.ts

# Test binary directly
echo '{"model":"gpt-4o-mini","query":"test","context":"test","config":{}}' | ./go/rlm
```

## Troubleshooting

### Binary not found

**Error**: `Go RLM binary not found at /path/to/bin/rlm`

**Solution**:
1. Ensure Go is installed: `go version`
2. Run build script: `node scripts/build-go-binary.js`
3. Or set `RLM_GO_BINARY` environment variable

### Permission denied

**Error**: `EACCES: permission denied`

**Solution**:
```bash
chmod +x bin/rlm
# or
chmod +x go/rlm
```

### Go not installed on CI

**Solution**: Add Go setup step to CI configuration (see examples above)

### Cross-compilation issues

**Solution**: Use CGO_ENABLED=0 for pure Go binary:
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o rlm ./cmd/rlm
```

## Performance Comparison

| Metric | Python | Go | Improvement |
|--------|--------|-----|-------------|
| Binary size | N/A (runtime) | 15MB | N/A |
| Memory | ~150MB | ~50MB | 3x less |
| Startup | ~500ms | ~10ms | 50x faster |
| REPL exec | ~50ms | ~70μs | 700x faster |
| Cold start | ~2s | ~15ms | 133x faster |

## Migration Timeline

1. **Phase 1**: Run both Python and Go in parallel (feature flag)
2. **Phase 2**: Default to Go, Python as fallback
3. **Phase 3**: Remove Python dependencies completely
4. **Phase 4**: Optimize binary size and distribution

## Support

For issues or questions about the migration:
- GitHub Issues: https://github.com/yourorg/recursive-llm-ts/issues
- Documentation: See README.md and MIGRATION_STATUS.md
