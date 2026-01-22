import { RLM, RLMAgentCoordinator } from '../src';
import { z } from 'zod';
import { longCallTranscript, mediumCallTranscript } from './data/call-transcript';

// Test schemas
const sentimentAnalysisSchema = z.object({
  sentimentValue: z.number().min(1).max(5),
  sentimentExplanation: z.string(),
  phrases: z.array(z.object({
    sentimentValue: z.number().min(1).max(5),
    phrase: z.string()
  })),
  keyMoments: z.array(z.object({
    phrase: z.string(),
    type: z.enum(['churn_mention', 'personnel_change', 'competitive_mention', 'budget_concern', 'positive_feedback', 'feature_request'])
  }))
});

const simpleSchema = z.object({
  summary: z.string(),
  keyPoints: z.array(z.string())
});

async function testBackwardCompatibility() {
  console.log('\n=== BACKWARD COMPATIBILITY TEST ===\n');
  
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 30
  });

  try {
    const result = await rlm.completion(
      'Summarize the key points of this conversation in 2-3 sentences',
      mediumCallTranscript
    );

    console.log('✅ Backward compatibility test PASSED');
    console.log('Result:', result.result.substring(0, 200) + '...');
    console.log('Stats:', result.stats);
  } catch (error) {
    console.error('❌ Backward compatibility test FAILED:', error);
  } finally {
    await rlm.cleanup();
  }
}

async function testBasicStructuredOutput() {
  console.log('\n=== BASIC STRUCTURED OUTPUT TEST ===\n');
  
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 10
  });

  try {
    const result = await rlm.structuredCompletion(
      'Analyze this conversation and extract key information',
      mediumCallTranscript,
      simpleSchema,
      { parallelExecution: false } // Test non-parallel path
    );

    console.log('✅ Basic structured output test PASSED');
    console.log('Result:', JSON.stringify(result.result, null, 2));
    console.log('Stats:', result.stats);
    
    // Validate schema
    if (!result.result.summary || !Array.isArray(result.result.keyPoints)) {
      throw new Error('Schema validation failed');
    }
  } catch (error) {
    console.error('❌ Basic structured output test FAILED:', error);
  } finally {
    await rlm.cleanup();
  }
}

async function testParallelExecution() {
  console.log('\n=== PARALLEL EXECUTION TEST ===\n');
  
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 15
  });

  try {
    const start = Date.now();
    
    const result = await rlm.structuredCompletion(
      'Perform comprehensive sentiment analysis on this call transcript',
      longCallTranscript,
      sentimentAnalysisSchema,
      { parallelExecution: true, maxRetries: 3 }
    );

    const duration = Date.now() - start;

    console.log('✅ Parallel execution test PASSED');
    console.log(`Duration: ${duration}ms`);
    console.log('\nSentiment Value:', result.result.sentimentValue);
    console.log('\nExplanation:', result.result.sentimentExplanation);
    console.log('\nPhrases:', result.result.phrases.length, 'phrases found');
    console.log('Sample phrases:', result.result.phrases.slice(0, 3));
    console.log('\nKey Moments:', result.result.keyMoments.length, 'moments found');
    console.log('Sample moments:', result.result.keyMoments.slice(0, 3));
    console.log('\nStats:', result.stats);
    
    // Validate schema
    if (result.result.sentimentValue < 1 || result.result.sentimentValue > 5) {
      throw new Error('Sentiment value out of range');
    }
    if (!result.result.sentimentExplanation || result.result.sentimentExplanation.length < 20) {
      throw new Error('Sentiment explanation too short');
    }
  } catch (error) {
    console.error('❌ Parallel execution test FAILED:', error);
  } finally {
    await rlm.cleanup();
  }
}

async function testCoordinatorAPI() {
  console.log('\n=== COORDINATOR API TEST ===\n');
  
  const coordinator = new RLMAgentCoordinator(
    'gpt-4o-mini',
    {
      api_key: process.env.OPENAI_API_KEY,
      max_iterations: 15
    },
    'auto',
    {
      parallelExecution: true,
      maxRetries: 3
    }
  );

  try {
    const result = await coordinator.processComplex(
      'Analyze sentiment and extract key moments from this conversation',
      longCallTranscript,
      sentimentAnalysisSchema
    );

    console.log('✅ Coordinator API test PASSED');
    console.log('Sentiment:', result.result.sentimentValue);
    console.log('Phrases:', result.result.phrases.length);
    console.log('Key Moments:', result.result.keyMoments.length);
    console.log('Stats:', result.stats);
  } catch (error) {
    console.error('❌ Coordinator API test FAILED:', error);
  } finally {
    await coordinator.cleanup();
  }
}

async function testSequentialVsParallel() {
  console.log('\n=== SEQUENTIAL VS PARALLEL COMPARISON ===\n');
  
  // Sequential test
  const rlmSeq = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 15
  });

  try {
    const seqStart = Date.now();
    const seqResult = await rlmSeq.structuredCompletion(
      'Analyze sentiment comprehensively',
      mediumCallTranscript,
      sentimentAnalysisSchema,
      { parallelExecution: false }
    );
    const seqDuration = Date.now() - seqStart;

    console.log(`Sequential execution: ${seqDuration}ms`);
    console.log(`LLM calls: ${seqResult.stats.llm_calls}`);
    
    await rlmSeq.cleanup();

    // Parallel test
    const rlmPar = new RLM('gpt-4o-mini', {
      api_key: process.env.OPENAI_API_KEY,
      max_iterations: 15
    });

    const parStart = Date.now();
    const parResult = await rlmPar.structuredCompletion(
      'Analyze sentiment comprehensively',
      mediumCallTranscript,
      sentimentAnalysisSchema,
      { parallelExecution: true }
    );
    const parDuration = Date.now() - parStart;

    console.log(`Parallel execution: ${parDuration}ms`);
    console.log(`LLM calls: ${parResult.stats.llm_calls}`);
    console.log(`\nSpeedup: ${(seqDuration / parDuration).toFixed(2)}x`);

    await rlmPar.cleanup();

    console.log('\n✅ Sequential vs Parallel comparison PASSED');
  } catch (error) {
    console.error('❌ Sequential vs Parallel comparison FAILED:', error);
  }
}

async function testErrorHandling() {
  console.log('\n=== ERROR HANDLING TEST ===\n');
  
  const rlm = new RLM('gpt-4o-mini', {
    api_key: process.env.OPENAI_API_KEY,
    max_iterations: 5
  });

  try {
    // Test with very strict schema that might fail
    const strictSchema = z.object({
      exactNumber: z.literal(42),
      impossibleField: z.string().regex(/^IMPOSSIBLE_PATTERN_XYZ123$/)
    });

    try {
      await rlm.structuredCompletion(
        'Extract data',
        'Short text',
        strictSchema,
        { maxRetries: 2 }
      );
      console.log('❌ Expected error was not thrown');
    } catch (error) {
      console.log('✅ Error handling test PASSED - Error correctly thrown:', (error as Error).message.substring(0, 100));
    }
  } catch (error) {
    console.error('❌ Error handling test FAILED:', error);
  } finally {
    await rlm.cleanup();
  }
}

async function runAllTests() {
  console.log('🧪 Starting Structured Output Test Suite\n');
  console.log('='.repeat(60));

  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) {
    console.error('❌ OPENAI_API_KEY environment variable not set');
    process.exit(1);
  }

  try {
    // Run tests in sequence
    await testBackwardCompatibility();
    await testBasicStructuredOutput();
    await testParallelExecution();
    await testCoordinatorAPI();
    await testSequentialVsParallel();
    await testErrorHandling();

    console.log('\n' + '='.repeat(60));
    console.log('\n✅ ALL TESTS COMPLETED\n');
  } catch (error) {
    console.error('\n❌ TEST SUITE FAILED:', error);
    process.exit(1);
  }
}

// Run tests
runAllTests();
