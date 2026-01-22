#!/usr/bin/env python3
"""Direct comparison: Python vs Go RLM implementations."""

import sys
import json
import subprocess
import time
import os
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent / "recursive-llm" / "src"))
from rlm import RLM

API_KEY = os.getenv("OPENAI_API_KEY")
if not API_KEY:
    raise ValueError("OPENAI_API_KEY environment variable is required")

# Test cases
TESTS = [
    {
        "name": "Simple Count",
        "query": "How many times does 'test' appear?",
        "context": "This is a test. Another test here. Final test.",
        "expected": "3"
    },
    {
        "name": "Word Count",
        "query": "How many words?",
        "context": "One two three four five",
        "expected": "5"
    },
    {
        "name": "Extract Numbers",
        "query": "List all the numbers",
        "context": "I have 5 apples, 10 oranges, and 3 bananas.",
        "expected": "5"  # Any number mentioned
    },
    {
        "name": "Long Document - Count Sections",
        "query": "How many sections (paragraphs) are in this document?",
        "context": """Section 1: Introduction
This is the first section of our document. It contains important information about the topic.

Section 2: Background
The background section provides context and historical information needed to understand the main content.

Section 3: Methodology  
Here we describe the methods used in our research and analysis process.

Section 4: Results
This section presents the key findings from our analysis and research work.

Section 5: Discussion
We discuss the implications of our results and compare them with existing research.

Section 6: Conclusion
Finally, we summarize the main points and provide recommendations for future work.""",
        "expected": "6"
    },
    {
        "name": "Long Document - Count Keywords",
        "query": "Count how many times the word 'section' appears (case insensitive)",
        "context": """Section 1: Introduction
This is the first section of our document. It contains important information about the topic.

Section 2: Background
The background section provides context and historical information needed to understand the main content.

Section 3: Methodology  
Here we describe the methods used in our research and analysis process.

Section 4: Results
This section presents the key findings from our analysis and research work.

Section 5: Discussion
We discuss the implications of our results and compare them with existing research.

Section 6: Conclusion
Finally, we summarize the main points and provide recommendations for future work.""",
        "expected": "11"  # "Section" appears 6 times, "section" appears 5 times = 11 total
    }
]

def test_python(query, context):
    """Test Python implementation."""
    rlm = RLM('gpt-4o-mini', api_key=API_KEY, max_iterations=15)
    start = time.time()
    result = rlm.completion(query=query, context=context)
    duration = time.time() - start
    return {
        'result': result,
        'stats': rlm.stats,
        'duration': duration
    }

def test_go(query, context):
    """Test Go implementation."""
    payload = {
        "model": "gpt-4o-mini",
        "query": query,
        "context": context,
        "config": {
            "api_key": API_KEY,
            "max_iterations": 15
        }
    }
    
    start = time.time()
    result = subprocess.run(
        ["./go/rlm"],
        input=json.dumps(payload),
        capture_output=True,
        text=True
    )
    duration = time.time() - start
    
    if result.returncode != 0:
        raise Exception(f"Go failed: {result.stderr}")
    
    response = json.loads(result.stdout)
    return {
        'result': response['result'],
        'stats': response['stats'],
        'duration': duration
    }

def compare_results(name, expected, py_result, go_result):
    """Compare and display results."""
    print(f"\n{'='*70}")
    print(f"Test: {name}")
    print(f"{'='*70}")
    print(f"Expected: {expected}")
    print(f"\nPython Result: {py_result['result']}")
    print(f"Python Stats: LLM calls={py_result['stats']['llm_calls']}, "
          f"Iterations={py_result['stats']['iterations']}, "
          f"Time={py_result['duration']:.2f}s")
    
    print(f"\nGo Result:     {go_result['result']}")
    print(f"Go Stats:     LLM calls={go_result['stats']['llm_calls']}, "
          f"Iterations={go_result['stats']['iterations']}, "
          f"Time={go_result['duration']:.2f}s")
    
    # Check accuracy
    py_correct = expected in str(py_result['result'])
    go_correct = expected in str(go_result['result'])
    
    print(f"\n✓ Python: {'✅ CORRECT' if py_correct else '❌ INCORRECT'}")
    print(f"✓ Go:     {'✅ CORRECT' if go_correct else '❌ INCORRECT'}")
    
    # Performance comparison
    speedup = py_result['duration'] / go_result['duration']
    print(f"\n⚡ Performance: Go is {speedup:.2f}x faster")
    
    return py_correct and go_correct

def main():
    """Run comparison tests."""
    print("🔬 RLM Comparison: Python vs Go")
    print("="*70)
    
    results = []
    
    for test in TESTS:
        try:
            print(f"\n🔄 Running: {test['name']}...")
            
            print("  Testing Python...")
            py_result = test_python(test['query'], test['context'])
            
            print("  Testing Go...")
            go_result = test_go(test['query'], test['context'])
            
            passed = compare_results(
                test['name'],
                test['expected'],
                py_result,
                go_result
            )
            
            results.append((test['name'], passed))
            
        except Exception as e:
            print(f"\n❌ Test '{test['name']}' failed: {e}")
            results.append((test['name'], False))
    
    # Summary
    print(f"\n\n{'='*70}")
    print("📋 SUMMARY")
    print(f"{'='*70}")
    
    for name, passed in results:
        status = "✅" if passed else "❌"
        print(f"{status} {name}")
    
    passed_count = sum(1 for _, p in results if p)
    total_count = len(results)
    
    print(f"\n{passed_count}/{total_count} tests passed")
    
    if passed_count == total_count:
        print("\n🎉 ALL TESTS PASSED! Python and Go implementations are equivalent!")
        return 0
    else:
        print(f"\n⚠️  {total_count - passed_count} test(s) failed")
        return 1

if __name__ == "__main__":
    sys.exit(main())
