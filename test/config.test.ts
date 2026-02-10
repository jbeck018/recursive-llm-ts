import { describe, it, expect } from 'vitest';
import { validateConfig, assertValidConfig } from '../src/config';
import { RLMConfigError } from '../src/errors';

describe('validateConfig', () => {
  it('valid config passes', () => {
    const result = validateConfig({
      api_key: 'test-key',
      max_depth: 5,
      max_iterations: 30,
    });
    expect(result.valid).toBe(true);
    expect(result.issues.filter(i => i.level === 'error')).toHaveLength(0);
  });

  it('warns on unknown keys', () => {
    const result = validateConfig({
      api_key: 'test',
      max_detph: 5, // typo
    } as any);
    const warnings = result.issues.filter(i => i.level === 'warning');
    expect(warnings.length).toBeGreaterThan(0);
    expect(warnings[0].message).toContain('max_detph');
    expect(warnings[0].message).toContain('typo');
  });

  it('errors on negative max_depth', () => {
    const result = validateConfig({ max_depth: -1 });
    const errors = result.issues.filter(i => i.level === 'error');
    expect(errors.length).toBe(1);
    expect(errors[0].field).toBe('max_depth');
    expect(result.valid).toBe(false);
  });

  it('errors on negative max_iterations', () => {
    const result = validateConfig({ max_iterations: 0 });
    const errors = result.issues.filter(i => i.level === 'error');
    expect(errors.length).toBe(1);
    expect(errors[0].field).toBe('max_iterations');
  });

  it('errors on invalid api_base URL', () => {
    const result = validateConfig({ api_base: 'not-a-url' });
    const errors = result.issues.filter(i => i.level === 'error');
    expect(errors.length).toBe(1);
    expect(errors[0].field).toBe('api_base');
  });

  it('accepts valid api_base URL', () => {
    const result = validateConfig({ api_base: 'https://api.openai.com' });
    expect(result.issues.filter(i => i.level === 'error')).toHaveLength(0);
  });

  it('warns on trace_enabled without endpoint', () => {
    const result = validateConfig({
      observability: { trace_enabled: true },
    });
    const warnings = result.issues.filter(i => i.level === 'warning');
    expect(warnings.some(w => w.field === 'observability.trace_endpoint')).toBe(true);
  });

  it('warns on langfuse without keys', () => {
    const result = validateConfig({
      observability: { langfuse_enabled: true },
    });
    const warnings = result.issues.filter(i => i.level === 'warning');
    expect(warnings.some(w => w.field.includes('langfuse'))).toBe(true);
  });

  it('validates meta_agent config', () => {
    const result = validateConfig({
      meta_agent: { enabled: 'yes' as any },
    });
    expect(result.issues.filter(i => i.level === 'error').length).toBeGreaterThan(0);
  });

  it('validates cache config', () => {
    const result = validateConfig({
      cache: { enabled: true, maxEntries: -1 },
    });
    expect(result.issues.filter(i => i.level === 'error').length).toBeGreaterThan(0);
  });

  it('validates retry config', () => {
    const result = validateConfig({
      retry: { maxRetries: -1 },
    });
    expect(result.issues.filter(i => i.level === 'error').length).toBeGreaterThan(0);
  });

  it('info on missing api key', () => {
    // Save and clear env
    const saved = process.env.OPENAI_API_KEY;
    delete process.env.OPENAI_API_KEY;

    const result = validateConfig({});
    const infos = result.issues.filter(i => i.level === 'info');
    expect(infos.some(i => i.field === 'api_key')).toBe(true);

    // Restore
    if (saved) process.env.OPENAI_API_KEY = saved;
  });
});

describe('assertValidConfig', () => {
  it('throws RLMConfigError on invalid config', () => {
    expect(() => assertValidConfig({ max_depth: -1 })).toThrow(RLMConfigError);
  });

  it('does not throw on valid config', () => {
    expect(() => assertValidConfig({ api_key: 'test', max_depth: 5 })).not.toThrow();
  });
});
