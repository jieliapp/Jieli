<script lang="ts">
 import { enabled as commercialEnabled, settingsItems } from '$commercial/edition';
  import { onMount } from 'svelte';
  import '$lib/styles/settings-layout.css';
  import SettingsNav from './SettingsNav.svelte';
  type SectionId = string;
  let activeSection = $state<SectionId>('account');
  function syncSection() {
    const hash = window.location.hash.slice(1);
    activeSection = navItems.find(item => item.id === hash)?.id ?? 'account';
  }
  function navigateTo(section: SectionId) {
    activeSection = section;
    window.location.hash = section;
    error = ''; feedback = '';
    accountErrorKey = '';
  }
  onMount(() => {
    window.addEventListener('hashchange', syncSection);
    return () => window.removeEventListener('hashchange', syncSection);
  });
 import { configApi } from '$lib/api/auth';
 import SettingsPanel from '$commercial/SettingsPanel.svelte';
 let editionEnabled = $state(false);
  import { t, locale, setLocale } from '$lib/stores/locale';
  import { session, updateSessionUser } from '$lib/stores/session';
  import { userApi } from '$lib/api/user';
  import { ApiError } from '$lib/api/client';
  import { copyToClipboard } from '$lib/utils';
  import { isSupportedAvatarMediaType, readFileAsDataURL } from '$lib/utils/avatar';
  import type { ApiKeyInfo, Profile } from '$lib/api/types';
  import Button from '$lib/components/ui/Button.svelte';
  import Input from '$lib/components/ui/Input.svelte';

  let profile = $state<Profile | null>(null);
  let key = $state<ApiKeyInfo | null>(null);
  let username = $state('');
  let displayName = $state('');
  let busy = $state(false);
  let error = $state('');
  let feedback = $state('');
  let showKey = $state(false);
  let tokenCopied = $state(false);
  let editingAccount = $state(false);
  let accountErrorKey = $state('');
  let avatarInput = $state<HTMLInputElement>();
  const accountDirty = $derived(username !== profile?.user.username || displayName !== (profile?.user.display_name ?? ''));
  function formatUsageCount(value: number): string {
    const absolute = Math.abs(value);
    if (absolute < 10_000) return value.toLocaleString();
    return `${value < 0 ? '−' : ''}${Math.round(absolute / 1_000)}k`;
  }
  function startAccountEdit() {
    username = profile?.user.username ?? '';
    displayName = profile?.user.display_name ?? '';
    error = ''; feedback = ''; accountErrorKey = ''; editingAccount = true;
  }
  function cancelAccountEdit() { editingAccount = false; error = ''; accountErrorKey = ''; }
  const profileStats = $derived(profile?.stats);
  const linesOfCodeParts = $derived(profileStats ? {
    added: '+' + formatUsageCount(profileStats.lines_added),
    removed: '−' + formatUsageCount(profileStats.lines_removed),
    net: '~' + formatUsageCount(profileStats.lines_added - profileStats.lines_removed)
  } : null);
  const text = (zh: string, en: string) => $locale === 'zh' ? zh : en;
  const navItems = $derived([
    { id: 'account' as const, icon: 'account' as const, label: $t('settings.account'), caption: '' },
    { id: 'security' as const, icon: 'security' as const, label: text('安全', 'Security'), caption: '' },
    ...(editionEnabled ? settingsItems($locale) : [])
  ]);

  async function run(action: () => Promise<void>) {
    if (busy) return;
    busy = true;
    error = ''; feedback = '';
    try { await action(); }
    catch (e) { error = e instanceof Error ? e.message : text('操作失败', 'Operation failed'); }
    finally { busy = false; }
  }
  async function refresh() {
    profile = await userApi.profile();
    updateSessionUser(profile.user);
  }
  onMount(() => { void run(async () => {
    editionEnabled = commercialEnabled && Boolean((await configApi.get()).cloud_billing);
    syncSection();
    await refresh();
    username = profile!.user.username;
    displayName = profile!.user.display_name ?? '';
    key = await userApi.apiKey();
  }); });
  async function save() {
    if (busy) return;
    error = ''; feedback = ''; accountErrorKey = '';
    const normalizedUsername = username.trim();
    if (/[A-Z]/.test(normalizedUsername)) {
      accountErrorKey = 'settings.usernameLowercaseRequired';
      return;
    }
    if (!/^[a-z0-9]{3,32}$/.test(normalizedUsername)) {
      accountErrorKey = 'settings.usernameInvalid';
      return;
    }
    busy = true;
    try {
      updateSessionUser(await userApi.updateProfile({ username: normalizedUsername, display_name: displayName }));
      await refresh();
      editingAccount = false;
      feedback = $t('settings.accountSaved');
    } catch (e) {
      accountErrorKey = e instanceof ApiError && e.status === 409
        ? 'settings.usernameUnavailable'
        : 'settings.accountSaveFailed';
    } finally { busy = false; }
  }
  async function avatar(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    await run(async () => {
      if (!isSupportedAvatarMediaType(file.type)) throw new Error($t('settings.unsupportedAvatarType'));
      updateSessionUser(await userApi.uploadAvatar({ data: await readFileAsDataURL(file), mediaType: file.type }));
      if (profile && $session.user) profile = { ...profile, user: $session.user };
      feedback = $t('settings.avatarUpdated');
    });
    input.value = '';
  }
  async function rotate() {
    if (!confirm($t('settings.rotateConfirm'))) return;
    await run(async () => {
      const result = await userApi.rotateApiKey();
      key = { has_key: true, api_key: result.api_key, prefix: result.prefix };
      showKey = false;
    });
  }
</script>
<svelte:head><title>{$t('settings.title')} · Jieli</title></svelte:head>

<div class="settings-frame">
  <SettingsNav {activeSection} {navItems} {navigateTo} />
  <main class="settings-content">
    {#if error}<p role="alert" class="error-text">{error}</p>{/if}
    {#if feedback}<p role="status">{feedback}</p>{/if}
    {#if activeSection === 'account'}
      <section class="content-flow account-flow">
        <header class="page-hero"><h1>{$t('settings.accountTitle')}</h1><p>{text('管理个人资料、会话同步和账户偏好。', 'Manage your profile, conversation sync, and account preferences.')}</p></header>
        <section class="surface profile-surface">
          <div class="profile-section-head">
            <h2>{$t('settings.account')}</h2>
          </div>
          {#if $session.user}
            <div class="profile-card">
              <div class="profile-main">
                <div class="avatar-column">
                  {#if editingAccount}
                    <button
                      class="avatar-button"
                      type="button"
                      aria-label={$t('settings.avatarUploadHint')}
                      title={$t('settings.avatarUploadHint')}
                      onclick={() => avatarInput?.click()}
                      disabled={busy}
                    >
                      {#if $session.user.avatar_url}
                        <img class="profile-avatar" src={$session.user.avatar_url} alt={$session.user.username} />
                      {:else}
                        <span class="profile-avatar avatar-fallback">{($session.user.display_name || $session.user.username).slice(0, 1).toUpperCase()}</span>
                      {/if}
                    </button>
                    <input
                      bind:this={avatarInput}
                      class="visually-hidden"
                      type="file"
                      accept={'image/png,image/jpeg,image/gif,image/webp'}
                      onchange={avatar}
                    />
                  {:else}
                    <div class="avatar-static">
                      {#if $session.user.avatar_url}
                        <img class="profile-avatar" src={$session.user.avatar_url} alt={$session.user.username} />
                      {:else}
                        <span class="profile-avatar avatar-fallback">{($session.user.display_name || $session.user.username).slice(0, 1).toUpperCase()}</span>
                      {/if}
                    </div>
                  {/if}
                </div>

                <div class="identity-copy">
                  {#if editingAccount}
                    <div class="account-form">
                      <label class="form-field">
                        <span>{$t('settings.displayName')}</span>
                        <Input bind:value={displayName} placeholder={$session.user.display_name || $session.user.username} />
                      </label>
                      <label class="form-field">
                        <span>{$t('settings.username')}</span>
                        <Input bind:value={username} placeholder={$session.user.username} oninput={(event) => {
                          username = (event.currentTarget as HTMLInputElement).value;
                          accountErrorKey = /[A-Z]/.test(username) ? 'settings.usernameLowercaseRequired' : '';
                        }} />
                      </label>
                      {#if $session.user.email}
                        <label class="form-field">
                          <span>{$t('settings.email')}</span>
                          <Input value={$session.user.email} disabled />
                        </label>
                      {/if}
                      {#if accountErrorKey}
                        <p class="account-form-feedback error-text" role="alert">{$t(accountErrorKey)}</p>
                      {/if}
                    </div>
                  {:else}
                    <div class="identity-grid">
                      <div>
                        <span class="mini-label">{$t('settings.displayName')}</span>
                        <strong>{($session.user.display_name || $session.user.username)}</strong>
                      </div>
                      <div>
                        <span class="mini-label">{$t('settings.username')}</span>
                        <strong>@{$session.user.username}</strong>
                      </div>
                      {#if $session.user.email}
                        <div>
                          <span class="mini-label">{$t('settings.email')}</span>
                          <strong>{$session.user.email}</strong>
                        </div>
                      {/if}
                    </div>
                  {/if}
                </div>
              </div>

              <div class="profile-actions">
                {#if editingAccount}
                  <Button variant="secondary" size="sm" onclick={cancelAccountEdit} disabled={busy}>
                    {$t('settings.cancelAccountEdit')}
                  </Button>
                  <Button variant="primary" size="sm" onclick={save} disabled={!accountDirty || !username.trim() || busy}>
                    {busy ? $t('settings.savingAccount') : $t('settings.saveAccount')}
                  </Button>
                {:else}
                  <Button variant="secondary" size="sm" onclick={startAccountEdit}>{$t('settings.editDetails')}</Button>
                  <Button href={$session.user ? `/@${$session.user.username}` : '/web/auth/signin'} variant="secondary" size="sm">
                    {$t('settings.viewProfile')}
                  </Button>
                {/if}
              </div>
            </div>

            {#if editingAccount && busy}
              <p class="avatar-hint">{$t('settings.savingAccount')}</p>
            {/if}
          {/if}
        </section>

        <section class="surface usage-overview">
          <div class="surface-head">
            <h2>{$t('settings.usageStatistics')}</h2>

          </div>
          <div class="account-usage-grid">
            <div>
              <span>{$t('settings.linesOfCode')}</span>
              <strong class="lines-code">
                {#if linesOfCodeParts}
                  <span class="line-added">{linesOfCodeParts.added}</span>
                  <span class="line-removed">{linesOfCodeParts.removed}</span>
                  <span class="line-net">{linesOfCodeParts.net}</span>
                {:else}
                  —
                {/if}
              </strong>
              <small>{profileStats ? $t('settings.filesChanged').replace('{count}', formatUsageCount(profileStats.files_changed ?? 0)) : $t('settings.syncStats')}</small>
            </div>
            <div>
              <span>{$t('settings.threads')}</span>
              <strong>{profileStats ? formatUsageCount(profileStats.total_threads ?? 0) : '—'}</strong>
              <small>{$t('settings.syncedThreads')}</small>
            </div>
            <div>
              <span>{$t('settings.messages')}</span>
              <strong>{profileStats ? formatUsageCount(profileStats.total_messages ?? 0) : '—'}</strong>
            </div>
          </div>
        </section>


        <section class="surface preferences-surface">
          <div class="surface-head"><h2>{$t('settings.preferences')}</h2></div>
          <div class="preference-row">
            <div class="preference-icon">◎</div>
            <div><strong>{$t('settings.language')}</strong></div>
            <div class="lang">
              {#each ['zh', 'en'] as language}
                <button type="button" class:active={$locale === language} disabled={busy} onclick={() => run(async () => { await userApi.setLocale(language); setLocale(language as 'zh' | 'en'); })}>{language === 'zh' ? '中文' : 'English'}</button>
              {/each}
            </div>
          </div>
          {#if profile}
            <div class="preference-row">
              <div class="preference-icon">↑</div>
              <div><strong>{text('上传消息', 'Uploaded messages')}</strong><p>{text('已使用', 'Used')} {profile.current_messages.toLocaleString()} / {profile.message_quota < 0 ? '∞' : profile.message_quota.toLocaleString()} {$t('settings.messages')}</p>
                {#if profile.usage_info?.last_upload_error?.code === 'message_quota_exceeded'}<p class="quota-alert">{$t('settings.messageQuotaExceededInline')}</p>{/if}
              </div>
              <div class="quota-status"><span class:unlimited={profile.message_quota < 0}>{profile.message_quota < 0 ? (profile.plan_level === 'pro' ? 'Pro · ' : '') + text('无限制', 'Unlimited') : profile.plan_level.toUpperCase()}</span></div>
            </div>
            <div class="preference-row">
              <div class="preference-icon">↺</div>
              <div>
                <strong>{profile.message_quota < 0 ? text('存储恢复', 'Storage recovery') : $t('settings.autoCleanup')}</strong>
                <p>{profile.message_quota < 0 ? text('当前不限制消息存储。点击恢复，可找回此前因清理隐藏的对话。', 'Message storage is unlimited. Restore conversations previously hidden by cleanup.') : text('超过存储额度时自动清理旧会话。', 'Automatically clean up old conversations when storage exceeds the limit.')}</p>
              </div>
              {#if profile.message_quota < 0}
                <Button variant="secondary" size="sm" disabled={busy} onclick={() => run(async () => { const result = await userApi.restoreThreads(); await refresh(); feedback = $t('settings.restoreThreadsSuccess').replace('{count}', result.restored.toLocaleString()); })}>{$t('settings.restoreThreads')}</Button>
              {:else}
                <button type="button" class="switch" class:on={profile.auto_cleanup_enabled} role="switch" aria-checked={profile.auto_cleanup_enabled} aria-label={$t('settings.autoCleanup')} disabled={busy} onclick={() => { const enabled = !profile!.auto_cleanup_enabled; if (enabled && !confirm(text('超过存储额度时会自动清理旧会话，确认开启？', 'Automatically clean up old conversations when storage exceeds the limit?'))) return; void run(async () => { await userApi.setAutoCleanup(enabled); await refresh(); }); }}><span class="switch-knob"></span></button>
              {/if}
            </div>
          {/if}
        </section>
      </section>
    {:else if activeSection === 'security'}
      <section class="content-flow security-flow">
        <header class="page-hero"><h1>{text('安全', 'Security')}</h1><p>{text('管理登录密码和同步访问密钥。', 'Manage your login password and sync access token.')}</p></header>
        <section class="surface security-card">
          <div><h2>{text('密码', 'Password')}</h2><p>{text('更改用于登录 Jieli 的密码。', 'Change the password you use to sign in to Jieli.')}</p></div>
          <Button href="/web/auth/password" variant="secondary">{text('修改密码', 'Change password')}</Button>
        </section>
        <section class="surface security-card">
          <div><h2>{text('访问密钥', 'Access token')}</h2><p>{text('用于 Jieli 插件身份验证和会话同步。', 'Used to authenticate Jieli plugins and sync conversations.')}</p><code>{`${key?.prefix ?? ''}••••••••`}</code></div>
    <div class="actions">
      <Button variant="secondary" disabled={!key?.api_key} onclick={() => run(async () => { if (!await copyToClipboard(key!.api_key!)) throw new Error(text('复制失败', 'Copy failed')); tokenCopied = true; window.setTimeout(() => (tokenCopied = false), 1500); })}>{tokenCopied ? text('已复制', 'Copied') : $t('settings.copyToken')}</Button>
      <Button variant="secondary" disabled={busy} onclick={rotate}>{text('轮换密钥', 'Rotate token')}</Button>
    </div>
        </section>
      </section>
    {:else if editionEnabled}
      <SettingsPanel {refresh} {profile} />
    {/if}
  </main>
</div>
<style>


.mini-label {
    color: var(--color-text-subtle);
    font-size: 11px;
    font-weight: 650;
    letter-spacing: 0;
    text-transform: uppercase;
  }
.content-flow {
    display: grid;
    gap: var(--space-5);
  }
.content-flow > * {
    min-width: 0;
  }
.account-flow {
    gap: var(--space-3);
  }
.account-flow .page-hero {
    order: 0;
    padding-top: 0;
  }
.account-flow .profile-surface {
    order: 1;
  }
.account-flow .usage-overview {
    order: 2;
  }

.account-flow .preferences-surface {
    order: 5;
  }
.account-flow .page-hero h1 {
    margin-bottom: var(--space-1);
    font-family: var(--font-sans);
    font-size: var(--text-2xl);
    font-weight: 650;
    line-height: 1.12;
  }
.account-flow .page-hero p {
    line-height: 1.5;
  }
.page-hero {
    padding-top: var(--space-1);
  }
.page-hero h1 {
    margin: var(--space-1) 0 var(--space-2);
    font-family: var(--font-serif);
    font-size: 2.25rem;
    font-weight: 500;
    line-height: 1.06;
  }
.page-hero p {
    max-width: 640px;
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    line-height: 1.75;
  }
.surface-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-5);
  }
.surface {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-bg-elevated);
    box-shadow: var(--shadow-sm);
  }
.surface {
    padding: var(--space-5);
  }
.surface-head {
    margin-bottom: var(--space-5);
  }
.surface-head h2 {
    margin: var(--space-1) 0 0;
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    font-weight: 500;
    line-height: 1.1;
  }
.profile-card {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: var(--space-5);
    align-items: center;
    min-height: 92px;
    padding: var(--space-3) var(--space-5);
  }
.profile-surface {
    padding: 0;
    overflow: hidden;
  }
.profile-section-head {
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--color-border);
  }
.profile-section-head h2 {
    margin: 0;
    color: var(--color-text);
    font-size: var(--text-base);
    font-weight: 700;
  }
.profile-main {
    min-width: 0;
    display: grid;
    grid-template-columns: 72px minmax(0, 1fr);
    align-items: center;
    gap: var(--space-5);
  }
.avatar-column {
    display: flex;
    align-items: center;
  }
.profile-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
.avatar-button,
.avatar-static {
    padding: 0;
    background: transparent;
    border: 0;
    border-radius: var(--radius-full);
  }
.avatar-button {
    cursor: pointer;
  }
.profile-avatar {
    display: block;
    width: 56px;
    height: 56px;
    border-radius: var(--radius-full);
    object-fit: cover;
    box-shadow: 0 0 0 1px var(--color-border);
  }
.avatar-fallback {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-accent-contrast);
    background: var(--color-accent);
    font-family: var(--font-serif);
    font-size: 28px;
  }
.identity-copy {
    min-width: 0;
    width: 100%;
  }
.identity-grid {
    width: 100%;
    display: grid;
    grid-template-columns: minmax(0, 0.8fr) minmax(0, 1fr) minmax(0, 1.35fr);
    gap: var(--space-5);
    align-items: center;
  }
.identity-grid div {
    min-width: 0;
    display: grid;
    gap: var(--space-1);
  }
.identity-grid strong {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text);
    font-size: var(--text-base);
    font-weight: 650;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
.account-form {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-3);
    width: min(100%, 760px);
  }
.account-form-feedback {
    grid-column: 1 / -1;
    margin: 0;
    font-size: var(--text-sm);
  }
.form-field {
    position: relative;
    display: grid;
  }
.form-field span,
.account-form .form-field span {
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
    font-weight: 650;
  }
.account-form .form-field span {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
.avatar-hint {
    margin: var(--space-3) 0 0;
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
  }
.error-text {
    color: var(--color-danger);
  }
.usage-overview {
    padding: 0;
    overflow: hidden;
  }
.usage-overview .surface-head {
    align-items: center;
    justify-content: space-between;
    margin: 0;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }
.usage-overview .surface-head h2 {
    margin: 0;
    font-family: var(--font-sans);
    font-size: var(--text-lg);
    font-weight: 650;
  }
.account-flow .preferences-surface .surface-head h2 {
    margin: 0;
    font-family: var(--font-sans);
    font-size: var(--text-lg);
    font-weight: 650;
  }
.account-flow .preferences-surface {
    padding: 0;
    overflow: hidden;
  }
.account-flow .preferences-surface .surface-head {
    align-items: center;
    justify-content: space-between;
    margin: 0;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }
.account-flow .preferences-surface > :not(.surface-head) {
    margin-right: var(--space-4);
    margin-left: var(--space-4);
  }
.account-flow .preferences-surface > .surface-head + * {
    margin-top: var(--space-4);
  }
.account-flow .preferences-surface > :not(.surface-head):last-child {
    margin-bottom: var(--space-4);
  }
.account-usage-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1px;
    background: var(--color-border);
  }
.account-usage-grid div {
    min-width: 0;
    display: grid;
    grid-template-rows: auto 28px auto;
    gap: var(--space-1);
    align-content: start;
    padding: var(--space-3) var(--space-4);
    background: color-mix(in srgb, var(--color-bg-elevated) 88%, var(--color-bg));
  }
.account-usage-grid > div > span {
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
    font-weight: 650;
    text-transform: uppercase;
  }
.account-usage-grid strong {
    min-width: 0;
    overflow: hidden;
    font-size: var(--text-2xl);
    font-weight: 500;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
.account-usage-grid strong {
    display: flex;
    align-items: center;
    overflow: hidden;
    font-size: var(--text-lg);
    line-height: 1;
    white-space: nowrap;
  }
.lines-code {
    display: flex;
    gap: var(--space-1);
    align-items: center;
    font-variant-numeric: tabular-nums;
  }
.lines-code .line-added {
    color: var(--color-success);
  }
.lines-code .line-removed {
    color: var(--color-danger);
  }
.lines-code .line-net {
    color: #d39a1f;
  }
.account-usage-grid small {
    color: var(--color-text-muted);
    font-size: var(--text-xs);
  }
@media (min-width: 901px) {
.account-usage-grid div {
      grid-template-rows: auto 24px auto;
      padding: var(--space-3);
    }
.account-flow .account-usage-grid {
      grid-template-columns: minmax(220px, 1.45fr) minmax(120px, 1fr) minmax(120px, 1fr);
    }
.account-usage-grid strong {
      font-size: var(--text-base);
    }
}
.preferences-surface {
    padding: var(--space-3) var(--space-4);
  }
.preferences-surface .surface-head {
    margin-bottom: var(--space-2);
  }
@media (max-width: 900px) {
.page-hero h1 {
      font-size: 1.9rem;
    }
.surface-head,
.profile-card,
.profile-actions {
      display: grid;
      justify-content: stretch;
    }
.profile-card {
      grid-template-columns: 1fr;
      min-height: 0;
      padding: var(--space-4);
    }
.profile-actions {
      display: flex;
      justify-content: start;
      gap: var(--space-2);
    }
.identity-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: var(--space-3);
    }
.account-form,
.account-usage-grid {
      grid-template-columns: 1fr;
    }
}
@media (max-width: 620px) {
.profile-main {
      grid-template-columns: 1fr;
      gap: var(--space-4);
      justify-items: start;
    }
.profile-actions {
      display: flex;
      flex-wrap: wrap;
    }
.profile-actions :global(a),
.profile-actions :global(button) {
      flex: 1 1 128px;
      justify-content: center;
    }
.identity-grid {
      grid-template-columns: 1fr;
    }
}

.actions { display:flex; flex-wrap:wrap; gap:var(--space-2); margin-block:var(--space-4); }
code { overflow-wrap:anywhere; }
label { display:grid; gap:var(--space-2); }
.visually-hidden { position:absolute; width:1px; height:1px; overflow:hidden; clip-path:inset(50%); }
@media(max-width:600px) { .profile-card, .profile-main, .identity-grid, .account-form { grid-template-columns:minmax(0,1fr); } .profile-actions { justify-content:flex-start; } }
.lang {
    display: inline-flex;
    gap: var(--space-1);
    padding: 3px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg-inset);
  }
.lang button {
    border: 0;
    border-radius: calc(var(--radius-md) - 2px);
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
    font: inherit;
    font-size: var(--text-xs);
    padding: var(--space-2) var(--space-3);
  }
.lang button.active {
    background: var(--color-bg-elevated);
    color: var(--color-text);
    box-shadow: var(--shadow-sm);
  }
.preferences-surface .lang button {
    padding: var(--space-1) var(--space-3);
  }
.preference-row {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr) auto;
    gap: var(--space-2);
    align-items: center;
    padding: var(--space-2) 0;
    border-top: 1px solid var(--color-border);
  }
.preference-row > div:nth-child(2) {
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }
.preference-row:first-of-type {
    border-top: 0;
    padding-top: 0;
  }
.preference-row:last-child {
    padding-bottom: 0;
  }
.preference-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg-inset);
    color: var(--color-text-muted);
  }
.preference-row strong {
    display: block;
    font-size: var(--text-sm);
  }
.preference-row p {
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-xs);
    line-height: 1.3;
    white-space: nowrap;
  }
.quota-alert {
    color: var(--color-danger);
  }
.quota-status {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    font-weight: 650;
    white-space: nowrap;
  }
.quota-status .unlimited {
    color: var(--color-success);
  }
.switch {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: flex-start;
    flex: 0 0 auto;
    width: 40px;
    height: 22px;
    border: none;
    border-radius: 999px;
    background: var(--color-border);
    cursor: pointer;
    padding: 0;
    transition: background 0.2s ease;
  }
.switch.on {
    background: var(--color-accent);
  }
.switch-knob {
    position: static;
    flex: 0 0 auto;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #fff;
    transform: translateX(2px);
    transition: transform 0.2s ease;
  }
.switch.on .switch-knob {
    transform: translateX(20px);
  }
.preference-row {
      display: grid;
      justify-content: stretch;
    }
.preference-row p {
      white-space: normal;
    }
.preference-row > div:nth-child(2) { flex-wrap:wrap; }
.preference-row p { white-space:normal; }
@media(max-width:600px) { .preference-row { grid-template-columns:28px minmax(0,1fr); } .preference-row > :last-child { grid-column:2; justify-self:start; } }

.security-flow .page-hero h1 { font-family:var(--font-sans); font-size:var(--text-2xl); font-weight:650; }
.security-card { display:flex; align-items:center; justify-content:space-between; gap:var(--space-5); }
.security-card > div:first-child { min-width:0; }
.security-card h2 { margin:0 0 var(--space-2); font-size:var(--text-lg); }
.security-card p { margin:0 0 var(--space-2); color:var(--color-text-muted); }
.security-card .actions { flex-shrink:0; margin:0; }
@media(max-width:900px) { .security-card { align-items:flex-start; flex-direction:column; } }
</style>
