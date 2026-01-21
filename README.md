# recursive-llm-ts

TypeScript bridge for [recursive-llm](https://github.com/grigori-gvadzabia/recursive-llm): Recursive Language Models for unbounded context processing.

## Installation

```bash
npm install recursive-llm-ts
```

**That's it!** Python is bundled via [JSPyBridge](https://github.com/extremeheat/JSPyBridge) - no additional setup required.

## Usage

```typescript
import { RLM } from 'recursive-llm-ts';

// Initialize RLM with a model
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

#### `cleanup(): Promise<void>`

Clean up the Python bridge and free resources.

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

This package uses [LiteLLM](https://github.com/BerriAI/litellm) under the hood, which supports **100+ LLM providers** including OpenAI, Anthropic, AWS Bedrock, Azure, Cohere, and more.

### Quick Reference

| Provider | Model Format | Required Config |
|----------|-------------|----------------|
| OpenAI | `gpt-4o`, `gpt-4o-mini` | `api_key` |
| Anthropic | `claude-3-5-sonnet-20241022` | `api_key` |
| AWS Bedrock | `bedrock/anthropic.claude-3-sonnet...` | AWS env vars |
| Azure OpenAI | `azure/gpt-4o` | `api_base`, `api_key`, `api_version` |
| Ollama | `ollama/llama3.2` | `api_base` (optional) |
| Custom | `openai/your-model` | `api_base`, `api_key` |

### Amazon Bedrock

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('bedrock/anthropic.claude-3-sonnet-20240229-v1:0', {
  api_key: process.env.AWS_ACCESS_KEY_ID,
  max_iterations: 15
});

// Set AWS credentials via environment variables:
// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION_NAME
```

### Azure OpenAI

```typescript
const rlm = new RLM('azure/gpt-4o', {
  api_base: 'https://your-resource.openai.azure.com',
  api_key: process.env.AZURE_API_KEY,
  api_version: '2024-02-15-preview' // Pass any LiteLLM params
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

### Other Providers

See the [LiteLLM documentation](https://docs.litellm.ai/docs/providers) for the complete list of supported providers and their configuration.

## How It Works

This package provides a TypeScript wrapper around the Python `recursive-llm` package, enabling seamless integration into Node.js/TypeScript applications. It uses [JSPyBridge (pythonia)](https://github.com/extremeheat/JSPyBridge) to provide direct Python interop - Python is bundled and runs in-process, so no external Python installation is needed.

The recursive-llm approach breaks down large contexts into manageable chunks and processes them recursively, allowing you to work with documents of any size without hitting token limits.

### Key Features

- ✅ **Zero Python setup** - Python runtime bundled via JSPyBridge
- ✅ **Direct interop** - Native Python-JavaScript bridge (no JSON serialization)
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
