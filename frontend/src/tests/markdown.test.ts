// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { renderMarkdown } from '../lib/components/thread/markdown';

describe('renderMarkdown', () => {
  it('renders basic markdown to HTML', () => {
    const html = renderMarkdown('# Title\n\nSome **bold** text.');
    expect(html).toContain('<h1>Title</h1>');
    expect(html).toContain('<strong>bold</strong>');
  });

  it('renders fenced code blocks', () => {
    const html = renderMarkdown('```js\nconst x = 1;\n```');
    expect(html).toContain('class="code-block"');
    expect(html).toContain('data-lang="js"');
    expect(html).toContain('class="code-copy"');
    expect(html).toContain('<code');
  });

  it('preserves plain text line breaks', () => {
    const html = renderMarkdown('url=redis://:[REDACTED]@127.0.0.1:6379/0\npassword=[REDACTED]');
    expect(html).toContain('url=redis://:[REDACTED]@127.0.0.1:6379/0<br>');
    expect(html).toContain('password=[REDACTED]');
  });

  it('renders Codex code comments as a compact review card after the remaining Markdown', () => {
    const html = renderMarkdown(
      '发现问题。\n\n::code-comment{title="[P1] 配置错误" body="缺少 low/high。" file="/workspace/example/backend/routes/neo.go" start=12 end=13 priority=1}\n\n除此之外没有问题。',
      { codeComments: true }
    );

    expect(html).not.toContain('::code-comment');
    expect(html).toContain('发现问题。');
    expect(html).toContain('除此之外没有问题。');
    expect(html).toContain('1 comment');
    expect(html).toContain('P1');
    expect(html).toContain('配置错误');
    expect(html).toContain('backend/routes/neo.go:12-13');
    expect(html.indexOf('除此之外没有问题。')).toBeLessThan(html.indexOf('1 comment'));
  });

  it('parses escaped quotes and closing braces inside code comment attributes', () => {
    const html = renderMarkdown(
      '::code-comment{title="[P2] Handle \\"quoted\\" values" body="JSON like {\\"ok\\": true} must remain intact." file="/repo/src/parser.ts" start=7 end=7 priority=2}',
      { codeComments: true }
    );

    expect(html).not.toContain('::code-comment');
    expect(html).toContain('Handle "quoted" values');
    expect(html).toContain('JSON like {"ok": true} must remain intact.');
    expect(html).toContain('/repo/src/parser.ts:7');
  });

  it('keeps code comment directives as text when structured rendering is disabled', () => {
    const html = renderMarkdown(
      '::code-comment{title="[P1] User text" body="Do not transform me." file="/repo/file.ts" start=1 end=1 priority=1}',
      { codeComments: false }
    );

    expect(html).toContain('::code-comment');
    expect(html).not.toContain('class="code-comments"');
  });

  it('groups multiple comments and preserves malformed directives as text', () => {
    const html = renderMarkdown(
      '::code-comment{title="[P1] First" body="Important." file="/workspace/example/backend/a.go" start=2 end=3 priority=1}\n\n::code-comment{title="[P3] Second" body="Minor." file="/workspace/example/backend/b.go" start=8 end=8 priority=3}\n\n::code-comment{title="Missing fields"}',
      { codeComments: true }
    );

    expect(html).toContain('2 comments');
    expect(html).toContain('backend/a.go:2-3');
    expect(html).toContain('backend/b.go:8');
    expect(html).toContain('::code-comment{title="Missing fields"}');
  });

  it('renders absolute http links as external links', () => {
    const html = renderMarkdown('https://jieli.app/threads/T-example');
    expect(html).toContain('href="https://jieli.app/threads/T-example"');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it('strips script tags from untrusted content', () => {
    const html = renderMarkdown('hi <script>alert(1)</script>');
    // html:false escapes raw HTML, so the script becomes inert text.
    expect(html).not.toContain('<script>');
    expect(html).toContain('&lt;script&gt;');
  });

  it('does not pass through raw inline HTML', () => {
    const html = renderMarkdown('<img src=x onerror=alert(1)>');
    // html:false escapes raw HTML, so no live <img> element is produced.
    expect(html).not.toContain('<img');
    expect(html).toContain('&lt;img');
  });
});
