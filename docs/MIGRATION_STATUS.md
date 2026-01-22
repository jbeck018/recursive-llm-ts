# RLM Python to Go Migration Status

## Overview
This document tracks the migration of the Recursive Language Model (RLM) from Python to Go, based on the original implementation at https://github.com/alexzhang13/rlm.

## Migration Goals
- Convert Python RLM to Go binary
- Enable TypeScript code to call Go binary instead of requiring Python packages
- Maintain full feature parity with Python implementation
- Ensure all tests pass

## Architecture Comparison

### Python Implementation (Source)
```
recursive-llm/src/rlm/
├── __init__.py          - Package exports
├── core.py              - Main RLM class with async/sync completion
├── types.py             - Type definitions (TypedDict)
├── parser.py            - FINAL() and FINAL_VAR() extraction
├── prompts.py           - System prompt builder
└── repl.py              - RestrictedPython-based safe code executor
```

### Go Implementation (Current)
```
go/
├── cmd/rlm/main.go              - CLI entry point (JSON stdin/stdout)
└── internal/rlm/
    ├── rlm.go                   - Main RLM struct and Completion method
    ├── types.go                 - Config and stats types
    ├── parser.go                - FINAL() parsing with regex
    ├── prompt.go                - System prompt builder
    ├── repl.go                  - JavaScript REPL via goja
    └── openai.go                - OpenAI-compatible API client
```

## Feature Comparison

### ✅ Completed Features

1. **Core RLM Logic**
   - [x] Main completion loop
   - [x] Message history management
   - [x] Max iterations tracking
   - [x] Max depth tracking
   - [x] Statistics (llm_calls, iterations, depth)

2. **REPL Execution**
   - [x] JavaScript execution via goja (Python → JS translation)
   - [x] Code block extraction (```python, ```javascript, ```js)
   - [x] Output capturing
   - [x] Last expression evaluation
   - [x] Output truncation (2000 chars)
   - [x] Error handling

3. **Parser**
   - [x] FINAL() extraction with triple/double/single quotes
   - [x] FINAL_VAR() extraction
   - [x] is_final() check

4. **API Client**
   - [x] OpenAI-compatible API calls
   - [x] Custom api_base support
   - [x] API key handling
   - [x] Timeout support
   - [x] Error handling

5. **Configuration**
   - [x] Model selection
   - [x] Recursive model support
   - [x] Max depth/iterations
   - [x] Extra params passthrough
   - [x] Config from map

6. **Recursive Calls**
   - [x] recursive_llm() function in REPL env
   - [x] Depth tracking for recursive calls
   - [x] Model switching for recursive calls

### ⚠️ Key Differences (Python → Go/JS)

1. **Language Runtime**
   - **Python**: RestrictedPython for safe Python execution
   - **Go**: goja for JavaScript execution
   - **Impact**: LLM must write JavaScript instead of Python
   - **Solution**: System prompt explicitly says "JavaScript REPL"

2. **Standard Library Mapping**
   - **Python**: `re`, `json`, `math`, `datetime`, `Counter`, `defaultdict`
   - **Go/JS**: All mapped to JavaScript equivalents in `jsBootstrap`
   - **Completeness**: ✅ Core functions implemented

3. **Async/Sync**
   - **Python**: Full async/await with `acompletion()` and sync `completion()`
   - **Go**: Only synchronous `Completion()` (no goroutines for parallel recursion yet)
   - **Impact**: No parallel recursive calls optimization

4. **LLM Provider Support**
   - **Python**: Uses LiteLLM (100+ providers: OpenAI, Anthropic, Ollama, etc.)
   - **Go**: Direct OpenAI-compatible API only
   - **Impact**: Limited to OpenAI API format, but works with:
     - OpenAI
     - Azure OpenAI
     - Ollama (with OpenAI compatibility)
     - llama.cpp server
     - Any OpenAI-compatible endpoint

### 🔴 Missing Features

1. **Tests**
   - [ ] Unit tests for parser
   - [ ] Unit tests for REPL
   - [ ] Integration tests
   - [ ] Mock LLM tests
   - [ ] Recursive call tests

2. **Error Types**
   - [ ] MaxIterationsError (currently uses generic error)
   - [ ] MaxDepthError (currently uses generic error)
   - [ ] REPLError (currently uses generic error)

3. **Advanced REPL Features**
   - [ ] Timeout enforcement (Python has it but doesn't enforce, Go doesn't implement)
   - [ ] More stdlib modules (if needed)

4. **Optimizations**
   - [ ] Parallel recursive calls (Python can do via async, Go doesn't yet)
   - [ ] Connection pooling for API client
   - [ ] Request/response caching

### 📝 Python-Specific Compatibility Patch

The Python implementation has a bug workaround (lines 57-71 in core.py):
```python
# Patch for recursive-llm-ts bug where config is passed as 2nd positional arg
if isinstance(recursive_model, dict):
    config = recursive_model
    # ... extract config fields
```

**Go Implementation**: Not needed - Go uses explicit Config struct, no positional arg confusion.

## JavaScript REPL Environment

The Go implementation provides these JavaScript globals (matching Python REPL):

| Python | JavaScript | Status |
|--------|-----------|--------|
| `context` | `context` | ✅ |
| `query` | `query` | ✅ |
| `recursive_llm()` | `recursive_llm()` | ✅ |
| `re.findall()` | `re.findall()` | ✅ |
| `re.search()` | `re.search()` | ✅ |
| `len()` | `len()` | ✅ |
| `print()` | `print()` / `console.log()` | ✅ |
| `json.loads/dumps` | `json.loads/dumps` | ✅ |
| `math.*` | `math.*` | ✅ |
| `datetime` | `datetime` (Date) | ✅ |
| `Counter()` | `Counter()` | ✅ |
| `defaultdict()` | `defaultdict()` | ✅ |
| `range()` | `range()` | ✅ |
| `sorted()` | `sorted()` | ✅ |
| `sum()` | `sum()` | ✅ |
| `min()` | `min()` | ✅ |
| `max()` | `max()` | ✅ |
| `enumerate()` | `enumerate()` | ✅ |
| `zip()` | `zip()` | ✅ |
| `any()` | `any()` | ✅ |
| `all()` | `all()` | ✅ |

## System Prompt Comparison

**Python Version**: "You are a Recursive Language Model. You interact with context through a **Python REPL** environment."

**Go Version**: "You are a Recursive Language Model. You interact with context through a **JavaScript REPL** environment."

Both provide same instructions, just with appropriate language examples.

## CLI Interface

The Go binary accepts JSON on stdin:
```json
{
  "model": "gpt-4o-mini",
  "query": "What is this about?",
  "context": "Long document here...",
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

Returns JSON on stdout:
```json
{
  "result": "The answer...",
  "stats": {
    "llm_calls": 3,
    "iterations": 2,
    "depth": 0
  }
}
```

Errors go to stderr with exit code 1.

## Testing Status

### Manual Testing
- [x] Binary compiles successfully
- [ ] Basic completion works with real API
- [ ] REPL executes JavaScript correctly
- [ ] Parser extracts FINAL() correctly
- [ ] Recursive calls work
- [ ] Error handling works

### Unit Tests
- [ ] Parser tests
- [ ] REPL tests
- [ ] Type conversion tests

### Integration Tests
- [ ] Full completion flow
- [ ] Mock API tests
- [ ] Recursive call tests

## Next Steps

### High Priority
1. **Create comprehensive test suite**
   - Unit tests for each module
   - Integration tests with mock LLM
   - Test against Python test cases

2. **Add custom error types**
   - MaxIterationsError
   - MaxDepthError
   - REPLError

3. **Test with real LLM**
   - Basic completion
   - Recursive calls
   - Compare results with Python version

### Medium Priority
4. **Optimize REPL**
   - VM pooling for better performance
   - More stdlib functions if needed

5. **Improve error messages**
   - Better context in errors
   - Stack traces for REPL errors

6. **Documentation**
   - Go API documentation
   - Migration guide for users

### Low Priority
7. **Async/parallel support**
   - Goroutines for parallel recursive calls
   - Context cancellation

8. **Additional features**
   - Streaming support
   - Request/response logging
   - Metrics/observability

## Known Issues

1. **No async support**: Go implementation is fully synchronous
   - Python can do parallel recursive calls via asyncio
   - Go version processes recursively but sequentially

2. **Limited LLM provider support**: Only OpenAI-compatible APIs
   - Python has LiteLLM (100+ providers)
   - Go requires OpenAI-format API

3. **JavaScript vs Python**: LLMs trained more on Python
   - May need more iterations to get correct JS
   - System prompt guides towards JS

## Testing Strategy

1. **Unit Tests**: Test each module in isolation
2. **Integration Tests**: Test full flow with mock API
3. **Comparison Tests**: Run same queries on Python and Go, compare results
4. **Real-world Tests**: Test with actual long documents and LLM APIs

## Success Criteria

✅ Migration is complete when:
- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Same queries produce equivalent results (Python vs Go)
- [ ] Performance is comparable or better
- [ ] TypeScript can call Go binary successfully
- [ ] Documentation is complete

## Resources

- Original Paper: https://alexzhang13.github.io/blog/2025/rlm/
- Python Source: https://github.com/alexzhang13/rlm
- goja (Go JS engine): https://github.com/dop251/goja
