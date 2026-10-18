<script lang="ts">
  import { page } from '$app/stores';
  import ErrorState from '$lib/components/layout/ErrorState.svelte';

  const status = $derived($page.status || 404);
  const titleKey = $derived(status === 404 ? 'shared.pageNotFound' : 'common.error');
</script>

<ErrorState {status} {titleKey} />
