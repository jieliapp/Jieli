<script lang="ts">
  import PasswordReset from '$lib/components/auth/PasswordReset.svelte';
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { loadSession } from '$lib/stores/session';
  import { locale } from '$lib/stores/locale';

  let token = $state('');
  let reset = $state(false);
  let busy = $state(false);
  let done = $state(false);
  let error = $state('');
  const text = (zh: string, en: string) => $locale === 'zh' ? zh : en;
  const title = $derived(done ? text('邮箱已验证', 'Email verified') : text('验证邮箱', 'Verify your email'));

  onMount(() => {
    token = window.location.hash.slice(1);
    reset = new URLSearchParams(window.location.search).get('purpose') === 'reset';
    history.replaceState(null, '', window.location.pathname + window.location.search);
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    try {
      await api.post('/auth/email/complete', { token, password: '' });
      done = true;
      token = '';
    } catch (e) {
      error = e instanceof Error ? e.message : text('验证失败，请重试。', 'Verification failed. Please try again.');
    } finally {
      busy = false;
    }
  }
</script>
<svelte:head>
  {#if !reset}<title>{title} · Jieli</title>{/if}
</svelte:head>
{#if reset}
 <PasswordReset enabled={Boolean(token)} submitPassword={async (newPassword) => { await api.post('/auth/email/complete', { token, password: newPassword }); token = ''; await loadSession(); }} />
{:else}
  <div class="auth-page">
    <div class="auth-content">
      <div class="brand"><img src="/brand/logo-240.png" alt="Jieli" /></div>
      <h1>{title}</h1>

      <section class="auth-card" aria-label={title} aria-busy={busy}>
        <div class="status-icon" class:verified={done} aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            {#if done}
              <path d="m6 12 4 4 8-8" />
            {:else}
              <rect x="3" y="5" width="18" height="14" rx="2" />
              <path d="m3 7 9 6 9-6" />
            {/if}
          </svg>
        </div>

        {#if done}
          <div class="message" role="status">
            <h2>{text('欢迎加入 Jieli', 'Welcome to Jieli')}</h2>
            <p>{text('邮箱验证成功。现在可以使用你的邮箱和密码登录了。', 'Your email is verified. You can now sign in with your email and password.')}</p>
          </div>
          <a class="primary" href="/web/auth/signin">{text('前往登录', 'Continue to sign in')}</a>
        {:else}
          <div class="message">
            <h2>{text('只差最后一步', 'One last step')}</h2>
            <p>{text('点击下方按钮确认你的邮箱，完成 Jieli 账号注册。', 'Confirm your email below to finish creating your Jieli account.')}</p>
          </div>
          <form onsubmit={submit}>
            {#if error}<p class="error" role="alert">{error}</p>{/if}
            {#if !token}<p class="error" role="alert">{text('验证链接无效，请使用邮件中的完整链接。', 'Invalid verification link. Open the complete link from your email.')}</p>{/if}
            <button class="primary" disabled={busy || !token} type="submit">{busy ? text('正在验证…', 'Verifying…') : text('验证邮箱', 'Verify email')}</button>
          </form>
          <p class="hint">{text('验证完成后，即可登录你的账号。', 'Once verified, you’re ready to sign in.')}</p>
        {/if}
      </section>

      {#if !done}<a class="back" href="/web/auth/signin"><span aria-hidden="true">‹</span> {text('返回登录', 'Back to sign in')}</a>{/if}
    </div>
  </div>
{/if}

<style>
  .auth-page { min-height: 100svh; padding: 48px 24px 28px; background: #1b1d19; color: #eeefe8; }
  .auth-content { width: 100%; max-width: 480px; margin: 0 auto; text-align: center; }
  .brand { margin-bottom: 30px; }
  .brand img { width: 92px; height: auto; filter: brightness(0) invert(1); }
  h1 { font-size: clamp(28px, 4vw, 34px); font-weight: 450; letter-spacing: -.025em; margin: 0 0 30px; }
  .auth-card { padding: 40px; border: 1px solid #3b3e36; border-radius: 20px; background: #242620; }
  .status-icon { display: grid; place-items: center; width: 64px; height: 64px; margin: 0 auto 24px; border: 1px solid #4b455f; border-radius: 50%; background: #302c3d; color: #b4a7f5; }
  .status-icon.verified { border-color: #45533b; background: #2d3827; color: #b9d5a6; }
  .status-icon svg { width: 30px; height: 30px; }
  h2 { margin: 0 0 12px; font-size: 20px; font-weight: 500; }
  .message p { margin: 0; color: #b2b5ab; font-size: 15px; line-height: 1.8; }
  form { display: flex; flex-direction: column; gap: 20px; margin-top: 28px; }
  .primary { display: flex; align-items: center; justify-content: center; width: 100%; min-height: 50px; padding: 11px 16px; border: 0; border-radius: 8px; background: #f2f9ed; color: #20251c; font: inherit; font-size: 16px; font-weight: 500; text-decoration: none; cursor: pointer; }
  a.primary { margin-top: 28px; }
  .primary:hover:not(:disabled) { background: #e1edd8; }
  .primary:disabled { opacity: .55; cursor: not-allowed; }
  [aria-busy="true"] .primary { cursor: wait; }
  .primary:focus-visible, .back:focus-visible { outline: 2px solid #b4a7f5; outline-offset: 4px; }
  .hint { margin: 20px 0 0; color: #b2b5ab; font-size: 13px; line-height: 1.6; }
  .error { margin: 0; color: #f0afa7; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
  .back { display: inline-block; margin-top: 30px; color: #b2b5ab; font-size: 15px; text-decoration: none; }
  .back span { margin-right: 6px; font-size: 23px; vertical-align: -1px; }
  .back:hover { color: #eeefe8; }
  @media (max-width: 520px) {
    .auth-page { padding: 32px 16px; }
    .auth-card { padding: 28px 24px; border-radius: 16px; }
  }
</style>
