import { describe, it, expect, vi } from 'vitest';
import { RLMEventEmitter } from '../src/events';

describe('RLMEventEmitter', () => {
  it('emits events to listeners', () => {
    const emitter = new RLMEventEmitter();
    const handler = vi.fn();

    emitter.on('llm_call', handler);
    emitter.emit('llm_call', {
      timestamp: Date.now(),
      type: 'llm_call',
      model: 'gpt-4o',
      queryLength: 100,
      contextLength: 5000,
    });

    expect(handler).toHaveBeenCalledOnce();
    expect(handler.mock.calls[0][0].model).toBe('gpt-4o');
  });

  it('supports multiple listeners', () => {
    const emitter = new RLMEventEmitter();
    const handler1 = vi.fn();
    const handler2 = vi.fn();

    emitter.on('error', handler1);
    emitter.on('error', handler2);
    emitter.emit('error', {
      timestamp: Date.now(),
      type: 'error',
      error: new Error('test'),
      operation: 'completion',
    });

    expect(handler1).toHaveBeenCalledOnce();
    expect(handler2).toHaveBeenCalledOnce();
  });

  it('removes listeners with off()', () => {
    const emitter = new RLMEventEmitter();
    const handler = vi.fn();

    emitter.on('cache', handler);
    emitter.off('cache', handler);
    emitter.emit('cache', { timestamp: Date.now(), type: 'cache', action: 'hit' });

    expect(handler).not.toHaveBeenCalled();
  });

  it('once() fires only once', () => {
    const emitter = new RLMEventEmitter();
    const handler = vi.fn();

    emitter.once('completion_start', handler);
    emitter.emit('completion_start', {
      timestamp: Date.now(),
      type: 'completion_start',
      model: 'gpt-4o',
      query: 'test',
      contextLength: 100,
      structured: false,
    });
    emitter.emit('completion_start', {
      timestamp: Date.now(),
      type: 'completion_start',
      model: 'gpt-4o',
      query: 'test2',
      contextLength: 100,
      structured: false,
    });

    expect(handler).toHaveBeenCalledOnce();
  });

  it('removeAllListeners() clears specific event', () => {
    const emitter = new RLMEventEmitter();
    const handler1 = vi.fn();
    const handler2 = vi.fn();

    emitter.on('llm_call', handler1);
    emitter.on('error', handler2);
    emitter.removeAllListeners('llm_call');

    emitter.emit('llm_call', { timestamp: Date.now(), type: 'llm_call', model: 'x', queryLength: 0, contextLength: 0 });
    emitter.emit('error', { timestamp: Date.now(), type: 'error', error: new Error(''), operation: '' });

    expect(handler1).not.toHaveBeenCalled();
    expect(handler2).toHaveBeenCalledOnce();
  });

  it('removeAllListeners() clears all events', () => {
    const emitter = new RLMEventEmitter();
    const handler = vi.fn();

    emitter.on('llm_call', handler);
    emitter.on('error', handler);
    emitter.removeAllListeners();

    emitter.emit('llm_call', { timestamp: Date.now(), type: 'llm_call', model: 'x', queryLength: 0, contextLength: 0 });
    expect(handler).not.toHaveBeenCalled();
  });

  it('reports listener count', () => {
    const emitter = new RLMEventEmitter();
    expect(emitter.listenerCount('llm_call')).toBe(0);

    emitter.on('llm_call', () => {});
    emitter.on('llm_call', () => {});
    expect(emitter.listenerCount('llm_call')).toBe(2);
  });

  it('does not throw when listener throws', () => {
    const emitter = new RLMEventEmitter();
    emitter.on('error', () => { throw new Error('listener error'); });

    expect(() => {
      emitter.emit('error', { timestamp: Date.now(), type: 'error', error: new Error(''), operation: '' });
    }).not.toThrow();
  });
});
