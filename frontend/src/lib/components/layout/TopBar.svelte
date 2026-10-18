<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { session, logout } from '$lib/stores/session';
  import { t, localizedHref } from '$lib/stores/locale';
  import { authApi } from '$lib/api/auth';
  import ThreadsIcon from '$lib/components/icons/ThreadsIcon.svelte';

  type AccountItem = { href: string; key: string; kind: 'profile' | 'threads' | 'settings' };

  function accountItems(username: string): AccountItem[] {
    return [
      { href: `/@${username}`, key: 'nav.profile', kind: 'profile' },
      { href: '/threads', key: 'nav.threads', kind: 'threads' },
      { href: '/web/settings', key: 'nav.settings', kind: 'settings' }
    ];
  }

  let open = $state(false);
  let root = $state<HTMLDivElement>();

  function isActive(href: string): boolean {
    return $page.url.pathname === href || $page.url.pathname.startsWith(href + '/');
  }

  function closeMenu() {
    open = false;
  }

  function toggleMenu() {
    open = !open;
  }

  function onWindowClick(e: MouseEvent) {
    if (open && root && !root.contains(e.target as Node)) open = false;
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') open = false;
  }

  async function signOut() {
    closeMenu();
    await logout();
    await goto('/');
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<header class="topbar">
  <div class="app-container bar-inner">
    <a class="brand" href={$session.user ? '/web/dashboard' : $localizedHref('/')} aria-label={$t('common.appName')}>
      <img class="brand-logo" src="/brand/logo-240.png" alt={$t('common.appName')} />
    </a>

    {#if $session.user}
      <nav class="nav" aria-label="Primary">
        <a class="nav-link" class:active={isActive('/threads')} href="/threads">
          <span class="nav-link-icon"><ThreadsIcon /></span>
          <span>{$t('nav.threads')}</span>
        </a>
      </nav>
    {:else}
      <div class="spacer"></div>
    {/if}

    <div class="right">
      {#if $session.user}
        <div class="account" bind:this={root}>
          <button
            type="button"
            class="account-trigger"
            aria-haspopup="menu"
            aria-expanded={open}
            onclick={toggleMenu}
          >
            {#if $session.user.avatar_url}
              <img class="avatar" src={$session.user.avatar_url} alt={$session.user.username} />
            {:else}
              <span class="avatar avatar-fallback">{$session.user.username.charAt(0).toUpperCase()}</span>
            {/if}
            <svg class="chevron" class:up={open} viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
              <path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" d="M4 6l4 4 4-4" />
            </svg>
          </button>

          {#if open}
            <div class="account-menu" role="menu">
              <div class="account-head">
                {#if $session.user.avatar_url}
                  <img class="menu-avatar" src={$session.user.avatar_url} alt="" />
                {:else}
                  <span class="menu-avatar avatar-fallback">{$session.user.username.charAt(0).toUpperCase()}</span>
                {/if}
                <div class="account-meta">
                  <div class="account-name">{$session.user.display_name || $session.user.username}</div>
                  {#if $session.user.email}
                    <div class="account-email">{$session.user.email}</div>
                  {:else}
                    <div class="account-email">@{$session.user.username}</div>
                  {/if}
                </div>
              </div>

              <div class="menu-section">
                {#each accountItems($session.user.username) as item (item.href)}
                  <a class="menu-item" class:active={isActive(item.href)} href={item.href} role="menuitem" onclick={closeMenu}>
                    <span class="menu-icon" aria-hidden="true">
                      {#if item.kind === 'profile'}
                        <svg viewBox="0 0 24 24" fill="none">
                          <circle cx="12" cy="12" r="9" stroke="currentColor" />
                          <circle cx="12" cy="10" r="3" stroke="currentColor" />
                          <path d="M6.5 18.5a6 6 0 0 1 11 0" stroke="currentColor" stroke-linecap="round" />
                        </svg>
                      {:else if item.kind === 'threads'}
                        <ThreadsIcon />
                      {:else}
                        <svg viewBox="0 0 24 24" fill="none">
                          <circle cx="12" cy="12" r="3" stroke="currentColor" />
                          <path d="M19.4 13a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06A1.7 1.7 0 0 0 15 19.36a1.7 1.7 0 0 0-1 .64l-.04.08a2 2 0 0 1-3.92 0L10 20a1.7 1.7 0 0 0-1-.64 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.7 1.7 0 0 0 4.64 15a1.7 1.7 0 0 0-.64-1l-.08-.04a2 2 0 0 1 0-3.92L4 10a1.7 1.7 0 0 0 .64-1 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.7 1.7 0 0 0 9 4.64a1.7 1.7 0 0 0 1-.64l.04-.08a2 2 0 0 1 3.92 0L14 4a1.7 1.7 0 0 0 1 .64 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.7 1.7 0 0 0 19.36 9c.05.37.27.72.64 1l.08.04a2 2 0 0 1 0 3.92L20 14c-.37.28-.59.63-.6 1Z" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" />
                        </svg>
                      {/if}
                    </span>
                    <span>{$t(item.key)}</span>
                  </a>
                {/each}
              </div>

              <div class="menu-section">
                <a class="menu-item" href="https://github.com/jieliapp/plugins" target="_blank" rel="noreferrer" role="menuitem" onclick={closeMenu}>
                  <span class="menu-icon" aria-hidden="true">
                    <svg viewBox="0 0 24 24" fill="none">
                      <path d="M12 3v12" stroke="currentColor" stroke-linecap="round" />
                      <path d="m7 10 5 5 5-5" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" />
                      <path d="M5 19h14" stroke="currentColor" stroke-linecap="round" />
                    </svg>
                  </span>
                  <span>{$t('nav.installPlugin')}</span>
                </a>
                <a class="menu-item" class:active={isActive($localizedHref('/manual'))} href={$localizedHref('/manual')} role="menuitem" onclick={closeMenu}>
                  <span class="menu-icon" aria-hidden="true">
                    <svg viewBox="0 0 24 24" fill="none">
                      <path d="M6 4h11a1 1 0 0 1 1 1v15H7a2 2 0 0 1-2-2V5a1 1 0 0 1 1-1Z" stroke="currentColor" stroke-linejoin="round" />
                      <path d="M8 8h7M8 12h7M7 20a2 2 0 0 1 0-4h11" stroke="currentColor" stroke-linecap="round" />
                    </svg>
                  </span>
                  <span>{$t('nav.userManual')}</span>
                </a>
              </div>

              {#if $session.user.is_admin}
                <div class="menu-section">
                  <a class="menu-item" class:active={isActive('/web/admin')} href="/web/admin" role="menuitem" onclick={closeMenu}>
                    <span class="menu-icon" aria-hidden="true">
                      <svg viewBox="0 0 24 24">
                        <rect x="4" y="4" width="16" height="7" rx="1.5" />
                        <rect x="4" y="13" width="16" height="7" rx="1.5" />
                        <path d="M8 7.5h.01M8 16.5h.01M13 7.5h3M13 16.5h3" />
                      </svg>
                    </span>
                    <span>{$t('admin.title')}</span>
                  </a>
                </div>
              {/if}

              <div class="menu-section">
                <button class="menu-item signout" type="button" role="menuitem" onclick={signOut}>
                  <span class="menu-icon" aria-hidden="true">
                    <svg viewBox="0 0 24 24" fill="none">
                      <path d="M10 5H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4" stroke="currentColor" stroke-linecap="round" />
                      <path d="M14 8l4 4-4 4M18 12H9" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" />
                    </svg>
                  </span>
                  <span>{$t('nav.signOut')}</span>
                </button>
              </div>
            </div>
          {/if}
        </div>
      {:else}
        <a class="nav-link" class:active={isActive($localizedHref('/manual'))} href={$localizedHref('/manual')}>{$t('nav.userManual')}</a>
        <a class="nav-link" class:active={isActive($localizedHref('/pricing'))} href={$localizedHref('/pricing')}>{$t('nav.pricing')}</a>
        <a class="nav-link" href={authApi.signInUrl()}>{$t('nav.signIn')}</a>
      {/if}
    </div>
  </div>
</header>

<style>
  .topbar {
    position: sticky;
    top: 0;
    z-index: 40;
    height: var(--topbar-height);
    background: rgba(255, 255, 255, 0.88);
    backdrop-filter: blur(8px);
    border-bottom: 1px solid var(--color-border);
  }

  .bar-inner {
    height: 100%;
    max-width: 1440px;
    padding-left: var(--space-8);
    padding-right: var(--space-8);
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .brand {
    display: inline-flex;
    align-items: center;
    flex: 0 0 auto;
  }

  .brand-logo {
    display: block;
    width: auto;
    height: 24px;
  }

  .spacer {
    flex: 1;
  }

  .nav {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex: 1;
  }

  .nav-link {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    background: none;
    border: none;
    cursor: pointer;
  }

  .nav-link-icon {
    display: inline-flex;
    width: 18px;
    height: 18px;
    flex: 0 0 auto;
  }

  .nav-link:hover {
    color: var(--color-text);
    background: var(--color-bg-hover);
  }

  .nav-link.active {
    color: var(--color-text);
    background: var(--color-bg-inset);
  }

  .right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .account {
    position: relative;
  }

  .account-trigger {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 4px 6px;
    border: 1px solid transparent;
    border-radius: var(--radius-full);
    background: transparent;
    color: var(--color-text);
    cursor: pointer;
  }

  .account-trigger:hover,
  .account-trigger[aria-expanded='true'] {
    background: var(--color-bg-hover);
    border-color: var(--color-border);
  }

  .avatar {
    width: 30px;
    height: 30px;
    border-radius: var(--radius-full);
    object-fit: cover;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .avatar-fallback {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .chevron {
    color: var(--color-text-muted);
    transition: transform 0.15s;
  }

  .chevron.up {
    transform: rotate(180deg);
  }

  .account-menu {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 60;
    width: 232px;
    overflow: hidden;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-bg-elevated);
    box-shadow: var(--shadow-md);
  }

  .account-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
  }

  .menu-avatar {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-full);
    object-fit: cover;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 auto;
  }

  .account-meta {
    min-width: 0;
  }

  .account-name,
  .account-email {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .account-name {
    color: var(--color-text);
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .account-email {
    margin-top: 1px;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
  }

  .menu-section {
    padding: var(--space-1);
    border-top: 1px solid var(--color-border);
  }

  .menu-item {
    width: 100%;
    min-height: 40px;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border: 0;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--color-text);
    font: inherit;
    font-size: var(--text-base);
    text-align: left;
    cursor: pointer;
  }

  .menu-item:hover,
  .menu-item.active {
    background: var(--color-bg-hover);
  }

  .menu-icon {
    width: 20px;
    height: 20px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 auto;
    color: var(--color-text);
  }

  .menu-icon svg {
    width: 20px;
    height: 20px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .signout {
    color: var(--color-text);
  }

  @media (max-width: 520px) {
    .account-menu {
      width: min(264px, calc(100vw - 24px));
    }
  }
</style>
