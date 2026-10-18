// Static English-only legal page; prerender into HTML for SEO. Overrides the
// root layout's ssr=false so this page can be rendered at build time.
export const ssr = true;
export const prerender = true;
