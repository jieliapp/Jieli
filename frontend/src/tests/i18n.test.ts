import { describe, expect, it } from 'vitest';
import en from '../lib/i18n/en.json';
import zh from '../lib/i18n/zh.json';
import {
  localeFromLanguage,
  lookup,
  resolveLocale
} from '../lib/i18n';

describe('localeFromLanguage', () => {
  it('returns zh for zh-* languages', () => {
    expect(localeFromLanguage('zh-CN')).toBe('zh');
    expect(localeFromLanguage('zh-TW')).toBe('zh');
    expect(localeFromLanguage('ZH')).toBe('zh');
  });

  it('returns en for non-zh languages', () => {
    expect(localeFromLanguage('en-US')).toBe('en');
    expect(localeFromLanguage('fr')).toBe('en');
    expect(localeFromLanguage(undefined)).toBe('en');
  });
});

describe('resolveLocale precedence', () => {
  it('prefers a valid stored manual selection', () => {
    expect(
      resolveLocale({ stored: 'zh', preferred: 'en', languages: ['en-US'] })
    ).toBe('zh');
  });

  it('falls back to preferred locale when no manual selection', () => {
    expect(
      resolveLocale({ stored: null, preferred: 'zh', languages: ['en-US'] })
    ).toBe('zh');
  });

  it('uses browser languages when no stored/preferred', () => {
    expect(resolveLocale({ languages: ['zh-CN', 'en'] })).toBe('zh');
    expect(resolveLocale({ languages: ['en-GB'] })).toBe('en');
  });

  it('ignores invalid stored values', () => {
    expect(resolveLocale({ stored: 'de', languages: ['zh-CN'] })).toBe('zh');
  });

  it('defaults to en with no signals', () => {
    expect(resolveLocale({})).toBe('en');
  });
});

describe('lookup', () => {
  const messages = { nav: { threads: 'Threads' }, flat: 'x' };

  it('resolves nested keys', () => {
    expect(lookup(messages, 'nav.threads')).toBe('Threads');
  });

  it('returns the key when missing', () => {
    expect(lookup(messages, 'nav.missing')).toBe('nav.missing');
    expect(lookup(messages, 'no.such.path')).toBe('no.such.path');
  });

  it('exposes archived labels in both locales', () => {
    expect(lookup(en, 'threads.archived')).toBe('Archived');
    expect(lookup(zh, 'threads.archived')).toBe('已归档');
  });

  it('exposes public profile copy labels in both locales', () => {
    expect(lookup(en, 'profile.copyPublicUrl')).toBe('Click to share your profile page');
    expect(lookup(en, 'profile.copiedPublicUrl')).toBe('Profile URL copied');
    expect(lookup(zh, 'profile.copyPublicUrl')).toBe('点击分享你的个人页面');
    expect(lookup(zh, 'profile.copiedPublicUrl')).toBe('个人页面链接已复制');
  });
});
