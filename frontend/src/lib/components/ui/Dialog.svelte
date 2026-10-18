<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    open = $bindable(false),
    title = '',
    onclose = undefined,
    children
  }: {
    open?: boolean;
    title?: string;
    onclose?: () => void;
    children: Snippet;
  } = $props();

  function close() {
    open = false;
    onclose?.();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }
</script>

<svelte:window {onkeydown} />

{#if open}
  <div
    class="overlay"
    role="button"
    tabindex="-1"
    onclick={close}
    onkeydown={() => {}}
  >
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={() => {}}
    >
      {#if title}
        <h2 class="dialog-title">{title}</h2>
      {/if}
      <div class="dialog-body">
        {@render children()}
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(17, 17, 17, 0.35);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4);
    z-index: 100;
  }

  .dialog {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    width: 100%;
    max-width: 440px;
    padding: var(--space-5);
  }

  .dialog-title {
    margin: 0 0 var(--space-4);
    font-size: var(--text-lg);
  }
</style>
