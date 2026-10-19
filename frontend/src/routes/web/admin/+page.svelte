<script lang="ts">
 import { enabled as commercialEnabled, adminItem } from '$commercial/edition';
  import { onMount } from 'svelte';
  import '$lib/styles/settings-layout.css';
  import '$lib/styles/admin-panels.css';
  import AdminPanel from '$commercial/AdminPanel.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { configApi } from '$lib/api/auth';
  import { api } from '$lib/api/client';
  import { locale, t } from '$lib/stores/locale';
  import { session, updateSessionUser } from '$lib/stores/session';
  import type { User } from '$lib/api/types';

  type Section = string;
  type AdminUser = User & { email: string; display_name: string };
  let activeSection = $state<Section>('users');
  let allowed = $state(false);
  let loading = $state(true);
  let accessError = $state('');
  let configError = $state('');
  let editionEnabled = $state(false);
 const extraSection = $derived(editionEnabled ? adminItem($locale) : null);
  let users = $state<AdminUser[]>([]);
  let page = $state(1);
  let creatingUser = $state(false);
  let editingUser = $state<AdminUser | null>(null);
  let username = $state('');
  let displayName = $state('');
  let email = $state('');
  let password = $state('');
  let usersBusy = $state(false);
  let usersError = $state('');
  let usersMessage = $state('');
  let emailBusy = $state(false);
  let emailError = $state('');
  let emailMessage = $state('');
  let smtp = $state({ public_base_url: '', host: '', port: 465, username: '', password: '', from: '', tls_mode: 'tls', tested: false, registration_enabled: false, has_password: false });
  const text = (zh: string, en: string) => $locale === 'zh' ? zh : en;

  function clearUserForm() {
    creatingUser = false; editingUser = null;
    email = ''; password = ''; username = ''; displayName = '';
  }

  function editUser(user: AdminUser) {
    usersError = ''; usersMessage = ''; creatingUser = false;
    editingUser = user; email = user.email; username = user.username;
    displayName = user.display_name; password = '';
  }

  async function saveUser() {
    if (editingUser) {
      const updated = await api.put<User>(`/admin/users/${editingUser.id}`, { email, username, display_name: displayName, password });
      if (updated.id === $session.user?.id) updateSessionUser(updated);
      clearUserForm();
      usersMessage = text('用户已更新。', 'User updated.');
      await loadUsers();
    } else {
      await api.post('/admin/users', { email, password });
      clearUserForm();
      usersMessage = text('账号已创建，请安全地交付临时密码。', 'Account created. Deliver the temporary password securely.');
      await loadUsers(1);
    }
  }

  async function toggleInactive(user: AdminUser) {
    await run('users', async () => {
      const updated = await api.put<AdminUser>(`/admin/users/${user.id}/inactive`, { inactive: !user.inactive });
      users = users.map(item => item.id === updated.id ? updated : item);
      usersMessage = updated.inactive ? text('用户已禁用。', 'User is inactive.') : text('用户已恢复。', 'User reactivated.');
    });
  }

  async function run(section: 'users' | 'email', action: () => Promise<void>) {
    if (section === 'users' ? usersBusy : emailBusy) return;
    if (section === 'users') { usersBusy = true; usersError = ''; usersMessage = ''; }
    else { emailBusy = true; emailError = ''; emailMessage = ''; }
    try { await action(); }
    catch (error) {
      let message = error instanceof Error ? error.message : text('操作失败', 'Operation failed');
      if (message === 'invalid public site URL') message = text('请填写有效的 HTTP 或 HTTPS 站点地址，不含登录信息、查询参数或片段。', 'Enter a valid HTTP or HTTPS site URL without credentials, query parameters, or fragments.');
      if (message === 'tested SMTP and public site URL are required') message = text('请先保存站点地址，并成功测试邮件服务。', 'Save the site URL and successfully test the email service first.');
      if (section === 'users') usersError = message;
      else emailError = message;
    } finally {
      if (section === 'users') usersBusy = false;
      else emailBusy = false;
    }
  }

  async function loadUsers(nextPage = page) {
    const result = await api.get<AdminUser[]>('/admin/users', { page: nextPage });
    users = result;
    page = nextPage;
  }

  async function loadConfig() {
    try { editionEnabled = commercialEnabled && Boolean((await configApi.get()).cloud_billing); }
    catch (error) { configError = error instanceof Error ? error.message : text('站点配置加载失败', 'Could not load site configuration'); }
  }

  onMount(() => {
    void (async () => {
      try {
        const me = await api.get<User>('/auth/me');
        if (!me.is_admin) throw new Error(text('需要管理员权限', 'Administrator required'));
        allowed = true;
        await Promise.all([
          loadConfig(),
          run('users', () => loadUsers()),
          run('email', async () => {
            const settings = await api.get<typeof smtp>('/admin/site-settings');
            smtp = { ...smtp, ...settings, password: '' };
          })
        ]);
      } catch (error) {
        accessError = error instanceof Error ? error.message : text('无法验证管理员权限', 'Could not verify administrator access');
      } finally { loading = false; }
    })();
  });
</script>

<svelte:head><title>{$t('admin.title')} · Jieli</title></svelte:head>

<div class="settings-frame admin-frame">
  {#if allowed}
    <aside class="settings-nav" aria-label={$t('admin.title')}>
      <div class="nav-title"><span>{$t('admin.title')}</span></div>
      <nav class="nav-list">
        <h2 class="nav-group">{text('身份与访问', 'Identity and access')}</h2>
        <button type="button" data-section="users" class:active={activeSection === 'users'} aria-current={activeSection === 'users' ? 'page' : undefined} aria-controls="users-panel" onclick={() => activeSection = 'users'}>
          <span class="nav-icon"><svg viewBox="0 0 20 20" aria-hidden="true"><circle cx="10" cy="7" r="3.2" /><path d="M4.8 16.2c.9-3 2.6-4.5 5.2-4.5s4.3 1.5 5.2 4.5" /></svg></span>
          <strong>{text('用户', 'Users')}</strong>
        </button>
        <button type="button" data-section="email" class:active={activeSection === 'email'} aria-current={activeSection === 'email' ? 'page' : undefined} aria-controls="email-panel" onclick={() => activeSection = 'email'}>
          <span class="nav-icon"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="3" y="4.5" width="14" height="11" rx="1.5" /><path d="m3.5 5 6.5 5 6.5-5" /></svg></span>
          <strong>{text('站点配置', 'Site Configuration')}</strong>
        </button>
        {#if extraSection}
          <h2 class="nav-group">{extraSection.group}</h2>
          <button type="button" data-section={extraSection.id} class:active={activeSection === extraSection.id} aria-current={activeSection === extraSection.id ? 'page' : undefined} aria-controls={`${extraSection.id}-panel`} onclick={() => activeSection = extraSection.id}>
            <span class="nav-icon"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="3" y="7" width="14" height="3" rx="1" /><path d="M4 10v7h12v-7M10 7v10M10 7S4 7 5 4s5 3 5 3 4-6 5-3-5 3-5 3" /></svg></span>
            <strong>{extraSection.label}</strong>
          </button>
        {/if}
      </nav>
    </aside>
  {/if}

  <main class="settings-content admin-content">
    {#if !allowed}
      <header class="admin-hero"><h1>{$t('admin.title')}</h1></header>
      {#if accessError}<p role="alert">{accessError}</p>
      {:else if loading}<p role="status">{text('正在验证管理员权限…', 'Checking administrator access…')}</p>{/if}
    {:else}
      {#if configError}<p role="alert">{configError}</p>{/if}
      <div id="users-panel" class="admin-flow" hidden={activeSection !== 'users'}>
        <header class="admin-hero"><h1>{editingUser ? text('编辑用户', 'Edit user') : creatingUser ? text('创建用户', 'Create user') : text('用户', 'Users')}</h1><p>{editingUser ? text('更新用户资料与登录凭据。', 'Update user details and sign-in credentials.') : creatingUser ? text('为新用户设置邮箱与临时登录密码。', 'Set an email address and temporary password for a new user.') : text('管理站点账号，为新用户创建登录凭据。', 'Manage site accounts and create credentials for new users.')}</p></header>
        {#if usersError && !creatingUser && !editingUser}<p role="alert">{usersError}</p>{/if}
        {#if usersMessage}<p role="status">{usersMessage}</p>{/if}
        {#if !creatingUser && !editingUser}
          <section class="admin-surface" aria-labelledby="user-list-title" aria-busy={usersBusy}>
            <div class="admin-surface-head"><h2 id="user-list-title">{text('用户列表', 'User accounts')}</h2><Button size="sm" disabled={usersBusy} onclick={() => { usersError = ''; usersMessage = ''; creatingUser = true; }}>{text('创建用户', 'Create user')}</Button></div>
            <div class="admin-table-scroll">
              <table class="admin-table"><thead><tr><th>ID</th><th>{text('邮箱', 'Email')}</th><th>{text('用户名', 'Username')}</th><th>{text('角色', 'Role')}</th><th>{text('操作', 'Actions')}</th></tr></thead>
                <tbody>{#each users as user (user.id)}<tr><td>{user.id}</td><td>{user.email || '—'}</td><td>{user.username}{#if user.inactive}<span class="inactive-label">{text('禁用', 'Inactive')}</span>{/if}</td><td>{user.is_admin ? text('管理员', 'Administrator') : text('用户', 'User')}</td><td><div class="user-actions"><button type="button" class="edit-user" disabled={usersBusy || user.inactive} aria-label={`${text('编辑用户', 'Edit user')} ${user.username}`} title={text('编辑用户', 'Edit user')} onclick={() => editUser(user)}><svg viewBox="0 0 20 20" aria-hidden="true"><path d="m12.5 3.5 4 4M3 17l4.5-1 10-10a1.5 1.5 0 0 0 0-2l-1.5-1.5a1.5 1.5 0 0 0-2 0l-10 10L3 17Z" /></svg></button><button type="button" class="inactive-switch" class:on={user.inactive} role="switch" aria-checked={Boolean(user.inactive)} aria-label={`${text('禁用', 'Inactive')} ${user.username}`} title={user.id === $session.user?.id ? text('无法禁用当前账号', 'Cannot disable your own account') : text('禁用', 'Inactive')} disabled={usersBusy || user.id === $session.user?.id} onclick={() => void toggleInactive(user)}><span class="switch-knob"></span></button></div></td></tr>{/each}</tbody>
              </table>
            </div>
            {#if usersBusy}<p class="admin-note" role="status">{text('正在加载…', 'Loading…')}</p>
            {:else if users.length === 0 && !usersError}<p class="admin-note">{text('暂无用户', 'No users yet')}</p>{/if}
            <div class="admin-actions admin-pagination">
              <span class="page-count">{text('第', 'Page')} {page}{text('页', '')}</span>
              <Button variant="secondary" size="sm" disabled={usersBusy || page === 1} onclick={() => void run('users', () => loadUsers(page - 1))}>{text('上一页', 'Previous')}</Button>
              <Button variant="secondary" size="sm" disabled={usersBusy || users.length < 50} onclick={() => void run('users', () => loadUsers(page + 1))}>{text('下一页', 'Next')}</Button>
            </div>
          </section>
        {:else}
          <section class="admin-surface" aria-labelledby="user-form-title" aria-busy={usersBusy}>
            <div class="admin-surface-head"><h2 id="user-form-title">{editingUser ? text('用户资料与登录凭据', 'User details and sign-in credentials') : text('登录凭据', 'Sign-in credentials')}</h2><Button variant="ghost" size="sm" disabled={usersBusy} onclick={() => { clearUserForm(); usersError = ''; usersMessage = ''; }}>{text('返回用户列表', 'Back to users')}</Button></div>
            <form class="admin-form" novalidate onsubmit={(event) => {
              event.preventDefault();
              if (usersBusy) return;
              if (!event.currentTarget.checkValidity()) {
                usersError = event.currentTarget.querySelector<HTMLInputElement>('input:invalid')!.validationMessage;
                usersMessage = '';
                return;
              }
              void run('users', saveUser);
            }}>
              <label for={editingUser ? 'edit-email' : 'create-email'}>{text('邮箱', 'Email')}<input id={editingUser ? 'edit-email' : 'create-email'} type="email" bind:value={email} autocomplete="off" disabled={usersBusy} required /></label>
              {#if editingUser}
                <label for="edit-username">{text('用户名', 'Username')}<input id="edit-username" bind:value={username} autocomplete="off" maxlength="32" disabled={usersBusy} required /></label>
                <label for="edit-display-name">{text('显示名称', 'Display name')}<input id="edit-display-name" bind:value={displayName} autocomplete="off" disabled={usersBusy} /></label>
              {/if}
              <label for={editingUser ? 'edit-password' : 'create-password'}>{editingUser ? text('新密码（留空保持不变）', 'New password (leave blank to keep current)') : text('临时密码', 'Temporary password')}<input id={editingUser ? 'edit-password' : 'create-password'} type="password" bind:value={password} autocomplete="new-password" minlength="8" maxlength="72" disabled={usersBusy} required={!editingUser} /></label>
              <p class="admin-hint">{editingUser ? (editingUser.id === $session.user?.id ? text('新密码至少 8 位；修改后保持当前登录。', 'Use at least 8 characters; your current session stays signed in.') : text('新密码至少 8 位；重设后，用户下次登录须更改密码。', 'Use at least 8 characters; after a reset, the user must change their password on next sign-in.')) : text('用户首次登录时必须更改密码。', 'The user must change this password on their first sign-in.')}</p>
              {#if usersError}<p role="alert">{usersError}</p>{/if}
              <div class="admin-actions"><Button type="submit" disabled={usersBusy}>{editingUser ? text('保存', 'Save') : text('创建', 'Create')}</Button></div>
            </form>
          </section>
        {/if}
      </div>

      <div id="email-panel" class="admin-flow" hidden={activeSection !== 'email'}>
        <header class="admin-hero"><h1>{text('站点配置', 'Site Configuration')}</h1><p>{text('管理站点的全局配置。', 'Manage global site configuration.')}</p></header>
        {#if emailError}<p role="alert">{emailError}</p>{/if}
        {#if emailMessage}<p role="status">{emailMessage}</p>{/if}
        <section class="admin-surface" aria-labelledby="public-base-url-title" aria-busy={emailBusy}>
          <div class="admin-surface-head"><h2 id="public-base-url-title">{text('站点地址', 'Site address')}</h2></div>
          <form class="admin-form" onsubmit={(event) => { event.preventDefault(); void run('email', async () => {
            await api.post('/admin/site-settings', { public_base_url: smtp.public_base_url });
            smtp.public_base_url = smtp.public_base_url.trim().replace(/\/+$/, '');
            if (!smtp.public_base_url) smtp.registration_enabled = false;
            emailMessage = text('站点地址已保存。', 'Site address saved.');
          }); }}>
            <p class="admin-hint">{text('请填写本站实际访问的地址，用来设置插件的 JIELI_BASE_URL 变量。', 'Enter this site’s actual access URL to configure the plugin’s JIELI_BASE_URL variable.')}</p>
            <label for="public-base-url">{text('公开站点地址', 'Public site URL')}<input id="public-base-url" type="url" bind:value={smtp.public_base_url} placeholder="https://example.com" /></label>
            <div class="admin-actions"><Button type="submit" disabled={emailBusy}>{text('保存站点地址', 'Save site URL')}</Button></div>
          </form>
        </section>
        <section class="admin-surface" aria-labelledby="smtp-title" aria-busy={emailBusy}>
          <div class="admin-surface-head"><h2 id="smtp-title">{text('邮件服务', 'Email service')}</h2></div>
          <form class="admin-form" onsubmit={(event) => { event.preventDefault(); void run('email', async () => {
            const { has_password: _hasPassword, ...config } = smtp;
            await api.post('/admin/smtp/test', config);
            smtp.password = ''; smtp.has_password = true; smtp.tested = true; smtp.registration_enabled = false;
            emailMessage = text('测试邮件已发送，配置已保存。', 'Test email sent; settings saved.');
          }); }}>
            <p class="admin-hint">{text('测试成功后保存 SMTP 配置，并关闭公开注册。需要另行开启注册。', 'A successful test saves SMTP settings and closes public registration. Enable registration separately.')}</p>
            <div class="admin-form-grid">
              <label for="smtp-host">SMTP Host<input id="smtp-host" bind:value={smtp.host} required /></label>
              <label for="smtp-port">{text('端口', 'Port')}<input id="smtp-port" type="number" bind:value={smtp.port} min="1" max="65535" required /></label>
              <label for="smtp-username">{text('用户名', 'Username')}<input id="smtp-username" bind:value={smtp.username} autocomplete="off" /></label>
              <label for="smtp-password">{smtp.has_password ? text('SMTP 密码（留空使用已保存密码）', 'SMTP password (leave blank to use the saved password)') : text('SMTP 密码', 'SMTP password')}<input id="smtp-password" type="password" bind:value={smtp.password} autocomplete="new-password" /></label>
              <label for="smtp-from">{text('发件邮箱', 'Sender email')}<input id="smtp-from" type="email" bind:value={smtp.from} required /></label>
              <label for="smtp-tls">TLS<select id="smtp-tls" bind:value={smtp.tls_mode}><option value="tls">TLS</option><option value="starttls">STARTTLS</option></select></label>
            </div>
            <div class="admin-actions"><Button type="submit" disabled={emailBusy}>{text('测试并保存', 'Test and save')}</Button></div>
          </form>
        </section>
        <section class="admin-surface" aria-labelledby="registration-title">
          <div class="admin-surface-head"><h2 id="registration-title">{text('公开注册', 'Public registration')}</h2></div>
          <div class="admin-setting-row">
            <div><strong>{smtp.registration_enabled ? text('已开启', 'Enabled') : text('已关闭', 'Disabled')}</strong><p class="admin-hint">{text('保存站点地址并成功测试邮件服务后，可单独开启公开注册。', 'Save the site URL and test the email service before enabling public registration.')}</p></div>
            <Button variant="secondary" disabled={emailBusy || !smtp.tested} onclick={() => void run('email', async () => {
              const enabled = !smtp.registration_enabled;
              await api.post('/admin/registration', { enabled });
              smtp.registration_enabled = enabled;
            })}>{smtp.registration_enabled ? text('关闭注册', 'Close registration') : text('开启注册', 'Open registration')}</Button>
          </div>
        </section>
      </div>

      {#if extraSection}
        <div id={`${extraSection.id}-panel`} class="admin-flow" hidden={activeSection !== extraSection.id}>
          <AdminPanel />
        </div>
      {/if}
    {/if}
  </main>
</div>

<style>
  .admin-frame .settings-nav { justify-content: start; }
  .admin-content { min-width: 0; }
  .page-count { margin-right: auto; color: var(--color-text-muted); font-size: var(--text-xs); }
  .nav-group { margin: var(--space-4) var(--space-3) var(--space-2); color: var(--color-text-subtle); font-size: var(--text-xs); font-weight: 600; }
  .nav-group:first-child { margin-top: 0; }
  .user-actions { display: flex; align-items: center; gap: var(--space-2); }
  .inactive-label { display: block; margin-top: var(--space-1); color: var(--color-danger); font-size: var(--text-xs); }
  .inactive-switch { position: relative; display: inline-flex; align-items: center; flex-shrink: 0; width: 40px; height: 22px; padding: 0; border: none; border-radius: 999px; background: var(--color-border); cursor: pointer; }
  .inactive-switch.on { background: var(--color-danger); }
  .inactive-switch:disabled { opacity: 0.5; cursor: default; }
  .switch-knob { width: 18px; height: 18px; border-radius: 50%; background: #fff; transform: translateX(2px); transition: transform 0.2s ease; }
  .inactive-switch.on .switch-knob { transform: translateX(20px); }
  .edit-user { display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: transparent; color: var(--color-text-muted); cursor: pointer; }
  .edit-user:hover { background: var(--color-bg-hover); color: var(--color-text); }
  .edit-user:disabled { opacity: 0.5; cursor: default; }
  .edit-user svg { width: 16px; height: 16px; fill: none; stroke: currentColor; stroke-width: 1.4; stroke-linejoin: round; stroke-linecap: round; }
  @media (max-width: 900px) {
    .nav-group { grid-column: 1 / -1; }
  }
</style>
