import { describe, it, expect } from 'vitest';
import { RLMStream, createSimulatedStream } from '../src/streaming';

describe('RLMStream', () => {
  it('pushes and iterates text chunks', async () => {
    const stream = new RLMStream();
    stream.push({ type: 'text', text: 'Hello', timestamp: Date.now() });
    stream.push({ type: 'text', text: ' World', timestamp: Date.now() });
    stream.complete({ llm_calls: 1, iterations: 1, depth: 1 });

    const chunks: string[] = [];
    for await (const chunk of stream) {
      if (chunk.type === 'text') chunks.push(chunk.text);
    }
    expect(chunks).toEqual(['Hello', ' World']);
  });

  it('toText() collects all text', async () => {
    const stream = new RLMStream();
    setTimeout(() => {
      stream.push({ type: 'text', text: 'Hello', timestamp: Date.now() });
      stream.push({ type: 'text', text: ' World', timestamp: Date.now() });
      stream.complete({ llm_calls: 1, iterations: 1, depth: 1 });
    }, 1);

    const text = await stream.toText();
    expect(text).toBe('Hello World');
  });

  it('toObject() returns last partial object', async () => {
    const stream = new RLMStream<{ name: string }>();
    setTimeout(() => {
      stream.push({ type: 'partial_object', object: { name: 'test' }, timestamp: Date.now() });
      stream.complete({ llm_calls: 1, iterations: 1, depth: 1 });
    }, 1);

    const obj = await stream.toObject();
    expect(obj).toEqual({ name: 'test' });
  });

  it('handles errors', async () => {
    const stream = new RLMStream();
    setTimeout(() => {
      stream.pushError(new Error('Test error'));
    }, 1);

    const text = await stream.toText();
    expect(text).toBe('');
  });
});

describe('createSimulatedStream', () => {
  it('splits text into chunks', async () => {
    const stream = createSimulatedStream(
      'Hello World, this is a test of streaming',
      { llm_calls: 1, iterations: 1, depth: 1 },
      undefined,
      10
    );

    const text = await stream.toText();
    expect(text).toBe('Hello World, this is a test of streaming');
  });

  it('supports abort controller', async () => {
    const controller = new AbortController();
    const stream = createSimulatedStream(
      'A'.repeat(1000),
      { llm_calls: 1, iterations: 1, depth: 1 },
      controller.signal,
      10
    );

    // Abort immediately
    controller.abort();

    const text = await stream.toText();
    expect(text.length).toBeLessThan(1000);
  });
});
