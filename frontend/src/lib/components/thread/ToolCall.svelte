<script lang="ts">
  import { t } from '$lib/stores/locale';
  import type { Snippet } from 'svelte';

  let {
    label = undefined,
    children
  }: {
    label?: string;
    children: Snippet;
  } = $props();

  let open = $state(false);
</script>

<div class="toolcall">
  <button class="toggle" onclick={() => (open = !open)} aria-expanded={open}>
    <span class="chev" class:open>›</span>
    {open ? $t('detail.hideWork') : $t('detail.showWork')}
    {#if label}<span class="label">{label}</span>{/if}
  </button>
  {#if open}
    <div class="body">{@render children()}</div>
  {/if}
</div>

<style>
  .toolcall {
    margin: var(--space-2) 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg-elevated);
  }

  .toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    cursor: pointer;
    text-align: left;
  }

  .toggle:hover {
    color: var(--color-text);
  }

  .chev {
    display: inline-block;
    transition: transform 0.15s;
  }

  .chev.open {
    transform: rotate(90deg);
  }

  .label {
    color: var(--color-text-subtle);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .body {
    padding: var(--space-3);
    border-top: 1px solid var(--color-border);
    font-size: var(--text-sm);
  }
</style>
