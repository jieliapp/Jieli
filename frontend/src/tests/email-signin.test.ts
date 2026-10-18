// @vitest-environment jsdom
// @ts-expect-error jsdom needs the Svelte client runtime.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import SignIn from '../routes/web/auth/signin/+page.svelte';
import { ApiError } from '$lib/api/client';
import { locale } from '$lib/stores/locale';
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
// @ts-expect-error onMount must execute in the client runtime.
vi.mock('svelte', async () => await import('../../node_modules/svelte/src/index-client.js'));
vi.mock('$lib/api/client', async (importOriginal) => ({ ...await importOriginal<typeof import('$lib/api/client')>(), api: mocks }));
let component: ReturnType<typeof mount>;
beforeEach(() => {
 locale.set('en');
 mocks.get.mockReset(); mocks.post.mockReset();
 mocks.get.mockImplementation(async (path: string) => path === '/auth/options'
 ? { registration_enabled: true, recovery_enabled: true, setup_available: false }
 : { dev_mode: false, github_oauth: true, google_oauth: false });
});
afterEach(async () => { if (component) await unmount(component); document.body.innerHTML = ''; });
const button = (text: string) => [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === text)!;
function fill(selector: string, value: string) { const input = document.querySelector<HTMLInputElement>(selector)!; input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); }
const submit = () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
async function open(continueLabel = 'Continue with email') { component = mount(SignIn, { target: document.body }); await vi.waitFor(() => expect(button(continueLabel)?.disabled).toBe(false)); }

it('advances email locally, supports password visibility and clears credentials when changing email', async () => {
 await open();
 expect(document.querySelector('input[type="password"]')).toBeNull();
 const initialGets = mocks.get.mock.calls.map(call => [...call]);
 fill('input[type="email"]', 'alice@example.com'); submit();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="current-password"]')).not.toBeNull());
 expect(mocks.post).not.toHaveBeenCalled();
 expect(mocks.get.mock.calls).toEqual(initialGets);
 expect(document.querySelector<HTMLInputElement>('input[type="email"]')?.readOnly).toBe(true);
 fill('input[autocomplete="current-password"]', 'secret-password');
 document.querySelector<HTMLButtonElement>('[aria-label="Show password"]')!.click();
 await vi.waitFor(() => expect(document.querySelector<HTMLInputElement>('input[autocomplete="current-password"]')!.type).toBe('text'));
 button('Change email').click();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="current-password"]')).toBeNull());
 expect(document.querySelector<HTMLInputElement>('input[type="email"]')!.value).toBe('alice@example.com');
 submit();
 await vi.waitFor(() => expect(document.querySelector<HTMLInputElement>('input[autocomplete="current-password"]')!.value).toBe(''));
});

it('submits registration name and password only on the second step then shows email confirmation', async () => {
 await open(); button('Sign up').click();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="nickname"]')).not.toBeNull());
 fill('input[autocomplete="nickname"]', 'Alice'); fill('input[type="email"]', 'alice@example.com'); submit();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="new-password"]')).not.toBeNull());
 expect(mocks.post).not.toHaveBeenCalled();
 mocks.post.mockResolvedValue({ sent: true });
 fill('input[autocomplete="new-password"]', 'secure-password'); submit();
 await vi.waitFor(() => expect(mocks.post).toHaveBeenCalledWith('/auth/register', { email: 'alice@example.com', password: 'secure-password', display_name: 'Alice' }));
 await vi.waitFor(() => expect(document.querySelector('[role="status"]')?.textContent).toContain('alice@example.com'));
 expect(document.querySelector('input[type="password"]')).toBeNull();
});

it('preserves the email during recovery and displays login errors without losing the step', async () => {
 await open(); fill('input[type="email"]', 'alice@example.com'); submit();
 await vi.waitFor(() => expect(button('Reset password')).toBeDefined());
 mocks.post.mockRejectedValueOnce(new Error('Invalid credentials'));
 fill('input[autocomplete="current-password"]', 'wrong-password'); submit();
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toContain('Invalid credentials'));
 button('Reset password').click();
 await vi.waitFor(() => expect(button('Send reset email')).toBeDefined());
 expect(document.querySelector<HTMLInputElement>('input[type="email"]')!.value).toBe('alice@example.com');
 mocks.post.mockResolvedValueOnce({ sent: true }); submit();
 await vi.waitFor(() => expect(mocks.post).toHaveBeenLastCalledWith('/auth/reset', { email: 'alice@example.com' }));
});

it.each([['zh', '邮箱或密码不正确。'], ['en', 'Incorrect email or password.']] as const)('localizes incorrect credentials in %s and preserves the password step', async (language, message) => {
 locale.set(language);
 await open(language === 'zh' ? '使用邮箱继续' : 'Continue with email');
 fill('input[type="email"]', 'alice@example.com'); submit();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="current-password"]')).not.toBeNull());
 mocks.post.mockRejectedValueOnce(new ApiError('invalid credentials', 401));
 fill('input[autocomplete="current-password"]', 'wrong-password'); submit();
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toBe(message));
 expect(mocks.post).toHaveBeenCalledWith('/auth/login', { email: 'alice@example.com', password: 'wrong-password' });
 expect(document.querySelector<HTMLInputElement>('input[type="email"]')?.value).toBe('alice@example.com');
 expect(document.querySelector<HTMLInputElement>('input[type="email"]')?.readOnly).toBe(true);
 expect(document.querySelector<HTMLInputElement>('input[autocomplete="current-password"]')?.value).toBe('wrong-password');
 expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(false);
});

it('honors registration, recovery, setup and provider availability', async () => {
 mocks.get.mockImplementation(async (path: string) => path === '/auth/options'
 ? { registration_enabled: false, recovery_enabled: false, setup_available: false }
 : { dev_mode: false, github_oauth: true, google_oauth: false });
 await open();
 expect(button('Sign up')).toBeUndefined(); expect(button('Initialize administrator')).toBeUndefined();
 expect(document.querySelector('a[href="/web/api/auth/github"]')).not.toBeNull();
 expect(document.querySelector('a[href="/web/api/auth/google"]')).toBeNull();
 fill('input[type="email"]', 'alice@example.com'); submit();
 await vi.waitFor(() => expect(document.querySelector('input[type="password"]')).not.toBeNull());
 expect(button('Reset password')).toBeUndefined();
});

it.each([['zh', '创建管理员', '两次输入的密码不一致。'], ['en', 'Create administrator', 'Passwords do not match.']] as const)('opens the first-install form directly and requires matching passwords in %s', async (language, label, mismatch) => {
 locale.set(language);
 mocks.get.mockImplementation(async (path: string) => path === '/auth/options'
 ? { registration_enabled: false, recovery_enabled: false, setup_available: true, setup_requires_token: false }
 : { dev_mode: false, github_oauth: true, google_oauth: false });
 await open(label);
 expect(document.querySelector('#setup-token')).toBeNull();
 expect(document.querySelector('a[href="/web/api/auth/github"]')).toBeNull();
 expect(button('Sign up')).toBeUndefined();
 fill('#auth-email', 'owner@example.com');
 fill('#auth-password', 'owner-password-value');
 fill('#auth-password-confirm', 'different-password'); submit();
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toBe(mismatch));
 expect(mocks.post).not.toHaveBeenCalled();
 fill('#auth-password-confirm', 'owner-password-value');
 mocks.post.mockRejectedValueOnce(new Error('Administrator already initialized')); submit();
 await vi.waitFor(() => expect(mocks.post).toHaveBeenCalledWith('/auth/setup', { email: 'owner@example.com', password: 'owner-password-value', password_confirm: 'owner-password-value' }));
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toContain('Administrator already initialized'));
 expect(document.querySelector('[role="status"]')).toBeNull();
 expect(document.querySelector<HTMLInputElement>('#auth-email')!.readOnly).toBe(false);
});

it('includes the operator token when the server requires protected setup', async () => {
 mocks.get.mockImplementation(async (path: string) => path === '/auth/options'
 ? { registration_enabled: false, recovery_enabled: false, setup_available: true, setup_requires_token: true }
 : { dev_mode: false, github_oauth: false, google_oauth: false });
 await open('Create administrator');
 fill('#auth-email', 'owner@example.com');
 fill('#auth-password', 'owner-password-value');
 fill('#auth-password-confirm', 'owner-password-value');
 fill('#setup-token', 'operator-token');
 mocks.post.mockRejectedValueOnce(new Error('Setup unavailable')); submit();
 await vi.waitFor(() => expect(mocks.post).toHaveBeenCalledWith('/auth/setup', { email: 'owner@example.com', password: 'owner-password-value', password_confirm: 'owner-password-value', token: 'operator-token' }));
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toContain('Setup unavailable'));
});

it('blocks duplicate registration while sending and rejects oversized multibyte passwords locally', async () => {
 await open(); button('Sign up').click();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="nickname"]')).not.toBeNull());
 fill('input[type="email"]', 'alice@example.com'); submit();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="new-password"]')).not.toBeNull());
 fill('input[autocomplete="new-password"]', '😀'.repeat(20)); submit();
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toContain('6–72 characters'));
 expect(mocks.post).not.toHaveBeenCalled();
 let complete!: (value: unknown) => void;
 mocks.post.mockImplementationOnce(() => new Promise(resolve => { complete = resolve; }));
 fill('input[autocomplete="new-password"]', 'secure-password'); submit(); submit();
 await vi.waitFor(() => expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true));
 expect(mocks.post).toHaveBeenCalledTimes(1);
 complete({ sent: true });
 await vi.waitFor(() => expect(document.querySelector('[role="status"]')).not.toBeNull());
});


it('accepts 80 Unicode display-name characters and a six-character ASCII password through native form submission', async () => {
 await open(); button('Sign up').click();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="nickname"]')).not.toBeNull());
 fill('input[autocomplete="nickname"]', '😀'.repeat(81)); fill('input[type="email"]', 'alice@example.com');
 document.querySelector<HTMLFormElement>('form')!.requestSubmit();
 await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toContain('80 characters'));
 fill('input[autocomplete="nickname"]', '😀'.repeat(80));
 document.querySelector<HTMLFormElement>('form')!.requestSubmit();
 await vi.waitFor(() => expect(document.querySelector('input[autocomplete="new-password"]')).not.toBeNull());
 fill('input[autocomplete="new-password"]', 'Ab1!xy');
 mocks.post.mockResolvedValue({ sent: true });
 document.querySelector<HTMLFormElement>('form')!.requestSubmit();
 await vi.waitFor(() => expect(mocks.post).toHaveBeenCalledWith('/auth/register', { email: 'alice@example.com', display_name: '😀'.repeat(80), password: 'Ab1!xy' }));
});
