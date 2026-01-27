# recursive-llm-ts

TypeScript/JavaScript package for [Recursive Language Models (RLM)](https://github.com/alexzhang13/rlm) - process unbounded context lengths with LLMs.

**Based on the paper**: [Recursive Language Models](https://alexzhang13.github.io/blog/2025/rlm/) by Alex Zhang and Omar Khattab (MIT, 2025)

## Features

✨ **Pure Go Implementation** - No Python dependencies required  
🚀 **50x Faster Startup** - Native binary vs Python runtime  
💾 **3x Less Memory** - Efficient Go implementation  
📦 **Single Binary** - Easy distribution and deployment  
🔄 **Unbounded Context** - Process 10M+ tokens without degradation  
🎯 **Provider Agnostic** - Works with OpenAI, Anthropic, Azure, Bedrock, local models  
🔍 **Structured Outputs** - Extract typed data with Zod schemas and parallel execution

## Installation

```bash
npm install recursive-llm-ts
```

### Prerequisites

- **Node.js 16+** or **Bun 1.0+**
- **Go 1.21+** (for building from source during install)

> **Note**: The package includes pre-built binaries for common platforms. Go is only needed if building from source.

### Go Binary (Automatic)

The `postinstall` script automatically builds the Go binary during installation. If Go is not available, the script will warn but not fail.

If you need to build manually:

```bash
# From the package directory
cd node_modules/recursive-llm-ts
node scripts/build-go-binary.js

# Or directly with Go
cd go && go build -o ../bin/rlm-go ./cmd/rlm
```

Override the binary path if needed:

```bash
export RLM_GO_BINARY=/custom/path/to/rlm-go
```

## Usage

### Automatic Runtime Detection (Recommended)

```typescript
import { RLM } from 'recursive-llm-ts';

// Initialize RLM with a model
// Automatically detects Node.js or Bun and uses appropriate bridge
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 15,
  api_key: process.env.OPENAI_API_KEY
});

// Process a query with unbounded context
const result = await rlm.completion(
  'What are the key points in this document?',
  longDocument
);

console.log(result.result);
console.log('Stats:', result.stats);
```

### Structured Outputs with Zod Schemas

Extract structured, typed data from any context using Zod schemas. Supports complex nested objects, arrays, enums, and automatic parallel execution for performance.

```typescript
import { RLM } from 'recursive-llm-ts';
import { z } from 'zod';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY
});

// Define your schema
const sentimentSchema = z.object({
  sentimentValue: z.number().min(1).max(5),
  sentimentExplanation: z.string(),
  keyPhrases: z.array(z.object({
    phrase: z.string(),
    sentiment: z.number()
  })),
  topics: z.array(z.enum(['pricing', 'features', 'support', 'competition']))
});

// Extract structured data
const result = await rlm.structuredCompletion(
  'Analyze the sentiment and extract key information',
  callTranscript,
  sentimentSchema
);

// result.result is fully typed!
console.log(result.result.sentimentValue); // number
console.log(result.result.keyPhrases); // Array<{phrase: string, sentiment: number}>
```

**Key Benefits:**
- ✅ **Type-safe** - Full TypeScript types from your Zod schema
- ✅ **Automatic validation** - Retries with error feedback if schema doesn't match
- ✅ **Parallel execution** - Complex schemas processed in parallel with goroutines (3-5x faster)
- ✅ **Deep nesting** - Supports arbitrarily nested objects and arrays
- ✅ **Enum support** - Validates enum values automatically

**Performance Options:**
```typescript
// Enable/disable parallel execution
const result = await rlm.structuredCompletion(
  query,
  context,
  schema,
  { 
    parallelExecution: true,  // default: true for complex schemas
    maxRetries: 3              // default: 3
  }
);
```

### Agent Coordinator (Advanced)

For complex multi-field schemas, use the coordinator API:

```typescript
import { RLMAgentCoordinator } from 'recursive-llm-ts';

const coordinator = new RLMAgentCoordinator(
  'gpt-4o-mini',
  { api_key: process.env.OPENAI_API_KEY },
  'auto',
  { parallelExecution: true }
);

const result = await coordinator.processComplex(
  'Extract comprehensive call analysis',
  transcript,
  complexSchema
);
```

### Bridge Selection

The package automatically uses the Go binary by default (if available). You can explicitly specify a bridge if needed:

```typescript
import { RLM } from 'recursive-llm-ts';

// Default: Auto-detection (prefers Go if available)
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 15,
  api_key: process.env.OPENAI_API_KEY
});

// Explicit: Force Go binary
const rlmGo = new RLM('gpt-4o-mini', {
  max_iterations: 15,
  api_key: process.env.OPENAI_API_KEY
}, 'go');

// Legacy: Use Python bridges (bunpy for Bun, pythonia for Node)
// Note: Requires separate Python dependencies
const rlmPython = new RLM('gpt-4o-mini', {}, 'bunpy');
```


## API

### `RLM`

Main class for recursive language model completions.

**Constructor:**
```typescript
constructor(model: string, rlmConfig?: RLMConfig)
```

- `model`: The LLM model to use (e.g., 'gpt-4o-mini', 'claude-3-sonnet')
- `rlmConfig`: Optional configuration object

**Methods:**

#### `completion(query: string, context: string): Promise<RLMResult>`

Process a query with the given context using recursive language models.

**Parameters:**
- `query`: The question or task to perform
- `context`: The context/document to process (can be arbitrarily large)

**Returns:**
- `Promise<RLMResult>`: Result containing the answer and statistics

#### `structuredCompletion<T>(query: string, context: string, schema: ZodSchema<T>, options?): Promise<StructuredRLMResult<T>>`

Extract structured, typed data from context using a Zod schema.

**Parameters:**
- `query`: The extraction task to perform
- `context`: The context/document to process
- `schema`: Zod schema defining the output structure
- `options`: Optional configuration
  - `parallelExecution?: boolean` - Enable parallel processing (default: true)
  - `maxRetries?: number` - Max validation retries (default: 3)

**Returns:**
- `Promise<StructuredRLMResult<T>>`: Typed result matching your schema

**Example:**
```typescript
const schema = z.object({ score: z.number(), summary: z.string() });
const result = await rlm.structuredCompletion('Analyze', doc, schema);
// result.result is typed as { score: number, summary: string }
```

#### `cleanup(): Promise<void>`

Clean up the bridge and free resources.

```typescript
await rlm.cleanup();
```

### Types

```typescript
interface RLMConfig {
  // Model configuration
  recursive_model?: string;      // Model to use for recursive calls (defaults to main model)
  
  // API configuration
  api_base?: string;             // Custom API base URL (for Azure, Bedrock, etc.)
  api_key?: string;              // API key for authentication
  
  // Execution limits
  max_depth?: number;            // Maximum recursion depth (default: 5)
  max_iterations?: number;       // Maximum REPL iterations per call (default: 30)
  pythonia_timeout?: number;     // Python bridge timeout in ms (default: 100000ms = 100s)
  go_binary_path?: string;       // Override path for Go binary (optional)
  
  // LiteLLM parameters - pass any additional parameters supported by LiteLLM
  api_version?: string;          // API version (e.g., for Azure)
  timeout?: number;              // Request timeout in seconds
  temperature?: number;          // Sampling temperature
  max_tokens?: number;           // Maximum tokens in response
  [key: string]: any;            // Any other LiteLLM parameters
}

interface RLMResult {
  result: string;
  stats: RLMStats;
}

interface RLMStats {
  llm_calls: number;
  iterations: number;
  depth: number;
}
```

## Environment Variables

Set your API key as an environment variable:

```bash
export OPENAI_API_KEY='your-api-key-here'
```

Or pass it in the configuration:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: 'your-api-key-here'
});
```

## Custom Providers

The Go binary uses an **OpenAI-compatible chat completion API** and works seamlessly with
[LiteLLM proxy](https://docs.litellm.ai/docs/simple_proxy) or any provider that supports the
OpenAI `/chat/completions` schema. This keeps the implementation provider-agnostic.

### Quick Reference

The Go binary speaks the **OpenAI chat completion schema**, so you can:

- Use OpenAI directly with `api_key`
- Use an OpenAI-compatible endpoint (Azure OpenAI, vLLM, Ollama)
- Use a LiteLLM proxy to reach providers like Anthropic, Bedrock, or Cohere

### Amazon Bedrock (via LiteLLM proxy)

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('bedrock/anthropic.claude-3-sonnet-20240229-v1:0', {
  api_base: 'http://localhost:4000', // LiteLLM proxy URL
  api_key: process.env.LITELLM_API_KEY,
  max_iterations: 15
});
```

### Azure OpenAI

```typescript
const rlm = new RLM('gpt-4o', {
  api_base: 'https://your-resource.openai.azure.com/openai/deployments/your-deployment',
  api_key: process.env.AZURE_API_KEY,
  api_version: '2024-02-15-preview' // Passed through to the OpenAI-compatible API
});
```

### Custom OpenAI-Compatible APIs

For providers with OpenAI-compatible APIs (e.g., local models, vLLM, Ollama):

```typescript
const rlm = new RLM('openai/your-model', {
  api_base: 'https://your-custom-endpoint.com/v1',
  api_key: 'your-key-here'
});
```

### Long-Running Processes

For large documents or queue-based processing that may take longer than the default 100s timeout:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 50,           // Allow more iterations for complex processing
  pythonia_timeout: 600000,     // 10 minutes timeout for Python bridge
  timeout: 300                  // 5 minutes timeout for LLM API calls
});

// Process very large document
const result = await rlm.completion(
  'Summarize all key points from this document',
  veryLargeDocument
);
```

### Other Providers

See the [LiteLLM documentation](https://docs.litellm.ai/docs/providers) for the complete list of supported providers and their configuration.

## Docker Deployment

### Basic Dockerfile with Go Build

To containerize your application that uses `recursive-llm-ts`, install Go 1.21+ in your Docker image to build the binary during `npm install`:

```dockerfile
FROM node:20-alpine

# Install Go 1.21+ for building the RLM binary
RUN apk add --no-cache go

# Set Go environment
ENV GOPATH=/go
ENV PATH=$PATH:$GOPATH/bin

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

ENV OPENAI_API_KEY=""
ENV NODE_ENV=production

CMD ["node", "your-app.js"]
```

### Multi-Stage Build (Recommended for Production)

For optimal image size and security, use a multi-stage build:

```dockerfile
# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS go-builder
WORKDIR /build
COPY go/go.mod go/go.sum ./
RUN go mod download
COPY go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o rlm-go ./cmd/rlm

# Stage 2: Build Node.js dependencies
FROM node:20-alpine AS node-builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev

# Stage 3: Final runtime image
FROM node:20-alpine
WORKDIR /app

COPY --from=node-builder /app/node_modules ./node_modules
COPY --from=go-builder /build/rlm-go ./bin/rlm-go
RUN chmod +x ./bin/rlm-go

COPY package*.json ./
COPY dist/ ./dist/

ENV NODE_ENV=production
ENV RLM_GO_BINARY=/app/bin/rlm-go
ENV OPENAI_API_KEY=""

CMD ["node", "dist/index.js"]
```

**Benefits:** Smaller image (~150MB vs ~500MB), faster builds with caching, more secure.

### Docker Compose

```yaml
version: '3.8'
services:
  app:
    build: .
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - NODE_ENV=production
    ports:
      - "3000:3000"
```

### Installing Go in Different Base Images

```dockerfile
# Alpine
RUN apk add --no-cache go

# Debian/Ubuntu
RUN apt-get update && apt-get install -y golang-1.21

# Or use pre-built binary (no Go required)
# Download from GitHub releases and copy to /app/bin/rlm-go
```

## Using the Go Module Directly

The Go implementation can be used as a standalone library in Go projects.

### Installation

```bash
go get github.com/jbeck018/recursive-llm-ts/go
```

### Usage

```go
package main

import (
    "fmt"
    "os"

    "github.com/jbeck018/recursive-llm-ts/go/rlm"
)

func main() {
    config := rlm.Config{
        MaxDepth:      5,
        MaxIterations: 30,
        APIKey:        os.Getenv("OPENAI_API_KEY"),
    }

    engine := rlm.New("gpt-4o-mini", config)

    answer, stats, err := engine.Completion(
        "What are the key points?",
        "Your long document here...",
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Answer: %s\n", answer)
    fmt.Printf("Stats: %d LLM calls, %d iterations\n",
        stats.LlmCalls, stats.Iterations)
}
```

### Structured Output

```go
schema := &rlm.JSONSchema{
    Type: "object",
    Properties: map[string]*rlm.JSONSchema{
        "summary": {Type: "string"},
        "score":   {Type: "number"},
    },
    Required: []string{"summary", "score"},
}

config := &rlm.StructuredConfig{
    Schema:     schema,
    MaxRetries: 3,
}

result, stats, err := engine.StructuredCompletion(
    "Summarize and score",
    document,
    config,
)
```

### Building from Source

```bash
cd go

# Standard build
go build -o rlm-go ./cmd/rlm

# Optimized (smaller binary)
go build -ldflags="-s -w" -o rlm-go ./cmd/rlm

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o rlm-linux-amd64 ./cmd/rlm
GOOS=darwin GOARCH=arm64 go build -o rlm-darwin-arm64 ./cmd/rlm
```

### Running Tests

```bash
cd go
go test -v ./rlm/...
```

## How It Works

This package provides a TypeScript wrapper around a Go implementation of Recursive-LLM, enabling seamless integration into Node.js/TypeScript applications without Python dependencies. The Go binary is built locally (or supplied via `RLM_GO_BINARY`) and invoked for completions.

The recursive-llm approach breaks down large contexts into manageable chunks and processes them recursively, allowing you to work with documents of any size without hitting token limits.

### Key Features

- ✅ **No Python dependency** - Go binary handles the full recursive loop
- ✅ **Provider-agnostic** - Works with OpenAI-compatible APIs or LiteLLM proxy
- ✅ **Type-safe** - Full TypeScript type definitions
- ✅ **Simple API** - Just `npm install` and start using

## Publishing

This package uses automated GitHub Actions workflows to publish to npm. See [RELEASE.md](RELEASE.md) for detailed instructions on publishing new versions.

**Quick start:**
```bash
npm version patch  # Bump version
git push origin main --tags  # Push tag
# Then create a GitHub release to trigger automatic npm publish
```

## License

MIT
