# RLM Pattern-Based Test Suite

## Overview

This test suite validates both Python and Go RLM implementations against the key patterns and capabilities described in the [RLM paper](https://alexzhang13.github.io/blog/2025/rlm/) by Alex Zhang and Omar Khattab (MIT, 2025).

**File**: `test_rlm_patterns.py`

## Test Categories

The suite includes **20 test cases** across **8 categories**, each testing specific RLM capabilities:

### 1. Peeking (2 tests)
**Paper Context**: "Similar to how a programmer will peek at a few entries when analyzing a dataset, the LM can peek at its context to observe any structure."

Tests:
- `peek_structured_list`: LM peeks at structured list data (100 items) to identify first 3
- `peek_json_structure`: LM peeks at JSON array to identify record structure

**What this validates**: The LM can intelligently sample the beginning of large contexts to understand structure without processing everything.

### 2. Grepping (3 tests)
**Paper Context**: "To reduce the search space of its context, rather than using semantic retrieval tools, the RLM with REPL can look for keywords or regex patterns to narrow down lines of interest."

Tests:
- `grep_email_addresses`: Find all emails using pattern matching
- `grep_question_lines`: Count lines ending with '?'
- `grep_ids_pattern`: Extract structured IDs (format: ID-####)

**What this validates**: The LM can use programmatic search (regex, string methods) to efficiently locate specific patterns in context.

### 3. Partition + Map (2 tests)
**Paper Context**: "A common pattern the RLM will perform is to chunk up the context into smaller sizes, and run several recursive LM calls to extract an answer or perform this semantic mapping."

Tests:
- `partition_sentiment_analysis`: Chunk 5 sections, classify each as positive/negative
- `partition_category_labeling`: Chunk 5 paragraphs, categorize as tech/sports/food

**What this validates**: The LM can recursively decompose context into chunks and apply semantic operations (classification, labeling) to each chunk.

### 4. Summarization (1 test)
**Paper Context**: "RLMs are a natural generalization of summarization-based strategies commonly used for managing the context window of LMs. RLMs commonly summarize information over subsets of the context for the outer LM to make decisions."

Tests:
- `summarize_sections`: Summarize 3 multi-paragraph sections (introduction, methods, results)

**What this validates**: The LM can recursively summarize subsets of long documents to extract key information.

### 5. Long-Input, Long-Output (2 tests)
**Paper Context**: "A particularly interesting and expensive case where LMs fail is in tasks that require long output generations. For example, you might give ChatGPT your list of papers and ask it to generate the BibTeX for all of them... RLMs with REPL environments should one-shot these tasks!"

Tests:
- `git_diff_tracking_simple`: Track sequence of git diffs to final file state (like LoCoDiff benchmark)
- `bibtex_generation`: Generate BibTeX for multiple papers

**What this validates**: The LM can programmatically process sequences of transformations and generate structured output.

### 6. Multi-Hop Reasoning (2 tests)
**Paper Context**: "The root LM can use regex queries to roughly narrow the context, then launch recursive LM calls over this context. This is particularly useful for arbitrary long context inputs, where indexing a retriever is expensive on the fly!"

Tests:
- `multi_hop_facts`: Chase relationships through multiple hops (Alice → Bob → Carol)
- `multi_hop_calculations`: Gather facts from different parts to compute total

**What this validates**: The LM can navigate complex reasoning chains across disconnected parts of context.

### 7. Edge Cases (3 tests)
Tests for robustness and correctness on edge cases:
- `edge_empty_result`: Handle queries with no matching results
- `edge_large_numbers`: Accurate computation with large numbers (sum = 24,444,442)
- `edge_nested_structure`: Count across nested hierarchical structure

**What this validates**: Implementation handles edge cases correctly without errors.

### 8. Context Rot Prevention (2 tests)
**Paper Context**: "Surprisingly, we find that RLMs also do not degrade in performance when given 10M+ tokens at inference time."

Tests:
- `context_rot_needle`: Find specific fact in 1000-line haystack (needle at line 500)
- `context_rot_last_item`: Extract last item from 500-item list

**What this validates**: Performance doesn't degrade with very long contexts (the core RLM promise).

## Usage

### Run all tests (both implementations):
```bash
python test_rlm_patterns.py
```

### Run specific implementation:
```bash
# Python only
python test_rlm_patterns.py --impl python

# Go only
python test_rlm_patterns.py --impl go
```

### Run specific category:
```bash
# Only peeking tests
python test_rlm_patterns.py --category peeking

# Only context rot tests
python test_rlm_patterns.py --category context_rot
```

### Run single test:
```bash
python test_rlm_patterns.py --test grep_email_addresses
```

## Output Format

### Per-Test Output
```
[1/20] peek_structured_list (peeking)
Query: What is the structure of this data? List the first 3 items.
Context: 1090 chars
  Running Python... ✅ 2.34s - All expected content found
    Result: The data consists of items numbered from 1 to 100...
    Stats: {'llm_calls': 2, 'iterations': 2, 'depth': 0}
  Running Go... ✅ 2.41s - All expected content found
    Result: The data consists of items numbered from 1 to 100...
    Stats: {'llm_calls': 2, 'iterations': 2, 'depth': 0}
  Parity: ✅ BOTH PASS
```

### Summary Output
```
================================================================================
SUMMARY BY CATEGORY
================================================================================

context_rot          | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
edge_cases           | Total:  3 | Python:  3/ 3 | Go:  2/ 3 | Both:  2/ 3
grepping             | Total:  3 | Python:  3/ 3 | Go:  3/ 3 | Both:  3/ 3
long_output          | Total:  2 | Python:  2/ 2 | Go:  1/ 2 | Both:  1/ 2
multi_hop            | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
partition_map        | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
peeking              | Total:  2 | Python:  2/ 2 | Go:  2/ 2 | Both:  2/ 2
summarization        | Total:  1 | Python:  1/ 1 | Go:  1/ 1 | Both:  1/ 1
```

### JSON Results File
Results are saved to `test_results_{impl}_{timestamp}.json`:

```json
{
  "timestamp": "2026-01-22T15:30:00",
  "model": "gpt-4o-mini",
  "tests": [
    {
      "name": "peek_structured_list",
      "category": "peeking",
      "query": "...",
      "context_length": 1090,
      "python": {
        "result": "...",
        "stats": {"llm_calls": 2, "iterations": 2, "depth": 0},
        "duration": 2.34,
        "valid": true,
        "validation_message": "All expected content found"
      },
      "go": {
        "result": "...",
        "stats": {"llm_calls": 2, "iterations": 2, "depth": 0},
        "duration": 2.41,
        "valid": true,
        "validation_message": "All expected content found"
      },
      "parity": "✅ BOTH PASS"
    }
  ]
}
```

## Validation Methods

Tests use three validation approaches:

1. **Exact Match**: Result must exactly match expected value
   ```python
   expected_exact="42"  # Result must be exactly "42"
   ```

2. **Contains Check**: Result must contain all expected substrings
   ```python
   expected_contains=["email1@example.com", "email2@test.org"]
   ```

3. **Custom Validator**: Lambda function for complex validation logic
   ```python
   validator=lambda r: (
       ("3" in r or "three" in r.lower()),
       "Should identify 3 items"
   )
   ```

## Key Metrics to Track

For each test, we track:
- **Result**: The final answer from RLM
- **LLM Calls**: Number of API calls (should be low for efficiency)
- **Iterations**: Number of REPL iterations (should be reasonable)
- **Duration**: Wall-clock time
- **Valid**: Whether result passes validation
- **Parity**: Whether Python and Go produce equivalent valid results

## Success Criteria

### Full Parity (Target)
- **20/20 tests pass** for both Python and Go
- **100% parity** - Both implementations produce equivalent valid results
- **Similar stats** - LLM calls and iterations should match within 10%

### Acceptable Threshold
- **18+/20 tests pass** (90%+ accuracy)
- **90%+ parity** - Most tests produce equivalent results
- Any failures are edge cases or LLM non-determinism issues

## Expected Patterns in Results

Based on the paper, successful RLM implementations should show:

1. **Low LLM call counts**: Most tests should use 2-4 LLM calls
   - 1 root LM call to decide strategy
   - 1-3 recursive calls for processing

2. **Low iteration counts**: Most tests should use 2-5 iterations
   - Not stuck in loops
   - Efficiently reaching FINAL() or FINAL_VAR()

3. **Recursive decomposition**: On partition_map tests, stats should show recursive calls

4. **No context degradation**: context_rot tests should succeed with high accuracy despite long context

## Why These Tests Matter

These tests directly validate the core claims from the RLM paper:

1. ✅ **Unbounded context**: Tests with 1000+ lines validate scaling beyond typical token limits
2. ✅ **No context rot**: Long context tests validate performance doesn't degrade
3. ✅ **Recursive decomposition**: Partition+map tests validate the core recursive pattern
4. ✅ **Programmatic processing**: Grepping/diff tracking validate REPL code execution
5. ✅ **Cost efficiency**: Low LLM call counts validate efficiency claims

## Extending the Test Suite

To add new tests, follow this pattern:

```python
TestCase(
    name="descriptive_name",
    category="existing_category",  # or new category
    query="What is your question?",
    context="The context to process...",
    
    # Choose ONE validation method:
    expected_exact="42",  # For exact matches
    # OR
    expected_contains=["substring1", "substring2"],  # For contains
    # OR
    validator=lambda r: (condition, "error message")  # For custom logic
)
```

Add your TestCase to the `TEST_CASES` list and run the suite!

## Troubleshooting

### "Python RLM not available"
Install Python RLM:
```bash
cd recursive-llm
pip install -e .
```

### "Go binary not found"
Build Go binary:
```bash
cd go
go build -o ../bin/rlm-go ./cmd/rlm
```

### "API key not found"
Set environment variable:
```bash
export OPENAI_API_KEY="sk-..."
```

### Tests timing out
Increase timeout in script (default: 120s):
```python
timeout=300  # 5 minutes
```

## Paper Reference

These tests are based on:
- **Paper**: "Recursive Language Models" by Alex Zhang and Omar Khattab
- **URL**: https://alexzhang13.github.io/blog/2025/rlm/
- **Full Paper**: https://arxiv.org/abs/2512.24601v1
- **Official Codebase**: https://github.com/alexzhang13/rlm

## Citation

If you use this test suite, please cite both the RLM paper and this implementation.
