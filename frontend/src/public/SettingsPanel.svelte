<script lang="ts">
  import type { Profile } from '$lib/api/types';
  let { refresh, profile }: { refresh: () => Promise<void>; profile: Profile | null } = $props();
</script>
<!-- Public edition has no commercial settings. -->
