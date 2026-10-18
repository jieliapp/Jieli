<script lang="ts">
  let {
    id = undefined,
    value = $bindable(''),
    placeholder = '',
    type = 'text',
    disabled = false,
    min = undefined,
    max = undefined,
    step = undefined,
    oninput = undefined
  }: {
    id?: string;
    value?: string;
    placeholder?: string;
    type?: string;
    disabled?: boolean;
    min?: string | number;
    max?: string | number;
    step?: string | number;
    oninput?: (e: Event) => void;
  } = $props();
</script>

<input class="input" {id} {type} {placeholder} {disabled} {min} {max} {step} bind:value {oninput} />

<style>
  .input {
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-size: var(--text-sm);
    transition: border-color 0.15s;
  }

  .input::placeholder {
    color: var(--color-text-subtle);
  }

  .input:focus {
    outline: none;
    border-color: var(--color-accent);
  }

  .input:disabled {
    opacity: 0.5;
  }
</style>
