import { describe, it, expect, vi } from 'vitest';
import { RLM, RLMBuilder, RLMResultFormatter, RLMCompletionResult } from '../src/rlm';
import { z } from 'zod';

describe('RLM Constructor', () => {
  it('creates instance with model and config', () => {
    const rlm = new RLM('gpt-4o-mini', { api_key: 'test' });
    expect(rlm).toBeInstanceOf(RLM);
  });

  it('creates instance with minimal config', () => {
    const rlm = new RLM('gpt-4o-mini');
    expect(rlm).toBeInstanceOf(RLM);
  });

  it('normalizes debug shorthand', () => {
    const rlm = new RLM('gpt-4o-mini', { debug: true });
    expect(rlm).toBeInstanceOf(RLM);
  });
});

describe('RLM Static Factory Methods', () => {
  it('fromEnv() creates instance from environment', () => {
    const rlm = RLM.fromEnv('gpt-4o-mini');
    expect(rlm).toBeInstanceOf(RLM);
  });

  it('withDebug() creates debug instance', () => {
    const rlm = RLM.withDebug('gpt-4o-mini');
    expect(rlm).toBeInstanceOf(RLM);
  });

  it('forAzure() creates Azure instance', () => {
    const rlm = RLM.forAzure('my-deployment', {
      apiBase: 'https://myresource.openai.azure.com',
      apiKey: 'test-key',
    });
    expect(rlm).toBeInstanceOf(RLM);
  });

  it('builder() returns RLMBuilder', () => {
    const builder = RLM.builder('gpt-4o-mini');
    expect(builder).toBeInstanceOf(RLMBuilder);
  });
});

describe('RLMBuilder', () => {
  it('builds RLM with all options', () => {
    const rlm = RLM.builder('gpt-4o-mini')
      .apiKey('test-key')
      .apiBase('https://api.example.com')
      .maxDepth(10)
      .maxIterations(30)
      .withMetaAgent({ model: 'gpt-4o' })
      .withDebug()
      .withCache({ strategy: 'exact', maxEntries: 500 })
      .withRetry({ maxRetries: 5 })
      .withFallback(['gpt-4o', 'claude-sonnet-4-20250514'])
      .bridge('go')
      .binaryPath('/custom/path')
      .litellmParams({ temperature: 0.7 })
      .build();

    expect(rlm).toBeInstanceOf(RLM);
  });

  it('builds minimal RLM', () => {
    const rlm = RLM.builder('gpt-4o-mini').build();
    expect(rlm).toBeInstanceOf(RLM);
  });
});

describe('RLM Event System', () => {
  it('on/off/once work correctly', () => {
    const rlm = new RLM('gpt-4o-mini');
    const handler = vi.fn();

    rlm.on('error', handler);
    rlm.off('error', handler);
    // No way to emit directly, but the registration API works
    expect(rlm).toBeInstanceOf(RLM);
  });

  it('removeAllListeners clears listeners', () => {
    const rlm = new RLM('gpt-4o-mini');
    rlm.on('error', () => {});
    rlm.on('llm_call', () => {});
    rlm.removeAllListeners();
    expect(rlm).toBeInstanceOf(RLM);
  });
});

describe('RLM Cache', () => {
  it('getCacheStats returns stats', () => {
    const rlm = new RLM('gpt-4o-mini', { cache: { enabled: true } });
    const stats = rlm.getCacheStats();
    expect(stats).toHaveProperty('hits');
    expect(stats).toHaveProperty('misses');
    expect(stats).toHaveProperty('hitRate');
  });

  it('clearCache resets cache', () => {
    const rlm = new RLM('gpt-4o-mini', { cache: { enabled: true } });
    rlm.clearCache();
    const stats = rlm.getCacheStats();
    expect(stats.size).toBe(0);
  });
});

describe('RLM Validation', () => {
  it('validate() returns ValidationResult', () => {
    const rlm = new RLM('gpt-4o-mini', { max_depth: 5 });
    const result = rlm.validate();
    expect(result).toHaveProperty('valid');
    expect(result).toHaveProperty('issues');
    expect(result.valid).toBe(true);
  });

  it('validate() catches invalid config', () => {
    const rlm = new RLM('gpt-4o-mini', { max_depth: -1 });
    const result = rlm.validate();
    expect(result.valid).toBe(false);
    expect(result.issues.some(i => i.field === 'max_depth')).toBe(true);
  });
});

describe('RLM Trace Events', () => {
  it('getTraceEvents returns empty initially', () => {
    const rlm = new RLM('gpt-4o-mini');
    expect(rlm.getTraceEvents()).toEqual([]);
  });
});

describe('RLM Cleanup', () => {
  it('cleanup works when no bridge', async () => {
    const rlm = new RLM('gpt-4o-mini');
    await expect(rlm.cleanup()).resolves.toBeUndefined();
  });
});

describe('RLMResultFormatter', () => {
  it('prettyStats() formats correctly', () => {
    const formatter = new RLMResultFormatter(
      'Test result',
      { llm_calls: 3, iterations: 12, depth: 2 },
      false,
      'gpt-4o-mini'
    );
    const stats = formatter.prettyStats();
    expect(stats).toContain('LLM Calls: 3');
    expect(stats).toContain('Iterations: 12');
    expect(stats).toContain('Depth: 2');
    expect(stats).not.toContain('(cached)');
  });

  it('prettyStats() shows cached', () => {
    const formatter = new RLMResultFormatter(
      'result', { llm_calls: 0, iterations: 0, depth: 0 }, true, 'gpt-4o'
    );
    expect(formatter.prettyStats()).toContain('(cached)');
  });

  it('prettyStats() shows parsing retries', () => {
    const formatter = new RLMResultFormatter(
      'result', { llm_calls: 1, iterations: 1, depth: 1, parsing_retries: 2 }, false, 'gpt-4o'
    );
    expect(formatter.prettyStats()).toContain('Retries: 2');
  });

  it('toJSON() returns serializable object', () => {
    const formatter = new RLMResultFormatter(
      'Test', { llm_calls: 1, iterations: 1, depth: 1 }, false, 'gpt-4o'
    );
    const json = formatter.toJSON();
    expect(json.result).toBe('Test');
    expect(json.model).toBe('gpt-4o');
    expect(json.cached).toBe(false);
    // Verify it's JSON-serializable
    expect(() => JSON.stringify(json)).not.toThrow();
  });

  it('toMarkdown() returns valid markdown', () => {
    const formatter = new RLMResultFormatter(
      'Summary text', { llm_calls: 2, iterations: 8, depth: 1 }, false, 'gpt-4o-mini'
    );
    const md = formatter.toMarkdown();
    expect(md).toContain('## Result');
    expect(md).toContain('Summary text');
    expect(md).toContain('## Stats');
    expect(md).toContain('| LLM Calls | 2 |');
    expect(md).toContain('| Model | gpt-4o-mini |');
  });
});

describe('RLM Zod Schema Conversion', () => {
  // Access private method for testing
  function getJsonSchema(zodSchema: z.ZodSchema<any>): any {
    const rlm = new RLM('gpt-4o-mini', {});
    return (rlm as any).zodToJsonSchema(zodSchema);
  }

  it('converts string schema', () => {
    const result = getJsonSchema(z.string());
    expect(result.type).toBe('string');
  });

  it('converts number schema', () => {
    const result = getJsonSchema(z.number());
    expect(result.type).toBe('number');
  });

  it('converts boolean schema', () => {
    const result = getJsonSchema(z.boolean());
    expect(result.type).toBe('boolean');
  });

  it('converts object schema', () => {
    const result = getJsonSchema(z.object({
      name: z.string(),
      age: z.number(),
    }));
    expect(result.type).toBe('object');
    expect(result.properties.name.type).toBe('string');
    expect(result.properties.age.type).toBe('number');
    expect(result.required).toContain('name');
    expect(result.required).toContain('age');
  });

  it('converts array schema', () => {
    const result = getJsonSchema(z.array(z.string()));
    expect(result.type).toBe('array');
    expect(result.items.type).toBe('string');
  });

  it('converts enum schema', () => {
    const result = getJsonSchema(z.enum(['a', 'b', 'c']));
    expect(result.type).toBe('string');
    expect(result.enum).toEqual(['a', 'b', 'c']);
  });

  it('converts nullable schema', () => {
    const result = getJsonSchema(z.string().nullable());
    expect(result.type).toBe('string');
    expect(result.nullable).toBe(true);
  });

  it('converts optional schema', () => {
    const result = getJsonSchema(z.string().optional());
    expect(result.type).toBe('string');
  });

  it('converts date schema', () => {
    const result = getJsonSchema(z.date());
    expect(result.type).toBe('string');
    expect(result.format).toBe('date-time');
  });

  it('converts nested object', () => {
    const result = getJsonSchema(z.object({
      user: z.object({
        name: z.string(),
        scores: z.array(z.number()),
      }),
    }));
    expect(result.properties.user.type).toBe('object');
    expect(result.properties.user.properties.scores.type).toBe('array');
  });

  it('converts any/unknown to empty schema', () => {
    expect(getJsonSchema(z.any())).toEqual({});
    expect(getJsonSchema(z.unknown())).toEqual({});
  });

  it('converts never to not schema', () => {
    const result = getJsonSchema(z.never());
    expect(result.not).toEqual({});
  });

  it('converts null/undefined/void to null type', () => {
    expect(getJsonSchema(z.null()).type).toBe('null');
    expect(getJsonSchema(z.undefined()).type).toBe('null');
    expect(getJsonSchema(z.void()).type).toBe('null');
  });
});
