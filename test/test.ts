import * as dotenv from 'dotenv';
import { RLM } from '../src/rlm';
import * as path from 'path';

dotenv.config();

const longDocument = `
The History of Artificial Intelligence

Introduction
Artificial Intelligence (AI) has transformed from a theoretical concept to a practical reality
over the past several decades. This document explores key milestones in AI development.

The 1950s: The Birth of AI
In 1950, Alan Turing published "Computing Machinery and Intelligence," introducing the famous
Turing Test. The term "Artificial Intelligence" was coined in 1956 at the Dartmouth Conference
by John McCarthy, Marvin Minsky, and others.

The 1960s-1970s: Early Optimism
During this period, researchers developed early AI programs like ELIZA (1966) and expert systems.
However, limitations in computing power led to the first "AI Winter" in the 1970s.

The 1980s-1990s: Expert Systems and Neural Networks
Expert systems became commercially successful in the 1980s. The backpropagation algorithm
revitalized neural network research in 1986.

The 2000s-2010s: Machine Learning Revolution
The rise of big data and powerful GPUs enabled deep learning breakthroughs. In 2012,
AlexNet won the ImageNet competition, marking a turning point for deep learning.

The 2020s: Large Language Models
GPT-3 (2020) and ChatGPT (2022) demonstrated unprecedented language understanding capabilities.
These models have billions of parameters and are trained on vast amounts of text data.

Conclusion
AI continues to evolve rapidly, with applications in healthcare, transportation, education,
and countless other domains. The future promises even more exciting developments.
` + 'Additional context paragraph. '.repeat(100);

async function main() {
  if (!process.env.OPENAI_API_KEY) {
    console.error('❌ Error: OPENAI_API_KEY not found!');
    console.error('');
    console.error('Please set up your API key in the .env file');
    return;
  }

  console.log('Query: What were the key milestones in AI development according to this document?');
  console.log(`Context length: ${longDocument.length} characters`);
  console.log('\nProcessing with RLM...\n');

  try {
    const rlm = new RLM('gpt-4o-mini', { 
      max_iterations: 15,
      api_key: process.env.OPENAI_API_KEY
    });

    const result = await rlm.completion(
      'What were the key milestones in AI development according to this document?',
      longDocument
    );

    console.log('Result:');
    console.log(result.result);
    console.log('\nStats:');
    console.log(`  LLM calls: ${result.stats.llm_calls}`);
    console.log(`  Iterations: ${result.stats.iterations}`);
  } catch (e) {
    console.error('Error:', e);
  }
}

main();
