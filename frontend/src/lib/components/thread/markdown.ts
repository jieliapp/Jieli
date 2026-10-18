import MarkdownIt from 'markdown-it';
import DOMPurify from 'dompurify';
import { codeToHtml, type BundledLanguage } from 'shiki';

const md = new MarkdownIt({
  html: false, // do not pass through raw HTML from untrusted thread content
  linkify: true,
  breaks: true
});

const copyIcon =
  '<svg viewBox="0 0 24 24" width="15" height="15" fill="none" aria-hidden="true">' +
  '<rect width="13" height="13" x="8" y="8" rx="2" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round" />' +
  '<path d="M16 8V5.5A1.5 1.5 0 0 0 14.5 4h-9A1.5 1.5 0 0 0 4 5.5v9A1.5 1.5 0 0 0 5.5 16H8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />' +
  '</svg>';

// Render fenced code blocks as a labeled container (language tag + copy button)
// so the transcript matches a desktop editor look. The <pre><code> stays plain
// escaped text here; MarkdownView highlights it with Shiki after mount.
md.renderer.rules.fence = (tokens, idx) => {
  const token = tokens[idx];
  const info = token.info ? token.info.trim() : '';
  const lang = info.split(/\s+/)[0] || '';
  const label = lang || 'text';
  const escaped = md.utils.escapeHtml(token.content);
  const langAttr = md.utils.escapeHtml(lang);
  const langLabel = md.utils.escapeHtml(label);
  return (
    `<div class="code-block" data-lang="${langAttr}">` +
    `<div class="code-head"><span class="code-lang">${langLabel}</span>` +
    `<button type="button" class="code-copy" aria-label="Copy code">${copyIcon}</button></div>` +
    `<pre class="code-body"><code>${escaped}</code></pre></div>`
  );
};

const defaultLinkOpen =
  md.renderer.rules.link_open ??
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx];
  const href = token.attrGet('href') ?? '';
  if (/^https?:\/\//i.test(href)) {
    token.attrSet('target', '_blank');
    token.attrSet('rel', 'noopener noreferrer');
  }
  return defaultLinkOpen(tokens, idx, options, env, self);
};

md.validateLink = (url: string) => {
  const normalized = url.trim().toLowerCase();
  if (/^(?:javascript|vbscript|data):/.test(normalized)) return false;
  if (/^[a-z][a-z0-9+.-]*:/i.test(normalized)) {
    return /^(?:https?|mailto|tel|ftp|file):/i.test(normalized);
  }
  return true;
};

const allowedUri = /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|sms|cid|xmpp|file):|[^a-z]|[a-z+.-]+(?:[^a-z+.-:]|$))/i;

/**
 * Render Markdown to sanitized HTML. Thread content is user-controlled and
 * must be treated as untrusted (frontend.md §11).
 *
 * Results are memoized by source string: a long thread re-renders the same
 * message content on every reactive update / progressive-render tick, so the
 * cache keeps markdown-it + DOMPurify off the main thread after first paint.
 */
const renderCache = new Map<string, string>();
const RENDER_CACHE_LIMIT = 4000;

export interface MarkdownRenderOptions {
  codeComments?: boolean;
}

interface CodeComment {
  title: string;
  body: string;
  file: string;
  start: number;
  end: number;
  priority: number;
}

function codeCommentPath(file: string, start: number, end: number): string {
  const relative = file.match(/\/github\/[^/]+\/(.+)$/)?.[1] ?? file;
  return `${relative}:${start}${end !== start ? `-${end}` : ''}`;
}

function extractCodeComments(source: string): { markdown: string; comments: CodeComment[] } {
  const comments: CodeComment[] = [];
  const prefix = '::code-comment{';
  let cursor = 0;
  let markdown = '';

  while (cursor < source.length) {
    const startIndex = source.indexOf(prefix, cursor);
    if (startIndex < 0) break;

    let quoted = false;
    let escaped = false;
    let endIndex = -1;
    for (let i = startIndex + prefix.length; i < source.length; i += 1) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (char === '\\' && quoted) {
        escaped = true;
        continue;
      }
      if (char === '"') {
        quoted = !quoted;
        continue;
      }
      if (char === '}' && !quoted) {
        endIndex = i;
        break;
      }
    }

    if (endIndex < 0) break;

    const attributes = source.slice(startIndex + prefix.length, endIndex);
    const values = new Map<string, string>();
    let attributeCursor = 0;
    let valid = true;
    while (attributeCursor < attributes.length) {
      while (/\s/.test(attributes[attributeCursor] ?? '')) attributeCursor += 1;
      if (attributeCursor >= attributes.length) break;
      const keyMatch = attributes.slice(attributeCursor).match(/^([A-Za-z][A-Za-z0-9_]*)=/);
      if (!keyMatch) {
        valid = false;
        break;
      }
      const key = keyMatch[1];
      attributeCursor += keyMatch[0].length;
      if (attributes[attributeCursor] === '"') {
        const valueStart = attributeCursor;
        attributeCursor += 1;
        let valueEscaped = false;
        while (attributeCursor < attributes.length) {
          const char = attributes[attributeCursor];
          if (valueEscaped) {
            valueEscaped = false;
          } else if (char === '\\') {
            valueEscaped = true;
          } else if (char === '"') {
            break;
          }
          attributeCursor += 1;
        }
        if (attributeCursor >= attributes.length) {
          valid = false;
          break;
        }
        const encoded = attributes.slice(valueStart, attributeCursor + 1);
        try {
          values.set(key, JSON.parse(encoded) as string);
        } catch {
          valid = false;
          break;
        }
        attributeCursor += 1;
      } else {
        const valueMatch = attributes.slice(attributeCursor).match(/^([^\s]+)/);
        if (!valueMatch) {
          valid = false;
          break;
        }
        values.set(key, valueMatch[1]);
        attributeCursor += valueMatch[1].length;
      }
    }

    const title = values.get('title');
    const body = values.get('body');
    const file = values.get('file');
    const start = Number(values.get('start'));
    const end = Number(values.get('end'));
    const priority = Number(values.get('priority'));
    if (
      !valid ||
      title === undefined ||
      body === undefined ||
      file === undefined ||
      !Number.isInteger(start) ||
      !Number.isInteger(end) ||
      !Number.isInteger(priority)
    ) {
      markdown += source.slice(cursor, endIndex + 1);
      cursor = endIndex + 1;
      continue;
    }

    markdown += source.slice(cursor, startIndex);
    comments.push({ title, body, file, start, end, priority });
    cursor = endIndex + 1;
  }

  markdown += source.slice(cursor);
  return { markdown, comments };
}

function renderCodeComments(comments: CodeComment[]): string {
  if (comments.length === 0) return '';
  const rows = comments
    .map((comment) => {
      const title = comment.title.replace(/^\[P\d\]\s*/, '');
      const priority = `P${comment.priority}`;
      return (
        '<details class="code-comment-row">' +
        '<summary>' +
        `<span class="code-comment-priority">${md.utils.escapeHtml(priority)}</span>` +
        `<span class="code-comment-title">${md.utils.escapeHtml(title)}</span>` +
        `<span class="code-comment-path">${md.utils.escapeHtml(codeCommentPath(comment.file, comment.start, comment.end))}</span>` +
        '</summary>' +
        `<p class="code-comment-body">${md.utils.escapeHtml(comment.body)}</p>` +
        '</details>'
      );
    })
    .join('');
  const label = `${comments.length} ${comments.length === 1 ? 'comment' : 'comments'}`;
  return (
    '<section class="code-comments">' +
    '<div class="code-comments-head">' +
    '<svg class="code-comments-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 17.5 4.5 20v-4.2A7 7 0 0 1 3 11.5C3 7.36 7.03 4 12 4s9 3.36 9 7.5S16.97 19 12 19c-1.82 0-3.52-.45-5-1.5Z"/><path d="M8 10h8M8 13h5"/></svg>' +
    `<span>${label}</span></div>` +
    `<div class="code-comments-list">${rows}</div>` +
    '</section>'
  );
}

export function renderMarkdown(source: string, options: MarkdownRenderOptions = {}): string {
  const input = source ?? '';
  const codeComments = options.codeComments === true;
  const key = `${codeComments ? 'comments' : 'markdown'}\u0000${input}`;
  const cached = renderCache.get(key);
  if (cached !== undefined) return cached;

  const parsed = codeComments ? extractCodeComments(input) : { markdown: input, comments: [] };
  const raw = md.render(parsed.markdown) + renderCodeComments(parsed.comments);
  const html = DOMPurify.sanitize(raw, {
    ADD_ATTR: ['target', 'rel'],
    ADD_URI_SAFE_ATTR: ['d'],
    ALLOWED_URI_REGEXP: allowedUri,
    FORBID_TAGS: ['style', 'iframe', 'form', 'script'],
    FORBID_ATTR: ['onerror', 'onload', 'onclick']
  });

  if (renderCache.size >= RENDER_CACHE_LIMIT) {
    const oldest = renderCache.keys().next().value;
    if (oldest !== undefined) renderCache.delete(oldest);
  }
  renderCache.set(key, html);
  return html;
}

/** Render Markdown inline (no block wrapper) for short snippets. */
export function renderInline(source: string): string {
  const raw = md.renderInline(source ?? '');
  return DOMPurify.sanitize(raw, {
    ALLOWED_URI_REGEXP: allowedUri
  });
}

let highlighterReady = false;

export type CodeTheme = 'light' | 'dark';

const SHIKI_THEME: Record<CodeTheme, string> = {
  light: 'vitesse-light',
  dark: 'github-dark'
};

const highlightCache = new Map<string, string>();
const HIGHLIGHT_CACHE_LIMIT = 2000;

/**
 * Highlight a single code block with Shiki. Falls back to a plain escaped
 * <pre> if the language/highlighter fails. Results are memoized per
 * theme+lang+code so re-renders stay off the main thread.
 */
export async function highlightCode(
  code: string,
  lang: string,
  theme: CodeTheme = 'light'
): Promise<string> {
  const key = `${theme}\u0000${lang || 'text'}\u0000${code}`;
  const cached = highlightCache.get(key);
  if (cached !== undefined) return cached;

  let html: string;
  try {
    const raw = await codeToHtml(code, {
      lang: (lang || 'text') as BundledLanguage,
      theme: SHIKI_THEME[theme]
    });
    highlighterReady = true;
    html = DOMPurify.sanitize(raw);
  } catch {
    const escaped = code
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
    html = `<pre class="shiki-fallback"><code>${escaped}</code></pre>`;
  }

  if (highlightCache.size >= HIGHLIGHT_CACHE_LIMIT) {
    const oldest = highlightCache.keys().next().value;
    if (oldest !== undefined) highlightCache.delete(oldest);
  }
  highlightCache.set(key, html);
  return html;
}

export function isHighlighterReady(): boolean {
  return highlighterReady;
}
