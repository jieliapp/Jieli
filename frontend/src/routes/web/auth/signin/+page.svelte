<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { api, ApiError } from '$lib/api/client';
  import type { User } from '$lib/api/types';
  import { locale, t } from '$lib/stores/locale';
  import { authApi, configApi } from '$lib/api/auth';

  type Mode = 'login' | 'register' | 'reset' | 'setup';
  let mode = $state<Mode>('login');
  let step = $state<'details' | 'password' | 'sent'>('details');
  let email = $state('');
  let displayName = $state('');
  let password = $state('');
  let setupToken = $state('');
  let showPassword = $state(false);
  let busy = $state(false);
  let loading = $state(true);
  let configError = $state(false);
  let error = $state('');
  let oauthError = $state('');
  let emailInput: HTMLInputElement | undefined = $state();
  let passwordInput: HTMLInputElement | undefined = $state();
  let options = $state({ registration_enabled: false, recovery_enabled: false, setup_available: false });
  let config = $state({ dev_mode: false, github_oauth: false, google_oauth: false });
  const text = (zh: string, en: string) => $locale === 'zh' ? zh : en;
  const title = $derived(step === 'sent' ? text('请查看邮箱', 'Check your email')
    : mode === 'register' ? text('注册 Jieli', 'Sign up for Jieli')
    : mode === 'reset' ? text('重置密码', 'Reset your password')
    : mode === 'setup' ? text('创建管理员', 'Create your administrator')
    : text('登录 Jieli', 'Sign in to Jieli'));

  async function loadOptions() {
    loading = true;
    configError = false;
    try {
      const [authOptions, serverConfig] = await Promise.all([
        api.get<typeof options>('/auth/options'), configApi.get()
      ]);
      options = authOptions;
      config = serverConfig;
    } catch {
      configError = true;
    } finally {
      loading = false;
    }
  }

  async function changeMode(next: Mode) {
    if (busy) return;
    mode = next;
    step = 'details';
    password = '';
    setupToken = '';
    showPassword = false;
    error = '';
    oauthError = '';
    await tick();
    emailInput?.focus();
  }

  async function changeEmail() {
    if (busy) return;
    step = 'details';
    password = '';
    showPassword = false;
    error = '';
    await tick();
    emailInput?.focus();
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy || loading || configError) return;
    error = '';
    oauthError = '';
    email = email.trim();
    if (step === 'details' && mode !== 'reset') {
      if (mode === 'register' && [...displayName.trim()].length > 80) {
        error = text('昵称不能超过 80 个字符。', 'Display name must not exceed 80 characters.');
        return;
      }
      step = 'password';
      await tick();
      passwordInput?.focus();
      return;
    }
    if (mode === 'register' || mode === 'setup') {

      if (!/^[\x21-\x7e]{6,72}$/.test(password)) {
        error = text('密码须为 6–72 个字符，仅限英文字母、数字和英文符号，不含空格。', 'Use 6–72 characters: English letters, numbers and symbols, without spaces.');
        return;
      }
    }
    busy = true;
    try {
      if (mode === 'reset') {
        await api.post('/auth/reset', { email });
      } else if (mode === 'register') {
        await api.post('/auth/register', { email, password, display_name: displayName.trim() });
      } else {
        const payload = mode === 'setup' ? { email, password, token: setupToken } : { email, password };
        const result = await api.post<User>(`/auth/${mode}`, payload);
        window.location.assign(result.must_change_password ? '/web/auth/password' : '/web/dashboard');
        return;
      }
      password = '';
      showPassword = false;
      step = 'sent';
    } catch (cause) {
      error = mode === 'login' && cause instanceof ApiError && cause.status === 401
        ? $t('signin.invalidCredentials')
        : cause instanceof Error ? cause.message : text('操作失败，请重试。', 'Something went wrong. Please try again.');
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    const reason = new URLSearchParams(window.location.search).get('error');
    if (reason === 'oauth_email_exists') oauthError = 'signin.oauthEmailExists';
    else if (reason === 'oauth_cancelled') oauthError = 'signin.oauthCancelled';
    void loadOptions();
  });
</script>

{#snippet GithubIcon()}
  <svg class="oauth-icon" viewBox="0 0 24 24" aria-hidden="true">
    <path
      fill="currentColor"
      d="M12 .5A11.5 11.5 0 0 0 8.36 22.9c.58.11.79-.25.79-.56v-2.15c-3.22.7-3.9-1.37-3.9-1.37-.53-1.34-1.29-1.7-1.29-1.7-1.05-.72.08-.7.08-.7 1.16.08 1.78 1.2 1.78 1.2 1.04 1.77 2.72 1.26 3.38.96.1-.75.4-1.26.73-1.55-2.57-.29-5.28-1.29-5.28-5.73 0-1.27.45-2.3 1.2-3.11-.12-.3-.52-1.48.11-3.07 0 0 .98-.31 3.16 1.19A10.9 10.9 0 0 1 12 5.92c.98 0 1.95.13 2.87.39 2.18-1.5 3.16-1.19 3.16-1.19.63 1.59.23 2.77.11 3.07.75.81 1.2 1.84 1.2 3.11 0 4.45-2.71 5.43-5.29 5.72.42.36.79 1.07.79 2.16v3.16c0 .31.21.67.8.56A11.5 11.5 0 0 0 12 .5Z"
    />
  </svg>
{/snippet}

{#snippet GoogleIcon()}
  <svg class="oauth-icon" viewBox="0 0 24 24" aria-hidden="true">
    <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.76h3.56c2.08-1.92 3.28-4.74 3.28-8.09Z" />
    <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.56-2.76c-.98.66-2.24 1.06-3.72 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.85A11 11 0 0 0 12 23Z" />
    <path fill="#FBBC05" d="M5.84 14.11a6.6 6.6 0 0 1 0-4.22V7.04H2.18a11 11 0 0 0 0 9.92l3.66-2.85Z" />
    <path fill="#EA4335" d="M12 5.36c1.62 0 3.06.56 4.21 1.64l3.15-3.15A10.56 10.56 0 0 0 12 1 11 11 0 0 0 2.18 7.04l3.66 2.85C6.71 7.29 9.14 5.36 12 5.36Z" />
  </svg>
{/snippet}


<svelte:head><title>{title} · Jieli</title></svelte:head>

<div class="auth-page">
  <div class="auth-content">
    <a class="brand" href="/" aria-label={text('Jieli 首页', 'Jieli home')}>
      <img src="/brand/logo-240.png" alt="Jieli" width="100" />
    </a>
    <h1>{title}</h1>

    <section class="auth-card" aria-label={title} aria-busy={busy || loading}>
      {#if configError}
        <p class="error" role="alert">{text('暂时无法加载登录选项，请重试。', 'Sign-in options could not be loaded. Please try again.')}</p>
        <button class="primary" onclick={loadOptions}>{text('重试', 'Try again')}</button>
      {:else if step === 'sent'}
        <div class="sent-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/></svg>
        </div>
        <div class="sent-message" role="status">
          <p>{text('如果这个邮箱可以继续操作，你会收到一封邮件：', 'If this email is eligible, you’ll receive a message at:')}</p>
          <strong>{email}</strong>
          <p>{mode === 'reset' ? text('点击邮件中的链接设置新密码。', 'Follow the link in the email to set a new password.') : text('点击邮件中的链接完成注册，然后登录。', 'Follow the link to verify your email, then sign in.')}</p>
        </div>
        <button class="primary" onclick={() => changeMode('login')}>{text('返回登录', 'Back to sign in')}</button>
        <p class="card-footer">{text('没收到邮件？请检查垃圾邮件文件夹。', 'No email? Check your spam folder.')}</p>
      {:else}
        <form onsubmit={submit}>
          {#if mode === 'register' && step === 'details'}
            <div class="field">
              <label for="auth-name">{text('昵称', 'Display name')} <span class="optional">{text('选填', 'optional')}</span></label>
              <input id="auth-name" autocomplete="nickname" bind:value={displayName} disabled={busy || loading} placeholder={text('希望大家怎么称呼你', 'What should we call you?')} />
            </div>
          {/if}
          <div class="field">
            <div class="label-row">
              <label for="auth-email">{text('邮箱', 'Email')}</label>
              {#if step === 'password'}<button type="button" class="text-button muted" onclick={changeEmail} disabled={busy}>{text('更改邮箱', 'Change email')}</button>{/if}
            </div>
            <input id="auth-email" type="email" autocomplete="username" autocapitalize="none" spellcheck="false" bind:value={email} bind:this={emailInput} readonly={step === 'password'} disabled={busy || loading} required placeholder="you@example.com" />
          </div>
          {#if mode === 'reset'}
            <p class="help">{text('我们会发送一封邮件，帮助你重新设置密码。', 'We’ll email you a link to reset your password.')}</p>
          {/if}
          {#if step === 'password'}
            <div class="field">
              <div class="label-row">
                <label for="auth-password">{text('密码', 'Password')}</label>
                {#if mode === 'login' && options.recovery_enabled}<button type="button" class="text-button muted" onclick={() => changeMode('reset')} disabled={busy}>{text('重置密码', 'Reset password')}</button>{/if}
              </div>
              <div class="password-field">
                <input id="auth-password" type={showPassword ? 'text' : 'password'} autocomplete={mode === 'login' ? 'current-password' : 'new-password'} bind:value={password} bind:this={passwordInput} disabled={busy} required placeholder={text(mode === 'login' ? '输入你的密码' : '设置一个安全的密码', mode === 'login' ? 'Your password' : 'Create a secure password')} aria-describedby={mode !== 'login' ? 'password-help' : undefined} />
                <button class="password-toggle" type="button" aria-label={showPassword ? text('隐藏密码', 'Hide password') : text('显示密码', 'Show password')} aria-pressed={showPassword} disabled={busy} onclick={() => showPassword = !showPassword}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>{#if !showPassword}<path d="m3 3 18 18"/>{/if}</svg>
                </button>
              </div>
              {#if mode !== 'login'}<p class="help" id="password-help">{text('至少 6 个字符，仅限英文字母、数字和英文符号，不含空格。', 'At least 6 characters: English letters, numbers and symbols, without spaces.')}</p>{/if}
            </div>
            {#if mode === 'setup'}
              <div class="field">
                <label for="setup-token">{text('初始化密钥', 'Setup token')}</label>
                <input id="setup-token" type="password" autocomplete="off" bind:value={setupToken} required disabled={busy} />
              </div>
            {/if}
          {/if}
          {#if error || oauthError}<p class="error" role="alert">{error || $t(oauthError)}</p>{/if}
          <button class="primary" type="submit" disabled={busy || loading}>
            {loading ? text('正在加载…', 'Loading…') : busy ? text('请稍候…', 'Please wait…') : mode === 'reset' ? text('发送重置邮件', 'Send reset email') : step === 'details' ? text('使用邮箱继续', 'Continue with email') : mode === 'setup' ? text('创建管理员', 'Create administrator') : mode === 'register' ? text('继续', 'Continue') : text('登录', 'Sign in')}
          </button>
        </form>

        {#if step === 'details' && (mode === 'login' || mode === 'register') && (config.google_oauth || config.github_oauth)}
          <div class="divider"><span>{text('或', 'or')}</span></div>
          <div class="providers">
            {#if config.google_oauth}<a class="provider" href={authApi.googleLoginUrl()} data-sveltekit-reload>{@render GoogleIcon()}{text('使用 Google 继续', 'Continue with Google')}</a>{/if}
            {#if config.github_oauth}<a class="provider" href={authApi.githubLoginUrl()} data-sveltekit-reload>{@render GithubIcon()}{text('使用 GitHub 继续', 'Continue with GitHub')}</a>{/if}
          </div>
        {/if}
        {#if mode === 'login' && options.registration_enabled}
          <p class="card-footer">{text('还没有账号？', 'Don’t have an account?')} <button class="text-button" disabled={busy} onclick={() => changeMode('register')}>{text('注册', 'Sign up')}</button></p>
        {:else if mode === 'register'}
          <p class="card-footer">{text('已有账号？', 'Already have an account?')} <button class="text-button" disabled={busy} onclick={() => changeMode('login')}>{text('登录', 'Sign in')}</button></p>
        {/if}
      {/if}
    </section>

    {#if step !== 'sent' && (step === 'password' || mode === 'reset' || mode === 'setup')}
      <button class="back text-button muted" disabled={busy} onclick={() => mode === 'reset' || mode === 'setup' ? changeMode('login') : changeEmail()}><span aria-hidden="true">‹</span> {text('更改方式', 'Change method')}</button>
    {/if}
    {#if mode === 'login' && step === 'details' && !loading && !configError}
      <div class="operator-links">
        {#if options.setup_available}<button class="text-button muted" onclick={() => changeMode('setup')}>{text('初始化管理员', 'Initialize administrator')}</button>{/if}
        {#if config.dev_mode}<a class="text-button muted" href={authApi.devLoginUrl()} data-sveltekit-reload>{text('开发环境登录', 'Dev login')}</a>{/if}
      </div>
    {/if}
    <p class="legal">{$t('signin.legalPrefix')} <a href="/terms">{$t('signin.terms')}</a> {$t('signin.legalAnd')} <a href="/privacy">{$t('signin.privacy')}</a></p>
  </div>
</div>

<style>
  .auth-page {
    --auth-bg: #1b1d19;
    --auth-card: #242620;
    --auth-border: #3b3e36;
    --auth-text: #eeefe8;
    --auth-muted: #b2b5ab;
    --auth-link: #b4a7f5;
    min-height: 100svh;
    background: var(--auth-bg);
    color: var(--auth-text);
    padding: 48px 24px 28px;
  }
  .auth-content { width: 100%; max-width: 480px; margin: 0 auto; text-align: center; }
  .brand { display: inline-flex; margin-bottom: 30px; }
  .brand img { width: 92px; height: auto; filter: brightness(0) invert(1); }
  h1 { font-size: clamp(28px, 4vw, 34px); font-weight: 450; letter-spacing: -.025em; margin: 0 0 30px; }
  .auth-card { padding: 40px; border: 1px solid var(--auth-border); border-radius: 20px; background: var(--auth-card); text-align: left; }
  form, .field, .providers { display: flex; flex-direction: column; }
  form { gap: 24px; }
  .field { gap: 9px; }
  label, .label-row { font-size: 15px; }
  .label-row { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
  .optional { margin-left: 6px; color: var(--auth-muted); font-size: 12px; }
  input { width: 100%; min-width: 0; min-height: 50px; padding: 11px 14px; border: 1px solid #66695f; border-radius: 8px; background: var(--auth-bg); color: var(--auth-text); font: inherit; font-size: 16px; line-height: 1.5; box-shadow: none; }
  input::placeholder { color: #93988b; }
  input:read-only { background: #363a30; border-color: transparent; color: #c4c8bd; }
  input:focus-visible { outline: 2px solid #9c87eb; outline-offset: 1px; border-color: #9c87eb; }
  button, a { -webkit-tap-highlight-color: transparent; }
  button { cursor: pointer; }
  button:disabled { cursor: wait; opacity: .55; }
  button:focus-visible, a:focus-visible { outline: 2px solid #b4a7f5; outline-offset: 4px; }
  .primary, .provider { width: 100%; min-height: 50px; padding: 11px 16px; border-radius: 8px; font: inherit; font-size: 16px; font-weight: 500; display: flex; align-items: center; justify-content: center; gap: 12px; text-align: center; }
  .primary { background: #f2f9ed; color: #20251c; border: 1px solid transparent; margin-top: 3px; }
  .primary:hover:not(:disabled) { background: #e1edd8; }
  .provider { border: 1px solid #66695f; color: var(--auth-text); background: transparent; text-decoration: none; }
  .provider:hover { border-color: #999e90; background: #2d3028; }
  .providers { gap: 12px; }
  .oauth-icon { width: 20px; height: 20px; flex-shrink: 0; }
  .divider { display: flex; align-items: center; gap: 16px; margin: 28px 0; color: var(--auth-muted); font-size: 14px; }
  .divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: #50544a; }
  .text-button { border: 0; padding: 0; background: transparent; font: inherit; color: var(--auth-link); text-decoration: none; }
  .text-button:hover:not(:disabled), .legal a:hover { color: #ddd3ff; text-decoration: underline; }
  .muted { color: var(--auth-muted); }
  .card-footer { margin: 28px 0 0; color: var(--auth-muted); font-size: 14px; text-align: center; }
  .card-footer button { margin-left: 5px; }
  .password-field { position: relative; }
  .password-field input { padding-right: 50px; }
  .password-toggle { position: absolute; right: 3px; top: 3px; width: 44px; height: 44px; border: 0; border-radius: 6px; background: transparent; color: var(--auth-muted); display: grid; place-items: center; }
  .password-toggle:hover { color: var(--auth-text); }
  .password-toggle svg { width: 20px; height: 20px; }
  .help { color: var(--auth-muted); font-size: 13px; line-height: 1.6; margin: 0; }
  .error { color: #ffa09c; margin: 0; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
  .error + .primary { margin-top: 24px; }
  .sent-icon { width: 52px; height: 52px; margin: 0 auto 24px; color: var(--auth-link); }
  .sent-icon svg { width: 100%; height: 100%; }
  .sent-message { text-align: center; color: var(--auth-muted); font-size: 15px; }
  .sent-message strong { display: block; margin-bottom: 16px; color: var(--auth-text); overflow-wrap: anywhere; }
  .back { margin-top: 24px; font-size: 15px; }
  .back span { font-size: 23px; margin-right: 6px; vertical-align: -1px; }
  .operator-links { display: flex; justify-content: center; gap: 20px; flex-wrap: wrap; margin-top: 24px; font-size: 13px; }
  .legal { margin: 36px auto 0; max-width: 380px; color: var(--auth-muted); font-size: 12px; line-height: 1.8; }
  .legal a { color: var(--auth-muted); text-decoration: underline; text-underline-offset: 3px; }
  @media (max-width: 520px) {
    .auth-page { padding: 32px 20px 24px; }
    .brand { margin-bottom: 24px; }
    h1 { margin-bottom: 26px; }
    .auth-card { padding: 28px 24px; border-radius: 16px; }
    .label-row { font-size: 14px; }
    .legal { margin-top: 28px; }
  }
</style>
