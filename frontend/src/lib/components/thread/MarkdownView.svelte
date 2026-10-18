<script lang="ts">
  import { renderMarkdown, highlightCode, type CodeTheme } from './markdown';
  import { tick } from 'svelte';

  let {
    source,
    codeTheme = 'light',
    codeComments = false
  }: { source: string; codeTheme?: CodeTheme; codeComments?: boolean } = $props();

  const html = $derived(renderMarkdown(source, { codeComments }));

  let container = $state<HTMLDivElement>();

  $effect(() => {
    // re-run when rendered html or theme changes
    void html;
    void codeTheme;
    const root = container;
    if (!root) return;
    let cancelled = false;

    (async () => {
      await tick();
      const blocks = root.querySelectorAll<HTMLElement>('.code-block');
      for (const block of blocks) {
        const code = block.querySelector('code');
        const body = block.querySelector<HTMLElement>('.code-body');
        if (!code || !body) continue;
        const raw = code.textContent ?? '';
        if (block.dataset.raw === undefined) block.dataset.raw = raw;
        const lang = block.dataset.lang ?? '';
        const highlighted = await highlightCode(block.dataset.raw, lang, codeTheme);
        if (cancelled) return;
        body.innerHTML = highlighted;
      }
    })();

    return () => {
      cancelled = true;
    };
  });

  $effect(() => {
    const root = container;
    if (!root) return;
    const handler = (event: MouseEvent) => {
      const button = (event.target as HTMLElement)?.closest<HTMLElement>('.code-copy');
      if (button) onCopy(button);
    };
    root.addEventListener('click', handler);
    return () => root.removeEventListener('click', handler);
  });

  const checkIcon =
    '<svg viewBox="0 0 24 24" width="15" height="15" fill="none" aria-hidden="true">' +
    '<path d="M5 12.5 10 17.5 19 7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" /></svg>';

  async function onCopy(button: HTMLElement) {
    const block = button.closest<HTMLElement>('.code-block');
    const raw = block?.dataset.raw;
    if (raw === undefined) return;
    try {
      await navigator.clipboard.writeText(raw);
      if (button.dataset.icon === undefined) button.dataset.icon = button.innerHTML;
      button.classList.add('copied');
      button.innerHTML = checkIcon;
      const original = button.dataset.icon;
      setTimeout(() => {
        button.classList.remove('copied');
        if (original !== undefined) button.innerHTML = original;
      }, 1200);
    } catch {
      /* clipboard unavailable */
    }
  }
</script>

<!-- Sanitized via DOMPurify in renderMarkdown -->
<div class="markdown code-theme-{codeTheme}" bind:this={container}>{@html html}</div>

<style>
  .markdown > :global(:first-child) {
    margin-top: 0;
  }

  .markdown {
    min-width: 0;
    max-width: 100%;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .markdown > :global(:last-child) {
    margin-bottom: 0;
  }

  .markdown :global(p) {
    margin: 0 0 0.8rem;
  }

  .markdown :global(h1),
  .markdown :global(h2),
  .markdown :global(h3),
  .markdown :global(h4) {
    margin: 1.1rem 0 0.55rem;
    line-height: 1.25;
  }

  .markdown :global(h1) {
    font-size: 1.2rem;
  }

  .markdown :global(h2) {
    font-size: 1.08rem;
  }

  .markdown :global(h3) {
    font-size: 1rem;
  }

  .markdown :global(h4) {
    font-size: 0.95rem;
  }

  .markdown :global(h1:first-child),
  .markdown :global(h2:first-child),
  .markdown :global(h3:first-child),
  .markdown :global(h4:first-child) {
    margin-top: 0;
  }

  .markdown :global(pre) {
    font-family: var(--font-code);
    font-size: 1em;
    line-height: 1.5;
    background: transparent;
    border: none;
    border-radius: 0;
    padding: 0;
    overflow-x: auto;
  }

  .markdown :global(.code-block) {
    margin: 0 0 0.8rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg-inset);
    overflow: hidden;
  }

  .code-theme-dark :global(.code-block) {
    border-color: #2b3038;
    background: #24292e;
  }

  .markdown :global(.code-head) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: 0.3rem 0.5rem 0.3rem 0.85rem;
    border-bottom: 1px solid var(--color-border);
    background: color-mix(in srgb, var(--color-bg-inset) 60%, transparent);
  }

  .code-theme-dark :global(.code-head) {
    border-bottom-color: #2b3038;
    background: #1f2428;
  }

  .markdown :global(.code-lang) {
    font-family: var(--font-code);
    font-size: 0.78em;
    color: var(--color-text-muted);
    text-transform: lowercase;
    user-select: none;
  }

  .code-theme-dark :global(.code-lang) {
    color: #9aa4af;
  }

  .markdown :global(.code-copy) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
  }

  .markdown :global(.code-copy:hover) {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .code-theme-dark :global(.code-copy) {
    color: #9aa4af;
  }

  .code-theme-dark :global(.code-copy:hover) {
    background: #30363d;
    color: #e6edf3;
  }

  .markdown :global(.code-copy.copied) {
    color: var(--color-success, #2e7d32);
  }

  .code-theme-dark :global(.code-copy.copied) {
    color: #4caf50;
  }

  .markdown :global(.code-body) {
    margin: 0;
    padding: 0.75rem 0.85rem;
    overflow-x: auto;
  }

  .markdown :global(.code-body pre) {
    margin: 0;
    background: transparent !important;
  }

  .markdown :global(.code-body .shiki) {
    background: transparent !important;
  }

  .markdown :global(code) {
    font-family: var(--font-code);
    background: color-mix(in srgb, var(--color-bg-inset) 72%, transparent);
    padding: 0 4px;
    border-radius: var(--radius-sm);
    color: var(--color-text);
    font-size: 1em;
  }

  .markdown :global(pre code),
  .markdown :global(.code-body code) {
    background: none;
    padding: 0;
    font-size: inherit;
  }

  .markdown :global(a) {
    word-break: break-word;
  }

  .markdown :global(ul),
  .markdown :global(ol) {
    margin: 0 0 0.8rem;
    padding-left: 1.5em;
  }

  .markdown :global(li + li) {
    margin-top: 0.25rem;
  }

  .markdown :global(blockquote) {
    margin: 0 0 0.8rem;
    padding-left: var(--space-3);
    border-left: 2px solid var(--color-border-strong);
    color: var(--color-text-muted);
  }

  .markdown :global(table) {
    border-collapse: collapse;
    width: auto;
    max-width: 100%;
    margin: var(--space-3) 0 var(--space-4);
    color: var(--color-text-muted);
    font-size: 0.95em;
    line-height: 1.45;
  }

  .markdown :global(th),
  .markdown :global(td) {
    border: none;
    border-bottom: 1px solid var(--color-border);
    padding: var(--space-1) var(--space-4) var(--space-1) 0;
    text-align: left;
    vertical-align: top;
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .markdown :global(th) {
    color: var(--color-text);
    font-weight: 600;
  }

  .markdown :global(th:last-child),
  .markdown :global(td:last-child) {
    padding-right: 0;
  }

  .markdown :global(img) {
    max-width: 100%;
  }

  .markdown :global(.code-comments) {
    margin: var(--space-4) 0 0;
    overflow: hidden;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-bg-elevated);
  }

  .markdown :global(.code-comments-head) {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: 52px;
    padding: 0 var(--space-4);
    border-bottom: 1px solid var(--color-border);
    color: var(--color-text);
    font-size: 0.95em;
    font-weight: 600;
  }

  .markdown :global(.code-comments-icon) {
    width: 20px;
    height: 20px;
    flex: 0 0 20px;
    fill: none;
    stroke: var(--color-text-muted);
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .markdown :global(.code-comment-row) {
    border-bottom: 1px solid var(--color-border);
  }

  .markdown :global(.code-comment-row:last-child) {
    border-bottom: 0;
  }

  .markdown :global(.code-comment-row summary) {
    display: grid;
    grid-template-columns: auto minmax(0, auto) minmax(0, 1fr);
    align-items: center;
    gap: var(--space-3);
    min-height: 44px;
    padding: 0 var(--space-4);
    cursor: pointer;
    list-style: none;
  }

  .markdown :global(.code-comment-row summary::-webkit-details-marker) {
    display: none;
  }

  .markdown :global(.code-comment-row summary:hover) {
    background: var(--color-bg-hover);
  }

  .markdown :global(.code-comment-priority) {
    padding: 1px 6px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    font-family: var(--font-code);
    font-size: 0.82em;
    line-height: 1.45;
  }

  .markdown :global(.code-comment-title) {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text);
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .markdown :global(.code-comment-path) {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text-muted);
    font-family: var(--font-code);
    font-size: 0.9em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .markdown :global(.code-comment-body) {
    margin: 0;
    padding: 0 var(--space-4) var(--space-4) calc(var(--space-4) + 42px);
    color: var(--color-text-muted);
    line-height: 1.55;
    white-space: pre-wrap;
  }

  @media (max-width: 640px) {
    .markdown :global(.code-comment-row summary) {
      grid-template-columns: auto minmax(0, 1fr);
      gap: var(--space-2) var(--space-3);
      padding-top: var(--space-2);
      padding-bottom: var(--space-2);
    }

    .markdown :global(.code-comment-path) {
      grid-column: 2;
      width: 100%;
    }

    .markdown :global(.code-comment-body) {
      padding-left: calc(var(--space-4) + 38px);
    }
  }
</style>
