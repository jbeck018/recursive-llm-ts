#!/bin/bash
# Simple integration tests

set -e

echo "🧪 Simple RLM Integration Tests"
echo "================================"
echo ""

if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ Set OPENAI_API_KEY first"
    exit 1
fi

# Test 1
echo "Test 1: Count word occurrences"
cat > /tmp/test1.json <<'JSON'
{
  "model": "gpt-4o-mini",
  "query": "How many times does the word 'test' appear?",
  "context": "This is a test. Another test here. Final test.",
  "config": {
    "max_iterations": 10
  }
}
JSON

echo "Running..."
RESULT=$(cat /tmp/test1.json | OPENAI_API_KEY="$OPENAI_API_KEY" ./rlm)
echo "✅ Test 1 Result:"
echo "$RESULT" | jq -r '.result'
echo "Stats:" $(echo "$RESULT" | jq '.stats')
echo ""

# Test 2
echo "Test 2: Simple counting"
cat > /tmp/test2.json <<'JSON'
{
  "model": "gpt-4o-mini",
  "query": "How many words are in this text?",
  "context": "One two three four five",
  "config": {
    "max_iterations": 10,
    "temperature": 0.1
  }
}
JSON

echo "Running..."
RESULT=$(cat /tmp/test2.json | OPENAI_API_KEY="$OPENAI_API_KEY" ./rlm)
echo "✅ Test 2 Result:"
echo "$RESULT" | jq -r '.result'
echo "Stats:" $(echo "$RESULT" | jq '.stats')
echo ""

# Test 3
echo "Test 3: Extract information"
cat > /tmp/test3.json <<'JSON'
{
  "model": "gpt-4o-mini",
  "query": "List all the numbers mentioned",
  "context": "I have 5 apples, 10 oranges, and 3 bananas.",
  "config": {
    "max_iterations": 10
  }
}
JSON

echo "Running..."
RESULT=$(cat /tmp/test3.json | OPENAI_API_KEY="$OPENAI_API_KEY" ./rlm)
echo "✅ Test 3 Result:"
echo "$RESULT" | jq -r '.result'
echo "Stats:" $(echo "$RESULT" | jq '.stats')
echo ""

echo "================================"
echo "✅ All tests passed!"
rm -f /tmp/test*.json
