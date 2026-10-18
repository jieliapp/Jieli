import { describe, expect, it } from 'vitest';
import { isSupportedAvatarMediaType } from '../lib/utils/avatar';

describe('isSupportedAvatarMediaType', () => {
  it('accepts attachment-backed avatar image formats', () => {
    expect(isSupportedAvatarMediaType('image/png')).toBe(true);
    expect(isSupportedAvatarMediaType('image/jpeg')).toBe(true);
    expect(isSupportedAvatarMediaType('image/gif')).toBe(true);
    expect(isSupportedAvatarMediaType('image/webp')).toBe(true);
  });

  it('rejects unsupported avatar media types', () => {
    expect(isSupportedAvatarMediaType('image/svg+xml')).toBe(false);
    expect(isSupportedAvatarMediaType('text/plain')).toBe(false);
    expect(isSupportedAvatarMediaType('')).toBe(false);
  });
});
