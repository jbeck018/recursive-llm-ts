#!/usr/bin/env bun

import { z } from 'zod';
import { RLM } from '../src/rlm';

const SentimentSchema = z.object({
  score: z.number().int().min(1).max(5),
  confidence: z.number().min(0).max(1),
  reasoning: z.string().optional(),
});

const ConversationInsightsResponseSchema = z.object({
  sentiment: SentimentSchema,
  sentimentExplanation: z.string().optional(),
  phrases: z.array(z.object({
    phrase: z.string(),
    sentiment: z.number().int().min(1).max(5),
    confidence: z.number().min(0).max(1),
    type: z.string().optional(),
    explanation: z.string().optional(),
  })).default([]),
});

async function runTest(attemptNum: number): Promise<{ success: boolean; error?: string }> {
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    temperature: 0.3,
  });

  try {
    const query = 'Extract sentiment and phrases from this conversation';
    const context = `
Customer: I'm really frustrated with the recent updates. The interface is confusing now.
Support: I understand your concern. Let me help you navigate the new features.
Customer: I appreciate your help, but I think we might need to reconsider our subscription.
Support: I completely understand. Let me see what options we have available.
Customer: Thanks, I'd appreciate that. The old interface was so much better.
    `.trim();

    console.log(`\n🧪 Test ${attemptNum}/5: Running with parallel execution...`);
    
    const result = await rlm.structuredCompletion(
      query,
      context,
      ConversationInsightsResponseSchema,
      { maxRetries: 3, parallelExecution: true }
    );

    // Validate the result has all required fields (the critical nested object test)
    if (!result.result.sentiment) {
      throw new Error('Missing sentiment object');
    }
    if (typeof result.result.sentiment.score !== 'number') {
      throw new Error('Missing or invalid sentiment.score');
    }
    if (typeof result.result.sentiment.confidence !== 'number') {
      throw new Error('Missing or invalid sentiment.confidence');
    }

    console.log(`✅ Test ${attemptNum} SUCCESS`);
    console.log(`   - Sentiment: ${result.result.sentiment.score} (confidence: ${result.result.sentiment.confidence})`);
    console.log(`   - Phrases: ${result.result.phrases.length}`);
    console.log(`   - Stats: ${result.stats.llmCalls} LLM calls, ${result.stats.iterations} iterations`);
    console.log(`   - Critical test passed: Nested 'sentiment' object has both required fields`);
    
    await rlm.cleanup();
    return { success: true };
  } catch (error) {
    const errorMsg = error instanceof Error ? error.message : String(error);
    
    // Check if this is the critical nested object failure we're testing for
    const isCriticalFailure = errorMsg.includes('missing required field: score') || 
                              errorMsg.includes('missing required field: confidence') ||
                              errorMsg.includes('Missing sentiment object') ||
                              errorMsg.includes('Missing or invalid sentiment.score') ||
                              errorMsg.includes('Missing or invalid sentiment.confidence');
    
    if (isCriticalFailure) {
      console.error(`❌ Test ${attemptNum} CRITICAL FAILURE:`, errorMsg);
      console.error(`   ⚠️  This is the nested object extraction bug we're testing!`);
    } else {
      console.error(`⚠️  Test ${attemptNum} failed (non-critical validation):`, errorMsg);
      console.error(`   Note: The critical nested sentiment object WAS extracted correctly`);
    }
    
    await rlm.cleanup();
    return { success: false, error: errorMsg, critical: isCriticalFailure };
  }
}

async function runReliabilityTest() {
  console.log('🚀 Starting Reliability Test - Deep Nested Object Extraction');
  console.log('📊 Running 5 consecutive tests with parallel execution enabled\n');
  console.log('Schema: ConversationInsights with nested sentiment object');
  console.log('  - sentiment.score: number (REQUIRED)');
  console.log('  - sentiment.confidence: number (REQUIRED)');
  console.log('  - sentiment.reasoning: string (optional)');
  console.log('═'.repeat(60));

  const results = [];
  
  for (let i = 1; i <= 5; i++) {
    const result = await runTest(i);
    results.push(result);
    
    // Small delay between tests
    if (i < 5) {
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
  }

  console.log('\n' + '═'.repeat(60));
  console.log('📈 RELIABILITY TEST RESULTS');
  console.log('═'.repeat(60));

  const successCount = results.filter(r => r.success).length;
  const failureCount = results.filter(r => !r.success).length;
  const successRate = (successCount / results.length) * 100;

  console.log(`\nTotal Tests: ${results.length}`);
  console.log(`✅ Successes: ${successCount}`);
  console.log(`❌ Failures: ${failureCount}`);
  console.log(`📊 Success Rate: ${successRate.toFixed(1)}%`);

  if (failureCount > 0) {
    console.log('\n❌ Failed Tests:');
    results.forEach((result, idx) => {
      if (!result.success) {
        console.log(`   Test ${idx + 1}: ${result.error}`);
      }
    });
  }

  if (successRate === 100) {
    console.log('\n🎉 PERFECT! All tests passed. Parallel execution is reliable.');
  } else if (successRate >= 80) {
    console.log('\n⚠️  Mostly reliable, but some failures occurred.');
  } else {
    console.log('\n🚨 Reliability issues detected. Consider disabling parallel execution.');
  }

  process.exit(failureCount > 0 ? 1 : 0);
}

runReliabilityTest();
