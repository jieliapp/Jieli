<script lang="ts">
  import { session } from '$lib/stores/session';
  import HomeSigned from '$lib/components/home/HomeSigned.svelte';
  import Spinner from '$lib/components/ui/Spinner.svelte';
  import { t } from '$lib/stores/locale';
  import Button from '$lib/components/ui/Button.svelte';

  const pageTitle = $derived(`${$t('common.appName')} — ${$t('home.signedTitle')}`);
</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

{#if !$session.loaded}
  <div class="app-container center">
    <Spinner />
  </div>
{:else if $session.user}
  <HomeSigned user={$session.user} />
{:else}
  <div class="app-container center empty">
    <p>{$t('signin.note')}</p>
    <Button href="/web/auth/signin" variant="primary">{$t('nav.signIn')}</Button>
  </div>
{/if}

<style>
  .center {
    display: flex;
    justify-content: center;
    padding: var(--space-10) 0;
  }

  .empty {
    align-items: center;
    flex-direction: column;
    gap: var(--space-4);
    color: var(--color-text-muted);
  }
</style>
