<script lang="ts">
  import { session } from '$lib/stores/session';
  import { t } from '$lib/stores/locale';
  import HomeUnsigned from '$lib/components/home/HomeUnsigned.svelte';
  import HomeSigned from '$lib/components/home/HomeSigned.svelte';

  const pageTitle = $derived(`${$t('common.appName')} — ${$t('home.tagline')}`);
  const pageDesc = $derived($t('home.subtitle'));
  let { data }: { data: { lang: 'en' | 'zh' } } = $props();
  const canonical = $derived(data.lang === 'zh' ? 'https://jieli.app/zh' : 'https://jieli.app/');
</script>

<svelte:head>
  <title>{pageTitle}</title>
  <meta name="description" content={pageDesc} />
  <link rel="canonical" href={canonical} />
  <link rel="alternate" hreflang="en" href="https://jieli.app/" />
  <link rel="alternate" hreflang="zh" href="https://jieli.app/zh" />
  <link rel="alternate" hreflang="x-default" href="https://jieli.app/" />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={pageTitle} />
  <meta property="og:description" content={pageDesc} />
  <meta property="og:url" content={canonical} />
</svelte:head>

{#if $session.loaded && $session.user}
  <HomeSigned user={$session.user} />
{:else}
  <HomeUnsigned />
{/if}
