import { derived, get, writable } from 'svelte/store';
import en from '../i18n/en.json';
import { messages as editionMessages } from '$commercial/messages';
import zh from '../i18n/zh.json';
import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  lookup,
  resolveLocale,
  SUPPORTED_LOCALES,
  type Locale
} from '../i18n';

const messagesByLocale: Record<Locale, Record<string, unknown>> = {
  en: { ...en, settings: { ...en.settings, ...editionMessages.en?.settings } },
  zh: { ...zh, settings: { ...zh.settings, ...editionMessages.zh?.settings } }
};

const browser = typeof window !== 'undefined';

function initialLocale(): Locale {
  if (!browser) return DEFAULT_LOCALE;
  return resolveLocale({
    stored: storedLocale(),
    languages: browserLanguages()
  });
}

export const locale = writable<Locale>(initialLocale());

/** Translation function store: `$t('nav.threads')`. */
export const t = derived(locale, ($locale) => {
  const messages = messagesByLocale[$locale] ?? messagesByLocale[DEFAULT_LOCALE];
  return (key: string) => lookup(messages, key);
});

/**
 * Prefix an app-internal path with the current locale segment (`/zh`) so that
 * navigation from a Chinese page stays on prerendered Chinese variants.
 * Only paths for localized marketing routes need this; English stays unprefixed.
 */
export const localizedHref = derived(locale, ($locale) => {
  return (path: string): string => {
    if ($locale !== 'zh') return path;
    if (path === '/') return '/zh';
    return '/zh' + path;
  };
});

/** Translate once outside reactive contexts. */
export function translate(key: string): string {
  const $locale = get(locale);
  const messages = messagesByLocale[$locale] ?? messagesByLocale[DEFAULT_LOCALE];
  return lookup(messages, key);
}

/**
 * Set locale manually: update UI, persist client-side, and (optionally)
 * notify the server when signed in. Server sync is handled by the caller.
 */
export function setLocale(next: Locale) {
  if (!SUPPORTED_LOCALES.includes(next)) return;
  locale.set(next);
  const storage = browserLocalStorage();
  if (browser) {
    try {
      storage?.setItem(LOCALE_STORAGE_KEY, next);
    } catch {
      // Browser storage can be unavailable in private or restricted contexts.
    }
    document.documentElement.lang = next;
  }
}

/** Apply a server-provided preferred locale only if no manual choice exists. */
export function applyPreferredLocale(preferred?: string | null) {
  if (!browser || !preferred) return;
  const stored = storedLocale();
  if (stored) return;
  const resolved = resolveLocale({ preferred, languages: browserLanguages() });
  locale.set(resolved);
  document.documentElement.lang = resolved;
}

function storedLocale(): string | null {
  try {
    return browserLocalStorage()?.getItem(LOCALE_STORAGE_KEY) ?? null;
  } catch {
    return null;
  }
}

function browserLocalStorage(): Storage | null {
  if (!browser || typeof localStorage === 'undefined') return null;
  return localStorage;
}

function browserLanguages(): readonly string[] {
  return typeof navigator === 'undefined' ? [] : navigator.languages;
}
