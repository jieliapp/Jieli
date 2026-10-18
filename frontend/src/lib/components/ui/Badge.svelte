<script lang="ts">
  import type { Snippet } from 'svelte';

  type Tone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger';

  let {
    tone = 'neutral',
    children
  }: {
    tone?: Tone;
    children: Snippet;
  } = $props();
</script>

<span class="badge badge-{tone}">{@render children()}</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: 500;
    line-height: 1.4;
    border: 1px solid var(--color-border);
  }

  .badge-neutral {
    background: var(--color-bg-inset);
    color: var(--color-text-muted);
  }

  .badge-accent {
    background: rgba(107, 142, 173, 0.14);
    color: var(--color-accent);
    border-color: rgba(107, 142, 173, 0.3);
  }

  .badge-success {
    background: rgba(36, 130, 50, 0.12);
    color: var(--color-success);
    border-color: rgba(36, 130, 50, 0.28);
  }

  .badge-warning {
    background: rgba(166, 124, 82, 0.14);
    color: var(--color-warning);
    border-color: rgba(166, 124, 82, 0.3);
  }

  .badge-danger {
    background: rgba(176, 75, 61, 0.12);
    color: var(--color-danger);
    border-color: rgba(176, 75, 61, 0.3);
  }
</style>
