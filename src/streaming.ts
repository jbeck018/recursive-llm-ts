/**
 * Streaming support for recursive-llm-ts.
 *
 * Provides `AsyncIterable`-based streaming for both text completions
 * and structured output, with AbortController support.
 */

import { RLMAbortError } from './errors';

// ─── Stream Chunk Types ──────────────────────────────────────────────────────

export type StreamChunkType = 'text' | 'partial_object' | 'usage' | 'error' | 'done';

export interface StreamChunkBase {
  type: StreamChunkType;
  timestamp: number;
}

export interface TextStreamChunk extends StreamChunkBase {
  type: 'text';
  text: string;
}

export interface PartialObjectStreamChunk<T = unknown> extends StreamChunkBase {
  type: 'partial_object';
  object: Partial<T>;
  /** JSON path of the field being populated */
  path?: string;
}

export interface UsageStreamChunk extends StreamChunkBase {
  type: 'usage';
  usage: {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
  };
}

export interface ErrorStreamChunk extends StreamChunkBase {
  type: 'error';
  error: Error;
}

export interface DoneStreamChunk extends StreamChunkBase {
  type: 'done';
  stats: {
    llm_calls: number;
    iterations: number;
    depth: number;
  };
}

export type StreamChunk<T = unknown> =
  | TextStreamChunk
  | PartialObjectStreamChunk<T>
  | UsageStreamChunk
  | ErrorStreamChunk
  | DoneStreamChunk;

// ─── Stream Options ──────────────────────────────────────────────────────────

export interface StreamOptions {
  /** AbortController signal to cancel the stream */
  signal?: AbortSignal;
  /** Called on each chunk (alternative to async iteration) */
  onChunk?: (chunk: StreamChunk) => void;
}

// ─── RLM Stream ──────────────────────────────────────────────────────────────

/**
 * An async iterable stream of completion chunks.
 *
 * Can be consumed with `for await...of` or by attaching an `onChunk` callback.
 *
 * @example
 * ```typescript
 * const stream = rlm.streamCompletion(query, context);
 * let fullText = '';
 * for await (const chunk of stream) {
 *   if (chunk.type === 'text') fullText += chunk.text;
 * }
 * ```
 */
export class RLMStream<T = unknown> implements AsyncIterable<StreamChunk<T>> {
  private chunks: StreamChunk<T>[] = [];
  private resolvers: Array<(value: IteratorResult<StreamChunk<T>>) => void> = [];
  private done = false;
  private error: Error | null = null;
  private signal?: AbortSignal;
  private abortHandler?: () => void;

  constructor(signal?: AbortSignal) {
    this.signal = signal;
    if (signal) {
      this.abortHandler = () => {
        this.pushError(new RLMAbortError());
      };
      signal.addEventListener('abort', this.abortHandler, { once: true });
    }
  }

  /** Push a chunk into the stream (called by the producer) */
  push(chunk: StreamChunk<T>): void {
    if (this.done) return;

    if (chunk.type === 'done') {
      this.done = true;
    }

    if (this.resolvers.length > 0) {
      const resolve = this.resolvers.shift()!;
      resolve({ value: chunk, done: false });
      if (chunk.type === 'done') {
        // Resolve any remaining waiters
        for (const r of this.resolvers) {
          r({ value: undefined as any, done: true });
        }
        this.resolvers = [];
      }
    } else {
      this.chunks.push(chunk);
    }
  }

  /** Signal an error on the stream */
  pushError(err: Error): void {
    if (this.done) return;
    this.error = err;
    this.done = true;

    // Reject any waiting resolvers
    for (const resolve of this.resolvers) {
      resolve({ value: { type: 'error', error: err, timestamp: Date.now() } as StreamChunk<T>, done: false });
    }
    this.resolvers = [];
    this.cleanup();
  }

  /** Mark the stream as complete */
  complete(stats: { llm_calls: number; iterations: number; depth: number }): void {
    this.push({ type: 'done', stats, timestamp: Date.now() } as DoneStreamChunk as StreamChunk<T>);
    this.cleanup();
  }

  private cleanup(): void {
    if (this.signal && this.abortHandler) {
      this.signal.removeEventListener('abort', this.abortHandler);
    }
  }

  /** Collect all text chunks into a single string */
  async toText(): Promise<string> {
    let text = '';
    for await (const chunk of this) {
      if (chunk.type === 'text') {
        text += (chunk as TextStreamChunk).text;
      }
    }
    return text;
  }

  /** Collect the final structured object (for structured streaming) */
  async toObject(): Promise<T | undefined> {
    let latest: Partial<T> | undefined;
    for await (const chunk of this) {
      if (chunk.type === 'partial_object') {
        latest = (chunk as PartialObjectStreamChunk<T>).object;
      }
    }
    return latest as T | undefined;
  }

  [Symbol.asyncIterator](): AsyncIterator<StreamChunk<T>> {
    return {
      next: (): Promise<IteratorResult<StreamChunk<T>>> => {
        // Check for abort
        if (this.signal?.aborted && !this.error) {
          this.pushError(new RLMAbortError());
        }

        // Return buffered chunks first
        if (this.chunks.length > 0) {
          const chunk = this.chunks.shift()!;
          if (chunk.type === 'done' || chunk.type === 'error') {
            return Promise.resolve({ value: chunk, done: false }).then(r => {
              return { value: undefined as any, done: true };
            });
          }
          return Promise.resolve({ value: chunk, done: false });
        }

        // Stream is done
        if (this.done) {
          return Promise.resolve({ value: undefined as any, done: true });
        }

        // Wait for next chunk
        return new Promise<IteratorResult<StreamChunk<T>>>((resolve) => {
          this.resolvers.push(resolve);
        });
      },
    };
  }
}

// ─── Stream Builder (for simulating streaming from non-streaming sources) ────

/**
 * Creates a simulated stream from a non-streaming completion result.
 * Useful as a compatibility bridge until full streaming is implemented in the Go binary.
 */
export function createSimulatedStream(
  text: string,
  stats: { llm_calls: number; iterations: number; depth: number },
  signal?: AbortSignal,
  chunkSize = 20
): RLMStream {
  const stream = new RLMStream(signal);

  // Simulate streaming by splitting text into chunks
  (async () => {
    try {
      for (let i = 0; i < text.length; i += chunkSize) {
        if (signal?.aborted) {
          stream.pushError(new RLMAbortError());
          return;
        }
        const chunk = text.slice(i, i + chunkSize);
        stream.push({ type: 'text', text: chunk, timestamp: Date.now() });
        // Small yield to allow consumer to process
        await new Promise(resolve => setImmediate(resolve));
      }
      stream.complete(stats);
    } catch (err) {
      stream.pushError(err instanceof Error ? err : new Error(String(err)));
    }
  })();

  return stream;
}
