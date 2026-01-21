/**
 * Example: Using recursive-llm-ts with Amazon Bedrock
 * 
 * This demonstrates how to use AWS Bedrock models with the recursive-llm package.
 * Make sure to set your AWS credentials as environment variables:
 * - AWS_ACCESS_KEY_ID
 * - AWS_SECRET_ACCESS_KEY  
 * - AWS_REGION_NAME (e.g., us-east-1)
 */

import { RLM } from './src';  // In your project: import { RLM } from 'recursive-llm-ts';

const longDocument = `
[Your long document content here...]
`.repeat(50);

async function main() {
  // Using Amazon Bedrock with Claude 3 Sonnet
  const rlm = new RLM('bedrock/anthropic.claude-3-sonnet-20240229-v1:0', {
    max_iterations: 15,
    temperature: 0.7,
    // AWS credentials from environment variables
    // AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION_NAME
  });

  try {
    console.log('Processing with Amazon Bedrock (Claude 3 Sonnet)...\n');
    
    const result = await rlm.completion(
      'Summarize the key points from this document',
      longDocument
    );

    console.log('Result:', result.result);
    console.log('\nStatistics:');
    console.log('- LLM Calls:', result.stats.llm_calls);
    console.log('- Iterations:', result.stats.iterations);
    console.log('- Depth:', result.stats.depth);

    await rlm.cleanup();
  } catch (error) {
    console.error('Error:', error);
  }
}

main();
