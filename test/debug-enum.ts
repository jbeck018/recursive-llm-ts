import { RLM } from '../src';
import { z } from 'zod';

const schema = z.object({
  keyMoments: z.array(z.object({
    phrase: z.string(),
    type: z.enum(['churn_mention', 'personnel_change', 'competitive_mention', 'budget_concern', 'positive_feedback', 'feature_request'])
  }))
});

async function test() {
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 15
  });

  const context = "The customer mentioned they might cancel. The manager quit. We have budget issues.";
  
  try {
    const result = await rlm.structuredCompletion(
      'Extract key moments',
      context,
      schema,
      { parallelExecution: false }
    );
    
    console.log('Result:', JSON.stringify(result.result, null, 2));
  } catch (error) {
    console.error('Error:', error);
    // Try to see raw data
    const rawResult = await rlm.completion(
      'Extract key moments as JSON array with phrase and type fields',
      context
    );
    console.log('Raw LLM output:', rawResult.result);
  } finally {
    await rlm.cleanup();
  }
}

test();
