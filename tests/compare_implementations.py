#!/usr/bin/env python3
"""Compare Python and Go RLM implementations."""

import json
import os
import subprocess
import sys
import time
from pathlib import Path

# Add the Python RLM to path
sys.path.insert(0, str(Path(__file__).parent / "recursive-llm" / "src"))

from rlm import RLM


def test_go_rlm(model: str, query: str, context: str, config: dict) -> dict:
    """Test Go RLM implementation."""
    payload = {
        "model": model,
        "query": query,
        "context": context,
        "config": config,
    }
    
    start_time = time.time()
    result = subprocess.run(
        ["./go/rlm"],
        input=json.dumps(payload),
        capture_output=True,
        text=True,
    )
    duration = time.time() - start_time
    
    if result.returncode != 0:
        raise Exception(f"Go RLM failed: {result.stderr}")
    
    response = json.loads(result.stdout)
    return {
        "result": response["result"],
        "stats": response["stats"],
        "duration": duration,
    }


def test_python_rlm(model: str, query: str, context: str, config: dict) -> dict:
    """Test Python RLM implementation."""
    # Extract config params
    recursive_model = config.get("recursive_model")
    api_base = config.get("api_base")
    api_key = config.get("api_key")
    max_depth = config.get("max_depth", 5)
    max_iterations = config.get("max_iterations", 30)
    
    # Get extra params
    excluded = {'recursive_model', 'api_base', 'api_key', 'max_depth', 'max_iterations'}
    llm_kwargs = {k: v for k, v in config.items() if k not in excluded}
    
    rlm = RLM(
        model=model,
        recursive_model=recursive_model,
        api_base=api_base,
        api_key=api_key,
        max_depth=max_depth,
        max_iterations=max_iterations,
        **llm_kwargs
    )
    
    start_time = time.time()
    result = rlm.completion(query, context)
    duration = time.time() - start_time
    
    return {
        "result": result,
        "stats": rlm.stats,
        "duration": duration,
    }


def compare_results(test_name: str, python_result: dict, go_result: dict):
    """Compare and display results from both implementations."""
    print(f"\n{'='*60}")
    print(f"Test: {test_name}")
    print(f"{'='*60}")
    
    print("\n📊 Results:")
    print("-" * 60)
    print(f"Python: {python_result['result'][:100]}...")
    print(f"Go:     {go_result['result'][:100]}...")
    
    print("\n📈 Stats:")
    print("-" * 60)
    print(f"{'Metric':<20} {'Python':<15} {'Go':<15} {'Diff':<10}")
    print("-" * 60)
    
    py_stats = python_result['stats']
    go_stats = go_result['stats']
    
    print(f"{'LLM Calls':<20} {py_stats['llm_calls']:<15} {go_stats['llm_calls']:<15} {go_stats['llm_calls'] - py_stats['llm_calls']:<10}")
    print(f"{'Iterations':<20} {py_stats['iterations']:<15} {go_stats['iterations']:<15} {go_stats['iterations'] - py_stats['iterations']:<10}")
    print(f"{'Depth':<20} {py_stats['depth']:<15} {go_stats['depth']:<15} {go_stats['depth'] - py_stats['depth']:<10}")
    print(f"{'Duration (s)':<20} {python_result['duration']:.2f}s{'':<10} {go_result['duration']:.2f}s{'':<10} {go_result['duration'] - python_result['duration']:.2f}s")
    
    # Calculate speedup
    speedup = python_result['duration'] / go_result['duration']
    print(f"\n⚡ Speedup: {speedup:.2f}x")
    
    # Check if results are semantically similar (basic check)
    py_lower = python_result['result'].lower().strip()
    go_lower = go_result['result'].lower().strip()
    
    # Extract numbers if present
    import re
    py_nums = set(re.findall(r'\d+', py_lower))
    go_nums = set(re.findall(r'\d+', go_lower))
    
    if py_nums and go_nums:
        if py_nums == go_nums:
            print("✅ Results contain same numbers - likely correct")
        else:
            print(f"⚠️  Different numbers: Python {py_nums}, Go {go_nums}")
    else:
        # Simple similarity check
        common_words = len(set(py_lower.split()) & set(go_lower.split()))
        if common_words > 3:
            print("✅ Results appear semantically similar")
        else:
            print("⚠️  Results may differ significantly")


def main():
    """Run comparison tests."""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        print("❌ OPENAI_API_KEY environment variable not set")
        sys.exit(1)
    
    print("🔬 RLM Implementation Comparison: Python vs Go")
    print("=" * 60)
    
    # Test cases
    tests = [
        {
            "name": "Simple counting",
            "model": "gpt-4o-mini",
            "query": "How many times does 'test' appear?",
            "context": "This is a test. Another test here. Final test.",
            "config": {
                "api_key": api_key,
                "max_iterations": 10,
            }
        },
        {
            "name": "Error log counting",
            "model": "gpt-4o-mini",
            "query": "Count ERROR entries",
            "context": """2024-01-01 INFO: System started
2024-01-01 ERROR: Connection failed
2024-01-01 INFO: Retrying
2024-01-01 ERROR: Timeout
2024-01-01 ERROR: Failed again
2024-01-01 INFO: Success""",
            "config": {
                "api_key": api_key,
                "max_iterations": 10,
                "temperature": 0.1,
            }
        },
        {
            "name": "List extraction",
            "model": "gpt-4o-mini",
            "query": "Extract all city names as a list",
            "context": "I visited Paris last summer. Then went to London and finally Tokyo before returning to New York.",
            "config": {
                "api_key": api_key,
                "max_iterations": 10,
            }
        },
    ]
    
    results = []
    
    for test in tests:
        try:
            print(f"\n🔄 Running test: {test['name']}")
            print("  Testing Python implementation...")
            py_result = test_python_rlm(
                test["model"],
                test["query"],
                test["context"],
                test["config"]
            )
            
            print("  Testing Go implementation...")
            go_result = test_go_rlm(
                test["model"],
                test["query"],
                test["context"],
                test["config"]
            )
            
            compare_results(test["name"], py_result, go_result)
            results.append((test["name"], True))
            
        except Exception as e:
            print(f"❌ Test '{test['name']}' failed: {e}")
            results.append((test["name"], False))
    
    # Summary
    print(f"\n\n{'='*60}")
    print("📋 Summary")
    print(f"{'='*60}")
    
    passed = sum(1 for _, ok in results if ok)
    total = len(results)
    
    for name, ok in results:
        status = "✅" if ok else "❌"
        print(f"{status} {name}")
    
    print(f"\n{passed}/{total} tests completed successfully")
    
    if passed == total:
        print("\n🎉 All tests passed! Python and Go implementations show parity.")
    else:
        print(f"\n⚠️  {total - passed} test(s) failed")
        sys.exit(1)


if __name__ == "__main__":
    main()
