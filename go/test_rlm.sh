#!/bin/bash
# Test script for Go RLM implementation

set -e

echo "=== Testing Go RLM Implementation ==="
echo ""

# Test 1: Simple FINAL response
echo "Test 1: Simple FINAL response"
cat <<'EOF' | ./rlm
{
  "model": "test-model",
  "query": "What is the answer?",
  "context": "The answer is 42.",
  "config": {
    "api_base": "http://mock-api.example.com",
    "api_key": "test-key",
    "max_depth": 5,
    "max_iterations": 30
  }
}
EOF
echo ""

# Test 2: Code extraction from markdown
echo "Test 2: JavaScript code execution"
cat <<'EOF' | ./rlm
{
  "model": "test-model",
  "query": "Count words",
  "context": "Hello World Test Document",
  "config": {
    "max_depth": 3,
    "max_iterations": 10
  }
}
EOF
echo ""

echo "=== All tests completed ==="
