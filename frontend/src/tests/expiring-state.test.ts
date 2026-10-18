import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearExpiringState, loadExpiringState, saveExpiringState } from '$lib/expiringState';

class MemoryStorage implements Pick<Storage, 'getItem' | 'setItem' | 'removeItem'> {
  private values = new Map<string, string>();

  getItem(key: string) {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string) {
    this.values.set(key, value);
  }

  removeItem(key: string) {
    this.values.delete(key);
  }
}

afterEach(() => {
  vi.useRealTimers();
});

describe('expiring storage state', () => {
  it('loads saved state until it expires', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
    const storage = new MemoryStorage();

    saveExpiringState(storage, 'checkout:', 'sub2_42', { outTradeNo: 'sub2_42' });

    expect(loadExpiringState<{ outTradeNo: string }>(storage, 'checkout:', 'sub2_42', 30 * 60 * 1000)).toEqual({
      outTradeNo: 'sub2_42'
    });

    vi.setSystemTime(new Date('2026-01-01T00:31:00Z'));

    expect(loadExpiringState(storage, 'checkout:', 'sub2_42', 30 * 60 * 1000)).toBeNull();
    expect(storage.getItem('checkout:sub2_42')).toBeNull();
  });

  it('clears malformed state', () => {
    const storage = new MemoryStorage();
    storage.setItem('checkout:sub2_42', '{bad json');

    expect(loadExpiringState(storage, 'checkout:', 'sub2_42', 30 * 60 * 1000)).toBeNull();
    expect(storage.getItem('checkout:sub2_42')).toBeNull();
  });

  it('clears envelopes without saved state', () => {
    const storage = new MemoryStorage();
    storage.setItem('checkout:sub2_42', JSON.stringify({ createdAtMs: Date.now() }));

    expect(loadExpiringState(storage, 'checkout:', 'sub2_42', 30 * 60 * 1000)).toBeNull();
    expect(storage.getItem('checkout:sub2_42')).toBeNull();
  });

  it('clears saved state by prefix and id', () => {
    const storage = new MemoryStorage();
    saveExpiringState(storage, 'setup:', 'seti_42', { setupIntentID: 'seti_42' });

    clearExpiringState(storage, 'setup:', 'seti_42');

    expect(loadExpiringState(storage, 'setup:', 'seti_42', 30 * 60 * 1000)).toBeNull();
  });
});
