<script lang="ts">
  import { t } from '$lib/stores/locale';
  import Button from '$lib/components/ui/Button.svelte';

  let {
    status = 404,
    titleKey = 'shared.pageNotFound',
    descriptionKey = 'shared.notFound',
    actionHref = '/threads',
    actionKey = 'nav.threads'
  }: {
    status?: number;
    titleKey?: string;
    descriptionKey?: string;
    actionHref?: string;
    actionKey?: string;
  } = $props();
</script>

<div class="app-container error-container">
  <section class="error-card" aria-labelledby="error-title">
    <p class="eyebrow">{status}</p>
    <h1 id="error-title">{$t(titleKey)}</h1>
    <p class="description">{$t(descriptionKey)}</p>
    <Button href={actionHref} variant="secondary">{$t(actionKey)}</Button>
  </section>
</div>

<style>
  .error-container {
    display: flex;
    justify-content: center;
    padding-top: var(--space-12);
  }

  .error-card {
    width: min(100%, 560px);
    padding: var(--space-8);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    text-align: center;
  }

  .eyebrow {
    margin-bottom: var(--space-2);
    color: var(--color-text-subtle);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.08em;
  }

  h1 {
    margin-bottom: var(--space-3);
    font-size: clamp(2.25rem, 5vw, 4rem);
    line-height: 1;
  }

  .description {
    margin-bottom: var(--space-6);
    color: var(--color-text-muted);
  }
</style>
