<script lang="ts">
  interface Option {
    value: string;
    label: string;
  }

  let {
    value = $bindable(''),
    options,
    placeholder = '',
    onchange = undefined
  }: {
    value?: string;
    options: Option[];
    placeholder?: string;
    onchange?: (value: string) => void;
  } = $props();

  let open = $state(false);
  let root: HTMLDivElement;

  const selected = $derived(options.find((o) => o.value === value));
  const buttonLabel = $derived(selected ? selected.label : placeholder);

  function toggle() {
    open = !open;
  }

  function select(option: Option) {
    value = option.value;
    open = false;
    onchange?.(option.value);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') open = false;
  }

  function onWindowClick(e: MouseEvent) {
    if (open && root && !root.contains(e.target as Node)) open = false;
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<div class="dropdown" bind:this={root}>
  <button
    type="button"
    class="trigger"
    class:placeholder={!selected}
    aria-haspopup="listbox"
    aria-expanded={open}
    onclick={toggle}
  >
    <span class="trigger-label">{buttonLabel}</span>
    <svg class="chevron" class:up={open} viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
      <path fill="none" stroke="currentColor" stroke-width="1.5" d="M4 6l4 4 4-4" />
    </svg>
  </button>

  {#if open}
    <ul class="menu" role="listbox">
      {#each options as option (option.value)}
        <li>
          <button
            type="button"
            class="option"
            class:active={option.value === value}
            role="option"
            aria-selected={option.value === value}
            onclick={() => select(option)}
          >
            {option.label}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .dropdown {
    position: relative;
  }

  .trigger {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    min-width: 140px;
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-size: var(--text-sm);
    cursor: pointer;
    transition: border-color 0.15s;
  }

  .trigger:hover {
    border-color: var(--color-border-strong);
  }

  .trigger[aria-expanded='true'] {
    border-color: var(--color-accent);
  }

  .trigger.placeholder .trigger-label {
    color: var(--color-text-subtle);
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

  .menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 20;
    min-width: 100%;
    max-height: 280px;
    overflow-y: auto;
    margin: 0;
    padding: var(--space-1);
    list-style: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
  }

  .option {
    display: block;
    width: 100%;
    padding: var(--space-2) var(--space-3);
    text-align: left;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text);
    font-size: var(--text-sm);
    white-space: nowrap;
    cursor: pointer;
  }

  .option:hover {
    background: var(--color-bg-hover);
  }

  .option.active {
    color: var(--color-accent);
  }
</style>
