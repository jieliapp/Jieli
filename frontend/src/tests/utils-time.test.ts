import { describe, expect, it, vi } from 'vitest';
import { timeAgo } from '../lib/utils';

describe('timeAgo', () => {
  it('does not show negative seconds when local clock is behind the server timestamp', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-06-13T12:00:00Z'));

    expect(timeAgo('2026-06-13T12:02:29Z')).toBe('0s');

    vi.useRealTimers();
  });
});
