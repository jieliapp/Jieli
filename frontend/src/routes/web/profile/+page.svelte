<script lang="ts">
  import { goto } from '$app/navigation';
  import { session } from '$lib/stores/session';

  $effect(() => {
    if (!$session.loaded) return;
    const username = $session.user?.username;
    goto(username ? `/@${username}` : '/web/auth/signin', { replaceState: true });
  });
</script>

<div class="app-container"></div>
