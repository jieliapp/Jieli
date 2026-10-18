import { describe, expect, it } from 'vitest';
import en from '../lib/i18n/en.json';
import zh from '../lib/i18n/zh.json';
import { lookup } from '../lib/i18n';
import { quotaLevel, quotaPercent } from '../lib/utils/quota';

describe('quotaLevel water marks', () => {
  it('is neutral below 90%', () => {
    expect(quotaLevel(0, 30000)).toBe('neutral');
    expect(quotaLevel(20999, 30000)).toBe('neutral');
    expect(quotaLevel(26999, 30000)).toBe('neutral');
  });

  it('is warning from 90% up to (but not including) 100%', () => {
    expect(quotaLevel(27000, 30000)).toBe('warning');
    expect(quotaLevel(29999, 30000)).toBe('warning');
  });

  it('is danger at or above 100% (quota full)', () => {
    expect(quotaLevel(30000, 30000)).toBe('danger');
    expect(quotaLevel(45000, 30000)).toBe('danger');
  });

  it('treats a zero or missing quota as neutral without dividing by zero', () => {
    expect(quotaLevel(10, 0)).toBe('neutral');
  });
});

describe('quotaPercent', () => {
  it('clamps to the 0..100 range', () => {
    expect(quotaPercent(0, 30000)).toBe(0);
    expect(quotaPercent(15000, 30000)).toBe(50);
    expect(quotaPercent(45000, 30000)).toBe(100);
    expect(quotaPercent(10, 0)).toBe(0);
  });
});

describe('auto-cleanup i18n', () => {
  it('exposes quota and cleanup labels in both locales', () => {
    expect(lookup(en, 'settings.autoCleanup')).toBe('Auto-cleanup');
    expect(lookup(en, 'settings.autoCleanupConfirmTitle')).toBe('Enable auto-cleanup?');
    expect(lookup(zh, 'settings.autoCleanup')).toBe('自动清理');
    expect(lookup(zh, 'settings.messageQuota')).toBe('消息存储');
  });

  it('exposes quota exceeded upload error copy in both locales', () => {
    expect(lookup(en, 'settings.messageQuotaExceededTitle')).toBe('Sync paused: message storage is full');
    expect(lookup(zh, 'settings.messageQuotaExceededTitle')).toBe('同步已暂停：消息存储已满');
  });
});
