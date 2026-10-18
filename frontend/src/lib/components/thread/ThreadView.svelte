<script lang="ts">
  import { t } from '$lib/stores/locale';
  import { timeAgo } from '$lib/utils';
  import { normalizeMessages, userAnchors } from './normalize';
  import { groupThreadBlocks } from './work-groups';
  import MarkdownView from './MarkdownView.svelte';
  import WorkGroup from './WorkGroup.svelte';
  import type { ThreadDetail } from '$lib/api/types';
  import type { RenderUnit } from './normalize';
  import { onMount, type Snippet } from 'svelte';

  let {
    thread,
    editableTitle = false,
    ontitlechange = undefined,
    titleAction = undefined
  }: {
    thread: ThreadDetail;
    editableTitle?: boolean;
    ontitlechange?: (title: string) => Promise<void> | void;
    titleAction?: Snippet;
  } = $props();

  const turns = $derived(normalizeMessages(thread));
  const anchors = $derived(userAnchors(turns));
  const blocks = $derived(groupThreadBlocks(turns));

  // Progressive rendering: a long thread (hundreds of turns) would otherwise
  // run markdown-it + DOMPurify for every block synchronously on first paint,
  // blocking the main thread for seconds even when all data is cached. Render
  // an initial slice immediately, then grow during idle time until complete.
  const INITIAL_RENDER = 40;
  const RENDER_BATCH = 40;
  let renderCount = $state(INITIAL_RENDER);
  const visibleBlocks = $derived(blocks.slice(0, renderCount));

  // Reset progressive-render state whenever the thread changes. Anchor ids are
  // reused across threads (turn-0, turn-1, ...), so the observed set and the
  // observer itself must be cleared or the new thread's anchors get skipped.
  $effect(() => {
    void thread.provider_thread_id;
    renderCount = INITIAL_RENDER;
    scrollObserver?.disconnect();
    observedAnchors.clear();
  });

  const requestIdle: (cb: () => void) => number =
    typeof requestIdleCallback === 'function'
      ? (cb) => requestIdleCallback(() => cb())
      : (cb) => window.setTimeout(cb, 16);
  const cancelIdle: (handle: number) => void =
    typeof cancelIdleCallback === 'function' ? cancelIdleCallback : clearTimeout;

  // Grow the render window during idle time until every block is mounted, so
  // navigation (anchors, in-page search) eventually sees the full transcript.
  // Reading renderCount synchronously makes this effect re-run after each
  // batch, scheduling the next one until the whole thread is rendered.
  $effect(() => {
    const current = renderCount;
    const total = blocks.length;
    if (current >= total) return;
    const handle = requestIdle(() => {
      renderCount = Math.min(current + RENDER_BATCH, total);
    });
    return () => cancelIdle(handle);
  });

  let activeId = $state<string>('');
  let editingTitle = $state(false);
  let titleDraft = $state('');
  let titleBusy = $state(false);
  let previewImage = $state<string | null>(null);
  let showScrollBottom = $state(false);
  let lastScrollY = 0;

  function jumpTo(id: string) {
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      return;
    }
    // Target not rendered yet: render everything, then scroll once mounted.
    renderCount = blocks.length;
    requestAnimationFrame(() => {
      document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
  }

  function updateScrollBottomButton() {
    const currentY = window.scrollY;
    const distanceToBottom = document.documentElement.scrollHeight - (currentY + window.innerHeight);
    const isScrollingDown = currentY > lastScrollY;
    showScrollBottom = isScrollingDown && currentY > 320 && distanceToBottom > window.innerHeight;
    lastScrollY = currentY;
  }

  function scrollToBottom() {
    // Force the full thread to render first; otherwise we only scroll to the
    // bottom of the currently-rendered slice and later batches leave us
    // stranded mid-thread.
    renderCount = blocks.length;
    showScrollBottom = false;
    requestAnimationFrame(() => {
      window.scrollTo({ top: document.documentElement.scrollHeight, behavior: 'smooth' });
    });
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') previewImage = null;
  }

  function imageUnits(units: RenderUnit[]) {
    return units.filter((unit): unit is Extract<RenderUnit, { kind: 'image' }> => unit.kind === 'image');
  }

  function textUnits(units: RenderUnit[]) {
    return units.filter((unit): unit is Extract<RenderUnit, { kind: 'text' }> => unit.kind === 'text');
  }

  let scrollObserver: IntersectionObserver | null = null;
  const observedAnchors = new Set<string>();

  onMount(() => {
    scrollObserver = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeId = entry.target.id;
          }
        }
      },
      { rootMargin: '-20% 0px -60% 0px', threshold: 0 }
    );
    updateScrollBottomButton();
    return () => {
      scrollObserver?.disconnect();
      scrollObserver = null;
      observedAnchors.clear();
    };
  });

  // Observe anchor elements as progressive rendering mounts them. Reruns when
  // anchors or the render window grow; already-observed ids are skipped.
  $effect(() => {
    void renderCount;
    if (!scrollObserver) return;
    for (const anchor of anchors) {
      if (observedAnchors.has(anchor.id)) continue;
      const el = document.getElementById(anchor.id);
      if (el) {
        scrollObserver.observe(el);
        observedAnchors.add(anchor.id);
      }
    }
  });

  function toolBody(input: unknown, output?: string): string {
    const parts: string[] = [];
    if (input !== undefined && input !== null && !(typeof input === 'object' && Object.keys(input).length === 0)) {
      parts.push('```json\n' + JSON.stringify(input, null, 2) + '\n```');
    }
    if (output) {
      parts.push('```\n' + output + '\n```');
    }
    return parts.join('\n\n');
  }

  const displayTitle = $derived(thread.title || thread.provider_thread_id);
  const ownerName = $derived(thread.owner?.display_name || thread.owner?.username || '');
  const ownerProfileUrl = $derived(thread.owner?.username ? `/@${encodeURIComponent(thread.owner.username)}` : '');

  function startTitleEdit() {
    titleDraft = displayTitle;
    editingTitle = true;
  }

  function cancelTitleEdit() {
    titleDraft = '';
    editingTitle = false;
  }

  async function saveTitle() {
    const next = titleDraft.trim();
    if (!next || next === displayTitle) {
      cancelTitleEdit();
      return;
    }
    titleBusy = true;
    try {
      await ontitlechange?.(next);
      editingTitle = false;
      titleDraft = '';
    } finally {
      titleBusy = false;
    }
  }
</script>

<div class="thread-view">
  {#if anchors.length > 1}
    <nav class="prompt-nav" aria-label={$t('detail.userMessages')}>
      <div class="nav-inner">
        <div class="bar-track">
          {#each anchors as anchor (anchor.id)}
            <button
              class="bar-item"
              class:active={anchor.id === activeId}
              onclick={() => { jumpTo(anchor.id); activeId = anchor.id; }}
              aria-label={anchor.preview}
              title={anchor.preview}
            ></button>
          {/each}
        </div>

        <div class="prompt-popup" role="menu" onwheel={(event) => event.stopPropagation()}>
          {#each anchors as anchor (anchor.id)}
            <button
              class="popup-item"
              class:active={anchor.id === activeId}
              onclick={() => { jumpTo(anchor.id); activeId = anchor.id; }}
              role="menuitem"
            >
              <span class="popup-index">{anchor.index + 1}</span>
              <span class="popup-preview">{anchor.preview}</span>
            </button>
          {/each}
        </div>
      </div>
    </nav>
  {/if}

  <article class="messages">
    <header class="thread-head">
      {#if editableTitle && editingTitle}
        <div class="title-edit">
          <input
            bind:value={titleDraft}
            disabled={titleBusy}
            aria-label={$t('detail.title')}
            onkeydown={(e) => {
              if (e.key === 'Enter') saveTitle();
              if (e.key === 'Escape') cancelTitleEdit();
            }}
          />
          <button class="title-action" onclick={saveTitle} disabled={titleBusy || !titleDraft.trim()}>
            {$t('common.save')}
          </button>
          <button class="title-action ghost" onclick={cancelTitleEdit} disabled={titleBusy}>
            {$t('common.cancel')}
          </button>
        </div>
      {:else}
        <div class="title-row">
          <h1>{displayTitle}</h1>
          {#if editableTitle}
            <button class="edit-title" onclick={startTitleEdit} aria-label={$t('detail.editTitle')}>✎</button>
          {/if}
          {#if titleAction}
            <div class="title-action-slot">
              {@render titleAction()}
            </div>
          {/if}
        </div>
      {/if}
      <div class="thread-meta">
        <div class="byline">
          {#if ownerProfileUrl}
            <a class="owner-link" href={ownerProfileUrl} aria-label={`View @${thread.owner?.username} profile`}>
              {#if thread.owner?.avatar_url}
                <img class="avatar" src={thread.owner.avatar_url} alt="" loading="lazy" />
              {/if}
              {#if ownerName}<span class="owner">{ownerName}</span>{/if}
              {#if ownerName && thread.owner?.username}<span class="dot">·</span>{/if}
              <span class="handle">@{thread.owner?.username}</span>
            </a>
          {:else}
            {#if thread.owner?.avatar_url}
              <img class="avatar" src={thread.owner.avatar_url} alt="" loading="lazy" />
            {/if}
            {#if ownerName}<span class="owner">{ownerName}</span>{/if}
          {/if}
          <span class="time">{timeAgo(thread.updated_at)}</span>
        </div>
      </div>
    </header>

    {#each visibleBlocks as block (block.id)}
      {#if block.kind === 'workGroup'}
        <WorkGroup units={block.units} bodyFor={(work) => work.kind === 'tool' ? toolBody(work.input, work.output) : work.text} />
      {:else}
        {@const turn = block.turn}
        <div id={turn.id} class="msg msg-{turn.role}">
          {#if turn.role === 'user' && turn.units.every((u) => u.kind === 'text' || u.kind === 'image')}
            <div class="bubble">
              {#if imageUnits(turn.units).length}
                <div class="attach-grid">
                  {#each imageUnits(turn.units) as unit, ui (`image-${ui}`)}
                    <button class="attach-button" type="button" onclick={() => (previewImage = unit.url)} aria-label="Open attachment preview">
                      <img class="attach" src={unit.url} alt="attachment" loading="lazy" />
                    </button>
                  {/each}
                </div>
              {/if}
              {#each textUnits(turn.units) as unit, ui (`text-${ui}`)}
                <MarkdownView source={unit.text} />
              {/each}
            </div>
          {:else}
            <div class="assistant">
              {#each turn.units as unit, ui (ui)}
                {#if unit.kind === 'text'}
                  <MarkdownView source={unit.text} codeComments />
                {:else if unit.kind === 'image'}
                  <button class="attach-button" type="button" onclick={() => (previewImage = unit.url)} aria-label="Open attachment preview">
                    <img class="attach" src={unit.url} alt="attachment" loading="lazy" />
                  </button>
                {/if}
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    {/each}
  </article>

  {#if showScrollBottom}
    <button
      class="scroll-bottom"
      type="button"
      aria-label="Scroll to latest message"
      title="Scroll to latest message"
      onclick={scrollToBottom}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M12 5v14m0 0 7-7m-7 7-7-7" />
      </svg>
    </button>
  {/if}
</div>

<svelte:window onkeydown={onKeydown} onscroll={updateScrollBottomButton} onresize={updateScrollBottomButton} />

{#if previewImage}
  <button class="image-preview-backdrop" type="button" aria-label="Close attachment preview" onclick={() => (previewImage = null)}>
    <img class="image-preview" src={previewImage} alt="attachment preview" />
  </button>
{/if}

<style>
  .thread-view {
    display: flex;
    justify-content: center;
    gap: var(--space-4);
    width: 100%;
    font-size: 0.86rem;
  }

  .prompt-nav {
    position: fixed;
    left: 16px;
    top: 50%;
    transform: translateY(-50%);
    z-index: 20;
  }

  /* flex row: bars + popup sit side-by-side with no gap to cross */
  .nav-inner {
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 0;
  }

  /* vertical strip of bars */
  .bar-track {
    display: flex;
    flex-direction: column;
    gap: 4px;
    align-items: flex-start;
    width: 40px;
    flex: 0 0 40px;
    box-sizing: border-box;
    padding: var(--space-2) var(--space-2) var(--space-2) 0;
    /* extend hit area so mouse can bridge into the popup */
    cursor: default;
  }

  .bar-item {
    display: block;
    height: 3px;
    width: 20px;
    background: var(--color-border);
    border: none;
    border-radius: 2px;
    padding: 0;
    cursor: pointer;
    transition: width 0.15s ease, background 0.15s ease;
  }

  .bar-item.active {
    width: 32px;
    background: var(--color-text-subtle);
  }

  .bar-item:hover {
    width: 32px;
    background: var(--color-text-muted);
  }

  /* popup hidden by default, shown when nav-inner is hovered */
  .prompt-popup {
    display: none;
    flex-direction: column;
    gap: 2px;
    width: min(280px, 40vw);
    max-height: min(64vh, 520px);
    overflow-y: auto;
    overscroll-behavior: contain;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    padding: var(--space-2);
    scrollbar-width: thin;
    scrollbar-color: var(--color-text-subtle) transparent;
  }

  .prompt-popup::-webkit-scrollbar {
    width: 8px;
  }

  .prompt-popup::-webkit-scrollbar-track {
    background: transparent;
  }

  .prompt-popup::-webkit-scrollbar-thumb {
    background: var(--color-text-subtle);
    border-radius: var(--radius-full);
  }

  .nav-inner:hover .prompt-popup {
    display: flex;
  }

  .popup-item {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    width: 100%;
    padding: 0.38rem var(--space-3);
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    text-align: left;
  }

  .popup-item:hover,
  .popup-item.active {
    background: var(--color-bg-hover);
  }

  .popup-index {
    flex-shrink: 0;
    font-size: var(--text-xs);
    color: var(--color-text-subtle);
    width: 1.4rem;
    text-align: right;
  }

  .popup-preview {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
    color: var(--color-text-muted);
  }

  .popup-item:hover .popup-preview,
  .popup-item.active .popup-preview {
    color: var(--color-text);
  }

  .messages {
    flex: 1;
    min-width: 0;
    max-width: 840px;
    margin: 0 auto;
  }

  .scroll-bottom {
    position: fixed;
    left: 50%;
    bottom: calc(var(--space-6) + env(safe-area-inset-bottom));
    z-index: 30;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    transform: translateX(-50%);
    border: 1px solid rgba(0, 0, 0, 0.12);
    border-radius: var(--radius-full);
    background: rgba(255, 255, 255, 0.92);
    color: var(--color-text);
    box-shadow: 0 8px 22px rgba(0, 0, 0, 0.14);
    cursor: pointer;
    backdrop-filter: blur(10px);
    transition: transform 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
  }

  .scroll-bottom:hover {
    transform: translateX(-50%) translateY(-1px);
    background: var(--color-bg-elevated);
    box-shadow: 0 10px 26px rgba(0, 0, 0, 0.18);
  }

  .scroll-bottom:focus-visible {
    outline: 2px solid var(--color-primary);
    outline-offset: 3px;
  }

  .scroll-bottom svg {
    width: 23px;
    height: 23px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .title-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    min-width: 0;
    position: relative;
  }

  .title-action-slot {
    display: none;
    flex: 0 0 auto;
    align-self: flex-start;
    margin-top: -0.35rem;
  }

  .thread-head h1 {
    font-size: clamp(1.25rem, 1.55vw, 1.55rem);
    flex: 1;
    min-width: 0;
    margin-bottom: var(--space-3);
    text-align: center;
    font-weight: 650;
    line-height: 1.16;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .edit-title {
    flex-shrink: 0;
    margin-top: 0.35rem;
    padding: var(--space-1);
    background: transparent;
    border: none;
    color: var(--color-text-subtle);
    cursor: pointer;
    font-size: var(--text-sm);
    opacity: 0;
    transition: opacity 0.15s ease, color 0.15s ease;
  }

  .title-row:hover .edit-title,
  .edit-title:focus-visible {
    opacity: 1;
  }

  .edit-title:hover {
    color: var(--color-text);
  }

  .title-edit {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
    max-width: 760px;
    margin-left: auto;
    margin-right: auto;
  }

  .title-edit input {
    min-width: 0;
    padding: 0.45rem var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-family: inherit;
    font-size: 0.95rem;
    font-weight: 500;
  }

  .title-action {
    padding: 0.45rem var(--space-3);
    border: 1px solid var(--color-primary);
    border-radius: var(--radius-md);
    background: var(--color-primary);
    color: var(--color-primary-contrast);
    cursor: pointer;
    font-size: 0.82rem;
  }

  .title-action.ghost {
    border-color: var(--color-border);
    background: transparent;
    color: var(--color-text-muted);
  }

  .title-action:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .thread-meta {
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 100%;
    max-width: 100%;
    margin: 0 auto var(--space-6);
    gap: 0.45rem;
  }

  .byline {
    display: flex;
    justify-content: center;
    flex-wrap: wrap;
    gap: 0.4rem;
    align-items: center;
    color: var(--color-text-muted);
    font-size: 0.82rem;
    line-height: 1.2;
    text-align: center;
  }

  .owner-link {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: inherit;
    text-decoration: none;
  }

  .owner-link:hover .owner,
  .owner-link:hover .handle {
    color: var(--color-text);
  }

  .avatar {
    width: 1.45rem;
    height: 1.45rem;
    border-radius: var(--radius-full);
    object-fit: cover;
    border: 1px solid var(--color-border);
  }

  .owner {
    color: var(--color-text);
    font-weight: 600;
  }

  .dot {
    color: var(--color-text-subtle);
  }

  .handle {
    font-family: inherit;
  }

  .mode {
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-size: var(--text-xs);
    padding: 1px 6px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-accent);
  }

  .msg {
    min-width: 0;
    max-width: 100%;
    margin-bottom: 2rem;
  }

  .msg-user {
    display: flex;
    justify-content: flex-end;
  }

  .bubble {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    min-width: 0;
    background: #e3e3e1;
    border-radius: var(--radius-lg);
    padding: 0.72rem 0.9rem;
    max-width: min(560px, 78%);
    color: #0b0c0b;
    font-size: 0.85rem;
    line-height: 1.46;
  }

  .bubble :global(.markdown > :first-child) {
    margin-top: 0;
  }

  .bubble :global(.markdown) {
    width: 100%;
  }

  .bubble :global(.markdown h1),
  .bubble :global(.markdown h2),
  .bubble :global(.markdown h3),
  .bubble :global(.markdown h4) {
    margin: 0.45rem 0 0.35rem;
    font-size: 1em;
    line-height: 1.35;
  }

  .bubble :global(.markdown > :last-child) {
    margin-bottom: 0;
  }

  .assistant {
    min-width: 0;
    max-width: 100%;
    margin: 0 auto;
    font-size: 0.85rem;
    line-height: 1.52;
  }

  .attach-grid {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--space-2);
    width: 100%;
    margin-bottom: var(--space-2);
  }

  .attach-button {
    display: block;
    width: 80px;
    height: 80px;
    padding: 0;
    background: transparent;
    border: none;
    cursor: zoom-in;
  }

  .attach {
    display: block;
    width: 80px;
    height: 80px;
    object-fit: cover;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
    background: var(--color-bg-inset);
  }

  .assistant .attach-button,
  .assistant .attach {
    width: auto;
    height: auto;
    max-width: min(320px, 100%);
    max-height: 320px;
    object-fit: contain;
  }

  .image-preview-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-6);
    background: rgba(0, 0, 0, 0.72);
    border: none;
    cursor: zoom-out;
  }

  .image-preview {
    max-width: min(960px, 92vw);
    max-height: 88vh;
    object-fit: contain;
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-lg);
  }

  @media (max-width: 1180px) {
    .prompt-nav {
      display: none;
    }
  }

  @media (max-width: 880px) {
    .thread-view {
      font-size: 0.95rem;
    }

    .title-row {
      display: grid;
      grid-template-columns: 2rem minmax(0, 1fr) 2rem;
      gap: var(--space-2);
    }

    .thread-head h1 {
      grid-column: 2;
      font-size: 1.32rem;
    }

    .bubble {
      max-width: min(620px, 84%);
      font-size: 0.95rem;
      line-height: 1.58;
    }

    .assistant {
      font-size: 0.95rem;
      line-height: 1.56;
    }

    .byline {
      font-size: 0.85rem;
    }

    .edit-title {
      display: none;
    }

    .title-action-slot {
      display: block;
      grid-column: 3;
    }
  }
</style>
