import { describe, it, expect, vi } from 'vitest';
import { withRetry, withFallback } from '../src/retry';
import { RLMRateLimitError, RLMAbortError, RLMError } from '../src/errors';

describe('withRetry', () => {
  it('returns result on first success', async () => {
    const fn = vi.fn().mockResolvedValue('success');
    const result = await withRetry(fn, { maxRetries: 3 });
    expect(result).toBe('success');
    expect(fn).toHaveBeenCalledOnce();
  });

  it('retries on retryable errors', async () => {
    const fn = vi.fn()
      .mockRejectedValueOnce(new RLMRateLimitError({ message: 'Rate limited' }))
      .mockResolvedValue('success');

    const result = await withRetry(fn, { maxRetries: 3, baseDelay: 1 });
    expect(result).toBe('success');
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it('throws after max retries', async () => {
    const err = new RLMRateLimitError({ message: 'Rate limited' });
    const fn = vi.fn().mockRejectedValue(err);

    await expect(withRetry(fn, { maxRetries: 2, baseDelay: 1 })).rejects.toThrow('Rate limited');
    expect(fn).toHaveBeenCalledTimes(3); // initial + 2 retries
  });

  it('does not retry non-retryable errors', async () => {
    const err = new RLMError('Auth failed', { code: 'AUTH', retryable: false });
    const fn = vi.fn().mockRejectedValue(err);

    await expect(withRetry(fn, { maxRetries: 3, baseDelay: 1 })).rejects.toThrow('Auth failed');
    expect(fn).toHaveBeenCalledOnce();
  });

  it('does not retry abort errors', async () => {
    const err = new RLMAbortError();
    const fn = vi.fn().mockRejectedValue(err);

    await expect(withRetry(fn, { maxRetries: 3, baseDelay: 1 })).rejects.toThrow(RLMAbortError);
    expect(fn).toHaveBeenCalledOnce();
  });

  it('calls onRetry callback', async () => {
    const onRetry = vi.fn();
    const fn = vi.fn()
      .mockRejectedValueOnce(new RLMRateLimitError({ message: 'Rate limited' }))
      .mockResolvedValue('ok');

    await withRetry(fn, { maxRetries: 3, baseDelay: 1, onRetry });
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onRetry.mock.calls[0][0]).toBe(1); // attempt number
  });

  it('respects rate limit retryAfter', async () => {
    const start = Date.now();
    const fn = vi.fn()
      .mockRejectedValueOnce(new RLMRateLimitError({ message: 'Limited', retryAfter: 0.01 })) // 10ms
      .mockResolvedValue('ok');

    await withRetry(fn, { maxRetries: 1, baseDelay: 1 });
    // Should have waited at least 10ms for retryAfter
    expect(Date.now() - start).toBeGreaterThanOrEqual(5);
  });

  it('uses exponential backoff', async () => {
    const onRetry = vi.fn();
    const fn = vi.fn()
      .mockRejectedValueOnce(new RLMRateLimitError({ message: 'Limited' }))
      .mockRejectedValueOnce(new RLMRateLimitError({ message: 'Limited' }))
      .mockResolvedValue('ok');

    await withRetry(fn, { maxRetries: 3, baseDelay: 10, backoff: 'exponential', jitter: false, onRetry });

    // Second retry should have longer delay than first
    const delay1 = onRetry.mock.calls[0][2];
    const delay2 = onRetry.mock.calls[1][2];
    expect(delay2).toBeGreaterThan(delay1);
  });

  it('respects abort signal', async () => {
    const controller = new AbortController();
    controller.abort();

    const fn = vi.fn().mockResolvedValue('ok');
    await expect(withRetry(fn, { maxRetries: 3 }, controller.signal)).rejects.toThrow(RLMAbortError);
  });
});

describe('withFallback', () => {
  it('uses first model on success', async () => {
    const fn = vi.fn().mockResolvedValue({ result: 'ok' });
    const result = await withFallback(fn, { models: ['gpt-4o', 'claude-sonnet-4-20250514'] }, { maxRetries: 0 });
    expect(fn).toHaveBeenCalledWith('gpt-4o');
    expect(result._usedModel).toBe('gpt-4o');
  });

  it('falls back to second model on failure', async () => {
    const fn = vi.fn()
      .mockRejectedValueOnce(new Error('Model unavailable'))
      .mockResolvedValue({ result: 'ok' });

    const result = await withFallback(fn, { models: ['gpt-4o', 'claude-sonnet-4-20250514'] }, { maxRetries: 0, baseDelay: 1 });
    expect(fn).toHaveBeenCalledTimes(2);
    expect(fn).toHaveBeenCalledWith('claude-sonnet-4-20250514');
    expect(result._usedModel).toBe('claude-sonnet-4-20250514');
  });

  it('throws if all models fail', async () => {
    const fn = vi.fn().mockRejectedValue(new Error('All fail'));
    await expect(
      withFallback(fn, { models: ['a', 'b'] }, { maxRetries: 0, baseDelay: 1 })
    ).rejects.toThrow('All fail');
  });

  it('throws with no models configured', async () => {
    const fn = vi.fn();
    await expect(withFallback(fn, { models: [] })).rejects.toThrow('No models configured');
  });
});
