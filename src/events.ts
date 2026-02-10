/**
 * Event system for recursive-llm-ts.
 *
 * Provides typed event emission for monitoring LLM operations,
 * validation retries, recursion progress, and more.
 */

// ─── Event Types ─────────────────────────────────────────────────────────────

export interface RLMEventMap {
  /** Fired when an LLM API call is made */
  'llm_call': LLMCallEvent;
  /** Fired when an LLM API call completes */
  'llm_response': LLMResponseEvent;
  /** Fired when a validation retry occurs */
  'validation_retry': ValidationRetryEvent;
  /** Fired when recursion depth changes */
  'recursion': RecursionEvent;
  /** Fired when meta-agent optimizes a query */
  'meta_agent': MetaAgentEvent;
  /** Fired on any error */
  'error': ErrorEvent;
  /** Fired when a completion starts */
  'completion_start': CompletionStartEvent;
  /** Fired when a completion ends */
  'completion_end': CompletionEndEvent;
  /** Fired when a cache lookup occurs */
  'cache': CacheEvent;
  /** Fired when a retry occurs */
  'retry': RetryEvent;
}

export type RLMEventType = keyof RLMEventMap;

interface BaseEvent {
  timestamp: number;
  type: string;
}

export interface LLMCallEvent extends BaseEvent {
  type: 'llm_call';
  model: string;
  queryLength: number;
  contextLength: number;
}

export interface LLMResponseEvent extends BaseEvent {
  type: 'llm_response';
  model: string;
  duration: number;
  tokenCount?: number;
}

export interface ValidationRetryEvent extends BaseEvent {
  type: 'validation_retry';
  attempt: number;
  maxRetries: number;
  error: string;
}

export interface RecursionEvent extends BaseEvent {
  type: 'recursion';
  depth: number;
  maxDepth: number;
  iteration: number;
}

export interface MetaAgentEvent extends BaseEvent {
  type: 'meta_agent';
  originalQuery: string;
  optimizedQuery: string;
  skipped: boolean;
  reason?: string;
}

export interface ErrorEvent extends BaseEvent {
  type: 'error';
  error: Error;
  operation: string;
}

export interface CompletionStartEvent extends BaseEvent {
  type: 'completion_start';
  model: string;
  query: string;
  contextLength: number;
  structured: boolean;
}

export interface CompletionEndEvent extends BaseEvent {
  type: 'completion_end';
  model: string;
  duration: number;
  stats: {
    llm_calls: number;
    iterations: number;
    depth: number;
  };
  cached: boolean;
}

export interface CacheEvent extends BaseEvent {
  type: 'cache';
  action: 'hit' | 'miss' | 'store';
  key?: string;
}

export interface RetryEvent extends BaseEvent {
  type: 'retry';
  attempt: number;
  maxRetries: number;
  delay: number;
  error: string;
}

// ─── Event Listener ──────────────────────────────────────────────────────────

type EventListener<T> = (event: T) => void;

// ─── Event Emitter ───────────────────────────────────────────────────────────

/**
 * Typed event emitter for RLM operations.
 *
 * @example
 * ```typescript
 * const rlm = new RLM('gpt-4o-mini');
 * rlm.on('llm_call', (event) => console.log('LLM call:', event.model));
 * rlm.on('error', (event) => reportToSentry(event.error));
 * ```
 */
export class RLMEventEmitter {
  private listeners = new Map<string, Set<EventListener<any>>>();

  /** Register an event listener */
  on<K extends RLMEventType>(event: K, listener: EventListener<RLMEventMap[K]>): this {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(listener);
    return this;
  }

  /** Register a one-time event listener */
  once<K extends RLMEventType>(event: K, listener: EventListener<RLMEventMap[K]>): this {
    const wrapper = (e: RLMEventMap[K]) => {
      this.off(event, wrapper);
      listener(e);
    };
    return this.on(event, wrapper);
  }

  /** Remove an event listener */
  off<K extends RLMEventType>(event: K, listener: EventListener<RLMEventMap[K]>): this {
    this.listeners.get(event)?.delete(listener);
    return this;
  }

  /** Remove all listeners for an event (or all events) */
  removeAllListeners(event?: RLMEventType): this {
    if (event) {
      this.listeners.delete(event);
    } else {
      this.listeners.clear();
    }
    return this;
  }

  /** Emit an event to all registered listeners */
  emit<K extends RLMEventType>(event: K, data: RLMEventMap[K]): void {
    const eventListeners = this.listeners.get(event);
    if (eventListeners) {
      for (const listener of eventListeners) {
        try {
          listener(data);
        } catch {
          // Don't let listener errors break the emitter
        }
      }
    }
  }

  /** Get the number of listeners for an event */
  listenerCount(event: RLMEventType): number {
    return this.listeners.get(event)?.size ?? 0;
  }
}
