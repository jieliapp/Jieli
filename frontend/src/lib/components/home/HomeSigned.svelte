<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/stores/locale';
  import { threadsApi } from '$lib/api/threads';
  import { timeAgo } from '$lib/utils';
  import type { ThreadSummary, User } from '$lib/api/types';
  import Spinner from '$lib/components/ui/Spinner.svelte';

  let { user }: { user: User } = $props();

  let threads = $state<ThreadSummary[]>([]);
  let loading = $state(true);
  let error = $state(false);

  onMount(async () => {
    try {
      const res = await threadsApi.recent(9);
      threads = res.threads ?? [];
    } catch {
      error = true;
    } finally {
      loading = false;
    }
  });
</script>

<div class="app-container signed-home">
  <div class="home-grid">
    <section class="panel recent-panel" aria-labelledby="recent-title">
      <div class="panel-header">
        <div class="panel-title">
          <svg class="panel-icon" viewBox="0 0 20 20" aria-hidden="true">
            <path d="M8.45 4.25c-3.05 0-5.5 1.9-5.5 4.25 0 1.2.65 2.26 1.7 3l-.58 1.88 2.2-1.12c.66.3 1.4.47 2.18.47 3.05 0 5.5-1.9 5.5-4.23 0-2.35-2.45-4.25-5.5-4.25Z" />
            <path d="M12.25 7.5c2.55.25 4.5 1.88 4.5 3.85 0 1.08-.58 2.03-1.52 2.72l.48 1.68-1.98-1.05c-.56.22-1.2.34-1.88.34-1.63 0-3.05-.7-3.78-1.75" opacity=".58" />
          </svg>
          <h2 id="recent-title">{$t('home.signedTitle')}</h2>
        </div>
        <a href="/threads">{$t('home.viewAll')}</a>
      </div>

      {#if loading}
        <div class="state"><Spinner /></div>
      {:else if error}
        <p class="state muted">{$t('common.error')}</p>
      {:else if threads.length === 0}
        <p class="state muted">{$t('common.empty')}</p>
      {:else}
        <ul class="thread-list">
          {#each threads as thread (thread.id)}
            <li>
              <a class="thread-row" href={`/threads/${thread.provider_thread_id}`}>
                <span class="title">{thread.title || thread.provider_thread_id}</span>
                <span class="time">{timeAgo(thread.updated_at)}</span>
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <aside class="side">
      <section class="panel sync-card" aria-labelledby="sync-title">
        <div class="sync-ornament" aria-hidden="true">
          <svg viewBox="0 0 24 24">
            <path d="M12 4v11" />
            <path d="m7.5 8.5 4.5-4.5 4.5 4.5" />
            <path d="M5 15.5V18a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-2.5" />
          </svg>
        </div>
        <h2 id="sync-title">{$t('home.syncTitle')}</h2>
        <p class="muted">{$t('home.syncDesc')}</p>
        <div class="quick">
          <a class="quick-link" href="/threads">
            <span>{$t('home.quickThreads')}</span>
            <span aria-hidden="true">→</span>
          </a>
          <a class="quick-link" href="/web/settings#security">
            <span>{$t('home.quickToken')}</span>
            <span aria-hidden="true">→</span>
          </a>
        </div>
      </section>
    </aside>
  </div>
</div>

<style>
  .signed-home {
    max-width: 1440px;
    min-height: calc(100vh - var(--topbar-height));
    padding-top: var(--space-8);
    padding-left: var(--space-8);
    padding-right: var(--space-8);
  }

  .home-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 360px);
    gap: var(--space-12);
    align-items: start;
    min-height: calc(100vh - var(--topbar-height) - var(--space-16));
  }

  .panel {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-sm);
  }

  .recent-panel {
    grid-column: 2;
    width: 100%;
    overflow: hidden;
    justify-self: end;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--color-border);
  }

  .panel-title {
    display: inline-flex;
    align-items: center;
    min-width: 0;
    gap: var(--space-2);
    color: var(--color-text-muted);
  }

  .panel-icon {
    width: 20px;
    height: 20px;
    flex: 0 0 auto;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.45;
    stroke-linecap: round;
    stroke-linejoin: round;
    shape-rendering: geometricPrecision;
  }

  .panel-header h2 {
    margin: 0;
    color: var(--color-text-muted);
    font-size: 0.95rem;
    font-weight: 500;
    letter-spacing: -0.02em;
  }

  .panel-header a {
    color: var(--color-text-muted);
    font-size: 0.78rem;
    white-space: nowrap;
  }

  .panel-header a:hover {
    color: var(--color-text);
  }

  .thread-list {
    display: grid;
    gap: 1px;
    list-style: none;
    margin: 0;
    padding: var(--space-3) var(--space-4) var(--space-4);
  }

  .thread-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-4);
    padding: 0.46rem var(--space-2);
    color: var(--color-text);
    border-radius: var(--radius-md);
  }

  .thread-row:hover {
    background: var(--color-bg-hover);
  }

  .title {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text);
    font-size: 0.86rem;
    font-weight: 400;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .time {
    color: var(--color-text-muted);
    font-size: 0.78rem;
    line-height: 1;
    white-space: nowrap;
  }

  .side {
    grid-column: 1;
    grid-row: 1;
    align-self: center;
    justify-self: center;
    width: min(420px, 100%);
  }

  .sync-card {
    display: grid;
    gap: var(--space-3);
    padding: var(--space-5);
    background: color-mix(in srgb, var(--color-bg) 82%, transparent);
  }

  .sync-ornament {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2.5rem;
    height: 2.5rem;
    color: var(--color-text-muted);
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
  }

  .sync-ornament svg {
    width: 1.15rem;
    height: 1.15rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .sync-card h2 {
    margin: var(--space-1) 0 0;
    font-size: 1rem;
    letter-spacing: -0.02em;
  }

  .quick {
    display: grid;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }

  .quick-link {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: 0.62rem var(--space-3);
    color: var(--color-text-muted);
    background: var(--color-bg-inset);
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
  }

  .quick-link:hover {
    color: var(--color-text);
    background: var(--color-bg-hover);
    border-color: var(--color-border);
  }

  .muted {
    color: var(--color-text-muted);
  }

  .state {
    padding: var(--space-8);
    text-align: center;
  }

  @media (max-width: 900px) {
    .signed-home {
      padding-top: var(--space-5);
      padding-left: var(--space-4);
      padding-right: var(--space-4);
    }

    .home-grid {
      grid-template-columns: 1fr;
      gap: var(--space-5);
      min-height: auto;
    }

    .recent-panel,
    .side {
      grid-column: 1;
      grid-row: auto;
      width: 100%;
    }

    .side {
      position: static;
      justify-self: stretch;
    }
  }

  @media (max-width: 640px) {
    .panel-header {
      padding: var(--space-4);
    }

    .thread-list {
      padding: var(--space-2) var(--space-3) var(--space-3);
    }

    .thread-row {
      gap: var(--space-3);
    }
  }
</style>
