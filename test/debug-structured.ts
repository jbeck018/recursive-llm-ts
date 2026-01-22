import { RLM } from '../src';
import { z } from 'zod';

const simpleSchema = z.object({
  summary: z.string(),
  keyPoints: z.array(z.string())
});

async function test() {
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 10
  });

  const context = "The meeting went well. We discussed pricing and features.";
  
  try {
    console.log('Testing with parallelExecution: false');
    const result = await rlm.structuredCompletion(
      'Extract key information',
      context,
      simpleSchema,
      { parallelExecution: false }
    );
    
    console.log('Raw result:', JSON.stringify(result, null, 2));
  } catch (error) {
    console.error('Error:', error);
  } finally {
    await rlm.cleanup();
  }
}

test();
