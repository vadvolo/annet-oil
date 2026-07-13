import crypto from 'crypto';

interface CacheEntry<T> {
  value: T;
  expiresAt: number;
}

export class ResponseCache<T> {
  private cache = new Map<string, CacheEntry<T>>();
  private ttlMs: number;

  constructor(ttlSeconds = 300) {
    this.ttlMs = ttlSeconds * 1000;
  }

  get(key: string): { value: T; hit: boolean } | null {
    const entry = this.cache.get(key);
    if (!entry) {
      return null;
    }

    if (Date.now() > entry.expiresAt) {
      this.cache.delete(key);
      return null;
    }

    return { value: entry.value, hit: true };
  }

  set(key: string, value: T): void {
    this.cache.set(key, {
      value,
      expiresAt: Date.now() + this.ttlMs,
    });
  }

  invalidate(key: string): void {
    this.cache.delete(key);
  }

  invalidateAll(): void {
    this.cache.clear();
  }

  static hashRequest(obj: unknown): string {
    const json = JSON.stringify(obj);
    return crypto.createHash('sha256').update(json).digest('hex');
  }
}
