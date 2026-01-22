# RLM Testing Guide - Quick Start

## Overview

We have created a comprehensive test suite based on the [RLM paper](https://alexzhang13.github.io/blog/2025/rlm/) patterns to validate full parity between Python and Go implementations.

## Test Files

1. **`test_rlm_patterns.py`** - Main test suite (20 tests across 8 categories)
2. **`RLM_PATTERN_TESTS.md`** - Detailed documentation of all test cases
3. **`compare_test.py`** - Simple comparison tests (already run)

## Quick Start

### 1. Ensure Prerequisites
```bash
# Check Go binary exists
ls bin/rlm-go

# Check Python RLM installed
cd recursive-llm && pip install -e . && cd ..

# Set API key
export OPENAI_API_KEY="your-key-here"
```

### 2. Run Full Test Suite
```bash
# Test both implementations (recommended)
python test_rlm_patterns.py

# Test only Go (faster, less API cost)
python test_rlm_patterns.py --impl go

# Test only Python
python test_rlm_patterns.py --impl python
```

### 3. Run Specific Categories
```bash
# Test only peeking pattern (2 tests)
python test_rlm_patterns.py --category peeking

# Test only grepping pattern (3 tests)
python test_rlm_patterns.py --category grepping

# Test context rot prevention (2 tests)
python test_rlm_patterns.py --category context_rot
```

### 4. Run Single Test
```bash
# Run just one test by name
python test_rlm_patterns.py --test grep_email_addresses
```

## Test Categories (20 Total Tests)

| Category | Tests | What It Validates |
|----------|-------|------------------|
| **peeking** | 2 | LM peeks at context structure without processing all |
| **grepping** | 3 | LM uses regex/search to find patterns efficiently |
| **partition_map** | 2 | LM chunks context and recursively processes |
| **summarization** | 1 | LM summarizes subsets of long documents |
| **long_output** | 2 | Git diff tracking, BibTeX generation (one-shot tasks) |
| **multi_hop** | 2 | Multi-hop reasoning across disconnected context parts |
| **edge_cases** | 3 | Robustness: empty results, large numbers, nested structures |
| **context_rot** | 2 | No degradation with very long contexts (1000+ lines) |

## Expected Results

### Success Criteria
- **Target**: 20/20 tests pass, 100% parity between Python and Go
- **Acceptable**: 18+/20 tests pass, 90%+ parity

### What "Parity" Means
Both implementations should:
1. Produce correct results (pass validation)
2. Use similar LLM call counts (within 10%)
3. Use similar iteration counts (within 10%)
4. Both succeed or both fail (not one passes, one fails)

## Output

### Console Output
```
[1/20] peek_structured_list (peeking)
Query: What is the structure of this data? List the first 3 items.
Context: 1090 chars
  Running Python... ✅ 2.34s - All expected content found
    Result: The data consists of items...
    Stats: {'llm_calls': 2, 'iterations': 2, 'depth': 0}
  Running Go... ✅ 2.41s - All expected content found
    Result: The data consists of items...
    Stats: {'llm_calls': 2, 'iterations': 2, 'depth': 0}
  Parity: ✅ BOTH PASS
```

### Summary
```
================================================================================
SUMMARY BY CATEGORY
================================================================================

context_rot          | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
edge_cases           | Total:  3 | Python:  3/ 3 | Go:  3/ 3 | Both:  3/ 3
grepping             | Total:  3 | Python:  3/ 3 | Go:  3/ 3 | Both:  3/ 3
long_output          | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
multi_hop            | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
partition_map        | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
peeking              | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
summarization        | Total:  1 | Python:  1/ 1 | Go:  1/ 1 | Both:  1/ 1
```

### JSON File
Results saved to `test_results_{impl}_{timestamp}.json` with full details for analysis.

## Cost Considerations

Running all 20 tests with both implementations = 40 API calls total.

**Estimated cost** (using gpt-4o-mini):
- Each test: ~2-4 LLM calls
- Average tokens per call: ~2000
- Total: ~160 LLM calls × 2000 tokens = ~320K tokens
- Cost: ~$0.05 - $0.10 total for full suite

**To reduce cost**:
```bash
# Test only Go (half the cost)
python test_rlm_patterns.py --impl go

# Test one category at a time
python test_rlm_patterns.py --category peeking
python test_rlm_patterns.py --category grepping
# etc.
```

## Debugging Failed Tests

### Check Test Details
If a test fails, look at:
1. **Result** - What did the LM return?
2. **Stats** - Did it use reasonable LLM calls/iterations?
3. **Validation message** - What was the expected vs actual?

### Common Issues

**"Extract Numbers" style failures** (Go returned "answer"):
- Check FINAL_VAR() extraction in `go/internal/rlm/parser.go`
- Compare with Python's `recursive-llm/src/rlm/parser.py`

**High iteration counts** (>10):
- LM might be stuck in loop
- Check prompt in `go/internal/rlm/prompt.go`
- Compare with `recursive-llm/src/rlm/prompts.py`

**Wrong results but similar stats**:
- LLM non-determinism (try running again)
- Or actual logic difference between implementations

**Timeouts**:
- Increase timeout in script (line 400): `timeout=300`
- Or test has issue with context size

## Integration with CI/CD

Add to your CI/CD pipeline:

```yaml
# .github/workflows/test.yml
- name: Run RLM Pattern Tests
  run: |
    export OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }}
    python test_rlm_patterns.py --impl go
  
- name: Upload Test Results
  uses: actions/upload-artifact@v2
  with:
    name: test-results
    path: test_results_*.json
```

## Next Steps After Testing

1. **If all tests pass** ✅
   - Document results in `PYTHON_VS_GO_COMPARISON.md`
   - Proceed with Python cleanup: `bash cleanup_python.sh`
   - Update package.json per `CI_CD_MIGRATION.md`
   - Deploy to production

2. **If some tests fail** ⚠️
   - Investigate specific failures
   - Compare LLM call patterns between Python and Go
   - Fix FINAL() extraction or REPL issues
   - Re-run failed tests: `python test_rlm_patterns.py --test failing_test_name`

3. **If many tests fail** ❌
   - Review Go implementation against Python source
   - Check system prompt differences
   - Validate REPL JavaScript environment
   - Test with simpler queries first

## Paper-Based Validation

These tests directly validate the RLM paper's key claims:

✅ **"RLMs can process unbounded context"** - Context rot tests (1000+ lines)  
✅ **"RLMs use peeking strategy"** - Peeking tests  
✅ **"RLMs use grepping strategy"** - Grepping tests  
✅ **"RLMs use partition + map"** - Partition map tests  
✅ **"RLMs one-shot long output tasks"** - Git diff tracking, BibTeX tests  
✅ **"RLMs maintain efficiency"** - Low LLM call counts across all tests  

## Resources

- **Full Paper**: https://arxiv.org/abs/2512.24601v1
- **Paper Website**: https://alexzhang13.github.io/blog/2025/rlm/
- **Official Python Code**: https://github.com/alexzhang13/rlm
- **Test Documentation**: `RLM_PATTERN_TESTS.md`
- **Migration Status**: `PYTHON_VS_GO_COMPARISON.md`

## Questions?

Review:
1. `RLM_PATTERN_TESTS.md` - Detailed test documentation
2. `PYTHON_VS_GO_COMPARISON.md` - Previous comparison results
3. `MIGRATION_STATUS.md` - Full migration details
4. Paper examples: https://alexzhang13.github.io/blog/2025/rlm/

---

**Ready to test?**
```bash
python test_rlm_patterns.py
```
