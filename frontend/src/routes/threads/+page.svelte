<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { t } from '$lib/stores/locale';
  import { threadsApi } from '$lib/api/threads';
  import { threadFilters, defaultFilters } from '$lib/stores/threads';
  import type { ProviderInfo, ThreadListFilters, ThreadListResult, ThreadSummary } from '$lib/api/types';
  import ThreadCard from '$lib/components/thread/ThreadCard.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Spinner from '$lib/components/ui/Spinner.svelte';
  import Dropdown from '$lib/components/ui/Dropdown.svelte';

  const AUTO_LOAD_PAGES = 2;
  type CategoryKind = 'all' | 'provider' | 'label';

  let threads = $state<ThreadSummary[]>([]);
  let total = $state(0);
  let pageSize = $state(defaultFilters.page_size ?? 20);
  let currentPage = $state(1);
  let loading = $state(true);
  let loadingMore = $state(false);
  let error = $state(false);
  let loadMoreError = $state(false);
  let autoLoadsRemaining = $state(AUTO_LOAD_PAGES);
  let manualLoadMode = $state(false);
  let sentinel = $state<HTMLDivElement | null>(null);

  let searchTerm = $state('');
  let provider = $state('');
  let repo = $state('');
  let label = $state('');
  let visibility = $state('');
  let categoryOpen = $state(false);
  let categoryRoot = $state<HTMLDivElement | null>(null);

  let providerOptions = $state<ProviderInfo[]>([]);
  let repoOptions = $state<string[]>([]);
  let labelOptions = $state<string[]>([]);

  const repoSelectOptions = $derived([
    { value: '', label: $t('threads.filterRepo') },
    ...repoOptions.map((r) => ({ value: r, label: r }))
  ]);
  const providerSelectOptions = $derived(providerOptions.map((p) => ({ value: p.id, label: p.label })));
  const categoryLabel = $derived(
    provider
      ? (providerSelectOptions.find((option) => option.value === provider)?.label ?? '')
      : label
        ? `#${label}`
        : $t('threads.filterCategory')
  );
  const visibilityOptions = $derived([
    { value: '', label: $t('threads.filterVisibility') },
    { value: 'private', label: $t('threads.private') },
    { value: 'unlisted', label: $t('threads.unlisted') },
    { value: 'archived', label: $t('threads.archived') }
  ]);

  let debounce: ReturnType<typeof setTimeout> | undefined;
  let composing = false;
  let controller: AbortController | undefined;
  let observer: IntersectionObserver | undefined;
  let requestID = 0;

  const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
  const hasMore = $derived(currentPage < totalPages);
  const showMoreButton = $derived(hasMore && (manualLoadMode || autoLoadsRemaining <= 0 || loadMoreError));

  async function loadFacets() {
    try {
      const facets = await threadsApi.facets();
      providerOptions = facets.providers ?? [];
      repoOptions = facets.repos ?? [];
      labelOptions = facets.labels ?? [];
      if (provider && !providerOptions.some((option) => option.id === provider)) provider = '';
    } catch {
      providerOptions = [];
      repoOptions = [];
      labelOptions = [];
    }
  }

  function buildFilters(page: number): ThreadListFilters {
    return {
      search: searchTerm,
      provider,
      repo,
      label,
      visibility: visibility === 'archived' ? '' : visibility,
      archived: visibility === 'archived',
      sort: 'updated',
      page,
      page_size: pageSize
    };
  }

  async function loadFirstPage() {
    loading = true;
    error = false;
    loadMoreError = false;
    manualLoadMode = false;
    autoLoadsRemaining = AUTO_LOAD_PAGES;
    currentPage = 1;
    controller?.abort();
    controller = new AbortController();
    const id = ++requestID;
    const filters = buildFilters(1);
    threadFilters.set(filters);
    try {
      const res: ThreadListResult = await threadsApi.list(filters, controller.signal);
      if (id !== requestID) return;
      threads = res.threads ?? [];
      total = res.total ?? threads.length;
      currentPage = res.page ?? 1;
      pageSize = res.page_size ?? pageSize;
    } catch (err) {
      // Ignore superseded requests: a newer search aborts this one, and a
      // stale rejection arriving after a fresh success must not flip error on.
      if (id !== requestID) return;
      if ((err as Error).name !== 'AbortError') error = true;
    } finally {
      if (id === requestID) loading = false;
    }
  }

  async function loadNextPage(manual = false) {
    if (loading || loadingMore || !hasMore) return;
    if (!manual && (manualLoadMode || autoLoadsRemaining <= 0)) return;
    if (manual) manualLoadMode = true;
    loadingMore = true;
    loadMoreError = false;
    const nextPage = currentPage + 1;
    const id = requestID;
    const filters = buildFilters(nextPage);
    try {
      const res: ThreadListResult = await threadsApi.list(filters);
      if (id !== requestID) return;
      const seen = new Set(threads.map((thread) => thread.id));
      threads = [...threads, ...(res.threads ?? []).filter((thread) => !seen.has(thread.id))];
      total = res.total ?? total;
      currentPage = res.page ?? nextPage;
      pageSize = res.page_size ?? pageSize;
      if (!manual) autoLoadsRemaining = Math.max(0, autoLoadsRemaining - 1);
      threadFilters.set(buildFilters(currentPage));
    } catch {
      loadMoreError = true;
      manualLoadMode = true;
    } finally {
      if (id === requestID) loadingMore = false;
    }
  }

  function onSearchInput() {
    // Skip pinyin/IME composition ticks: firing mid-composition both wastes a
    // request on the intermediate romanization and is the request most likely
    // to be aborted by the next keystroke. compositionend triggers the search.
    if (composing) return;
    clearTimeout(debounce);
    debounce = setTimeout(() => {
      loadFirstPage();
    }, 300);
  }

  function onCompositionStart() {
    composing = true;
  }

  function onCompositionEnd() {
    composing = false;
    onSearchInput();
  }

  function applyFilters() {
    loadFirstPage();
  }

  function toggleCategory() {
    categoryOpen = !categoryOpen;
  }

  function selectCategory(kind: CategoryKind, value = '') {
    if (kind === 'provider') {
      provider = value;
      label = '';
    } else if (kind === 'label') {
      provider = '';
      label = value;
    } else {
      provider = '';
      label = '';
    }
    categoryOpen = false;
    applyFilters();
  }

  function isCategoryActive(kind: CategoryKind, value = '') {
    if (kind === 'provider') return provider === value;
    if (kind === 'label') return label === value;
    return provider === '' && label === '';
  }

  function onWindowClick(e: MouseEvent) {
    if (categoryOpen && categoryRoot && !categoryRoot.contains(e.target as Node)) categoryOpen = false;
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') categoryOpen = false;
  }

  onMount(() => {
    const f = get(threadFilters);
    searchTerm = f.search ?? '';
    provider = f.provider ?? '';
    repo = f.repo ?? '';
    label = f.label ?? '';
    visibility = f.archived ? 'archived' : f.visibility ?? '';
    pageSize = f.page_size ?? pageSize;
    loadFacets();
    loadFirstPage();
    return () => {
      controller?.abort();
      observer?.disconnect();
    };
  });

  $effect(() => {
    if (!sentinel) return;
    observer?.disconnect();
    observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) loadNextPage(false);
      },
      { rootMargin: '360px 0px' }
    );
    observer.observe(sentinel);
    return () => observer?.disconnect();
  });
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<div class="app-container">
  <h1 class="page-title">{$t('threads.title')}</h1>

  <div class="toolbar">
    <div class="search">
      <svg class="search-icon" viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
        <path
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          d="M11 11l3 3M7 12a5 5 0 1 0 0-10 5 5 0 0 0 0 10Z"
        />
      </svg>
      <input
        class="search-input"
        placeholder={$t('threads.search')}
        bind:value={searchTerm}
        oninput={onSearchInput}
        oncompositionstart={onCompositionStart}
        oncompositionend={onCompositionEnd}
      />
    </div>
    <Dropdown
      bind:value={repo}
      options={repoSelectOptions}
      placeholder={$t('threads.filterRepo')}
      onchange={applyFilters}
    />
    <div class="category-dropdown" bind:this={categoryRoot}>
      <button
        type="button"
        class="category-trigger"
        aria-haspopup="listbox"
        aria-expanded={categoryOpen}
        onclick={toggleCategory}
      >
        <span class="trigger-label">{categoryLabel}</span>
        <svg class="chevron" class:up={categoryOpen} viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
          <path fill="none" stroke="currentColor" stroke-width="1.5" d="M4 6l4 4 4-4" />
        </svg>
      </button>

      {#if categoryOpen}
        <div class="category-menu" role="listbox" aria-label={$t('threads.filterCategory')}>
          <button
            type="button"
            class="category-option"
            class:active={isCategoryActive('all')}
            role="option"
            aria-selected={isCategoryActive('all')}
            onclick={() => selectCategory('all')}
          >
            <span class="radio" aria-hidden="true"></span>
            <span>{$t('threads.filterCategory')}</span>
          </button>

          {#if providerSelectOptions.length > 0}
            <div class="category-section">
              <p class="category-heading">{$t('threads.categoryProvider')}</p>
              {#each providerSelectOptions as option (option.value)}
                <button
                  type="button"
                  class="category-option"
                  class:active={isCategoryActive('provider', option.value)}
                  role="option"
                  aria-selected={isCategoryActive('provider', option.value)}
                  onclick={() => selectCategory('provider', option.value)}
                >
                  <span class="radio" aria-hidden="true"></span>
                  <span>{option.label}</span>
                </button>
              {/each}
            </div>
          {/if}

          {#if labelOptions.length > 0}
            <div class="category-section">
              <p class="category-heading">{$t('threads.categoryLabel')}</p>
              {#each labelOptions as option (option)}
                <button
                  type="button"
                  class="category-option"
                  class:active={isCategoryActive('label', option)}
                  role="option"
                  aria-selected={isCategoryActive('label', option)}
                  onclick={() => selectCategory('label', option)}
                >
                  <span class="radio" aria-hidden="true"></span>
                  <span>#{option}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>
    <Dropdown
      bind:value={visibility}
      options={visibilityOptions}
      placeholder={$t('threads.filterVisibility')}
      onchange={applyFilters}
    />
  </div>

  {#if loading}
    <div class="state"><Spinner /></div>
  {:else if error}
    <div class="state">
      <p class="muted">{$t('common.error')}</p>
      <Button variant="secondary" onclick={loadFirstPage}>{$t('common.retry')}</Button>
    </div>
  {:else if threads.length === 0}
    <div class="state"><p class="muted">{$t('threads.empty')}</p></div>
  {:else}
    <div class="list">
      {#each threads as thread (thread.id)}
        <ThreadCard {thread} search={searchTerm} />
      {/each}
    </div>

    <div class="load-more" bind:this={sentinel}>
      {#if loadingMore}
        <Spinner />
        <span class="page-info">{$t('threads.loadingMore')}</span>
      {:else if showMoreButton}
        {#if loadMoreError}
          <p class="muted">{$t('common.error')}</p>
        {/if}
        <Button variant="secondary" size="sm" onclick={() => loadNextPage(true)}>
          {$t('threads.showMore')}
        </Button>
      {:else if !hasMore}
        <span class="page-info">{$t('threads.end')}</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .page-title {
    max-width: 820px;
    margin: 0 auto var(--space-5);
    font-size: var(--text-2xl);
  }

  .toolbar {
    display: grid;
    grid-template-columns: minmax(220px, 1fr) minmax(0, 160px) minmax(0, 140px) minmax(0, 140px);
    gap: var(--space-3);
    max-width: 820px;
    margin: 0 auto var(--space-5);
  }

  .search {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .search:focus-within {
    border-color: var(--color-accent);
  }

  .search-icon {
    color: var(--color-text-subtle);
    flex-shrink: 0;
  }

  .search-input {
    flex: 1;
    min-width: 0;
    border: none;
    background: transparent;
    color: var(--color-text);
    font-size: var(--text-sm);
  }

  .search-input::placeholder {
    color: var(--color-text-subtle);
  }

  .search-input:focus {
    outline: none;
  }

  .toolbar :global(.dropdown),
  .toolbar :global(.trigger),
  .category-dropdown,
  .category-trigger {
    width: 100%;
    min-width: 0;
  }

  .category-dropdown {
    position: relative;
  }

  .category-trigger {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-size: var(--text-sm);
    cursor: pointer;
    transition: border-color 0.15s;
  }

  .category-trigger:hover {
    border-color: var(--color-border-strong);
  }

  .category-trigger[aria-expanded='true'] {
    border-color: var(--color-accent);
  }

  .trigger-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chevron {
    flex-shrink: 0;
    color: var(--color-text-subtle);
    transition: transform 0.15s;
  }

  .chevron.up {
    transform: rotate(180deg);
  }

  .category-menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 20;
    min-width: 220px;
    max-height: 320px;
    overflow-y: auto;
    padding: var(--space-1);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
  }

  .category-section {
    padding-top: var(--space-1);
    margin-top: var(--space-1);
    border-top: 1px solid var(--color-border);
  }

  .category-heading {
    margin: var(--space-2) var(--space-2) var(--space-1);
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
    font-weight: 600;
  }

  .category-option {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2);
    text-align: left;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text);
    font-size: var(--text-sm);
    white-space: nowrap;
    cursor: pointer;
  }

  .category-option:hover {
    background: var(--color-bg-hover);
  }

  .category-option.active {
    color: var(--color-accent);
  }

  .radio {
    width: 12px;
    height: 12px;
    border: 1px solid var(--color-border-strong);
    border-radius: 999px;
    flex-shrink: 0;
  }

  .category-option.active .radio {
    border: 3px solid var(--color-accent);
  }

  .list {
    display: flex;
    flex-direction: column;
    max-width: 820px;
    margin: 0 auto;
  }

  .list > :global(* + *) {
    border-top: 1px solid var(--color-border);
  }

  .load-more {
    min-height: 72px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    max-width: 820px;
    margin: var(--space-5) auto 0;
  }

  .page-info {
    color: var(--color-text-muted);
    font-size: var(--text-sm);
  }

  .state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-10) 0;
  }

  .muted {
    color: var(--color-text-muted);
  }

  @media (max-width: 900px) {
    .toolbar {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    }

    .search {
      grid-column: 1 / -1;
    }
  }
</style>
