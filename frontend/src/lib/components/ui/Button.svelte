<script lang="ts">
  import type { Snippet } from 'svelte';

  type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';
  type Size = 'sm' | 'md';

  let {
    variant = 'primary',
    size = 'md',
    type = 'button',
    href = undefined,
    reload = false,
    disabled = false,
    onclick = undefined,
    children
  }: {
    variant?: Variant;
    size?: Size;
    type?: 'button' | 'submit' | 'reset';
    href?: string;
    reload?: boolean;
    disabled?: boolean;
    onclick?: (e: MouseEvent) => void;
    children: Snippet;
  } = $props();
</script>

{#if href}
  <a class="btn btn-{variant} btn-{size}" class:disabled {href} data-sveltekit-reload={reload} {onclick}>
    {@render children()}
  </a>
{:else}
  <button class="btn btn-{variant} btn-{size}" {type} {disabled} {onclick}>
    {@render children()}
  </button>
{/if}

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
    white-space: nowrap;
    text-decoration: none;
  }

  .btn-md {
    padding: var(--space-2) var(--space-4);
    font-size: var(--text-sm);
  }

  .btn-sm {
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-xs);
  }

  .btn-primary {
    background: var(--color-primary);
    color: var(--color-primary-contrast);
  }
  .btn-primary:hover {
    background: var(--color-primary-hover);
    color: var(--color-primary-contrast);
  }

  .btn-secondary {
    background: var(--color-secondary);
    border-color: var(--color-border);
    color: var(--color-text);
  }
  .btn-secondary:hover {
    background: var(--color-secondary-hover);
    border-color: var(--color-border-strong);
    color: var(--color-text);
  }

  .btn-ghost {
    background: transparent;
    color: var(--color-text-muted);
  }
  .btn-ghost:hover {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .btn-danger {
    background: transparent;
    border-color: var(--color-border);
    color: var(--color-danger);
  }
  .btn-danger:hover {
    background: rgba(176, 75, 61, 0.1);
    border-color: var(--color-danger);
  }

  .btn:disabled,
  .btn.disabled {
    opacity: 0.5;
    pointer-events: none;
  }
</style>
