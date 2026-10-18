import { sveltekit } from '@sveltejs/kit/vite';
import type { Plugin } from 'vite';
import { defineConfig } from 'vitest/config';

function jieliThreadAgentHint(): Plugin {
  return {
    name: 'jieli-thread-agent-hint',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = req.url?.split('?')[0] ?? '';
        const match = /^\/threads\/(T-[^/.]+)$/.exec(url);
        if (!match || wantsBrowserHTML(req.headers.accept, req.headers['user-agent'])) {
          next();
          return;
        }
        const threadID = match[1];
        const message =
          `This is a Jieli thread page served by a SPA. Agents should not use WebFetch or curl on /threads/${threadID}. ` +
          `If the Jieli plugin is installed, use the jieli skill. The skill reads /threads/${threadID}.md with Authorization: Bearer $JIELI_API_KEY.\n`;
        res.statusCode = 401;
        res.setHeader('Content-Type', 'text/plain; charset=utf-8');
        res.end(message);
      });
    }
  };
}

function wantsBrowserHTML(accept: string | string[] | undefined, userAgent: string | string[] | undefined): boolean {
  const ua = String(Array.isArray(userAgent) ? userAgent.join(' ') : (userAgent ?? '')).toLowerCase();
  if (
    ua.includes('curl/') ||
    ua.includes('wget/') ||
    ua.includes('python-requests') ||
    ua.includes('go-http-client') ||
    ua.includes('node-fetch') ||
    ua.includes('undici')
  ) {
    return false;
  }
  return String(Array.isArray(accept) ? accept.join(',') : (accept ?? '')).toLowerCase().includes('text/html');
}

export default defineConfig({
  plugins: [jieliThreadAgentHint(), sveltekit()],
  server: {
    proxy: {
      '/web/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '^/threads/[^/]+\\.(md|json)$': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/attachments': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  test: {
    include: ['src/tests/**/*.{test,spec}.{js,ts}', ...(process.env.JIELI_EDITION === 'cloud' ? ['src/cloud/tests/**/*.{test,spec}.{js,ts}'] : [])]
  }
});
