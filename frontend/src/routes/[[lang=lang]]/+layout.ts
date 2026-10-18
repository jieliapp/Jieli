import { browser } from '$app/environment';
import { locale } from '$lib/stores/locale';
import { LOCALE_STORAGE_KEY, resolveLocale, type Locale } from '$lib/i18n';

// This group re-enables SSR + prerender (the root layout disables SSR for the
// SPA shell). Pages here are prerendered into static HTML for SEO.
export const ssr = true;
export const prerender = true;
export const trailingSlash = 'ignore';

export const load = ({ params, url }: { params: { lang?: string }; url: URL }) => {
  const lang: Locale = params.lang === 'zh' ? 'zh' : 'en';
  if (browser) {
    // Explicit marketing URLs are stable by path: /zh* is Chinese, unprefixed
    // subpages are English. Only / follows stored/browser preference because Go
    // may serve the Chinese home at the unprefixed entry based on Accept-Language.
    const browserHomeLocale = resolveLocale({
      stored: localStorage.getItem(LOCALE_STORAGE_KEY),
      languages: navigator.languages
    });
    locale.set(params.lang === 'zh' ? 'zh' : url.pathname === '/' ? browserHomeLocale : lang);
  } else {
    // Prerender: deterministic language by URL so each variant is stable.
    locale.set(lang);
  }
  return { lang };
};
