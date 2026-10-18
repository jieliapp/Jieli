import type { Handle } from '@sveltejs/kit';

export const handle: Handle = async ({ event, resolve }) => {
  const htmlLang = event.url.pathname === '/zh' || event.url.pathname.startsWith('/zh/') ? 'zh' : 'en';

  return resolve(event, {
    transformPageChunk: ({ html }) => html.replace('<html lang="en"', `<html lang="${htmlLang}"`)
  });
};
