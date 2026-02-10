/**
 * Retry and resilience layer for recursive-llm-ts.
 *
 * Provides configurable retry with exponential backoff, jitter,
 * and provider fallback chains.
 */

import { RLMError, RLMRateLimitError, RLMTimeoutError, RLMAbortError } from './errors';

// ─── Retry Config ────────────────────────────────────────────────────────────

export interface RetryConfig {
  /** Maximum number of retries (default: 3) */
  maxRetries?: number;
  /** Backoff strategy (default: 'exponential') */
  backoff?: 'exponential' | 'linear' | 'fixed';
  /** Base delay in milliseconds (default: 1000) */
  baseDelay?: number;
  /** Maximum delay in milliseconds (default: 30000) */
  maxDelay?: number;
  /** Add jitter to delays (default: true) */
  jitter?: boolean;
  /** Error types that should be retried */
  retryableErrors?: string[];
  /** Called before each retry with retry info */
  onRetry?: (attempt: number, error: Error, delay: number) => void;
}

/** Fallback model configuration */
export interface FallbackConfig {
  /** Ordered list of fallback models to try */
  models?: string[];
  /** Strategy for fallback selection */
  strategy?: 'sequential' | 'round-robin';
}

// ─── Resolved Config ─────────────────────────────────────────────────────────

interface ResolvedRetryConfig {
  maxRetries: number;
  backoff: 'exponential' | 'linear' | 'fixed';
  baseDelay: number;
  maxDelay: number;
  jitter: boolean;
  retryableErrors: Set<string>;
  onRetry?: (attempt: number, error: Error, delay: number) => void;
}

const DEFAULT_RETRYABLE_ERRORS = new Set([
  'RATE_LIMIT',
  'TIMEOUT',
  'PROVIDER',
  'UNKNOWN',
]);

function resolveConfig(config: RetryConfig = {}): ResolvedRetryConfig {
  return {
    maxRetries: config.maxRetries ?? 3,
    backoff: config.backoff ?? 'exponential',
    baseDelay: config.baseDelay ?? 1000,
    maxDelay: config.maxDelay ?? 30000,
    jitter: config.jitter ?? true,
    retryableErrors: config.retryableErrors
      ? new Set(config.retryableErrors)
      : DEFAULT_RETRYABLE_ERRORS,
    onRetry: config.onRetry,
  };
}

// ─── Delay Calculation ───────────────────────────────────────────────────────

function calculateDelay(attempt: number, config: ResolvedRetryConfig): number {
  let delay: number;

  switch (config.backoff) {
    case 'exponential':
      delay = config.baseDelay * Math.pow(2, attempt);
      break;
    case 'linear':
      delay = config.baseDelay * (attempt + 1);
      break;
    case 'fixed':
      delay = config.baseDelay;
      break;
    default:
      delay = config.baseDelay;
  }

  // Cap at max
  delay = Math.min(delay, config.maxDelay);

  // Add jitter (0.5x to 1.5x)
  if (config.jitter) {
    delay = delay * (0.5 + Math.random());
  }

  return Math.round(delay);
}

// ─── Retryable Check ─────────────────────────────────────────────────────────

function isRetryable(error: Error, config: ResolvedRetryConfig): boolean {
  // Never retry abort errors
  if (error instanceof RLMAbortError) return false;

  // Check explicit retryable flag
  if (error instanceof RLMError) {
    if (!error.retryable) return false;
    return config.retryableErrors.has(error.code);
  }

  // For non-RLM errors, check message heuristics
  const msg = error.message.toLowerCase();
  if (msg.includes('rate limit') || msg.includes('429')) return true;
  if (msg.includes('timeout') || msg.includes('etimedout')) return true;
  if (msg.includes('econnreset') || msg.includes('econnrefused')) return true;
  if (msg.includes('500') || msg.includes('502') || msg.includes('503') || msg.includes('504')) return true;

  return false;
}

// ─── Sleep Helper ────────────────────────────────────────────────────────────

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new RLMAbortError());
      return;
    }

    const timer = setTimeout(resolve, ms);

    if (signal) {
      const onAbort = () => {
        clearTimeout(timer);
        reject(new RLMAbortError());
      };
      signal.addEventListener('abort', onAbort, { once: true });
    }
  });
}

// ─── Retry Executor ──────────────────────────────────────────────────────────

/**
 * Execute a function with retry logic.
 *
 * @example
 * ```typescript
 * const result = await withRetry(
 *   () => rlm.completion(query, context),
 *   { maxRetries: 3, backoff: 'exponential' }
 * );
 * ```
 */
export async function withRetry<T>(
  fn: () => Promise<T>,
  config?: RetryConfig,
  signal?: AbortSignal
): Promise<T> {
  const resolved = resolveConfig(config);
  let lastError: Error | undefined;

  for (let attempt = 0; attempt <= resolved.maxRetries; attempt++) {
    try {
      if (signal?.aborted) throw new RLMAbortError();
      return await fn();
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      lastError = error;

      // Check if we should retry
      if (attempt >= resolved.maxRetries || !isRetryable(error, resolved)) {
        throw error;
      }

      // Special handling for rate limit with retry-after
      let delay: number;
      if (error instanceof RLMRateLimitError && error.retryAfter) {
        delay = error.retryAfter * 1000;
      } else {
        delay = calculateDelay(attempt, resolved);
      }

      // Notify callback
      resolved.onRetry?.(attempt + 1, error, delay);

      // Wait before retrying
      await sleep(delay, signal);
    }
  }

  throw lastError || new Error('Retry failed');
}

/**
 * Execute a function with fallback models.
 * Tries each model in order until one succeeds.
 *
 * @example
 * ```typescript
 * const result = await withFallback(
 *   (model) => rlm.completion(query, context, model),
 *   { models: ['gpt-4o', 'claude-sonnet-4-20250514', 'gemini-2.0-flash'] }
 * );
 * ```
 */
export async function withFallback<T>(
  fn: (model: string) => Promise<T>,
  fallbackConfig: FallbackConfig,
  retryConfig?: RetryConfig,
  signal?: AbortSignal
): Promise<T & { _usedModel?: string }> {
  const models = fallbackConfig.models || [];
  if (models.length === 0) {
    throw new RLMError('No models configured for fallback', { code: 'CONFIG', retryable: false });
  }

  let lastError: Error | undefined;

  for (const model of models) {
    try {
      if (signal?.aborted) throw new RLMAbortError();
      const result = await withRetry(() => fn(model), retryConfig, signal);
      return { ...result as any, _usedModel: model };
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err));
      // Continue to next model
    }
  }

  throw lastError || new Error('All fallback models failed');
}
