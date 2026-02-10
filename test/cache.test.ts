import { describe, it, expect, beforeEach } from 'vitest';
import { RLMCache, MemoryCache, FileCache } from '../src/cache';
import * as fs from 'fs';
import * as path from 'path';

describe('MemoryCache', () => {
  let cache: MemoryCache;

  beforeEach(() => {
    cache = new MemoryCache(100);
  });

  it('stores and retrieves values', () => {
    cache.set('key1', { data: 'hello' }, 3600);
    expect(cache.get('key1')).toEqual({ data: 'hello' });
  });

  it('returns undefined for missing keys', () => {
    expect(cache.get('nonexistent')).toBeUndefined();
  });

  it('checks key existence', () => {
    cache.set('key1', 'value', 3600);
    expect(cache.has('key1')).toBe(true);
    expect(cache.has('key2')).toBe(false);
  });

  it('deletes keys', () => {
    cache.set('key1', 'value', 3600);
    expect(cache.delete('key1')).toBe(true);
    expect(cache.has('key1')).toBe(false);
    expect(cache.delete('key1')).toBe(false);
  });

  it('clears all entries', () => {
    cache.set('key1', 'a', 3600);
    cache.set('key2', 'b', 3600);
    cache.clear();
    expect(cache.size()).toBe(0);
  });

  it('respects TTL', async () => {
    cache.set('key1', 'value', 0.001); // 1ms TTL
    await new Promise(resolve => setTimeout(resolve, 10));
    expect(cache.get('key1')).toBeUndefined();
  });

  it('evicts oldest when at capacity', () => {
    const smallCache = new MemoryCache(3);
    smallCache.set('a', 1, 3600);
    smallCache.set('b', 2, 3600);
    smallCache.set('c', 3, 3600);
    smallCache.set('d', 4, 3600); // Should evict 'a'
    expect(smallCache.get('a')).toBeUndefined();
    expect(smallCache.get('d')).toBe(4);
  });

  it('reports size', () => {
    cache.set('a', 1, 3600);
    cache.set('b', 2, 3600);
    expect(cache.size()).toBe(2);
  });
});

describe('RLMCache', () => {
  it('returns miss when disabled', () => {
    const cache = new RLMCache({ enabled: false });
    cache.store('gpt-4o', 'query', 'context', { result: 'answer' });
    const result = cache.lookup('gpt-4o', 'query', 'context');
    expect(result.hit).toBe(false);
  });

  it('stores and retrieves completions', () => {
    const cache = new RLMCache({ enabled: true, strategy: 'exact' });
    const value = { result: 'answer', stats: { llm_calls: 1, iterations: 5, depth: 1 } };
    cache.store('gpt-4o', 'What is X?', 'context about X', value);

    const result = cache.lookup('gpt-4o', 'What is X?', 'context about X');
    expect(result.hit).toBe(true);
    expect(result.value).toEqual(value);
  });

  it('misses on different query', () => {
    const cache = new RLMCache({ enabled: true });
    cache.store('gpt-4o', 'query1', 'context', { result: 'a' });
    const result = cache.lookup('gpt-4o', 'query2', 'context');
    expect(result.hit).toBe(false);
  });

  it('misses on different model', () => {
    const cache = new RLMCache({ enabled: true });
    cache.store('gpt-4o', 'query', 'context', { result: 'a' });
    const result = cache.lookup('gpt-4o-mini', 'query', 'context');
    expect(result.hit).toBe(false);
  });

  it('misses on different context', () => {
    const cache = new RLMCache({ enabled: true });
    cache.store('gpt-4o', 'query', 'context1', { result: 'a' });
    const result = cache.lookup('gpt-4o', 'query', 'context2');
    expect(result.hit).toBe(false);
  });

  it('tracks stats', () => {
    const cache = new RLMCache({ enabled: true });
    cache.store('gpt-4o', 'q', 'c', { result: 'x' });
    cache.lookup('gpt-4o', 'q', 'c'); // hit
    cache.lookup('gpt-4o', 'q2', 'c'); // miss

    const stats = cache.getStats();
    expect(stats.hits).toBe(1);
    expect(stats.misses).toBe(1);
    expect(stats.hitRate).toBe(0.5);
  });

  it('clears cache and resets stats', () => {
    const cache = new RLMCache({ enabled: true });
    cache.store('gpt-4o', 'q', 'c', { result: 'x' });
    cache.clear();
    const stats = cache.getStats();
    expect(stats.size).toBe(0);
    expect(stats.hits).toBe(0);
  });

  it('disabled when strategy is none', () => {
    const cache = new RLMCache({ enabled: true, strategy: 'none' });
    expect(cache.enabled).toBe(false);
  });
});

describe('FileCache', () => {
  const cacheDir = '/tmp/rlm-test-cache-' + Date.now();

  beforeEach(() => {
    if (fs.existsSync(cacheDir)) {
      fs.rmSync(cacheDir, { recursive: true });
    }
  });

  it('stores and retrieves values', () => {
    const cache = new FileCache(cacheDir);
    cache.set('key1', { data: 'hello' }, 3600);
    expect(cache.get('key1')).toEqual({ data: 'hello' });
  });

  it('returns undefined for missing keys', () => {
    const cache = new FileCache(cacheDir);
    expect(cache.get('nonexistent')).toBeUndefined();
  });

  it('respects TTL', async () => {
    const cache = new FileCache(cacheDir);
    cache.set('key1', 'value', 0.001);
    await new Promise(resolve => setTimeout(resolve, 10));
    expect(cache.get('key1')).toBeUndefined();
  });

  it('clears all entries', () => {
    const cache = new FileCache(cacheDir);
    cache.set('a', 1, 3600);
    cache.set('b', 2, 3600);
    cache.clear();
    expect(cache.size()).toBe(0);
  });

  it('deletes individual entries', () => {
    const cache = new FileCache(cacheDir);
    cache.set('key1', 'value', 3600);
    expect(cache.delete('key1')).toBe(true);
    expect(cache.has('key1')).toBe(false);
  });
});
