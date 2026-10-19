<script lang="ts">
  import { locale } from '$lib/stores/locale';
  let { submitPassword, enabled = true, signedIn = false }: {
    submitPassword: (password: string) => Promise<void>;
    enabled?: boolean;
    signedIn?: boolean;
  } = $props();
  let password = $state('');
  let confirmation = $state('');
  let visible = $state(false);
  let confirmationVisible = $state(false);
  let busy = $state(false);
  let error = $state('');
  let done = $state(false);
  import { onMount } from 'svelte';
  onMount(() => {
    const handleHistory = () => {
      if (!window.location.pathname.startsWith('/web/auth/')) window.location.reload();
    };
    window.addEventListener('popstate', handleHistory);
    return () => window.removeEventListener('popstate', handleHistory);
  });
  const text = (zh: string, en: string) => $locale === 'zh' ? zh : en;
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy || !enabled) return;
    error = '';
    if (password !== confirmation) { error = text('两次输入的密码不一致。', 'Passwords do not match.'); return; }

    if (!/^[\x21-\x7e]{6,72}$/.test(password)) { error = text('密码须为 6–72 个字符，仅限英文字母、数字和英文符号，不含空格。', 'Use 6–72 characters: English letters, numbers and symbols, without spaces.'); return; }
    busy = true;
    try {
      await submitPassword(password);
      password = ''; confirmation = ''; done = true;
    } catch (e) { error = e instanceof Error ? e.message : text('修改失败，请重试。', 'Update failed. Please try again.'); }
    finally { busy = false; }
  }
</script>
<svelte:head><title>{text('重置密码', 'Reset password')} · Jieli</title></svelte:head>
<div class="auth-page">
  <div class="auth-content">
    <div class="brand"><img src="/brand/logo-240.png" alt="Jieli" /></div>
    <h1>{text('重置密码', 'Reset password')}</h1>
    <div class="auth-card">
      {#if done}
        <p role="status">{text('密码已更新。', 'Password updated.')}</p>
      {:else}
        <form onsubmit={submit}>
          <div class="field">
            <label for="new-password">{text('新密码', 'New password')}</label>
            <div class="password-field">
              <input id="new-password" type={visible ? 'text' : 'password'} bind:value={password} autocomplete="new-password" required disabled={busy} placeholder={text('创建新密码', 'Create a new password')} aria-describedby="password-help" />
              <button class="toggle" type="button" aria-label={text('显示或隐藏新密码', 'Show or hide new password')} aria-pressed={visible} onclick={() => visible = !visible}>◉</button>
            </div>
            <small id="password-help">{text('至少 6 个字符，仅限英文字母、数字和英文符号，不含空格。', 'At least 6 characters: English letters, numbers and symbols, without spaces.')}</small>
          </div>
          <div class="field">
            <label for="confirm-password">{text('确认新密码', 'Confirm new password')}</label>
            <div class="password-field">
              <input id="confirm-password" type={confirmationVisible ? 'text' : 'password'} bind:value={confirmation} autocomplete="new-password" required disabled={busy} placeholder={text('请再次输入您的新密码', 'Enter your new password again')} />
              <button class="toggle" type="button" aria-label={text('显示或隐藏确认密码', 'Show or hide password confirmation')} aria-pressed={confirmationVisible} onclick={() => confirmationVisible = !confirmationVisible}>◉</button>
            </div>
          </div>
          {#if error}<p role="alert">{error}</p>{/if}
          {#if !enabled}<p role="alert">{text('重置链接无效，请重新申请。', 'Invalid reset link. Request a new one.')}</p>{/if}
          <button class="primary" type="submit" disabled={busy || !enabled}>{text(busy ? '正在保存…' : '继续', busy ? 'Saving…' : 'Continue')}</button>
        </form>
      {/if}
    </div>
    <a class="back" href={signedIn ? '/web/settings#security' : '/web/auth/signin'}>‹ {signedIn ? text('返回', 'Back') : text('返回登录', 'Back to sign in')}</a>
  </div>
</div>
<style>
  .auth-page { min-height:100svh; padding:48px 24px 28px; background:#1b1d19; color:#eeefe8; }
  .auth-content { width:100%; max-width:480px; margin:0 auto; text-align:center; }
  .brand { margin-bottom:30px; }
  .brand img { width:92px; height:auto; filter:brightness(0) invert(1); }
  h1 { font-size:clamp(28px,4vw,34px); font-weight:450; margin:0 0 30px; }
  .auth-card { padding:40px; border:1px solid #3b3e36; border-radius:20px; background:#242620; text-align:left; }
  form,.field { display:flex; flex-direction:column; gap:24px; }
  .field { gap:9px; }
  label { font-size:15px; }
  .password-field { position:relative; }
  input { width:100%; min-width:0; min-height:50px; padding:11px 48px 11px 14px; border:1px solid #66695f; border-radius:8px; background:#1b1d19; color:#eeefe8; font:inherit; font-size:16px; }
  input:focus-visible { outline:2px solid #9c87eb; outline-offset:1px; }
  button { cursor:pointer; font:inherit; }
  button:disabled { opacity:.55; cursor:wait; }
  .toggle { position:absolute; right:8px; top:5px; height:40px; width:36px; background:transparent; border:0; color:#eeefe8; }
  .primary { min-height:50px; background:#f2f9ed; color:#20251c; border:0; border-radius:8px; font-size:16px; }
  small,.back { color:#b2b5ab; }
  .back { display:inline-block; margin-top:30px; text-decoration:none; }
  [role="alert"] { color:#f0afa7; margin:0; }
  @media(max-width:520px) { .auth-page { padding:32px 16px; } .auth-card { padding:28px 24px; } }
</style>
