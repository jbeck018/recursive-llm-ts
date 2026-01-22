# RLM Go Binary

Go implementation of Recursive Language Models (RLM) based on the [original Python implementation](https://github.com/alexzhang13/rlm).

## Overview

This is a self-contained Go binary that implements the RLM algorithm, allowing language models to process extremely long contexts (100k+ tokens) by storing context as a variable and allowing recursive exploration.

**Key Difference from Python**: Uses JavaScript REPL instead of Python REPL for code execution.

## Building

```bash
# Build the binary
go build -o rlm ./cmd/rlm

# Run tests
go test ./internal/rlm/... -v

# Build with optimization
go build -ldflags="-s -w" -o rlm ./cmd/rlm
```

## Usage

The binary accepts JSON input on stdin and returns JSON output on stdout.

### Input Format

```json
{
  "model": "gpt-4o-mini",
  "query": "What are the main themes?",
  "context": "Your long document here...",
  "config": {
    "recursive_model": "gpt-4o-mini",
    "api_base": "https://api.openai.com/v1",
    "api_key": "sk-...",
    "max_depth": 5,
    "max_iterations": 30,
    "temperature": 0.7
  }
}
```

### Output Format

```json
{
  "result": "The main themes are...",
  "stats": {
    "llm_calls": 3,
    "iterations": 2,
    "depth": 0
  }
}
```

### Example

```bash
# Basic usage
echo '{
  "model": "gpt-4o-mini",
  "query": "Summarize this",
  "context": "Long document...",
  "config": {
    "api_key": "sk-..."
  }
}' | ./rlm

# With environment variable for API key
export OPENAI_API_KEY="sk-..."
echo '{
  "model": "gpt-4o-mini",
  "query": "What is this about?",
  "context": "Document text..."
}' | ./rlm
```

## Configuration Options

All fields in `config` are optional and have defaults:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `recursive_model` | string | Same as `model` | Cheaper model for recursive calls |
| `api_base` | string | `https://api.openai.com/v1` | API endpoint URL |
| `api_key` | string | From `OPENAI_API_KEY` env | API key for authentication |
| `max_depth` | int | 5 | Maximum recursion depth |
| `max_iterations` | int | 30 | Maximum REPL iterations per call |
| `temperature` | float | 0.7 | LLM temperature (0-2) |
| `timeout` | int | 60 | HTTP timeout in seconds |

Any other fields in `config` are passed as extra parameters to the LLM API.

## JavaScript REPL Environment

The LLM can write JavaScript code to explore the context. Available globals:

### Core Variables
- `context` - The document to analyze (string)
- `query` - The user's question (string)
- `recursive_llm(sub_query, sub_context)` - Recursively process sub-context

### String Operations
```javascript
context.slice(0, 100)           // First 100 chars
context.split('\n')             // Split by newline
context.length                  // String length
```

### Regex (Python-style API)
```javascript
re.findall("ERROR", context)    // Find all matches
re.search("ERROR", context)     // Find first match
```

### Built-in Functions
```javascript
len(context)                    // Length of string/array
print("hello")                  // Print output
console.log("hello")            // Same as print
```

### JSON
```javascript
json.loads('{"key":"value"}')   // Parse JSON
json.dumps({key: "value"})      // Stringify JSON
```

### Array Operations
```javascript
range(5)                        // [0, 1, 2, 3, 4]
range(2, 5)                     // [2, 3, 4]
sorted([3, 1, 2])               // [1, 2, 3]
sum([1, 2, 3])                  // 6
min([1, 2, 3])                  // 1
max([1, 2, 3])                  // 3
enumerate(['a', 'b'])           // [[0,'a'], [1,'b']]
zip([1, 2], ['a', 'b'])         // [[1,'a'], [2,'b']]
any([false, true])              // true
all([true, true])               // true
```

### Counting & Grouping
```javascript
Counter("hello")                // {h:1, e:1, l:2, o:1}
defaultdict(() => 0)            // Dict with default values
```

### Math
```javascript
Math.floor(3.7)                 // 3
Math.ceil(3.2)                  // 4
Math.max(1, 2, 3)               // 3
```

### Returning Results
```javascript
// Option 1: Direct answer (write as text, not code)
FINAL("The answer is 42")

// Option 2: Return a variable
const answer = "The answer is 42"
FINAL_VAR(answer)
```

## Supported LLM Providers

Works with any OpenAI-compatible API:

- **OpenAI**: `model: "gpt-4o"`, `model: "gpt-4o-mini"`
- **Azure OpenAI**: Set custom `api_base`
- **Ollama**: `api_base: "http://localhost:11434/v1"`, `model: "llama3.2"`
- **llama.cpp**: `api_base: "http://localhost:8000/v1"`
- **vLLM**: `api_base: "http://localhost:8000/v1"`
- Any other OpenAI-compatible endpoint

## Architecture

```
cmd/rlm/main.go              # CLI entry point (JSON I/O)
internal/rlm/
├── rlm.go                   # Core RLM logic
├── types.go                 # Config and stats types
├── parser.go                # FINAL() extraction
├── prompt.go                # System prompt builder
├── repl.go                  # JavaScript REPL (goja)
└── openai.go                # OpenAI API client
```

## Error Handling

Errors are written to stderr with exit code 1:

```bash
# Missing model
echo '{"query":"test"}' | ./rlm
# stderr: Missing model in request payload

# API error
echo '{
  "model": "invalid",
  "query": "test",
  "context": "test"
}' | ./rlm 2>&1
# stderr: LLM request failed (401): ...
```

## Testing

```bash
# Run all tests
go test ./internal/rlm/... -v

# Run specific test
go test ./internal/rlm -run TestParser -v

# With coverage
go test ./internal/rlm/... -cover

# Benchmark
go test ./internal/rlm/... -bench=. -benchmem
```

## Performance

- **Binary size**: ~15MB (uncompressed), ~5MB (compressed with UPX)
- **Memory**: ~50MB baseline + context size
- **Startup**: <10ms
- **REPL overhead**: ~1-2ms per iteration

## Comparison with Python Implementation

| Feature | Python | Go |
|---------|--------|-----|
| **REPL Language** | Python (RestrictedPython) | JavaScript (goja) |
| **LLM Providers** | 100+ via LiteLLM | OpenAI-compatible only |
| **Async Support** | ✅ Full async/await | ❌ Synchronous only |
| **Distribution** | Requires Python runtime | ✅ Single binary |
| **Startup Time** | ~500ms | ~10ms |
| **Memory Usage** | ~150MB | ~50MB |

## Known Limitations

1. **JavaScript vs Python**: LLMs are more familiar with Python, may need more iterations
2. **No async**: Recursive calls are sequential, not parallel
3. **OpenAI API only**: Doesn't support all LiteLLM providers
4. **No streaming**: Full response only

## Integration with TypeScript

From Node.js/TypeScript:

```typescript
import { spawn } from 'child_process';

interface RLMRequest {
  model: string;
  query: string;
  context: string;
  config?: {
    api_key?: string;
    max_depth?: number;
    max_iterations?: number;
  };
}

interface RLMResponse {
  result: string;
  stats: {
    llm_calls: number;
    iterations: number;
    depth: number;
  };
}

async function callRLM(request: RLMRequest): Promise<RLMResponse> {
  return new Promise((resolve, reject) => {
    const proc = spawn('./rlm');
    let stdout = '';
    let stderr = '';

    proc.stdout.on('data', (data) => { stdout += data; });
    proc.stderr.on('data', (data) => { stderr += data; });

    proc.on('close', (code) => {
      if (code !== 0) {
        reject(new Error(stderr || `Exit code ${code}`));
      } else {
        resolve(JSON.parse(stdout));
      }
    });

    proc.stdin.write(JSON.stringify(request));
    proc.stdin.end();
  });
}

// Usage
const result = await callRLM({
  model: 'gpt-4o-mini',
  query: 'What is this about?',
  context: longDocument,
  config: {
    api_key: process.env.OPENAI_API_KEY,
  },
});

console.log(result.result);
console.log(`Stats: ${result.stats.llm_calls} LLM calls`);
```

## Troubleshooting

### "Missing model in request payload"
Include the `model` field in your JSON input.

### "LLM request failed (401)"
Check your API key is correct and has sufficient credits.

### "max iterations exceeded"
Increase `max_iterations` in config, or simplify your query.

### "max recursion depth exceeded"
Increase `max_depth` in config.

### "Execution error: ReferenceError: xyz is not defined"
Check the JavaScript syntax. Use `console.log()` not `print()`, or ensure `print()` is available.

## Contributing

1. Write tests for new features
2. Ensure all tests pass: `go test ./internal/rlm/... -v`
3. Format code: `go fmt ./...`
4. Update documentation

## License

MIT License - Same as the original Python implementation

## Acknowledgments

- Based on [Recursive Language Models paper](https://alexzhang13.github.io/blog/2025/rlm/) by Alex Zhang and Omar Khattab (MIT)
- Original Python implementation: https://github.com/alexzhang13/rlm
- JavaScript engine: [goja](https://github.com/dop251/goja)
