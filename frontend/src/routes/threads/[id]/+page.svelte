<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { t } from '$lib/stores/locale';
  import { session } from '$lib/stores/session';
  import { threadsApi } from '$lib/api/threads';
  import { ApiError } from '$lib/api/client';
  import type { ThreadDetail } from '$lib/api/types';
  import ThreadView from '$lib/components/thread/ThreadView.svelte';
  import ThreadSidebar from '$lib/components/thread/ThreadSidebar.svelte';
  import ErrorState from '$lib/components/layout/ErrorState.svelte';
  import Spinner from '$lib/components/ui/Spinner.svelte';
  import Button from '$lib/components/ui/Button.svelte';

  let thread = $state<ThreadDetail | null>(null);
  let loading = $state(true);
  let status = $state<'ok' | 'notfound' | 'unauthorized' | 'error'>('ok');
  const canEdit = $derived(Boolean(thread && $session.user && thread.owner?.username === $session.user.username));

  async function load() {
    if (($page.params.id ?? '').endsWith('.md') || ($page.params.id ?? '').endsWith('.json')) {
      location.href = `/threads/${$page.params.id}`;
      return;
    }
    loading = true;
    status = 'ok';
    try {
      thread = await threadsApi.detail($page.params.id ?? '');
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) status = 'notfound';
      else if (err instanceof ApiError && err.status === 401) status = 'unauthorized';
      else status = 'error';
    } finally {
      loading = false;
    }
  }

  async function saveTitle(title: string) {
    if (!thread) return;
    const next = title.trim();
    if (!next || next === thread.title) return;
    const updated = await threadsApi.updateMeta(thread.id, { title: next });
    thread = { ...thread, title: updated.title };
  }

  onMount(load);
</script>

<svelte:head>
  {#if thread}<title>{thread.title} · Jieli</title>{/if}
</svelte:head>

<div class="app-container detail-container">
  {#if loading}
    <div class="state"><Spinner /></div>
  {:else if status === 'notfound'}
    <ErrorState />
  {:else if status === 'unauthorized'}
    <div class="state">
      <p class="muted">{$t('common.error')}</p>
      <Button href="/auth/signin" variant="primary">{$t('nav.signIn')}</Button>
    </div>
  {:else if status === 'error'}
    <div class="state">
      <p class="muted">{$t('common.error')}</p>
      <Button variant="secondary" onclick={load}>{$t('common.retry')}</Button>
    </div>
  {:else if thread}
    <div class="detail-grid">
      <ThreadView {thread} editableTitle={canEdit} ontitlechange={saveTitle}>
        {#snippet titleAction()}
          <ThreadSidebar thread={thread as ThreadDetail} readonly={!canEdit} onchange={(next) => (thread = next)} mobile />
        {/snippet}
      </ThreadView>
      <ThreadSidebar bind:thread readonly={!canEdit} />
    </div>
  {/if}
</div>

<style>
  .detail-grid {
    position: relative;
    display: flex;
    justify-content: center;
    align-items: flex-start;
  }

  .detail-container {
    width: 100%;
    max-width: none;
    margin-top: -0.5rem;
    padding-left: var(--space-6);
    padding-right: var(--space-6);
  }

  .detail-grid :global(.thread-view) {
    min-width: 0;
    width: min(880px, calc(100vw - 580px));
  }

  .detail-grid :global(.sidebar) {
    position: absolute;
    top: 0;
    right: var(--space-2);
  }

  @media (max-width: 1480px) {
    .detail-grid :global(.sidebar) {
      right: calc(var(--space-2) - var(--space-6));
    }
  }

  @media (max-width: 1180px) {
    .detail-grid {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: var(--space-6);
    }

    .detail-container {
      padding-left: var(--space-3);
      padding-right: var(--space-3);
    }

    .detail-grid :global(.thread-view) {
      grid-column: 1;
      width: 100%;
    }

    .detail-grid :global(.sidebar) {
      display: none;
    }
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
</style>
