import * as dotenv from 'dotenv';
import { RLM, BridgeType } from '../src';

dotenv.config();

async function runTests() {
  console.log('🧪 TypeScript Integration Tests for Go Binary');
  console.log('='.repeat(60));
  console.log('');

  if (!process.env.OPENAI_API_KEY) {
    console.error('❌ Error: OPENAI_API_KEY not found!');
    console.error('');
    console.error('Please set up your API key in the .env file');
    process.exit(1);
  }

  const tests = [
    {
      name: 'Test 1: Simple context analysis',
      query: 'How many times does the word "test" appear?',
      context: 'This is a test. Another test here. Final test.',
      config: { max_iterations: 10 }
    },
    {
      name: 'Test 2: Counting errors in logs',
      query: 'Count how many ERROR entries are in the logs',
      context: `2024-01-01 INFO: System started
2024-01-01 ERROR: Connection failed
2024-01-01 INFO: Retrying
2024-01-01 ERROR: Timeout
2024-01-01 ERROR: Failed again
2024-01-01 INFO: Success`,
      config: { max_iterations: 10, temperature: 0.1 }
    },
    {
      name: 'Test 3: Extract information',
      query: 'List all the city names mentioned',
      context: 'I visited Paris last summer. Then went to London and finally Tokyo before returning to New York.',
      config: { max_iterations: 10 }
    },
    {
      name: 'Test 4: Long context',
      query: 'How many paragraphs are there?',
      context: Array(50).fill('This is paragraph number X. It contains some sample text.\\n\\n').join(''),
      config: { max_iterations: 15 }
    }
  ];

  let passed = 0;
  let failed = 0;

  for (const test of tests) {
    console.log(`\\n📝 ${test.name}`);
    console.log('-'.repeat(60));

    try {
      const rlm = new RLM('gpt-4o-mini', {
        ...test.config,
        api_key: process.env.OPENAI_API_KEY
      }, 'go');  // Force Go bridge

      const startTime = Date.now();
      const result = await rlm.completion(test.query, test.context);
      const duration = Date.now() - startTime;

      console.log(`✅ Passed`);
      console.log(`Result: ${result.result.substring(0, 100)}${result.result.length > 100 ? '...' : ''}`);
      console.log(`Stats: ${result.stats.llm_calls} LLM calls, ${result.stats.iterations} iterations`);
      console.log(`Duration: ${(duration / 1000).toFixed(2)}s`);

      passed++;
    } catch (error: any) {
      console.log(`❌ Failed: ${error.message}`);
      failed++;
    }
  }

  console.log('\\n' + '='.repeat(60));
  console.log('📋 Summary');
  console.log('='.repeat(60));
  console.log(`✅ Passed: ${passed}`);
  console.log(`❌ Failed: ${failed}`);
  console.log(`Total: ${tests.length}`);

  if (failed > 0) {
    process.exit(1);
  } else {
    console.log('\\n🎉 All tests passed!');
  }
}

runTests().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
