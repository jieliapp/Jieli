<script lang="ts">
  import { onMount } from 'svelte';
  import '$lib/styles/app.css';
  import TopBar from '$lib/components/layout/TopBar.svelte';
  import { session, loadSession } from '$lib/stores/session';
  import { locale } from '$lib/stores/locale';
  import { page } from '$app/state';
  import type { Snippet } from 'svelte';

  let { children }: { children: Snippet } = $props();
  const hideFooter = $derived(['/web/auth/signin', '/web/auth/password', '/web/auth/email'].includes(page.url.pathname));

  $effect(() => {
    if ($session.user?.must_change_password && page.url.pathname !== '/web/auth/password' && page.url.pathname !== '/web/auth/email') window.location.assign('/web/auth/password');
  });

  onMount(() => {
    loadSession();
    document.documentElement.lang = $locale;
  });
</script>

{#if !hideFooter}<TopBar />{/if}
<main class="page" class:auth-entry={hideFooter}>
  {@render children()}
</main>
{#if !hideFooter}
  <footer class="site-footer">
    <div class="app-container footer-inner">
      <span class="copyright">
        © 2026 Jieli
      </span>
      <nav aria-label="Legal">
        <a href="/privacy">Privacy Policy</a>
        <a href="/terms">Terms of Service</a>
        <a href="https://github.com/jieliapp/jieli/issues" target="_blank" rel="noreferrer">Issues</a>
        <a href="mailto:admin@jieli.app">Contact</a>
      </nav>
    </div>
  </footer>
{/if}

<style>
  .page {
    min-height: calc(100vh - var(--topbar-height) - 72px);
    padding: var(--space-6) 0 var(--space-10);
    background: var(--color-bg);
  }

  .page.auth-entry { min-height: 100svh; padding: 0; }

  .site-footer {
    border-top: 1px solid var(--color-border);
    background: var(--color-bg);
  }

  .footer-inner {
    min-height: 72px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    color: var(--color-text-subtle);
    font-size: var(--text-sm);
  }

  nav {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    flex-wrap: wrap;
  }

  a {
    color: var(--color-text-muted);
    text-decoration: none;
  }

  a:hover {
    color: var(--color-text);
  }

  @media (max-width: 640px) {
    .footer-inner {
      align-items: flex-start;
      flex-direction: column;
      justify-content: center;
      padding-top: var(--space-4);
      padding-bottom: var(--space-4);
    }
  }
</style>
