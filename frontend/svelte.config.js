import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: 'dist',
      assets: 'dist',
      fallback: 'spa.html',
      precompress: false,
      strict: false
    }),
    alias: {
      $lib: 'src/lib',
      $commercial: 'src/public'
    },
    prerender: {
      // Links like /web/api/auth/github point at backend routes that don't
      // exist during prerender; ignore them instead of failing the build.
      handleHttpError: ({ path, message }) => {
        if (path.startsWith('/web/api/') || path.startsWith('/api/') || path.startsWith('/attachments/')) {
          return;
        }
        throw new Error(message);
      }
    }
  }
};

export default config;
