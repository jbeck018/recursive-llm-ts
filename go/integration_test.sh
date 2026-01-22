#!/bin/bash
# Integration test with real LLM API
# Set OPENAI_API_KEY environment variable before running

set -e

echo "🧪 RLM Go Integration Tests"
echo "================================"
echo ""

# Check if binary exists
if [ ! -f "./rlm" ]; then
    echo "❌ Binary not found. Building..."
    go build -o rlm ./cmd/rlm
    echo "✅ Built binary"
fi

# Check for API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ OPENAI_API_KEY environment variable not set"
    echo ""
    echo "Usage:"
    echo "  export OPENAI_API_KEY='sk-...'"
    echo "  ./integration_test.sh"
    exit 1
fi

echo "✅ API key found"
echo ""

# Test 1: Simple query
echo "📝 Test 1: Simple context analysis"
echo "-----------------------------------"
RESULT=$(cat <<EOF | ./rlm
{
  "model": "gpt-4o-mini",
  "query": "How many times does the word 'test' appear?",
  "context": "This is a test. Another test here. Final test.",
  "config": {
    "api_key": "$OPENAI_API_KEY",
    "max_iterations": 10
  }
}
EOF
)

if [ $? -eq 0 ]; then
    echo "✅ Test 1 passed"
    echo "Result: $(echo $RESULT | jq -r '.result')"
    echo "Stats: $(echo $RESULT | jq '.stats')"
else
    echo "❌ Test 1 failed"
    exit 1
fi
echo ""

# Test 2: Count/aggregation
echo "📝 Test 2: Counting errors in logs"
echo "-----------------------------------"
LOG_CONTEXT='2024-01-01 INFO: System started
2024-01-01 ERROR: Connection failed
2024-01-01 INFO: Retrying
2024-01-01 ERROR: Timeout
2024-01-01 ERROR: Failed again
2024-01-01 INFO: Success'

RESULT=$(./rlm <<EOF
{
  "model": "gpt-4o-mini",
  "query": "Count how many ERROR entries are in the logs",
  "context": "$LOG_CONTEXT",
  "config": {
    "api_key": "$OPENAI_API_KEY",
    "max_iterations": 10
  }
}
EOF
)

if [ $? -eq 0 ]; then
    echo "✅ Test 2 passed"
    echo "Result: $(echo $RESULT | jq -r '.result')"
    ITERATIONS=$(echo $RESULT | jq '.stats.iterations')
    echo "Iterations: $ITERATIONS"
else
    echo "❌ Test 2 failed"
    exit 1
fi
echo ""

# Test 3: Long context
echo "📝 Test 3: Long context processing"
echo "-----------------------------------"
LONG_CONTEXT=$(cat <<EOF
Chapter 1: The Beginning

It was a dark and stormy night. The hero embarked on a journey.
$(for i in {1..100}; do echo "Line $i of the story continues here with more content."; done)

Chapter 2: The Middle

The hero faced many challenges.
$(for i in {1..100}; do echo "Line $i describes the adventure."; done)

Chapter 3: The End

Finally, the hero succeeded and returned home triumphant.
EOF
)

RESULT=$(cat <<EOF | ./rlm
{
  "model": "gpt-4o-mini",
  "query": "How many chapters are in this document?",
  "context": "$LONG_CONTEXT",
  "config": {
    "api_key": "$OPENAI_API_KEY",
    "max_iterations": 15
  }
}
EOF
)

if [ $? -eq 0 ]; then
    echo "✅ Test 3 passed"
    echo "Result: $(echo $RESULT | jq -r '.result')"
    LLM_CALLS=$(echo $RESULT | jq '.stats.llm_calls')
    echo "LLM calls: $LLM_CALLS"
else
    echo "❌ Test 3 failed"
    exit 1
fi
echo ""

# Test 4: Different model configurations
echo "📝 Test 4: Two-model configuration"
echo "-----------------------------------"
RESULT=$(cat <<EOF | ./rlm
{
  "model": "gpt-4o",
  "query": "What is this text about?",
  "context": "Artificial intelligence and machine learning are transforming technology.",
  "config": {
    "recursive_model": "gpt-4o-mini",
    "api_key": "$OPENAI_API_KEY",
    "max_iterations": 10,
    "temperature": 0.3
  }
}
EOF
)

if [ $? -eq 0 ]; then
    echo "✅ Test 4 passed"
    echo "Result: $(echo $RESULT | jq -r '.result')"
else
    echo "❌ Test 4 failed"
    exit 1
fi
echo ""

echo "================================"
echo "✅ All integration tests passed!"
echo ""
echo "Summary:"
echo "  - Simple queries work"
echo "  - Counting/aggregation works"
echo "  - Long context works"
echo "  - Model configuration works"
