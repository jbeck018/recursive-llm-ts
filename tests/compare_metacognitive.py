#!/usr/bin/env python3
"""
Compare normal vs metacognitive mode on test suite
"""

import os
import sys
import json
import subprocess

# Load env
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")

def run_test(category, use_metacognitive=False):
    """Run test with specified mode"""
    env = os.environ.copy()
    
    # Prepare input
    test_query = "How many lines contain questions (end with '?')?"
    test_context = """
        This is a statement.
        Is this a question?
        Another statement here.
        What about this one?
        No question mark here
        Really, are you sure?
        Final statement.
    """
    
    input_data = {
        "model": "gpt-4o-mini",
        "query": test_query,
        "context": test_context,
        "config": {
            "api_key": OPENAI_API_KEY,
            "max_iterations": 30,
            "use_metacognitive": use_metacognitive
        }
    }
    
    import time
    start_time = time.time()
    
    result = subprocess.run(
        ["bin/rlm-go"],
        input=json.dumps(input_data),
        capture_output=True,
        text=True,
        timeout=180  # 3 minutes for complex queries
    )
    
    elapsed = time.time() - start_time
    
    if result.returncode != 0:
        return {"error": result.stderr, "elapsed": elapsed}
    
    try:
        output = json.loads(result.stdout)
        output["elapsed"] = elapsed
        return output
    except json.JSONDecodeError as e:
        return {"error": f"JSON decode error: {e}\nOutput: {result.stdout[:500]}", "elapsed": elapsed}

print("Testing Normal vs Metacognitive Mode")
print("=" * 60)

print("\n1. Testing NORMAL mode...")
normal = run_test("test", use_metacognitive=False)
if "error" in normal:
    print(f"   ❌ ERROR: {normal['error']}")
    print(f"   Elapsed: {normal.get('elapsed', 0):.2f}s")
else:
    print(f"   Result: {normal.get('result', 'ERROR')}")
    print(f"   Stats: {normal.get('stats', {})}")
    print(f"   Elapsed: {normal.get('elapsed', 0):.2f}s")

print("\n2. Testing METACOGNITIVE mode...")
meta = run_test("test", use_metacognitive=True)
if "error" in meta:
    print(f"   ❌ ERROR: {meta['error']}")
    print(f"   Elapsed: {meta.get('elapsed', 0):.2f}s")
else:
    print(f"   Result: {meta.get('result', 'ERROR')}")
    print(f"   Stats: {meta.get('stats', {})}")
    print(f"   Elapsed: {meta.get('elapsed', 0):.2f}s")

print("\n" + "=" * 60)
print("COMPARISON:")
print(f"  Normal:        {normal.get('result')} (calls: {normal.get('stats', {}).get('llm_calls', 0)})")
print(f"  Metacognitive: {meta.get('result')} (calls: {meta.get('stats', {}).get('llm_calls', 0)})")

# Check if both correct
expected = "3"
normal_correct = str(normal.get('result', '')).strip() == expected
meta_correct = str(meta.get('result', '')).strip() == expected

print(f"\n  Normal correct:        {'✅' if normal_correct else '❌'}")
print(f"  Metacognitive correct: {'✅' if meta_correct else '❌'}")
