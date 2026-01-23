#!/usr/bin/env bun

import { z } from 'zod';
import { RLM } from './src/rlm';

const SentimentSchema = z.object({
  score: z.number().int().min(1).max(5),
  confidence: z.number().min(0).max(1),
  reasoning: z.string().optional(),
  customerOnlyScore: z.number().int().min(1).max(5).optional(),
  prospectOnlyScore: z.number().int().min(1).max(5).optional(),
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

// Access private method via prototype
const rlm = new RLM('gpt-4o-mini');
const jsonSchema = (rlm as any).zodToJsonSchema(ConversationInsightsResponseSchema);

console.log('Full Schema:');
console.log(JSON.stringify(jsonSchema, null, 2));

console.log('\n\nSentiment Field Schema:');
console.log(JSON.stringify(jsonSchema.properties.sentiment, null, 2));

console.log('\n\nTop-level required fields:', jsonSchema.required);
console.log('Sentiment required fields:', jsonSchema.properties.sentiment.required);
