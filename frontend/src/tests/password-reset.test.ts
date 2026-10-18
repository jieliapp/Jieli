// @vitest-environment jsdom
// @ts-expect-error jsdom uses the client runtime.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, expect, it, vi } from 'vitest';
import PasswordPage from '../routes/web/auth/password/+page.svelte';
const calls = vi.hoisted(() => ({ post: vi.fn(async () => ({})), refresh: vi.fn(async () => {}) }));
vi.mock('$lib/api/client', () => ({ api: { post: calls.post } }));
vi.mock('$lib/stores/session', () => ({ loadSession: calls.refresh }));
import PasswordReset from '$lib/components/auth/PasswordReset.svelte';
let component: ReturnType<typeof mount>;
afterEach(async () => { if (component) await unmount(component); document.body.innerHTML = ''; });
function fill(value: string) {
  document.querySelectorAll<HTMLInputElement>('input').forEach(input => {
    input.value = value; input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}
function submit() { document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })); }
it('rejects non-ASCII and accepts a matching six-character password and clears the form after saving', async () => {
  const save = vi.fn(async () => {});
  component = mount(PasswordReset, { target: document.body, props: { submitPassword: save, signedIn: true } });
  await vi.waitFor(() => expect(document.querySelectorAll('input').length).toBe(2));
  expect(document.querySelector('[autocomplete="current-password"]')).toBeNull();
  fill('short'); submit();
  await vi.waitFor(() => expect(document.querySelector('[role="alert"]')).not.toBeNull());
  expect(save).not.toHaveBeenCalled();
  fill('密'.repeat(25)); submit(); expect(save).not.toHaveBeenCalled();
  fill('Ab1!xy'); submit();
  await vi.waitFor(() => expect(save).toHaveBeenCalledWith('Ab1!xy'));
  await vi.waitFor(() => expect(document.querySelector('form')).toBeNull());
  expect(document.querySelector('[role="status"]')).not.toBeNull();
  expect(document.querySelector('a')?.getAttribute('href')).toBe('/web/settings#security');
});
it('keeps visibility controls independent and prevents submissions without a reset credential', async () => {
  const save = vi.fn();
  component = mount(PasswordReset, { target: document.body, props: { submitPassword: save, enabled: false } });
  await vi.waitFor(() => expect(document.querySelectorAll('input').length).toBe(2));
  (document.querySelector('button[type="button"]') as HTMLButtonElement).click();
  await vi.waitFor(() => expect(document.querySelector('input')?.type).toBe('text'));
  expect(document.querySelectorAll('input')[1].type).toBe('password');
  fill('replacement-password'); submit();
  expect(save).not.toHaveBeenCalled();
});

it('submits the session password page without an old password and refreshes session', async () => {
  component = mount(PasswordPage, { target: document.body });
  await vi.waitFor(() => expect(document.querySelectorAll('input').length).toBe(2));
  fill('replacement-password'); submit();
  await vi.waitFor(() => expect(calls.post).toHaveBeenCalledWith('/auth/password', { password: 'replacement-password' }));
  await vi.waitFor(() => expect(calls.refresh).toHaveBeenCalled());
  await vi.waitFor(() => expect(document.querySelector('form')).toBeNull());
});
