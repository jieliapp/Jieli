<script lang="ts">
  import { tick } from 'svelte';
  import { t } from '$lib/stores/locale';
  import { cliCommand, modeLabel } from '$lib/providers/registry';
  import { copyToClipboard } from '$lib/utils';
  import { threadsApi } from '$lib/api/threads';
  import Button from '$lib/components/ui/Button.svelte';
  import Dialog from '$lib/components/ui/Dialog.svelte';
  import type { ThreadDetail, Visibility } from '$lib/api/types';

  type VisibilityOption = {
    value: Visibility;
    labelKey: 'threads.private' | 'threads.unlisted';
    descriptionKey: 'detail.privateDesc' | 'detail.unlistedDesc';
  };

  const visibilityOptions: VisibilityOption[] = [
    { value: 'private', labelKey: 'threads.private', descriptionKey: 'detail.privateDesc' },
    { value: 'unlisted', labelKey: 'threads.unlisted', descriptionKey: 'detail.unlistedDesc' }
  ];

  let {
    thread = $bindable(),
    readonly = false,
    onchange = undefined,
    mobile = false
  }: {
    thread: ThreadDetail;
    readonly?: boolean;
    onchange?: (t: ThreadDetail) => void;
    mobile?: boolean;
  } = $props();

  const cli = $derived(cliCommand(thread));
  const mode = $derived(modeLabel(thread));
  const linesChanged = $derived(thread.lines_changed ?? 0);
  const hasChanges = $derived(Boolean(thread.lines_added || thread.lines_removed || linesChanged || thread.files_changed));
  const fold = $derived(formatFileURI(thread.working_directory || thread.workspace_root || ''));
  const context = $derived(contextLabel());
  const labels = $derived(Array.isArray(thread.labels) ? thread.labels : []);

  let newLabel = $state('');
  let addingLabel = $state(false);
  let labelInput = $state<HTMLInputElement>();
  let renameDraft = $state('');
  let renameInput = $state<HTMLInputElement>();
  let copied = $state('');
  let confirmDelete = $state(false);
  let renameOpen = $state(false);
  let busy = $state(false);
  let shareOpen = $state(false);
  let menuOpen = $state(false);
  let mobileOpen = $state(false);
  let mobileSubmenu = $state<'copy' | 'export' | 'visibility' | null>(null);
  let shareRoot: HTMLDivElement;
  let menuRoot: HTMLDivElement;
  let mobileRoot = $state<HTMLDivElement>();

  async function copy(text: string | null | undefined, key: string) {
    if (!text) return;
    if (await copyToClipboard(text)) {
      copied = key;
      setTimeout(() => (copied = ''), 1500);
    }
  }

  function contextLabel(): string | null {
    if (typeof thread.context_percent === 'number' && typeof thread.context_window_tokens === 'number') {
      return `${thread.context_percent}% / ${formatNumber(thread.context_window_tokens)}`;
    }
    if (typeof thread.context_percent === 'number') return `${thread.context_percent}%`;
    if (typeof thread.context_window_tokens === 'number') return formatNumber(thread.context_window_tokens);
    return null;
  }

  function formatNumber(value: number): string {
    return new Intl.NumberFormat().format(value);
  }

  function formatCompact(value: number): string {
    const abs = Math.abs(value);
    if (abs < 1000) return formatNumber(value);
    const rounded = value / 1000;
    return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)}k`;
  }

  function formatFileURI(value: string): string {
    if (!value) return '';
    const withoutScheme = value.startsWith('file://') ? value.slice('file://'.length) : value;
    try {
      return decodeURIComponent(withoutScheme);
    } catch {
      return withoutScheme;
    }
  }

  function basenamePath(value: string): string {
    if (!value) return '';
    const trimmed = value.replace(/[\\/]+$/, '');
    const parts = trimmed.split(/[\\/]+/).filter(Boolean);
    return parts.at(-1) ?? trimmed;
  }

  function safeRepoUrl(repoUrl: string | null | undefined): string | null {
    const value = (repoUrl ?? '').trim();
    if (!value) return null;
    try {
      const parsed = new URL(value);
      return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.href : null;
    } catch {
      return null;
    }
  }

  function exportUrl(extension: 'md' | 'json'): string {
    return `/threads/${encodeURIComponent(thread.provider_thread_id)}.${extension}`;
  }

  function openExport(extension: 'md' | 'json') {
    const opened = window.open(exportUrl(extension), '_blank', 'noopener,noreferrer');
    if (!opened) {
      location.href = exportUrl(extension);
    }
  }

  function currentThreadUrl(): string {
    return `${location.origin}${location.pathname}${location.search}`;
  }

  function copyThreadUrl(e?: MouseEvent) {
    e?.stopPropagation();
    copy(currentThreadUrl(), 'thread-url');
  }

  async function shareThread(e?: MouseEvent) {
    e?.stopPropagation();
    const url = currentThreadUrl();
    const title = thread.title || thread.provider_thread_id;
    mobileOpen = false;
    if (typeof navigator.share === 'function') {
      try {
        await navigator.share({ title, text: title, url });
        return;
      } catch (err) {
        if (err instanceof DOMException && err.name === 'AbortError') return;
      }
    }
    await copy(url, 'thread-url');
  }

  function toggleMobileMenu() {
    mobileOpen = !mobileOpen;
    if (!mobileOpen) mobileSubmenu = null;
  }

  function closeMobileMenu() {
    mobileOpen = false;
    mobileSubmenu = null;
  }

  function toggleMobileSubmenu(next: 'copy' | 'export' | 'visibility') {
    mobileSubmenu = mobileSubmenu === next ? null : next;
  }

  function exportMarkdown() {
    menuOpen = false;
    mobileOpen = false;
    openExport('md');
  }

  function exportJSON() {
    menuOpen = false;
    mobileOpen = false;
    openExport('json');
  }

  async function renameThread() {
    menuOpen = false;
    if (readonly) return;
    closeMobileMenu();
    const current = thread.title || thread.provider_thread_id;
    renameDraft = current;
    renameOpen = true;
    await tick();
    renameInput?.focus();
    renameInput?.select();
  }

  function cancelRename() {
    renameOpen = false;
    renameDraft = '';
  }

  async function saveRename() {
    if (readonly) return;
    const current = thread.title || thread.provider_thread_id;
    const next = renameDraft.trim();
    if (!next || next === current) return;
    busy = true;
    try {
      const updated = await threadsApi.updateMeta(thread.id, { title: next });
      thread = { ...thread, title: updated.title };
      renameOpen = false;
      renameDraft = '';
      onchange?.(thread);
    } finally {
      busy = false;
    }
  }

  async function setVisibility(next: Visibility) {
    shareOpen = false;
    if (readonly || next === thread.visibility) return;
    busy = true;
    try {
      const updated = await threadsApi.updateMeta(thread.id, { visibility: next });
      thread = { ...thread, visibility: updated.visibility, share_slug: updated.share_slug };
      onchange?.(thread);
    } finally {
      busy = false;
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      shareOpen = false;
      menuOpen = false;
      mobileOpen = false;
      mobileSubmenu = null;
    }
  }

  function onWindowClick(e: MouseEvent) {
    if (mobileOpen && mobileRoot && !mobileRoot.contains(e.target as Node)) mobileOpen = false;
    if (shareOpen && shareRoot && !shareRoot.contains(e.target as Node)) shareOpen = false;
    if (menuOpen && menuRoot && !menuRoot.contains(e.target as Node)) menuOpen = false;
  }

  async function addLabel() {
    if (readonly) return;
    const label = newLabel.trim();
    if (!label || labels.includes(label)) {
      if (!label) addingLabel = false;
      return;
    }
    busy = true;
    try {
      const nextLabels = [...labels, label];
      const updated = await threadsApi.updateMeta(thread.id, { labels: nextLabels });
      thread = { ...thread, labels: updated.labels ?? nextLabels };
      newLabel = '';
      addingLabel = false;
      onchange?.(thread);
    } finally {
      busy = false;
    }
  }

  async function startAddLabel() {
    if (readonly) return;
    addingLabel = true;
    await tick();
    labelInput?.focus();
  }

  function cancelAddLabel() {
    newLabel = '';
    addingLabel = false;
  }

  async function removeLabel(label: string) {
    if (readonly) return;
    busy = true;
    try {
      const nextLabels = labels.filter((l) => l !== label);
      const updated = await threadsApi.updateMeta(thread.id, { labels: nextLabels });
      thread = { ...thread, labels: updated.labels ?? nextLabels };
      onchange?.(thread);
    } finally {
      busy = false;
    }
  }

  async function toggleArchive() {
    menuOpen = false;
    closeMobileMenu();
    if (readonly) return;
    busy = true;
    try {
      const updated = await threadsApi.archive(thread.id, !thread.archived);
      thread = { ...thread, archived: updated.archived };
      onchange?.(thread);
    } finally {
      busy = false;
    }
  }

  async function doDelete() {
    menuOpen = false;
    closeMobileMenu();
    busy = true;
    try {
      await threadsApi.remove(thread.id);
      location.href = '/threads';
    } finally {
      busy = false;
    }
  }
</script>

{#snippet InfoIcon(kind: 'repo' | 'branch' | 'folder' | 'mode' | 'effort' | 'context' | 'messages' | 'files')}
  <span class="info-icon" aria-hidden="true">
    {#if kind === 'repo'}
      <svg viewBox="0 0 16 16">
        <path d="M4 2.75h6.25A1.75 1.75 0 0 1 12 4.5v8.75H5.5A1.5 1.5 0 0 1 4 11.75v-9Z" />
        <path d="M5.5 13.25A1.5 1.5 0 0 1 4 11.75V4.5A1.75 1.75 0 0 1 5.75 2.75" />
        <path d="M6.25 5.5h3.5M6.25 7.5h3" />
      </svg>
    {:else if kind === 'branch'}
      <svg viewBox="0 0 16 16">
        <circle cx="5" cy="3.5" r="1.35" />
        <circle cx="5" cy="12.5" r="1.35" />
        <circle cx="11" cy="5.5" r="1.35" />
        <path d="M5 4.85v6.3M6.35 8H8a3 3 0 0 0 3-3" />
      </svg>
    {:else if kind === 'folder'}
      <svg viewBox="0 0 16 16">
        <path d="M2.5 5.25A1.25 1.25 0 0 1 3.75 4h3l1.25 1.5h4.25a1.25 1.25 0 0 1 1.25 1.25v5A1.25 1.25 0 0 1 12.25 13h-8.5a1.25 1.25 0 0 1-1.25-1.25v-6.5Z" />
      </svg>
    {:else if kind === 'mode'}
      <svg viewBox="0 0 16 16">
        <path d="M8 2.5 13 13H3L8 2.5Z" />
      </svg>
    {:else if kind === 'effort'}
      <svg viewBox="0 0 16 16">
        <path d="M8 2.5 12.5 8 8 13.5 3.5 8z" />
      </svg>
    {:else if kind === 'context'}
      <svg viewBox="0 0 16 16">
        <path d="M8 2.5a5.5 5.5 0 1 0 5.5 5.5" />
        <path d="M8 8 12 4" />
        <path d="M10.75 2.75h2.5v2.5" />
      </svg>
    {:else if kind === 'messages'}
      <svg viewBox="0 0 16 16">
        <path d="M3.25 4.25A2.25 2.25 0 0 1 5.5 2h5A2.25 2.25 0 0 1 12.75 4.25v3.5A2.25 2.25 0 0 1 10.5 10h-3l-3.25 2.5V10A2.25 2.25 0 0 1 2 7.75v-3.5Z" />
      </svg>
    {:else}
      <svg viewBox="0 0 16 16">
        <path d="M4 2.75h5.25L12 5.5v7.75H4z" />
        <path d="M9.25 2.75V5.5H12" />
      </svg>
    {/if}
  </span>
{/snippet}

{#snippet MenuIcon(kind: 'share' | 'copy' | 'id' | 'markdown' | 'json' | 'pen' | 'tag' | 'archive' | 'trash' | 'lock' | 'link' | 'terminal' | 'cost')}
  <span class="mobile-menu-icon" aria-hidden="true">
    {#if kind === 'share'}
      <svg viewBox="0 0 24 24">
        <path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5" />
        <path d="M5 14v4.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V14" />
      </svg>
    {:else if kind === 'copy'}
      <svg viewBox="0 0 24 24">
        <rect width="13" height="13" x="8" y="8" rx="2" />
        <path d="M16 8V5.5A1.5 1.5 0 0 0 14.5 4h-9A1.5 1.5 0 0 0 4 5.5v9A1.5 1.5 0 0 0 5.5 16H8" />
      </svg>
    {:else if kind === 'id'}
      <svg viewBox="0 0 24 24">
        <path d="M8 5H6a2 2 0 0 0-2 2v2a2 2 0 0 1-2 2 2 2 0 0 1 2 2v4a2 2 0 0 0 2 2h2" />
        <path d="M16 5h2a2 2 0 0 1 2 2v2a2 2 0 0 0 2 2 2 2 0 0 0-2 2v4a2 2 0 0 1-2 2h-2" />
      </svg>
    {:else if kind === 'markdown' || kind === 'json'}
      <svg viewBox="0 0 24 24">
        <path d="M6 3.5h8l4 4V20.5H6z" />
        <path d="M14 3.5V8h4" />
        {#if kind === 'json'}
          <path d="M10 12.5 8.5 14 10 15.5M14 12.5l1.5 1.5L14 15.5" />
        {:else}
          <path d="M8.5 15.5v-3l1.7 2 1.7-2v3M14 12.5v3h1.5" />
        {/if}
      </svg>
    {:else if kind === 'pen'}
      <svg viewBox="0 0 24 24">
        <path d="m4 20 4.5-1 10-10a2.1 2.1 0 0 0-3-3l-10 10L4 20Z" />
        <path d="m14.5 6.5 3 3" />
      </svg>
    {:else if kind === 'tag'}
      <svg viewBox="0 0 24 24">
        <path d="M20 13.5 13.5 20a2 2 0 0 1-2.8 0L4 13.3A2 2 0 0 1 3.4 12V5.5a2 2 0 0 1 2-2H12a2 2 0 0 1 1.4.6L20 10.7a2 2 0 0 1 0 2.8Z" />
        <circle cx="8" cy="8" r="1.2" />
      </svg>
    {:else if kind === 'archive'}
      <svg viewBox="0 0 24 24">
        <path d="M4 7h16M6 7v12h12V7M9 11h6" />
        <path d="M5 4h14v3H5z" />
      </svg>
    {:else if kind === 'trash'}
      <svg viewBox="0 0 24 24">
        <path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3" />
      </svg>
    {:else if kind === 'lock'}
      <svg viewBox="0 0 24 24">
        <rect width="15" height="10" x="4.5" y="10" rx="2" />
        <path d="M8 10V7a4 4 0 0 1 8 0v3" />
      </svg>
    {:else if kind === 'link'}
      <svg viewBox="0 0 24 24">
        <path d="M10.5 13.5a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1" />
        <path d="M13.5 10.5a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1" />
      </svg>
    {:else if kind === 'terminal'}
      <svg viewBox="0 0 24 24">
        <path d="m5 8 4 4-4 4M11 17h8" />
      </svg>
    {:else}
      <svg viewBox="0 0 24 24">
        <path d="M12 3v18M17 7.5A4 4 0 0 0 12 5a4 4 0 0 0 0 8 4 4 0 0 1 0 8 4 4 0 0 1-5-2.5" />
      </svg>
    {/if}
  </span>
{/snippet}

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

{#snippet SidebarPanel()}
  <div class="top-panel">
    <div class="top-actions">
      <div class="row badges" bind:this={shareRoot}>
        {#if readonly}
          <span class="share-pill readonly-pill">
            <span>{thread.visibility === 'unlisted' ? $t('threads.unlisted') : $t('detail.share')}</span>
            <svg class="share-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M10 13.5a4 4 0 0 0 5.7 0l2.1-2.1a4 4 0 0 0-5.7-5.7l-1.2 1.2" />
              <path d="M14 10.5a4 4 0 0 0-5.7 0l-2.1 2.1a4 4 0 0 0 5.7 5.7l1.2-1.2" />
            </svg>
          </span>
        {:else}
          <button
            class="share-pill"
            type="button"
            onclick={() => (shareOpen = !shareOpen)}
            aria-haspopup="dialog"
            aria-expanded={shareOpen}
            disabled={busy}
          >
            <span>{thread.visibility === 'unlisted' ? $t('threads.unlisted') : $t('detail.share')}</span>
            <svg class="share-icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M10 13.5a4 4 0 0 0 5.7 0l2.1-2.1a4 4 0 0 0-5.7-5.7l-1.2 1.2" />
              <path d="M14 10.5a4 4 0 0 0-5.7 0l-2.1 2.1a4 4 0 0 0 5.7 5.7l1.2-1.2" />
            </svg>
          </button>
        {/if}
        <button
          class="copy-url-button"
          type="button"
          title={$t('detail.copyUrl')}
          aria-label={$t('detail.copyUrl')}
          onclick={copyThreadUrl}
          disabled={busy}
        >
          {#if copied === 'thread-url'}
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="m5 12 5 5L20 7" />
            </svg>
          {:else}
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <rect width="14" height="14" x="8" y="8" rx="2" />
              <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
            </svg>
          {/if}
        </button>
        {#if copied === 'thread-url'}
          <span class="copied-text">{$t('common.copied')}</span>
        {/if}
        {#if shareOpen && !readonly}
          <div class="share-popover" role="dialog" aria-label={$t('detail.visibility')}>
            <div class="share-options">
              {#each visibilityOptions as option (option.value)}
                <button
                  class="visibility-option"
                  class:active={thread.visibility === option.value}
                  onclick={() => setVisibility(option.value)}
                  disabled={busy}
                  type="button"
                >
                  <span class="check">{thread.visibility === option.value ? '✓' : ''}</span>
                  <span class="option-copy">
                    <span class="option-title">{$t(option.labelKey)}</span>
                    <span class="option-desc">{$t(option.descriptionKey)}</span>
                  </span>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <div class="menu-wrap" bind:this={menuRoot}>
        <button
          class="menu-button"
          type="button"
          aria-label={$t('detail.moreActions')}
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          onclick={() => (menuOpen = !menuOpen)}
          disabled={busy}
        >
          ⋯
        </button>
        {#if menuOpen}
          <div class="actions-menu" role="menu">
            <button class="menu-item" role="menuitem" onclick={exportMarkdown}>
              <span class="menu-icon">▣</span>
              <span>{$t('detail.exportMarkdown')}</span>
            </button>
            <button class="menu-item" role="menuitem" onclick={exportJSON}>
              <span class="menu-icon">{'{}'}</span>
              <span>{$t('detail.exportJson')}</span>
            </button>
            <button class="menu-item" role="menuitem" onclick={() => { menuOpen = false; copy(thread.provider_thread_id, 'id'); }}>
              <span class="menu-icon">#</span>
              <span>{copied === 'id' ? $t('common.copied') : $t('detail.copyThreadId')}</span>
            </button>
            {#if !readonly}
              <div class="menu-separator"></div>
              <button class="menu-item" role="menuitem" onclick={renameThread} disabled={busy}>
                <span class="menu-icon">✎</span>
                <span>{$t('detail.renameThread')}</span>
              </button>
              <button class="menu-item" role="menuitem" onclick={toggleArchive} disabled={busy}>
                <span class="menu-icon">▤</span>
                <span>{thread.archived ? $t('detail.unarchive') : $t('detail.archiveThread')}</span>
              </button>
              <button class="menu-item danger" role="menuitem" onclick={() => { menuOpen = false; confirmDelete = true; }} disabled={busy}>
                <span class="menu-icon">□</span>
                <span>{$t('detail.deleteThread')}</span>
              </button>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <div class="status">
      {#if thread.repo}
        {@const repoUrl = safeRepoUrl(thread.repo_url)}
        {#if repoUrl}
          <div class="info-row" title={thread.repo}>
            {@render InfoIcon('repo')}<a class="info-link" href={repoUrl} target="_blank" rel="noreferrer">{thread.repo}</a>
          </div>
        {:else}
          <div class="info-row" title={thread.repo}>
            {@render InfoIcon('repo')}<span>{thread.repo}</span>
          </div>
        {/if}
      {/if}
      {#if thread.branch}
        <div class="info-row" title={thread.branch}>
          {@render InfoIcon('branch')}<span>{thread.branch}</span>
        </div>
      {/if}
      {#if fold}
        <div class="info-row" title={fold}>
          {@render InfoIcon('folder')}<span>{basenamePath(fold)}</span>
        </div>
      {/if}
      {#if mode}<div class="info-row">{@render InfoIcon('mode')}<span>{mode}</span></div>{/if}
      {#if context}
        <div class="info-row">
          {@render InfoIcon('context')}<span>{context}</span>
        </div>
      {/if}
      {#if hasChanges}
        <div class="info-row change-row">
          {@render InfoIcon('files')}
          <div class="change-stats">
            <span class="added">+{formatCompact(thread.lines_added)}</span>
            <span class="removed">-{formatCompact(thread.lines_removed)}</span>
            <span class="changed">~{formatCompact(linesChanged)}</span>
          </div>
        </div>
      {/if}
    </div>

    <section class="block labels-block" aria-label={$t('detail.labels')}>
      <h3>{$t('detail.labels')}</h3>
      <div class="label-editor">
        <svg class="label-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M20.6 13.5 13.5 20.6a2 2 0 0 1-2.8 0L3.4 13.3A2 2 0 0 1 2.8 12V5a2 2 0 0 1 2-2h7a2 2 0 0 1 1.4.6l7.4 7.1a2 2 0 0 1 0 2.8Z" stroke="currentColor" stroke-linejoin="round" />
          <circle cx="7.5" cy="7.5" r="1.25" fill="currentColor" />
        </svg>

        <div class="label-list">
          {#each labels as label (label)}
            <span class="label-chip">
              <span>{label}</span>
              {#if !readonly}
                <button class="remove-label" type="button" onclick={() => removeLabel(label)} aria-label={`Remove ${label}`} disabled={busy}>×</button>
              {/if}
            </span>
          {/each}

          {#if addingLabel && !readonly}
            <input
              class="label-input"
              placeholder={$t('detail.addLabel')}
              bind:value={newLabel}
              bind:this={labelInput}
              onkeydown={(e) => {
                if (e.key === 'Enter') addLabel();
                if (e.key === 'Escape') cancelAddLabel();
              }}
              onblur={() => {
                if (!newLabel.trim()) addingLabel = false;
              }}
            />
          {/if}
        </div>

        {#if !readonly}
          <button class="add-label-button" type="button" onclick={addingLabel ? addLabel : startAddLabel} disabled={busy} aria-label={$t('detail.addLabel')}>
            +
          </button>
        {/if}
      </div>
    </section>

    {#if cli}
      <section class="block">
        <h3>{$t('detail.openInCli')}</h3>
        <div class="cli">
          <code title={cli}>{cli}</code>
          <Button size="sm" variant="ghost" onclick={() => copy(cli, 'cli')}>
            {copied === 'cli' ? $t('common.copied') : $t('common.copy')}
          </Button>
        </div>
      </section>
    {/if}

  </div>
{/snippet}

{#snippet MobileMenu()}
  <div class="mobile-menu">
    <div class="mobile-menu-section">
      <button class="mobile-menu-row" type="button" onclick={shareThread}>
        {@render MenuIcon('share')}
        <span class="mobile-menu-label">{$t('detail.share')}</span>
      </button>
      <button class="mobile-menu-row has-drawer" class:active={mobileSubmenu === 'copy'} type="button" onclick={() => toggleMobileSubmenu('copy')}>
        {@render MenuIcon('copy')}
        <span class="mobile-menu-label">{$t('detail.copyMenu')}</span>
        <span class="mobile-row-chev">›</span>
      </button>
      <button class="mobile-menu-row has-drawer" class:active={mobileSubmenu === 'export'} type="button" onclick={() => toggleMobileSubmenu('export')}>
        {@render MenuIcon('markdown')}
        <span class="mobile-menu-label">{$t('detail.export')}</span>
        <span class="mobile-row-chev">›</span>
      </button>
    </div>

    {#if mobileSubmenu === 'copy'}
      <div class="mobile-submenu copy-submenu" role="menu" aria-label="Copy">
        <button class="mobile-menu-row" type="button" onclick={(e) => copyThreadUrl(e)} role="menuitem">
          {@render MenuIcon('link')}
          <span class="mobile-menu-label">{copied === 'thread-url' ? $t('common.copied') : $t('detail.threadUrl')}</span>
        </button>
        <button class="mobile-menu-row" type="button" onclick={() => copy(thread.provider_thread_id, 'id')} role="menuitem">
          {@render MenuIcon('id')}
          <span class="mobile-menu-label">{copied === 'id' ? $t('common.copied') : $t('detail.threadId')}</span>
        </button>
      </div>
    {:else if mobileSubmenu === 'export'}
      <div class="mobile-submenu export-submenu" role="menu" aria-label="Export">
        <button class="mobile-menu-row" type="button" onclick={exportMarkdown} role="menuitem">
          {@render MenuIcon('markdown')}
          <span class="mobile-menu-label">Markdown</span>
        </button>
        <button class="mobile-menu-row" type="button" onclick={exportJSON} role="menuitem">
          {@render MenuIcon('json')}
          <span class="mobile-menu-label">JSON</span>
        </button>
      </div>
    {:else if mobileSubmenu === 'visibility'}
      <div class="mobile-submenu visibility-submenu" role="menu" aria-label={$t('detail.visibility')}>
        {#each visibilityOptions as option (option.value)}
          <button
            class="mobile-menu-row"
            class:active={thread.visibility === option.value}
            onclick={() => setVisibility(option.value)}
            disabled={busy}
            type="button"
            role="menuitem"
          >
            <span class="check">{thread.visibility === option.value ? '✓' : ''}</span>
            <span class="mobile-menu-label">{$t(option.labelKey)}</span>
          </button>
        {/each}
      </div>
    {/if}

    <div class="mobile-menu-section mobile-facts">
      {#if thread.cost_display}
        <div class="mobile-fact-row">
          {@render MenuIcon('cost')}
          <span>{thread.cost_display}</span>
        </div>
      {/if}
      {#if thread.repo}
        {@const repoUrl = safeRepoUrl(thread.repo_url)}
        <div class="mobile-fact-row" title={thread.repo}>
          {@render InfoIcon('repo')}
          {#if repoUrl}
            <a class="mobile-fact-link" href={repoUrl} target="_blank" rel="noreferrer">{thread.repo}</a>
          {:else}
            <span>{thread.repo}</span>
          {/if}
        </div>
      {/if}
      {#if thread.branch}
        <div class="mobile-fact-row" title={thread.branch}>
          {@render InfoIcon('branch')}<span>{thread.branch}</span>
        </div>
      {/if}
      {#if fold}
        <div class="mobile-fact-row" title={fold}>
          {@render InfoIcon('folder')}<span>{basenamePath(fold)}</span>
        </div>
      {/if}
      {#if mode}
        <div class="mobile-fact-row">
          {@render InfoIcon('mode')}<span>{mode}</span>
        </div>
      {/if}
      {#if context}
        <div class="mobile-fact-row">
          {@render InfoIcon('context')}<span>{context}</span>
        </div>
      {/if}
    </div>

    <div class="mobile-menu-section">
      <div class="mobile-section-head">
        {@render MenuIcon('tag')}
        <span>{$t('detail.labels')}</span>
        {#if !readonly}
          <button class="mobile-inline-action" type="button" onclick={addingLabel ? addLabel : startAddLabel} disabled={busy} aria-label={$t('detail.addLabel')}>+</button>
        {/if}
      </div>
      {#if labels.length || addingLabel}
        <div class="mobile-label-list">
          {#each labels as label (label)}
            <span class="label-chip">
              <span>{label}</span>
              {#if !readonly}
                <button class="remove-label" type="button" onclick={() => removeLabel(label)} aria-label={`Remove ${label}`} disabled={busy}>×</button>
              {/if}
            </span>
          {/each}
          {#if addingLabel && !readonly}
            <input
              class="label-input"
              placeholder={$t('detail.addLabel')}
              bind:value={newLabel}
              bind:this={labelInput}
              onkeydown={(e) => {
                if (e.key === 'Enter') addLabel();
                if (e.key === 'Escape') cancelAddLabel();
              }}
              onblur={() => {
                if (!newLabel.trim()) addingLabel = false;
              }}
            />
          {/if}
        </div>
      {/if}
    </div>

    <div class="mobile-menu-section">
      {#if !readonly}
        <button class="mobile-menu-row" type="button" onclick={renameThread} disabled={busy}>
          {@render MenuIcon('pen')}
          <span class="mobile-menu-label">{$t('detail.rename')}</span>
        </button>
        <button class="mobile-menu-row" type="button" onclick={toggleArchive} disabled={busy}>
          {@render MenuIcon('archive')}
          <span class="mobile-menu-label">{thread.archived ? $t('detail.unarchive') : $t('detail.archive')}</span>
        </button>
        <button class="mobile-menu-row danger" type="button" onclick={() => { mobileOpen = false; confirmDelete = true; }} disabled={busy}>
          {@render MenuIcon('trash')}
          <span class="mobile-menu-label">{$t('common.delete')}</span>
        </button>
      {/if}
      <button
        class="mobile-menu-row has-drawer"
        class:active={mobileSubmenu === 'visibility'}
        class:static-row={readonly}
        type="button"
        onclick={() => {
          if (!readonly) toggleMobileSubmenu('visibility');
        }}
        disabled={readonly || busy}
      >
        {@render MenuIcon(thread.visibility === 'unlisted' ? 'link' : 'lock')}
        <span class="mobile-menu-label">{thread.visibility === 'unlisted' ? $t('threads.unlisted') : $t('threads.private')}</span>
        <span class="mobile-row-chev">›</span>
      </button>
    </div>
  </div>
{/snippet}

{#if mobile}
  <div class="mobile-sidebar" bind:this={mobileRoot}>
    <button
      class="mobile-sidebar-trigger"
      class:active={mobileOpen}
      type="button"
      aria-label={$t('detail.moreActions')}
      aria-haspopup="dialog"
      aria-expanded={mobileOpen}
      onclick={toggleMobileMenu}
    >
      ⋯
    </button>
    {#if mobileOpen}
      <div class="mobile-sidebar-panel" role="dialog" aria-label={$t('detail.moreActions')}>
        {@render MobileMenu()}
      </div>
    {/if}
  </div>
{:else}
  <aside class="sidebar">
    {@render SidebarPanel()}
  </aside>
{/if}

<Dialog bind:open={renameOpen} title={$t('detail.renamePrompt')}>
  <form
    class="rename-form"
    onsubmit={(e) => {
      e.preventDefault();
      saveRename();
    }}
  >
    <input
      class="rename-input"
      bind:value={renameDraft}
      bind:this={renameInput}
      disabled={busy}
      aria-label={$t('detail.title')}
      onkeydown={(e) => {
        if (e.key === 'Escape') cancelRename();
      }}
    />
    <div class="dialog-actions">
      <Button variant="secondary" onclick={cancelRename} disabled={busy}>{$t('common.cancel')}</Button>
      <Button type="submit" disabled={busy || !renameDraft.trim()}>{$t('common.save')}</Button>
    </div>
  </form>
</Dialog>

<Dialog bind:open={confirmDelete} title={$t('detail.delete')}>
  <p>{$t('detail.deleteConfirm')}</p>
  <div class="dialog-actions">
    <Button variant="secondary" onclick={() => (confirmDelete = false)}>{$t('common.cancel')}</Button>
    <Button variant="danger" onclick={doDelete} disabled={busy}>{$t('common.delete')}</Button>
  </div>
</Dialog>

<style>
  .sidebar {
    width: 260px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    font-family: var(--font-mono);
    position: sticky;
    top: calc(var(--topbar-height) + var(--space-6));
    align-self: flex-start;
  }

  .mobile-sidebar {
    position: relative;
    display: none;
    flex: 0 0 auto;
    margin-bottom: var(--space-3);
  }

  .mobile-sidebar-trigger {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    padding: 0;
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--color-text-muted);
    font-size: 1.15rem;
    line-height: 1;
    cursor: pointer;
  }

  .mobile-sidebar-trigger:hover,
  .mobile-sidebar-trigger.active {
    border-color: var(--color-border);
    background: color-mix(in srgb, var(--color-bg-inset) 72%, var(--color-bg));
    color: var(--color-text);
  }

  .mobile-sidebar-panel {
    position: absolute;
    top: calc(100% + var(--space-2));
    right: 0.35rem;
    z-index: 45;
    width: min(12rem, calc(100vw - var(--space-8)));
  }

  .mobile-menu {
    position: relative;
    overflow: visible;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
  }

  .mobile-menu-section {
    padding: var(--space-1);
  }

  .mobile-menu-section + .mobile-menu-section {
    border-top: 1px solid var(--color-border);
  }

  .mobile-menu-row,
  .mobile-section-head,
  .mobile-fact-row {
    display: grid;
    grid-template-columns: 1.05rem minmax(0, 1fr);
    align-items: center;
    column-gap: 0.45rem;
    width: 100%;
    min-height: 1.95rem;
    padding: 0.28rem var(--space-2);
    color: var(--color-text);
    font-size: 0.76rem;
    line-height: 1.15;
    text-align: left;
  }

  .mobile-menu-row {
    border: none;
    border-radius: var(--radius-md);
    background: transparent;
    cursor: pointer;
  }

  .mobile-menu-row:hover {
    background: var(--color-bg-hover);
  }

  .mobile-menu-row.active {
    background: var(--color-bg-hover);
  }

  .mobile-menu-row:disabled {
    cursor: default;
  }

  .mobile-menu-row.static-row:disabled {
    opacity: 1;
  }

  .mobile-menu-row.danger {
    color: var(--color-danger);
  }

  .mobile-menu-label,
  .mobile-fact-row span:last-child,
  .mobile-fact-link {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-menu-row.has-drawer {
    grid-template-columns: 1.05rem minmax(0, 1fr) 0.5rem;
  }

  .mobile-row-chev {
    color: var(--color-text-muted);
    font-size: 1.15rem;
    line-height: 1;
    justify-self: end;
  }

  .mobile-submenu {
    position: absolute;
    right: calc(100% - 0.35rem);
    z-index: 50;
    width: 8.5rem;
    padding: var(--space-1);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
  }

  .copy-submenu {
    top: 2.75rem;
  }

  .export-submenu {
    top: 5.1rem;
  }

  .visibility-submenu {
    bottom: 0.35rem;
    width: 8.5rem;
  }

  .mobile-facts {
    padding-top: var(--space-2);
    padding-bottom: var(--space-2);
  }

  .mobile-fact-row {
    min-height: 1.7rem;
    padding-top: 0.14rem;
    padding-bottom: 0.14rem;
    color: var(--color-text);
  }

  .mobile-fact-row .info-icon {
    color: currentColor;
  }

  .mobile-fact-row .info-icon svg {
    width: 1rem;
    height: 1rem;
    stroke-width: 1.8;
  }

  .mobile-fact-link {
    color: inherit;
    text-decoration: none;
  }

  .mobile-fact-link:hover {
    color: var(--color-link);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .mobile-section-head {
    min-height: 1.8rem;
    color: var(--color-text);
    font-weight: 400;
  }

  .mobile-inline-action {
    justify-self: end;
    width: 1.3rem;
    height: 1.3rem;
    padding: 0;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
  }

  .mobile-inline-action:hover {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .mobile-section-head:has(.mobile-inline-action) {
    grid-template-columns: 1.05rem minmax(0, 1fr) auto;
  }

  .mobile-label-list {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: 0 var(--space-2) var(--space-2) 2.05rem;
  }

  .mobile-menu-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1rem;
    height: 1rem;
    color: currentColor;
  }

  .mobile-menu-icon svg {
    width: 1rem;
    height: 1rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .row {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
    position: relative;
  }

  .top-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-4);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: none;
  }

  .top-actions {
    position: relative;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .top-actions .badges {
    position: static;
    min-width: 0;
  }

  .top-actions :global(.badge) {
    min-height: 1.45rem;
    padding: 0 var(--space-2);
    font-size: 0.72rem;
    line-height: 1;
  }

  .share-pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    min-height: 1.45rem;
    padding: 0 var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-bg-inset) 72%, var(--color-bg));
    color: var(--color-text-muted);
    font-size: 0.72rem;
    font-weight: 500;
    line-height: 1;
    cursor: pointer;
  }

  .share-pill:hover,
  .share-pill[aria-expanded='true'] {
    border-color: var(--color-border-strong);
    color: var(--color-text);
  }

  .readonly-pill {
    cursor: default;
  }

  .readonly-pill:hover {
    border-color: var(--color-border);
    color: var(--color-text-muted);
  }

  .share-icon {
    width: 0.82rem;
    height: 0.82rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .copy-url-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.45rem;
    height: 1.45rem;
    padding: 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-bg-inset) 72%, var(--color-bg));
    color: var(--color-text-muted);
    cursor: pointer;
  }

  .copy-url-button:hover {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .copy-url-button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .copy-url-button svg {
    width: 0.9rem;
    height: 0.9rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .copied-text {
    align-self: center;
    color: var(--color-text-muted);
    font-size: 0.72rem;
    line-height: 1;
  }

  .menu-wrap {
    position: relative;
    flex: 0 0 auto;
  }

  .menu-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-bg-inset) 72%, var(--color-bg));
    color: var(--color-text);
    font-size: 1.1rem;
    line-height: 1;
    cursor: pointer;
  }

  .menu-button:hover,
  .menu-button[aria-expanded='true'] {
    border-color: var(--color-border-strong);
    background: var(--color-bg-hover);
  }

  .menu-button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .actions-menu {
    position: absolute;
    top: calc(100% + var(--space-2));
    right: 0;
    z-index: 35;
    width: 210px;
    padding: var(--space-1);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
  }

  .menu-item {
    display: grid;
    grid-template-columns: 1.35rem minmax(0, 1fr);
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: 0.38rem var(--space-2);
    border: none;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--color-text);
    font-size: 0.82rem;
    text-align: left;
    cursor: pointer;
  }

  .menu-item:hover {
    background: var(--color-bg-hover);
  }

  .menu-item:disabled {
    opacity: 0.55;
    cursor: default;
  }

  .menu-item.danger {
    color: var(--color-danger);
  }

  .menu-icon {
    display: inline-flex;
    justify-content: center;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    font-size: 0.82rem;
  }

  .menu-item.danger .menu-icon {
    color: var(--color-danger);
  }

  .menu-separator {
    height: 1px;
    margin: var(--space-2) calc(var(--space-2) * -1);
    background: var(--color-border);
  }

  .status {
    display: flex;
    flex-direction: column;
    gap: 0.42rem;
    padding-top: var(--space-1);
  }

  .info-row {
    display: grid;
    grid-template-columns: 1.35rem minmax(0, 1fr);
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    color: var(--color-text-muted);
    font-size: 0.82rem;
    line-height: 1.25;
  }

  .info-row span:last-child {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .info-link {
    min-width: 0;
    overflow: hidden;
    color: inherit;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .info-link:hover {
    color: var(--color-link);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .info-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1rem;
    height: 1rem;
    color: var(--color-text-subtle);
  }

  .info-icon svg {
    width: 0.9rem;
    height: 0.9rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .change-row {
    align-items: center;
  }

  .change-stats {
    display: flex;
    min-width: 0;
    gap: var(--space-1);
    margin-left: 0;
    font-family: var(--font-mono);
    font-size: 0.86rem;
  }

  .added {
    color: var(--color-added);
  }

  .removed {
    color: var(--color-removed);
  }

  .changed {
    color: var(--color-changed);
  }

  .share-popover {
    position: absolute;
    top: calc(100% + var(--space-2));
    right: calc(var(--space-4) * -1);
    z-index: 30;
    width: min(320px, calc(100vw - var(--space-8)));
    padding: var(--space-2) 0;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
  }

  .share-options {
    display: flex;
    flex-direction: column;
  }

  .visibility-option {
    display: grid;
    grid-template-columns: 1.4rem minmax(0, 1fr);
    gap: var(--space-2);
    width: 100%;
    padding: 0.48rem var(--space-3);
    background: transparent;
    border: none;
    border-radius: 0;
    color: var(--color-text-muted);
    text-align: left;
    cursor: pointer;
  }

  .visibility-option:hover,
  .visibility-option.active {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .visibility-option:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .check {
    color: var(--color-text);
    font-size: 0.86rem;
    font-weight: 650;
    line-height: 1.1;
  }

  .option-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 2px;
  }

  .option-title {
    color: var(--color-text);
    font-size: 0.86rem;
    font-weight: 500;
    line-height: 1.2;
  }

  .option-desc {
    color: var(--color-text-muted);
    font-size: 0.78rem;
    line-height: 1.3;
  }

  .block h3 {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    margin-bottom: 0.35rem;
    text-transform: none;
    letter-spacing: 0;
    font-weight: 600;
  }

  .labels-block {
    margin-top: 0;
  }

  .label-editor {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .label-icon {
    width: 0.95rem;
    height: 0.95rem;
    flex: 0 0 auto;
    color: var(--color-text-muted);
    stroke-width: 1.8;
  }

  .label-list {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    flex: 1;
    min-width: 0;
  }

  .label-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    max-width: 100%;
    background: var(--color-bg-inset);
    border-radius: var(--radius-sm);
    padding: 0.2rem var(--space-2) 0.2rem var(--space-3);
    color: var(--color-text-muted);
    font-size: 0.76rem;
    line-height: 1.25;
  }

  .label-chip span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .remove-label,
  .add-label-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: 0;
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
    line-height: 1;
  }

  .remove-label {
    width: 1rem;
    height: 1rem;
    padding: 0;
    font-size: 1rem;
  }

  .remove-label:hover {
    color: var(--color-text);
  }

  .add-label-button {
    width: 1.05rem;
    height: 1.05rem;
    flex: 0 0 auto;
    padding: 0;
    border-radius: var(--radius-sm);
    font-size: 0.95rem;
    font-weight: 400;
  }

  .add-label-button:hover {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .add-label-button:disabled,
  .remove-label:disabled {
    opacity: 0.45;
    cursor: default;
  }

  .label-input {
    width: 7.5rem;
    min-width: 4.5rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-bg-elevated);
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 1.3;
    padding: 0.2rem var(--space-2);
  }

  .label-input:focus {
    border-color: var(--color-border-strong);
    outline: none;
  }

  .rename-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .rename-input {
    width: 100%;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg);
    color: var(--color-text);
    font: inherit;
    font-size: var(--text-sm);
    line-height: 1.4;
    padding: 0.55rem var(--space-3);
  }

  .rename-input:focus {
    border-color: var(--color-border-strong);
    outline: none;
  }

  .cli {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.3rem var(--space-2);
  }

  .cli code {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 0.7rem;
    white-space: nowrap;
  }

  .cli :global(.btn-sm) {
    padding: 0.1rem var(--space-2);
    font-size: 0.68rem;
    border-radius: var(--radius-sm);
  }

  .cli :global(.btn-ghost) {
    background: color-mix(in srgb, var(--color-bg) 78%, var(--color-bg-inset));
    border-color: var(--color-border);
    color: var(--color-text);
  }

  .cli :global(.btn-ghost:hover) {
    background: var(--color-bg-hover);
    border-color: var(--color-border-strong);
    color: var(--color-text);
  }

  @media (max-width: 880px) {
    .mobile-sidebar {
      display: block;
    }
  }

  .dialog-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    margin-top: var(--space-4);
  }
</style>
