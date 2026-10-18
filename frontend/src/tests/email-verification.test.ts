// @vitest-environment jsdom
// @ts-expect-error jsdom needs Svelte's client mount API.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import EmailVerification from '../routes/web/auth/email/+page.svelte';

const api = vi.hoisted(() => ({ post: vi.fn() }));
const refresh = vi.hoisted(() => vi.fn(async () => {}));
vi.mock('$lib/stores/session', () => ({ loadSession: refresh }));
// @ts-expect-error onMount must use the client runtime in jsdom.
vi.mock('svelte', async () => await import('../../node_modules/svelte/src/index-client.js'));
vi.mock('$lib/api/client', () => ({ api }));

let component: ReturnType<typeof mount>;
beforeEach(() => { api.post.mockReset(); refresh.mockClear(); });
afterEach(async () => {
  if (component) await unmount(component);
  document.body.innerHTML = '';
  history.replaceState(null, '', '/');
});

describe('email verification links', () => {
  it('removes the token from the address bar and submits it only on confirmation', async () => {
    history.replaceState(null, '', '/web/auth/email?purpose=register#one-time-token');
    api.post.mockResolvedValue({ verified: true });
    component = mount(EmailVerification, { target: document.body });
    await vi.waitFor(() => expect(location.hash).toBe(''));
    expect(api.post).not.toHaveBeenCalled();
    document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await vi.waitFor(() => expect(api.post).toHaveBeenCalledWith('/auth/email/complete', { token: 'one-time-token', password: '' }));
    await vi.waitFor(() => expect(document.querySelector('[role="status"]')).not.toBeNull());
    expect(document.querySelector('a')?.getAttribute('href')).toBe('/web/auth/signin');
    expect(document.querySelector('form')).toBeNull();
  });

  it('sends the new reset password and displays a rejected or expired link error', async () => {
    history.replaceState(null, '', '/web/auth/email?purpose=reset#reset-token');
    api.post.mockRejectedValue(new Error('Verification link expired'));
    component = mount(EmailVerification, { target: document.body });
    await vi.waitFor(() => expect(document.querySelector('input[type="password"]')).not.toBeNull());
    for (const input of document.querySelectorAll<HTMLInputElement>('input')) {
      input.value = 'replacement-password';
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }
    document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await vi.waitFor(() => expect(api.post).toHaveBeenCalledWith('/auth/email/complete', { token: 'reset-token', password: 'replacement-password' }));
    await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toBe('Verification link expired'));
    expect(location.hash).toBe('');
    expect(document.querySelector('form')).not.toBeNull();
  });
});

it('rejects mismatched confirmation without consuming the reset token', async () => {
  history.replaceState(null, '', '/web/auth/email?purpose=reset#reset-token');
  component = mount(EmailVerification, { target: document.body });
  await vi.waitFor(() => expect(document.querySelectorAll('input').length).toBe(2));
  const fields = document.querySelectorAll<HTMLInputElement>('input');
  fields[0].value = 'replacement-password'; fields[1].value = 'different-password';
  fields.forEach(input => input.dispatchEvent(new Event('input', { bubbles: true })));
  document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  await vi.waitFor(() => expect(document.querySelector('[role="alert"]')).not.toBeNull());
  expect(api.post).not.toHaveBeenCalled();
});

it('completes email reset and refreshes stale session state', async () => {
  history.replaceState(null, '', '/web/auth/email?purpose=reset#reset-token');
  api.post.mockResolvedValue({ verified: true });
  component = mount(EmailVerification, { target: document.body });
  await vi.waitFor(() => expect(document.querySelectorAll('input').length).toBe(2));
  document.querySelectorAll<HTMLInputElement>('input').forEach(input => { input.value = 'replacement-password'; input.dispatchEvent(new Event('input', { bubbles: true })); });
  document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  await vi.waitFor(() => expect(refresh).toHaveBeenCalled());
  await vi.waitFor(() => expect(document.querySelector('form')).toBeNull());
  expect(document.querySelector('a')?.getAttribute('href')).toBe('/web/auth/signin');
});
