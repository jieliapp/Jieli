<script lang="ts">
  import { api } from '$lib/api/client';
  import PasswordReset from '$lib/components/auth/PasswordReset.svelte';
  import { loadSession } from '$lib/stores/session';
  async function submitPassword(password: string) {
    await api.post('/auth/password', { password });
    await loadSession();
  }
</script>
<PasswordReset {submitPassword} signedIn />
