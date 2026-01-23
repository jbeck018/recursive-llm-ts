#!/usr/bin/env bun

import { z } from 'zod';
import { RLM } from './src/rlm';

const SentimentSchema = z.object({
  score: z.number().int().min(1).max(5),
  confidence: z.number().min(0).max(1),
  reasoning: z.string().optional(),
});

const ConversationInsightsResponseSchema = z.object({
  sentiment: SentimentSchema,
  phrases: z.array(z.object({
    phrase: z.string(),
    sentiment: z.number().int().min(1).max(5),
    confidence: z.number().min(0).max(1),
  })).default([]),
});

async function testValidationFeedback() {
  const rlm = new RLM(process.env.MODEL_ID || 'gpt-4o-mini', {
    api_base: process.env.BEDROCK_URL,
    api_key: process.env.BEDROCK_API_KEY,
    custom_llm_provider: 'openai',
    temperature: 0.3,
  });

  try {
    const query = 'Extract sentiment and phrases from this conversation';
    const context = `
Customer: I'm really frustrated with the recent updates. The interface is confusing now.
Support: I understand your concern. Let me help you navigate the new features.
Customer: I appreciate your help, but I think we might need to reconsider our subscription.
    `.trim();

    console.log('🧪 Testing structured completion with validation feedback...\n');
    
    const result = await rlm.structuredCompletion(
      query,
      context,
      ConversationInsightsResponseSchema,
      { maxRetries: 3, parallelExecution: true }
    );

    console.log('✅ Success! Result:');
    console.log(JSON.stringify(result.result, null, 2));
    console.log('\n📊 Stats:');
    console.log(JSON.stringify(result.stats, null, 2));
  } catch (error) {
    console.error('❌ Test failed:', error);
  } finally {
    await rlm.cleanup();
  }
}

testValidationFeedback();
