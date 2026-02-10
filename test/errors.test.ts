import { describe, it, expect } from 'vitest';
import {
  RLMError,
  RLMValidationError,
  RLMRateLimitError,
  RLMTimeoutError,
  RLMProviderError,
  RLMBinaryError,
  RLMConfigError,
  RLMSchemaError,
  RLMAbortError,
  classifyError,
} from '../src/errors';

describe('Error Hierarchy', () => {
  it('RLMError has code, retryable, suggestion', () => {
    const err = new RLMError('test error', { code: 'TEST', retryable: true, suggestion: 'Try again' });
    expect(err.message).toBe('test error');
    expect(err.code).toBe('TEST');
    expect(err.retryable).toBe(true);
    expect(err.suggestion).toBe('Try again');
    expect(err.name).toBe('RLMError');
    expect(err).toBeInstanceOf(Error);
  });

  it('RLMError defaults retryable to false', () => {
    const err = new RLMError('test', { code: 'X' });
    expect(err.retryable).toBe(false);
  });

  it('RLMValidationError contains expected/received', () => {
    const err = new RLMValidationError({
      message: 'Schema mismatch',
      expected: { type: 'string' },
      received: 42,
      zodErrors: [{ path: ['name'], message: 'Expected string' }],
    });
    expect(err.code).toBe('VALIDATION');
    expect(err.retryable).toBe(true);
    expect(err.expected).toEqual({ type: 'string' });
    expect(err.received).toBe(42);
    expect(err.zodErrors).toHaveLength(1);
    expect(err).toBeInstanceOf(RLMError);
    expect(err).toBeInstanceOf(Error);
  });

  it('RLMRateLimitError with retryAfter', () => {
    const err = new RLMRateLimitError({ message: 'Too many requests', retryAfter: 30 });
    expect(err.code).toBe('RATE_LIMIT');
    expect(err.retryable).toBe(true);
    expect(err.retryAfter).toBe(30);
    expect(err.suggestion).toContain('30');
  });

  it('RLMRateLimitError without retryAfter', () => {
    const err = new RLMRateLimitError({ message: 'Rate limited' });
    expect(err.retryAfter).toBeUndefined();
    expect(err.suggestion).toContain('backoff');
  });

  it('RLMTimeoutError has elapsed and limit', () => {
    const err = new RLMTimeoutError({ message: 'Timed out', elapsed: 5000, limit: 3000 });
    expect(err.code).toBe('TIMEOUT');
    expect(err.retryable).toBe(true);
    expect(err.elapsed).toBe(5000);
    expect(err.limit).toBe(3000);
  });

  it('RLMProviderError retryable for 5xx', () => {
    const err = new RLMProviderError({ message: 'Server error', statusCode: 500, provider: 'openai' });
    expect(err.retryable).toBe(true);
    expect(err.statusCode).toBe(500);
    expect(err.provider).toBe('openai');
  });

  it('RLMProviderError non-retryable for 4xx', () => {
    const err = new RLMProviderError({ message: 'Bad request', statusCode: 400, provider: 'openai' });
    expect(err.retryable).toBe(false);
  });

  it('RLMBinaryError has binaryPath', () => {
    const err = new RLMBinaryError({ message: 'Not found', binaryPath: './bin/rlm-go' });
    expect(err.code).toBe('BINARY');
    expect(err.retryable).toBe(false);
    expect(err.binaryPath).toBe('./bin/rlm-go');
  });

  it('RLMConfigError has field and value', () => {
    const err = new RLMConfigError({ message: 'Invalid', field: 'max_depth', value: -1 });
    expect(err.code).toBe('CONFIG');
    expect(err.field).toBe('max_depth');
    expect(err.value).toBe(-1);
  });

  it('RLMSchemaError has path and constraint', () => {
    const err = new RLMSchemaError({ message: 'Bad schema', path: 'properties.name', constraint: 'type required' });
    expect(err.code).toBe('SCHEMA');
    expect(err.path).toBe('properties.name');
    expect(err.constraint).toBe('type required');
  });

  it('RLMAbortError has default message', () => {
    const err = new RLMAbortError();
    expect(err.code).toBe('ABORTED');
    expect(err.retryable).toBe(false);
    expect(err.message).toBe('Operation was aborted');
  });

  it('RLMAbortError accepts custom message', () => {
    const err = new RLMAbortError('User cancelled');
    expect(err.message).toBe('User cancelled');
  });
});

describe('classifyError', () => {
  it('classifies rate limit errors', () => {
    const err = classifyError(new Error('429 Too Many Requests'));
    expect(err).toBeInstanceOf(RLMRateLimitError);
    expect(err.code).toBe('RATE_LIMIT');
  });

  it('classifies rate limit from message', () => {
    const err = classifyError(new Error('rate limit exceeded'));
    expect(err).toBeInstanceOf(RLMRateLimitError);
  });

  it('extracts retryAfter from message', () => {
    const err = classifyError(new Error('Rate limited. Retry-After: 60'));
    expect(err).toBeInstanceOf(RLMRateLimitError);
    expect((err as RLMRateLimitError).retryAfter).toBe(60);
  });

  it('classifies timeout errors', () => {
    const err = classifyError(new Error('Request timeout'));
    expect(err).toBeInstanceOf(RLMTimeoutError);
  });

  it('classifies ETIMEDOUT', () => {
    const err = classifyError(new Error('connect ETIMEDOUT'));
    expect(err).toBeInstanceOf(RLMTimeoutError);
  });

  it('classifies binary not found errors', () => {
    const err = classifyError(new Error('ENOENT: no such file'), { binaryPath: './bin/rlm-go' });
    expect(err).toBeInstanceOf(RLMBinaryError);
  });

  it('classifies provider errors from HTTP status codes', () => {
    const err = classifyError(new Error('API returned status: 500'));
    expect(err).toBeInstanceOf(RLMProviderError);
    expect((err as RLMProviderError).statusCode).toBe(500);
  });

  it('classifies validation errors', () => {
    const err = classifyError(new Error('JSON schema validation failed'));
    expect(err).toBeInstanceOf(RLMValidationError);
  });

  it('returns generic RLMError for unknown errors', () => {
    const err = classifyError(new Error('Something unexpected'));
    expect(err).toBeInstanceOf(RLMError);
    expect(err.code).toBe('UNKNOWN');
  });

  it('accepts string input', () => {
    const err = classifyError('rate limit exceeded');
    expect(err).toBeInstanceOf(RLMRateLimitError);
  });
});
