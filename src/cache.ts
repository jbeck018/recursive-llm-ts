/**
 * Caching layer for recursive-llm-ts completions.
 *
 * Provides exact-match caching to avoid redundant API calls for
 * identical query+context pairs. Supports in-memory and file-based storage.
 */

import * as fs from 'fs';
import * as path from 'path';
import * as crypto from 'crypto';

// ─── Cache Config ────────────────────────────────────────────────────────────

export interface CacheConfig {
  /** Enable/disable caching (default: false) */
  enabled?: boolean;
  /** Cache strategy (default: 'exact') */
  strategy?: 'exact' | 'none';
  /** Maximum number of cached entries (default: 1000) */
  maxEntries?: number;
  /** Time-to-live in seconds (default: 3600 = 1 hour) */
  ttl?: number;
  /** Storage backend (default: 'memory') */
  storage?: 'memory' | 'file';
  /** Directory for file-based cache (default: .rlm-cache) */
  cacheDir?: string;
}

// ─── Cache Entry ─────────────────────────────────────────────────────────────

interface CacheEntry<T = unknown> {
  key: string;
  value: T;
  createdAt: number;
  ttl: number;
  hitCount: number;
}

// ─── Cache Stats ─────────────────────────────────────────────────────────────

export interface CacheStats {
  hits: number;
  misses: number;
  size: number;
  hitRate: number;
  evictions: number;
}

// ─── Cache Key Generator ─────────────────────────────────────────────────────

function generateCacheKey(model: string, query: string, context: string, config?: Record<string, unknown>): string {
  const data = JSON.stringify({ model, query, context, config });
  return crypto.createHash('sha256').update(data).digest('hex');
}

// ─── Cache Interface ─────────────────────────────────────────────────────────

export interface CacheProvider {
  get<T>(key: string): T | undefined;
  set<T>(key: string, value: T, ttl: number): void;
  has(key: string): boolean;
  delete(key: string): boolean;
  clear(): void;
  size(): number;
}

// ─── In-Memory Cache ─────────────────────────────────────────────────────────

export class MemoryCache implements CacheProvider {
  private store = new Map<string, CacheEntry>();
  private maxEntries: number;

  constructor(maxEntries = 1000) {
    this.maxEntries = maxEntries;
  }

  get<T>(key: string): T | undefined {
    const entry = this.store.get(key);
    if (!entry) return undefined;

    // Check TTL
    if (Date.now() - entry.createdAt > entry.ttl * 1000) {
      this.store.delete(key);
      return undefined;
    }

    entry.hitCount++;
    return entry.value as T;
  }

  set<T>(key: string, value: T, ttl: number): void {
    // Evict oldest if at capacity
    if (this.store.size >= this.maxEntries && !this.store.has(key)) {
      const oldestKey = this.store.keys().next().value;
      if (oldestKey !== undefined) {
        this.store.delete(oldestKey);
      }
    }

    this.store.set(key, {
      key,
      value,
      createdAt: Date.now(),
      ttl,
      hitCount: 0,
    });
  }

  has(key: string): boolean {
    const entry = this.store.get(key);
    if (!entry) return false;
    if (Date.now() - entry.createdAt > entry.ttl * 1000) {
      this.store.delete(key);
      return false;
    }
    return true;
  }

  delete(key: string): boolean {
    return this.store.delete(key);
  }

  clear(): void {
    this.store.clear();
  }

  size(): number {
    // Clean expired entries
    const now = Date.now();
    for (const [key, entry] of this.store) {
      if (now - entry.createdAt > entry.ttl * 1000) {
        this.store.delete(key);
      }
    }
    return this.store.size;
  }
}

// ─── File-Based Cache ────────────────────────────────────────────────────────

export class FileCache implements CacheProvider {
  private cacheDir: string;
  private maxEntries: number;

  constructor(cacheDir = '.rlm-cache', maxEntries = 1000) {
    this.cacheDir = path.resolve(cacheDir);
    this.maxEntries = maxEntries;
    if (!fs.existsSync(this.cacheDir)) {
      fs.mkdirSync(this.cacheDir, { recursive: true });
    }
  }

  private filePath(key: string): string {
    return path.join(this.cacheDir, `${key}.json`);
  }

  get<T>(key: string): T | undefined {
    const fp = this.filePath(key);
    if (!fs.existsSync(fp)) return undefined;

    try {
      const data = JSON.parse(fs.readFileSync(fp, 'utf-8')) as CacheEntry;
      if (Date.now() - data.createdAt > data.ttl * 1000) {
        fs.unlinkSync(fp);
        return undefined;
      }
      return data.value as T;
    } catch {
      return undefined;
    }
  }

  set<T>(key: string, value: T, ttl: number): void {
    const entry: CacheEntry = {
      key,
      value,
      createdAt: Date.now(),
      ttl,
      hitCount: 0,
    };

    try {
      fs.writeFileSync(this.filePath(key), JSON.stringify(entry), 'utf-8');
    } catch {
      // Silently fail on write errors
    }
  }

  has(key: string): boolean {
    return this.get(key) !== undefined;
  }

  delete(key: string): boolean {
    const fp = this.filePath(key);
    if (fs.existsSync(fp)) {
      fs.unlinkSync(fp);
      return true;
    }
    return false;
  }

  clear(): void {
    if (fs.existsSync(this.cacheDir)) {
      const files = fs.readdirSync(this.cacheDir);
      for (const file of files) {
        if (file.endsWith('.json')) {
          fs.unlinkSync(path.join(this.cacheDir, file));
        }
      }
    }
  }

  size(): number {
    if (!fs.existsSync(this.cacheDir)) return 0;
    return fs.readdirSync(this.cacheDir).filter(f => f.endsWith('.json')).length;
  }
}

// ─── RLM Cache Manager ──────────────────────────────────────────────────────

export class RLMCache {
  private provider: CacheProvider;
  private config: Required<CacheConfig>;
  private stats: CacheStats = { hits: 0, misses: 0, size: 0, hitRate: 0, evictions: 0 };

  constructor(config: CacheConfig = {}) {
    this.config = {
      enabled: config.enabled ?? false,
      strategy: config.strategy ?? 'exact',
      maxEntries: config.maxEntries ?? 1000,
      ttl: config.ttl ?? 3600,
      storage: config.storage ?? 'memory',
      cacheDir: config.cacheDir ?? '.rlm-cache',
    };

    if (this.config.storage === 'file') {
      this.provider = new FileCache(this.config.cacheDir, this.config.maxEntries);
    } else {
      this.provider = new MemoryCache(this.config.maxEntries);
    }
  }

  /** Check if caching is enabled */
  get enabled(): boolean {
    return this.config.enabled && this.config.strategy !== 'none';
  }

  /** Look up a cached result */
  lookup<T>(model: string, query: string, context: string, extra?: Record<string, unknown>): { hit: boolean; value?: T } {
    if (!this.enabled) return { hit: false };

    const key = generateCacheKey(model, query, context, extra);
    const value = this.provider.get<T>(key);

    if (value !== undefined) {
      this.stats.hits++;
      this.updateHitRate();
      return { hit: true, value };
    }

    this.stats.misses++;
    this.updateHitRate();
    return { hit: false };
  }

  /** Store a result in the cache */
  store<T>(model: string, query: string, context: string, value: T, extra?: Record<string, unknown>): void {
    if (!this.enabled) return;

    const key = generateCacheKey(model, query, context, extra);
    this.provider.set(key, value, this.config.ttl);
    this.stats.size = this.provider.size();
  }

  /** Get cache statistics */
  getStats(): CacheStats {
    this.stats.size = this.provider.size();
    return { ...this.stats };
  }

  /** Clear the cache */
  clear(): void {
    this.provider.clear();
    this.stats = { hits: 0, misses: 0, size: 0, hitRate: 0, evictions: 0 };
  }

  private updateHitRate(): void {
    const total = this.stats.hits + this.stats.misses;
    this.stats.hitRate = total > 0 ? this.stats.hits / total : 0;
  }
}
