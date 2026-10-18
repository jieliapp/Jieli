<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { t } from '$lib/stores/locale';
  import { threadsApi } from '$lib/api/threads';
  import type { ThreadDetail } from '$lib/api/types';
  import ThreadView from '$lib/components/thread/ThreadView.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Spinner from '$lib/components/ui/Spinner.svelte';

  let thread = $state<ThreadDetail | null>(null);
  let loading = $state(true);
  // Single ambiguous failure state — must not leak whether a private thread exists.
  let failed = $state(false);

  onMount(async () => {
    try {
      thread = await threadsApi.shared($page.params.slug ?? '');
    } catch {
      failed = true;
    } finally {
      loading = false;
    }
  });
</script>

<div class="app-container">
  {#if loading}
    <div class="state"><Spinner /></div>
  {:else if failed || !thread}
    <div class="state"><p class="muted">{$t('shared.notFound')}</p></div>
  {:else}
    <div class="shared-head">
      <Badge tone="neutral">{$t('shared.readOnly')}</Badge>
    </div>
    <ThreadView {thread} />
  {/if}
</div>

<style>
  .shared-head {
    margin-bottom: var(--space-5);
  }

  .state {
    display: flex;
    justify-content: center;
    padding: var(--space-10) 0;
  }

  .muted {
    color: var(--color-text-muted);
  }
</style>
