export type Locale = 'en' | 'zh';

export const SUPPORTED_LOCALES: Locale[] = ['en', 'zh'];
export const DEFAULT_LOCALE: Locale = 'en';
export const LOCALE_STORAGE_KEY = 'jieli.locale';

/**
 * Resolve locale from a browser language string.
 * Chinese for `zh-*`, otherwise English.
 */
export function localeFromLanguage(lang: string | undefined | null): Locale {
  if (!lang) return DEFAULT_LOCALE;
  return lang.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

/**
 * Resolve the effective locale following frontend.md §7 precedence:
 * 1. manual selection (localStorage)
 * 2. signed-in user preferred_locale
 * 3. browser language
 * 4. default (en)
 */
export function resolveLocale(opts: {
  stored?: string | null;
  preferred?: string | null;
  languages?: readonly string[];
}): Locale {
  const { stored, preferred, languages } = opts;
  if (stored && SUPPORTED_LOCALES.includes(stored as Locale)) {
    return stored as Locale;
  }
  if (preferred && SUPPORTED_LOCALES.includes(preferred as Locale)) {
    return preferred as Locale;
  }
  const candidates = languages && languages.length ? languages : [];
  for (const lang of candidates) {
    if (lang.toLowerCase().startsWith('zh')) return 'zh';
    if (lang.toLowerCase().startsWith('en')) return 'en';
  }
  return DEFAULT_LOCALE;
}

/** Resolve a dotted key path against a messages object. */
export function lookup(
  messages: Record<string, unknown>,
  key: string
): string {
  const parts = key.split('.');
  let current: unknown = messages;
  for (const part of parts) {
    if (current && typeof current === 'object' && part in current) {
      current = (current as Record<string, unknown>)[part];
    } else {
      return key;
    }
  }
  return typeof current === 'string' ? current : key;
}
