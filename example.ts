/**
 * Example usage of recursive-llm-ts
 * 
 * This example demonstrates how to use the RLM class to process
 * large documents that exceed typical LLM context windows.
 */

import { RLM } from './src';  // In your project: import { RLM } from 'recursive-llm-ts';

// Sample long document
const longDocument = `
Your very long document content here...
This can be thousands of pages and RLM will handle it recursively.
`.repeat(100);

async function main() {
  // Initialize RLM with your preferred model
  const rlm = new RLM('gpt-4o-mini', {
    max_iterations: 15,
    api_key: process.env.OPENAI_API_KEY,
    // recursive_model: 'gpt-3.5-turbo', // Optional: use different model for recursive calls
    // max_depth: 10 // Optional: maximum recursion depth
  });

  try {
    // Process the query with unlimited context
    const result = await rlm.completion(
      'Summarize the key points from this document',
      longDocument
    );

    console.log('Result:', result.result);
    console.log('\nStatistics:');
    console.log('- LLM Calls:', result.stats.llm_calls);
    console.log('- Iterations:', result.stats.iterations);
    console.log('- Depth:', result.stats.depth);

    // Clean up resources
    await rlm.cleanup();
  } catch (error) {
    console.error('Error:', error);
  }
}

main();
