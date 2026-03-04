import { describe, it, expect } from 'vitest';
import {
  RLMContextOverflowError,
  RLMError,
  classifyError,
} from '../src/errors';
import type { ContextOverflowConfig } from '../src/bridge-interface';
import { RLMBuilder } from '../src/rlm';

// ─── RLMContextOverflowError ────────────────────────────────────────────────

describe('RLMContextOverflowError', () => {
  it('creates error with correct properties', () => {
    const err = new RLMContextOverflowError({
      message: 'context overflow',
      modelLimit: 32768,
      requestTokens: 40354,
    });

    expect(err.modelLimit).toBe(32768);
    expect(err.requestTokens).toBe(40354);
    expect(err.code).toBe('CONTEXT_OVERFLOW');
    expect(err.retryable).toBe(true);
    expect(err.name).toBe('RLMContextOverflowError');
  });

  it('extends RLMError', () => {
    const err = new RLMContextOverflowError({
      message: 'overflow',
      modelLimit: 32768,
      requestTokens: 40354,
    });
    expect(err).toBeInstanceOf(RLMError);
    expect(err).toBeInstanceOf(Error);
  });

  it('includes token counts in suggestion', () => {
    const err = new RLMContextOverflowError({
      message: 'overflow',
      modelLimit: 32768,
      requestTokens: 40354,
    });
    expect(err.suggestion).toContain('40354');
    expect(err.suggestion).toContain('32768');
  });
});

// ─── classifyError: Context Overflow Detection ──────────────────────────────

describe('classifyError - context overflow detection', () => {
  it('classifies OpenAI context overflow', () => {
    const err = classifyError(
      new Error("This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens.")
    );
    expect(err).toBeInstanceOf(RLMContextOverflowError);
    const coe = err as RLMContextOverflowError;
    expect(coe.modelLimit).toBe(32768);
    expect(coe.requestTokens).toBe(40354);
  });

  it('classifies vLLM/Ray Serve wrapped error', () => {
    const vllmError = `(Request ID: cmpl-123) This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens. (client_request_id 456)`;
    const err = classifyError(new Error(vllmError));
    expect(err).toBeInstanceOf(RLMContextOverflowError);
    const coe = err as RLMContextOverflowError;
    expect(coe.modelLimit).toBe(32768);
    expect(coe.requestTokens).toBe(40354);
  });

  it('classifies Azure format', () => {
    const err = classifyError(
      new Error("This model's maximum context length is 8192 tokens, however you requested 12000 tokens")
    );
    expect(err).toBeInstanceOf(RLMContextOverflowError);
    const coe = err as RLMContextOverflowError;
    expect(coe.modelLimit).toBe(8192);
  });

  it('classifies context_length_exceeded error code', () => {
    const err = classifyError(new Error('context_length_exceeded'));
    // This matches via the keyword check, not the specific parser
    expect(err).toBeInstanceOf(RLMContextOverflowError);
  });

  it('classifies "too many input tokens" error', () => {
    const err = classifyError(new Error('too many input tokens for this model'));
    expect(err).toBeInstanceOf(RLMContextOverflowError);
  });

  it('does NOT classify regular 400 errors as overflow', () => {
    const err = classifyError(new Error('Bad request: invalid model parameter'));
    expect(err).not.toBeInstanceOf(RLMContextOverflowError);
  });

  it('does NOT classify rate limit errors as overflow', () => {
    const err = classifyError(new Error('429 Too Many Requests'));
    expect(err).not.toBeInstanceOf(RLMContextOverflowError);
  });

  it('classifies overflow from string input', () => {
    const err = classifyError("This model's maximum context length is 4096 tokens. However, your request has 5000 input tokens.");
    expect(err).toBeInstanceOf(RLMContextOverflowError);
  });
});

// ─── ContextOverflowConfig Type ─────────────────────────────────────────────

describe('ContextOverflowConfig', () => {
  it('allows all optional fields', () => {
    const config: ContextOverflowConfig = {};
    expect(config.enabled).toBeUndefined();
    expect(config.strategy).toBeUndefined();
  });

  it('accepts all strategy values including new ones', () => {
    const configs: ContextOverflowConfig[] = [
      { strategy: 'mapreduce' },
      { strategy: 'truncate' },
      { strategy: 'chunked' },
      { strategy: 'tfidf' },
      { strategy: 'textrank' },
      { strategy: 'refine' },
    ];
    expect(configs).toHaveLength(6);
    expect(configs[0].strategy).toBe('mapreduce');
    expect(configs[1].strategy).toBe('truncate');
    expect(configs[2].strategy).toBe('chunked');
    expect(configs[3].strategy).toBe('tfidf');
    expect(configs[4].strategy).toBe('textrank');
    expect(configs[5].strategy).toBe('refine');
  });

  it('accepts all configuration options', () => {
    const config: ContextOverflowConfig = {
      enabled: true,
      max_model_tokens: 32768,
      strategy: 'mapreduce',
      safety_margin: 0.15,
      max_reduction_attempts: 3,
    };
    expect(config.enabled).toBe(true);
    expect(config.max_model_tokens).toBe(32768);
    expect(config.safety_margin).toBe(0.15);
    expect(config.max_reduction_attempts).toBe(3);
  });
});

// ─── RLMBuilder: withContextOverflow ────────────────────────────────────────

describe('RLMBuilder.withContextOverflow', () => {
  it('enables overflow with default config', () => {
    const builder = new RLMBuilder('test-model')
      .withContextOverflow();

    // The builder config should have context_overflow enabled
    const config = (builder as any).config;
    expect(config.context_overflow).toBeDefined();
    expect(config.context_overflow.enabled).toBe(true);
  });

  it('accepts custom overflow config', () => {
    const builder = new RLMBuilder('test-model')
      .withContextOverflow({
        strategy: 'truncate',
        max_model_tokens: 16384,
        safety_margin: 0.2,
        max_reduction_attempts: 5,
      });

    const config = (builder as any).config;
    expect(config.context_overflow.enabled).toBe(true);
    expect(config.context_overflow.strategy).toBe('truncate');
    expect(config.context_overflow.max_model_tokens).toBe(16384);
    expect(config.context_overflow.safety_margin).toBe(0.2);
    expect(config.context_overflow.max_reduction_attempts).toBe(5);
  });

  it('chains with other builder methods', () => {
    const builder = new RLMBuilder('test-model')
      .apiKey('test-key')
      .withContextOverflow({ strategy: 'chunked' })
      .maxDepth(3);

    const config = (builder as any).config;
    expect(config.api_key).toBe('test-key');
    expect(config.context_overflow.strategy).toBe('chunked');
    expect(config.max_depth).toBe(3);
  });
});

// ─── Config Serialization ───────────────────────────────────────────────────

describe('ContextOverflow config passes to Go bridge', () => {
  it('includes context_overflow in config', () => {
    const builder = new RLMBuilder('test-model')
      .apiKey('test-key')
      .withContextOverflow({ strategy: 'mapreduce', max_model_tokens: 32768 });

    const config = (builder as any).config;
    expect(config.context_overflow).toEqual({
      enabled: true,
      strategy: 'mapreduce',
      max_model_tokens: 32768,
    });
  });

  it('serializes all config fields for Go bridge', () => {
    const overflowConfig: ContextOverflowConfig = {
      enabled: true,
      max_model_tokens: 65536,
      strategy: 'chunked',
      safety_margin: 0.1,
      max_reduction_attempts: 5,
    };

    // Verify the config is JSON-serializable (needed for Go bridge IPC)
    const serialized = JSON.stringify({ context_overflow: overflowConfig });
    const parsed = JSON.parse(serialized);
    expect(parsed.context_overflow.enabled).toBe(true);
    expect(parsed.context_overflow.max_model_tokens).toBe(65536);
    expect(parsed.context_overflow.strategy).toBe('chunked');
    expect(parsed.context_overflow.safety_margin).toBe(0.1);
    expect(parsed.context_overflow.max_reduction_attempts).toBe(5);
  });
});

// ─── Error Overflow Ratio ──────────────────────────────────────────────────

describe('RLMContextOverflowError ratios', () => {
  it('computes overflow percentage', () => {
    const err = new RLMContextOverflowError({
      message: 'overflow',
      modelLimit: 32768,
      requestTokens: 40354,
    });
    // 40354 / 32768 = 1.2314... so ~23% over
    const ratio = err.requestTokens / err.modelLimit;
    expect(ratio).toBeGreaterThan(1.2);
    expect(ratio).toBeLessThan(1.3);
  });

  it('handles large overflows', () => {
    const err = new RLMContextOverflowError({
      message: 'overflow',
      modelLimit: 4096,
      requestTokens: 100000,
    });
    const ratio = err.requestTokens / err.modelLimit;
    expect(ratio).toBeGreaterThan(24);
  });

  it('handles edge case with zero model limit', () => {
    const err = new RLMContextOverflowError({
      message: 'overflow',
      modelLimit: 0,
      requestTokens: 1000,
    });
    expect(err.modelLimit).toBe(0);
    expect(err.requestTokens).toBe(1000);
  });
});

// ─── New Strategies: TF-IDF, TextRank, Refine ──────────────────────────────

describe('TF-IDF strategy config', () => {
  it('accepts tfidf strategy in builder', () => {
    const builder = new RLMBuilder('test-model')
      .withContextOverflow({ strategy: 'tfidf' });

    const config = (builder as any).config;
    expect(config.context_overflow.strategy).toBe('tfidf');
    expect(config.context_overflow.enabled).toBe(true);
  });

  it('serializes tfidf config for Go bridge', () => {
    const config: ContextOverflowConfig = {
      enabled: true,
      strategy: 'tfidf',
      safety_margin: 0.2,
    };
    const serialized = JSON.stringify({ context_overflow: config });
    const parsed = JSON.parse(serialized);
    expect(parsed.context_overflow.strategy).toBe('tfidf');
  });
});

describe('TextRank strategy config', () => {
  it('accepts textrank strategy in builder', () => {
    const builder = new RLMBuilder('test-model')
      .withContextOverflow({ strategy: 'textrank' });

    const config = (builder as any).config;
    expect(config.context_overflow.strategy).toBe('textrank');
    expect(config.context_overflow.enabled).toBe(true);
  });

  it('serializes textrank config for Go bridge', () => {
    const config: ContextOverflowConfig = {
      enabled: true,
      strategy: 'textrank',
      max_model_tokens: 32768,
    };
    const serialized = JSON.stringify({ context_overflow: config });
    const parsed = JSON.parse(serialized);
    expect(parsed.context_overflow.strategy).toBe('textrank');
    expect(parsed.context_overflow.max_model_tokens).toBe(32768);
  });
});

describe('Refine strategy config', () => {
  it('accepts refine strategy in builder', () => {
    const builder = new RLMBuilder('test-model')
      .withContextOverflow({ strategy: 'refine' });

    const config = (builder as any).config;
    expect(config.context_overflow.strategy).toBe('refine');
    expect(config.context_overflow.enabled).toBe(true);
  });

  it('serializes refine config for Go bridge', () => {
    const config: ContextOverflowConfig = {
      enabled: true,
      strategy: 'refine',
      max_reduction_attempts: 5,
    };
    const serialized = JSON.stringify({ context_overflow: config });
    const parsed = JSON.parse(serialized);
    expect(parsed.context_overflow.strategy).toBe('refine');
    expect(parsed.context_overflow.max_reduction_attempts).toBe(5);
  });
});

describe('Strategy type safety', () => {
  it('all six strategies are type-safe', () => {
    const strategies: ContextOverflowConfig['strategy'][] = [
      'mapreduce', 'truncate', 'chunked', 'tfidf', 'textrank', 'refine',
    ];
    expect(strategies).toHaveLength(6);
    // Each should be assignable to the strategy type
    for (const s of strategies) {
      const config: ContextOverflowConfig = { strategy: s };
      expect(config.strategy).toBe(s);
    }
  });

  it('new strategies chain with all builder options', () => {
    for (const strategy of ['tfidf', 'textrank', 'refine'] as const) {
      const builder = new RLMBuilder('test-model')
        .apiKey('key')
        .withContextOverflow({
          strategy,
          max_model_tokens: 16384,
          safety_margin: 0.1,
          max_reduction_attempts: 4,
        })
        .maxDepth(5);

      const config = (builder as any).config;
      expect(config.context_overflow.strategy).toBe(strategy);
      expect(config.context_overflow.max_model_tokens).toBe(16384);
      expect(config.api_key).toBe('key');
      expect(config.max_depth).toBe(5);
    }
  });
});
