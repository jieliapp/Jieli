// @vitest-environment jsdom
// @ts-expect-error use Svelte client runtime in jsdom
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { writable } from 'svelte/store';
import TopBar from '$lib/components/layout/TopBar.svelte';
import Admin from '../routes/web/admin/+page.svelte';
import { locale } from '$lib/stores/locale';
import { session } from '$lib/stores/session';
import type { User } from '$lib/api/types';

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
// @ts-expect-error client onMount
vi.mock('svelte', async () => await import('../../node_modules/svelte/src/index-client.js'));
vi.mock('$lib/api/client', async (original) => ({ ...await original<typeof import('$lib/api/client')>(), api }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/stores', () => ({ page: writable({ url: new URL('http://localhost/web/admin') }) }));

let component: ReturnType<typeof mount> | undefined;
let admin: boolean;
let cloud: boolean;
beforeEach(() => {
  vi.resetAllMocks(); locale.set('en'); admin = true; cloud = false;
  session.set({ user: { username: 'operator', is_admin: true } as User, loaded: true, loading: false });
  api.get.mockImplementation(async (path: string) => {
    if (path === '/auth/me') return { username: 'operator', is_admin: admin };
    if (path === '/config') return { cloud_billing: cloud };
    if (path === '/admin/users') return [{ id: 1, email: 'user@example.com', username: 'tester', display_name: 'Tester Name' }];
    if (path === '/admin/site-settings') return { public_base_url: '', tested: false, registration_enabled: false };
    if (path === '/admin/membership-codes') return [];
    throw new Error(`Unexpected GET ${path}`);
  });
  api.post.mockResolvedValue({});
});
afterEach(async () => {
  if (component) await unmount(component);
  component = undefined;
  document.body.innerHTML = ''; session.set({ user: null, loaded: false, loading: false });
});
const button = (label: string) => [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === label)!;
const navigate = async (id: string) => {
  document.querySelector<HTMLButtonElement>(`nav button[data-section="${id}"]`)!.click();
  await vi.waitFor(() => expect(document.querySelector(`nav button[data-section="${id}"]`)?.getAttribute('aria-current')).toBe('page'));
  expect(document.getElementById(`${id}-panel`)?.hasAttribute('hidden')).toBe(false);
  for (const other of ['users', 'email', 'codes'].filter(section => section !== id)) {
    const panel = document.getElementById(`${other}-panel`);
    if (panel) expect(panel.hasAttribute('hidden')).toBe(true);
  }
};
const fill = (id: string, value: string) => {
  const input = document.getElementById(id) as HTMLInputElement;
  input.value = value; input.dispatchEvent(new Event('input', { bubbles: true }));
};
async function openAdmin() {
  component = mount(Admin, { target: document.body });
  await vi.waitFor(() => expect(document.querySelector('nav button[data-section="users"]')).not.toBeNull());
  await vi.waitFor(() => expect(document.body.textContent).toContain('user@example.com'));
}

async function openCreateForm() {
  button('Create user').click();
  await vi.waitFor(() => expect(document.getElementById('create-email')).not.toBeNull());
  expect(document.querySelector('#users-panel table')).toBeNull();
  const password = document.getElementById('create-password') as HTMLInputElement;
  expect(password.minLength).toBe(8); expect(password.maxLength).toBe(72);
}

it.each([['en', 'Create user', 'Back to users'], ['zh', '创建用户', '返回用户列表']] as const)('opens a separate creation form from the list header and returns without creating (%s)', async (language, create, back) => {
  locale.set(language); await openAdmin();
  expect(document.querySelector('#users-panel form')).toBeNull();
  const createButton = button(create);
  expect(document.querySelector('.admin-surface-head')?.contains(createButton)).toBe(true);
  createButton.click();
  await vi.waitFor(() => expect(document.getElementById('create-email')).not.toBeNull());
  expect(document.querySelector('#users-panel table')).toBeNull();
  fill('create-email', 'draft@example.com'); fill('create-password', 'test-draft-password');
  button(back).click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel table')).not.toBeNull());
  expect(document.querySelector('#users-panel form')).toBeNull();
  expect(api.post).not.toHaveBeenCalled();
  button(create).click();
  await vi.waitFor(() => expect((document.getElementById('create-password') as HTMLInputElement).value).toBe(''));
});

it.each([['en', 'Site Administration'], ['zh', '站点管理']] as const)('places the admin block immediately above sign out and closes it on click (%s)', async (language, label) => {
  locale.set(language); component = mount(TopBar, { target: document.body });
  document.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!.click();
  await vi.waitFor(() => expect(document.querySelector('[role="menu"]')).not.toBeNull());
  const entry = document.querySelector<HTMLAnchorElement>('a[href="/web/admin"]')!;
  expect(entry.textContent?.trim()).toBe(label);
  expect(entry.querySelector('.menu-icon svg')).not.toBeNull();
  expect(entry.parentElement?.previousElementSibling?.textContent).toContain(language === 'en' ? 'Manual' : '用户手册');
  expect(entry.parentElement?.nextElementSibling?.querySelector('.signout')).not.toBeNull();
  expect(entry.parentElement?.classList.contains('menu-section')).toBe(true);
  entry.addEventListener('click', event => event.preventDefault());
  entry.click(); await vi.waitFor(() => expect(document.querySelector('[role="menu"]')).toBeNull());
});

it('ordinary users have no admin block or extra separator', async () => {
  session.set({ user: { username: 'tester', is_admin: false } as User, loaded: true, loading: false });
  component = mount(TopBar, { target: document.body });
  document.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!.click();
  await vi.waitFor(() => expect(document.querySelector('[role="menu"]')).not.toBeNull());
  expect(document.querySelector('a[href="/web/admin"]')).toBeNull();
  expect(document.querySelectorAll('.menu-section')).toHaveLength(3);
});

it('switches the selected administration panel and keeps SMTP failure local', async () => {
  const get = api.get.getMockImplementation()!;
  api.get.mockImplementation(async (path: string) => {
    if (path === '/admin/site-settings') throw new Error('Site settings unavailable');
    return get(path);
  });
  await openAdmin();
  expect(document.querySelector('#users-panel')?.hasAttribute('hidden')).toBe(false);
  expect(document.querySelector('#email-panel')?.hasAttribute('hidden')).toBe(true);
  expect(document.querySelector('nav button[data-section="codes"]')).toBeNull();
  expect(document.querySelector('#codes-panel')).toBeNull();
  await navigate('email');
  expect(document.querySelector('#users-panel')?.hasAttribute('hidden')).toBe(true);
  expect(document.querySelector('#email-panel [role="alert"]')?.textContent).toContain('Site settings unavailable');
  await navigate('users');
  expect(document.querySelector('#users-panel [role="alert"]')).toBeNull();
  expect(button('Create user').disabled).toBe(false);
});

it('denies non-admin access without requesting privileged data', async () => {
  admin = false;
  component = mount(Admin, { target: document.body });
  await vi.waitFor(() => expect(document.querySelector('[role="alert"]')?.textContent).toContain('Administrator required'));
  expect(document.querySelector('nav')).toBeNull();
  expect(api.get.mock.calls.map(call => call[0])).toEqual(['/auth/me']);
});

it('creates a user, clears the temporary password and refreshes the list', async () => {
  await openAdmin(); await openCreateForm();
  fill('create-email', 'new@example.com'); fill('create-password', 'Ab1!xyzw');
  api.post.mockImplementation(async (path: string) => {
    if (path === '/admin/users') api.get.mockResolvedValue([{ id: 2, email: 'new@example.com', username: 'newuser' }]);
    return {};
  });
  button('Create').click();
  await vi.waitFor(() => expect(api.post).toHaveBeenCalledWith('/admin/users', { email: 'new@example.com', password: 'Ab1!xyzw' }));
  await vi.waitFor(() => expect(document.querySelector('tbody')?.textContent).toContain('new@example.com'));
  expect(document.querySelector('#users-panel form')).toBeNull();
  expect(document.querySelector('#users-panel [role="status"]')?.textContent).toContain('Deliver the temporary password securely');
  await openCreateForm();
  expect((document.getElementById('create-password') as HTMLInputElement).value).toBe('');
  expect(document.body.textContent).not.toContain('Ab1!xyzw');
});

it('keeps a failed creation local with its draft intact', async () => {
  await openAdmin(); await openCreateForm(); fill('create-email', 'new@example.com'); fill('create-password', 'temporary-test-password');
  api.post.mockRejectedValueOnce(new Error('Email already exists'));
  button('Create').click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel [role="alert"]')?.textContent).toContain('Email already exists'));
  expect((document.getElementById('create-password') as HTMLInputElement).value).toBe('temporary-test-password');
  expect((document.getElementById('create-email') as HTMLInputElement).value).toBe('new@example.com');
  await navigate('email'); expect(document.querySelector('#email-panel [role="alert"]')).toBeNull();
});

it('tests SMTP before separately enabling registration and keeps failed toggles unchanged', async () => {
  await openAdmin(); await navigate('email');
  expect(button('Open registration').disabled).toBe(true);
  fill('smtp-host', 'smtp.example.com'); fill('smtp-from', 'sender@example.com'); fill('smtp-password', 'test-smtp-password');
  fill('smtp-port', '587'); fill('smtp-username', 'mailer');
  const tls = document.getElementById('smtp-tls') as HTMLSelectElement;
  tls.value = 'starttls'; tls.dispatchEvent(new Event('change', { bubbles: true }));
  let submitted: unknown;
  api.post.mockImplementationOnce(async (_path: string, payload: unknown) => {
    submitted = JSON.parse(JSON.stringify(payload)); return {};
  });
  button('Test and save').click();
  await vi.waitFor(() => expect(button('Open registration').disabled).toBe(false));
  expect(api.post.mock.calls.map(call => call[0])).toEqual(['/admin/smtp/test']);
  expect(submitted).toMatchObject({ host: 'smtp.example.com', port: 587, username: 'mailer', password: 'test-smtp-password', from: 'sender@example.com', tls_mode: 'starttls' });
  expect((document.getElementById('smtp-password') as HTMLInputElement).value).toBe('');
  button('Open registration').click();
  await vi.waitFor(() => expect(button('Close registration')).toBeDefined());
  expect(api.post).toHaveBeenCalledWith('/admin/registration', { enabled: true });
  api.post.mockRejectedValueOnce(new Error('Registration update failed'));
  button('Close registration').click();
  await vi.waitFor(() => expect(document.querySelector('#email-panel [role="alert"]')?.textContent).toContain('Registration update failed'));
  expect(button('Close registration')).toBeDefined();
  await vi.waitFor(() => expect(button('Test and save').disabled).toBe(false));
  fill('smtp-password', 'another-test-password'); button('Test and save').click();
  await vi.waitFor(() => expect(button('Open registration')).toBeDefined());
  expect(api.post.mock.calls.filter(call => call[0] === '/admin/registration')).toHaveLength(2);
});

it('allows testing saved SMTP settings without entering the password again', async () => {
  const get = api.get.getMockImplementation()!;
  api.get.mockImplementation(async (path: string, query?: { page: number }) => path === '/admin/site-settings'
    ? { host: 'smtp.example.com', port: 465, username: 'mailer', from: 'sender@example.com', tls_mode: 'tls', tested: true, registration_enabled: false, has_password: true }
    : get(path, query));
  await openAdmin(); await navigate('email');
  expect(document.querySelector('label[for="smtp-password"]')?.textContent).toContain('leave blank to use the saved password');
  let submitted: unknown;
  api.post.mockImplementationOnce(async (_path: string, payload: unknown) => { submitted = payload; return {}; });
  button('Test and save').click();
  await vi.waitFor(() => expect(button('Open registration').disabled).toBe(false));
  expect(submitted).toMatchObject({ host: 'smtp.example.com', username: 'mailer', password: '' });
});

it('saves the public site URL from site configuration', async () => {
  await openAdmin(); await navigate('email');
  fill('public-base-url', 'http://example.com:8080');
  button('Save site URL').click();
  await vi.waitFor(() => expect(api.post).toHaveBeenCalledWith('/admin/site-settings', { public_base_url: 'http://example.com:8080' }));
  await vi.waitFor(() => expect(document.body.textContent).toContain('Site address saved.'));
});

it('a failed SMTP test does not enable registration or clear the draft', async () => {
  await openAdmin(); await navigate('email');
  fill('smtp-host', 'smtp.example.com'); fill('smtp-from', 'sender@example.com'); fill('smtp-password', 'test-smtp-password');
  api.post.mockRejectedValueOnce(new Error('SMTP test failed'));
  button('Test and save').click();
  await vi.waitFor(() => expect(document.querySelector('#email-panel [role="alert"]')?.textContent).toContain('SMTP test failed'));
  expect(button('Open registration').disabled).toBe(true);
  expect((document.getElementById('smtp-password') as HTMLInputElement).value).toBe('test-smtp-password');
});

it('paginates users and leaves the current page intact when the next fetch fails', async () => {
  const get = api.get.getMockImplementation()!;
  api.get.mockImplementation(async (path: string, query?: { page: number }) => {
    if (path === '/admin/users') {
      if (query?.page === 2) throw new Error('Next page unavailable');
      return Array.from({ length: 50 }, (_, id) => ({ id, username: `user${id}`, email: 'user@example.com' }));
    }
    return get(path);
  });
  await openAdmin(); expect(button('Previous').disabled).toBe(true);
  button('Next').click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel [role="alert"]')?.textContent).toContain('Next page unavailable'));
  expect(button('Previous').disabled).toBe(true);
  expect(document.querySelector('tbody')?.textContent).toContain('user49');
  api.get.mockResolvedValueOnce([{ id: 51, username: 'lastuser', email: 'last@example.com' }]);
  button('Next').click();
  await vi.waitFor(() => expect(document.querySelector('tbody')?.textContent).toContain('lastuser'));
  expect(button('Previous').disabled).toBe(false); expect(button('Next').disabled).toBe(true);
});

it.skipIf(process.env.JIELI_EDITION !== 'cloud')('shows codes only in cloud mode and preserves one-time plaintext while switching panels', async () => {
  cloud = true; await openAdmin(); await navigate('codes');
  expect(document.querySelector('#codes-panel .admin-surface')).not.toBeNull();
  api.post.mockResolvedValueOnce({ code: 'test-one-time-code' });
  api.get.mockResolvedValueOnce([{ id: 7, code: 'jpro_1234567890abcdef', days: 30, disabled: false, redeemed_by: 0 }]);
  button('Create single-use code').click();
  await vi.waitFor(() => expect(document.querySelector('#codes-panel code')?.textContent).toBe('test-one-time-code'));
  expect(api.post).toHaveBeenCalledWith('/admin/membership-codes', { days: 30 });
  const codeRow = document.querySelector('#codes-panel tbody tr')!;
  expect(codeRow.textContent).toContain('jpro_123');
  expect(codeRow.textContent).not.toContain('jpro_1234567890abcdef');
  const reveal = codeRow.querySelector<HTMLButtonElement>('.reveal-code')!;
  expect(reveal.getAttribute('aria-label')).toBe('Show full code');
  reveal.click();
  await vi.waitFor(() => expect(codeRow.textContent).toContain('jpro_1234567890abcdef'));
  expect(reveal.getAttribute('aria-label')).toBe('Hide full code');
  reveal.click();
  await vi.waitFor(() => expect(codeRow.textContent).toContain('jpro_123'));
  expect(codeRow.textContent).not.toContain('jpro_1234567890abcdef');
  await navigate('users'); await navigate('codes');
  expect(document.querySelector('#codes-panel code')?.textContent).toBe('test-one-time-code');
  api.get.mockResolvedValueOnce([{ id: 7, days: 30, disabled: true, redeemed_by: 0 }]);
  button('Disable').click();
  await vi.waitFor(() => expect(button('Disable').disabled).toBe(true));
  expect(api.post).toHaveBeenCalledWith('/admin/membership-codes/7/disable');
  await vi.waitFor(() => expect(document.querySelector('#codes-panel tbody tr')?.textContent).toContain('Disabled'));
  await vi.waitFor(() => expect(document.querySelector('#codes-panel [aria-busy]')?.getAttribute('aria-busy')).toBe('false'));
  expect(button('Disable').disabled).toBe(true);
});


it.each([['en', 'Site Administration'], ['zh', '站点管理']] as const)('localizes the page title and sidebar title (%s)', async (language, title) => {
  locale.set(language); await openAdmin();
  expect(document.title).toBe(`${title} · Jieli`);
  expect(document.querySelector('aside')?.getAttribute('aria-label')).toBe(title);
  expect(document.querySelector('aside .nav-title')?.textContent).toBe(title);
});

it('shows the numeric user ID in the user list', async () => {
  await openAdmin();
  expect(document.querySelector('#users-panel thead')?.textContent).toContain('ID');
  expect(document.querySelector('#users-panel tbody tr td')?.textContent).toBe('1');
});

it('preserves user and SMTP drafts across panel navigation', async () => {
  await openAdmin(); await openCreateForm(); fill('create-email', 'draft@example.com'); fill('create-password', 'draft-test-password');
  await navigate('email'); fill('smtp-host', 'draft-smtp.example.com'); fill('smtp-password', 'draft-smtp-password');
  await navigate('users');
  expect((document.getElementById('create-email') as HTMLInputElement).value).toBe('draft@example.com');
  expect((document.getElementById('create-password') as HTMLInputElement).value).toBe('draft-test-password');
  await navigate('email');
  expect((document.getElementById('smtp-host') as HTMLInputElement).value).toBe('draft-smtp.example.com');
  expect((document.getElementById('smtp-password') as HTMLInputElement).value).toBe('draft-smtp-password');
});

it('shows access loading and does not expose controls before verification', async () => {
  let resolveAccess!: (value: unknown) => void;
  api.get.mockImplementationOnce(() => new Promise(resolve => { resolveAccess = resolve; }));
  component = mount(Admin, { target: document.body });
  await vi.waitFor(() => expect(document.querySelector('[role="status"]')?.textContent).toContain('Checking administrator access'));
  expect(document.querySelector('nav')).toBeNull(); expect(document.querySelector('form')).toBeNull();
  resolveAccess({ is_admin: true });
  await vi.waitFor(() => expect(document.querySelector('nav')).not.toBeNull());
});

it('prevents duplicate user submissions while allowing the email panel to operate', async () => {
  await openAdmin(); await openCreateForm(); fill('create-email', 'pending@example.com'); fill('create-password', 'pending-test-password');
  let resolveCreate!: (value: unknown) => void;
  api.post.mockImplementationOnce(() => new Promise(resolve => { resolveCreate = resolve; }));
  button('Create').click();
  await vi.waitFor(() => expect(button('Create').disabled).toBe(true));
  expect(document.querySelector('#users-panel [aria-busy]')?.getAttribute('aria-busy')).toBe('true');
  expect(button('Back to users').disabled).toBe(true);
  document.querySelector<HTMLFormElement>('#users-panel form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  expect(api.post).toHaveBeenCalledTimes(1);
  await navigate('email'); expect(button('Test and save').disabled).toBe(false);
  resolveCreate({});
  await vi.waitFor(() => expect(button('Create user').disabled).toBe(false));
});

it.skipIf(process.env.JIELI_EDITION !== 'cloud')('keeps code creation errors local and can recover without displaying nonexistent plaintext', async () => {
  cloud = true; await openAdmin(); await navigate('codes');
  api.post.mockRejectedValueOnce(new Error('Code generation failed')); button('Create single-use code').click();
  await vi.waitFor(() => expect(document.querySelector('#codes-panel [role="alert"]')?.textContent).toContain('Code generation failed'));
  expect(document.querySelector('#codes-panel code')).toBeNull();
  await navigate('users'); expect(document.querySelector('#users-panel [role="alert"]')).toBeNull();
  await navigate('codes'); api.post.mockResolvedValueOnce({ code: 'recovered-test-code' });
  button('Create single-use code').click();
  await vi.waitFor(() => expect(document.querySelector('#codes-panel code')?.textContent).toBe('recovered-test-code'));
  expect(document.querySelector('#codes-panel [role="alert"]')).toBeNull();
  await unmount(component!); component = undefined;
  await openAdmin(); await navigate('codes');
  expect(document.querySelector('#codes-panel code')).toBeNull();
});

it.skipIf(process.env.JIELI_EDITION !== 'cloud')('keeps code disabling errors local and prevents disabling redeemed or already disabled codes', async () => {
  cloud = true;
  const get = api.get.getMockImplementation()!;
  api.get.mockImplementation(async (path: string) => path === '/admin/membership-codes' ? [
    { id: 7, code: 'jpro_1234567890abcdef', days: 30, disabled: false, redeemed_by: 0 },
    { id: 8, code: 'jpro_disabled', days: 30, disabled: true, redeemed_by: 0 },
    { id: 9, code: 'jpro_redeemed', days: 30, disabled: false, redeemed_by: 2, redeemed_by_username: 'member' }
  ] : get(path));
  await openAdmin(); await navigate('codes');
  const disableButtons = [...document.querySelectorAll<HTMLButtonElement>('#codes-panel tbody td:last-child button')];
  expect(disableButtons.map(button => button.disabled)).toEqual([false, true, true]);
  expect(document.querySelector('#codes-panel tbody tr:nth-child(3)')?.textContent).toContain('Used by member');
  api.post.mockRejectedValueOnce(new Error('Code disable failed')); disableButtons[0].click();
  await vi.waitFor(() => expect(document.querySelector('#codes-panel [role="alert"]')?.textContent).toContain('Code disable failed'));
  expect(disableButtons[0].disabled).toBe(false);
  await navigate('email'); expect(document.querySelector('#email-panel [role="alert"]')).toBeNull();
});

it.skipIf(process.env.JIELI_EDITION !== 'cloud')('prevents duplicate code generation while its request is pending', async () => {
  cloud = true; await openAdmin(); await navigate('codes');
  let resolveCode!: (value: unknown) => void;
  api.post.mockImplementationOnce(() => new Promise(resolve => { resolveCode = resolve; }));
  button('Create single-use code').click();
  await vi.waitFor(() => expect(button('Create single-use code').disabled).toBe(true));
  expect(document.querySelector('#codes-panel [aria-busy]')?.getAttribute('aria-busy')).toBe('true');
  document.querySelector<HTMLFormElement>('#codes-panel form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  expect(api.post).toHaveBeenCalledTimes(1);
  resolveCode({ code: 'pending-test-code' });
  await vi.waitFor(() => expect(button('Create single-use code').disabled).toBe(false));
});


it('shows email pending state, prevents duplicate SMTP tests and displays section success', async () => {
  await openAdmin(); await navigate('email');
  fill('smtp-host', 'smtp.example.com'); fill('smtp-from', 'sender@example.com');
  let resolveSMTP!: (value: unknown) => void;
  api.post.mockImplementationOnce(() => new Promise(resolve => { resolveSMTP = resolve; }));
  button('Test and save').click();
  await vi.waitFor(() => expect(button('Test and save').disabled).toBe(true));
  expect(document.querySelector('#email-panel [aria-busy]')?.getAttribute('aria-busy')).toBe('true');
  expect(button('Open registration').disabled).toBe(true);
  document.querySelector<HTMLFormElement>('#email-panel form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  expect(api.post).toHaveBeenCalledTimes(1);
  await navigate('users'); expect(button('Create user').disabled).toBe(false);
  resolveSMTP({}); await navigate('email');
  await vi.waitFor(() => expect(button('Test and save').disabled).toBe(false));
  expect(button('Open registration').disabled).toBe(false);
  expect(document.querySelector('#email-panel [role="status"]')?.textContent).toBe('Test email sent; settings saved.');
  expect(document.querySelector('#email-panel [aria-busy]')?.getAttribute('aria-busy')).toBe('false');
});

it.each([['en', 'Edit user', 'Back to users'], ['zh', '编辑用户', '返回用户列表']] as const)('opens an accessible edit form with prefilled public fields and cancels (%s)', async (language, title, back) => {
  locale.set(language); await openAdmin();
  const edit = document.querySelector<HTMLButtonElement>(`button[aria-label="${title} tester"]`)!;
  expect(edit).not.toBeNull(); expect(edit.querySelector('svg')).not.toBeNull(); edit.click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel h1')?.textContent).toBe(title));
  expect(document.querySelector('#users-panel table')).toBeNull(); expect(document.getElementById('create-email')).toBeNull();
  expect((document.getElementById('edit-email') as HTMLInputElement).value).toBe('user@example.com');
  expect((document.getElementById('edit-username') as HTMLInputElement).value).toBe('tester');
  expect((document.getElementById('edit-display-name') as HTMLInputElement).value).toBe('Tester Name');
  expect((document.getElementById('edit-password') as HTMLInputElement).value).toBe('');
  expect(document.querySelector('.settings-frame')?.contains(document.getElementById('edit-email'))).toBe(true);
  const labels = [...document.querySelectorAll('#users-panel label')].map(label => label.textContent?.trim());
  expect(labels).toEqual(language === 'en' ? ['Email', 'Username', 'Display name', 'New password (leave blank to keep current)'] : ['邮箱', '用户名', '显示名称', '新密码（留空保持不变）']);
  fill('edit-email', 'cancel@example.com'); fill('edit-username', 'cancelled'); fill('edit-display-name', 'Cancelled'); fill('edit-password', 'cancel-password'); button(back).click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel table')).not.toBeNull());
  expect(api.put).not.toHaveBeenCalled();
  document.querySelector<HTMLButtonElement>(`button[aria-label="${title} tester"]`)!.click();
  await vi.waitFor(() => expect((document.getElementById('edit-password') as HTMLInputElement).value).toBe(''));
  expect((document.getElementById('edit-email') as HTMLInputElement).value).toBe('user@example.com');
  expect((document.getElementById('edit-username') as HTMLInputElement).value).toBe('tester');
  expect((document.getElementById('edit-display-name') as HTMLInputElement).value).toBe('Tester Name');
});

async function openEditForm() {
  document.querySelector<HTMLButtonElement>('button[aria-label="Edit user tester"]')!.click();
  await vi.waitFor(() => expect(document.getElementById('edit-email')).not.toBeNull());
}

it('saves all editable fields, refreshes the current page and clears the password draft', async () => {
  const get = api.get.getMockImplementation()!;
  const rows = Array.from({ length: 50 }, (_, index) => ({ id: index + 10, email: 'user@example.com', username: `user${index}`, display_name: 'User' }));
  api.get.mockImplementation(async (path: string, query?: { page: number }) => path === '/admin/users' && query?.page === 1 ? rows : get(path));
  await openAdmin(); button('Next').click();
  await vi.waitFor(() => expect(document.querySelector('.page-count')?.textContent).toContain('2'));
  await openEditForm();
  fill('edit-email', 'changed@example.com'); fill('edit-username', 'changed'); fill('edit-display-name', 'Changed Name'); fill('edit-password', 'Ab1!xyzw');
  api.put.mockResolvedValueOnce({ id: 1, email: 'changed@example.com', username: 'changed', display_name: 'Changed Name' });
  const getAfter = api.get.getMockImplementation()!;
  api.get.mockImplementation(async (path: string, query?: { page: number }) => path === '/admin/users' && query?.page === 2 ? [{ id: 1, email: 'changed@example.com', username: 'changed', display_name: 'Changed Name' }] : getAfter(path, query));
  button('Save').click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel table')?.textContent).toContain('changed@example.com'));
  expect(api.put).toHaveBeenCalledWith('/admin/users/1', { email: 'changed@example.com', username: 'changed', display_name: 'Changed Name', password: 'Ab1!xyzw' });
  expect(document.querySelector('#users-panel [role="status"]')?.textContent).toBe('User updated.');
  expect(document.querySelector('.page-count')?.textContent).toContain('2');
  expect(document.getElementById('edit-password')).toBeNull();
  button('Create user').click();
  await vi.waitFor(() => expect((document.getElementById('create-password') as HTMLInputElement).value).toBe(''));
});

it('keeps edit drafts across section navigation and failed saves and can retry with a blank password', async () => {
  await openAdmin(); await openEditForm(); fill('edit-email', 'draft@example.com'); fill('edit-display-name', 'Draft Name'); fill('edit-username', 'draftuser'); fill('edit-password', 'draft-password');
  const password = document.getElementById('edit-password') as HTMLInputElement;
  expect(password.required).toBe(false); expect(password.minLength).toBe(8); expect(password.maxLength).toBe(72);
  await navigate('email'); await navigate('users');
  expect((document.getElementById('edit-email') as HTMLInputElement).value).toBe('draft@example.com');
  expect((document.getElementById('edit-username') as HTMLInputElement).value).toBe('draftuser');
  expect((document.getElementById('edit-password') as HTMLInputElement).value).toBe('draft-password');
  expect((document.getElementById('edit-display-name') as HTMLInputElement).value).toBe('Draft Name');
  api.put.mockRejectedValueOnce(new Error('email unavailable')); button('Save').click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel [role="alert"]')?.textContent).toBe('email unavailable'));
  expect(document.querySelector('#users-panel table')).toBeNull();
  expect((document.getElementById('edit-email') as HTMLInputElement).value).toBe('draft@example.com');
  expect((document.getElementById('edit-username') as HTMLInputElement).value).toBe('draftuser');
  expect((document.getElementById('edit-password') as HTMLInputElement).value).toBe('draft-password');
  expect((document.getElementById('edit-display-name') as HTMLInputElement).value).toBe('Draft Name');
  expect(button('Save').disabled).toBe(false);
  fill('edit-password', ''); api.put.mockResolvedValueOnce({ id: 1 }); button('Save').click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel table')).not.toBeNull());
  expect(api.put).toHaveBeenLastCalledWith('/admin/users/1', { email: 'draft@example.com', username: 'draftuser', display_name: 'Draft Name', password: '' });
  expect(document.querySelector('#users-panel [role="alert"]')).toBeNull();
});

it('prevents duplicate saves and freezes fields and cancellation while editing is pending', async () => {
  await openAdmin(); await openEditForm();
  let resolveSave!: (value: unknown) => void;
  api.put.mockImplementationOnce(() => new Promise(resolve => { resolveSave = resolve; }));
  button('Save').click();
  await vi.waitFor(() => expect(button('Save').disabled).toBe(true));
  expect(button('Back to users').disabled).toBe(true);
  expect([...document.querySelectorAll<HTMLInputElement>('#users-panel input')].every(field => field.disabled)).toBe(true);
  expect(document.querySelector('#users-panel [aria-busy]')?.getAttribute('aria-busy')).toBe('true');
  document.querySelector<HTMLFormElement>('#users-panel form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  expect(api.put).toHaveBeenCalledTimes(1);
  await navigate('email'); expect(button('Test and save').disabled).toBe(false);
  resolveSave({ id: 1 }); await navigate('users');
  await vi.waitFor(() => expect(document.querySelector('#users-panel table')).not.toBeNull());
});

it('updates the session after editing the signed-in administrator', async () => {
  session.set({ user: { id: 1, username: 'tester', is_admin: true } as User, loaded: true, loading: false });
  await openAdmin(); await openEditForm();
  expect(document.querySelector('#users-panel .admin-hint')?.textContent).toContain('your current session stays signed in');
  fill('edit-username', 'newoperator'); fill('edit-display-name', 'New Operator');
  api.put.mockResolvedValueOnce({ id: 1, username: 'newoperator', display_name: 'New Operator', is_admin: true });
  button('Save').click();
  await vi.waitFor(() => expect(document.querySelector('#users-panel table')).not.toBeNull());
  let user: User | null = null; const unsubscribe = session.subscribe(value => user = value.user); unsubscribe();
  expect(user).toMatchObject({ id: 1, username: 'newoperator', display_name: 'New Operator', is_admin: true });
});

it.each(['create', 'edit'] as const)('shows validation and API errors in the same place above the %s submit action', async (mode) => {
  await openAdmin();
  if (mode === 'create') await openCreateForm(); else await openEditForm();
  const form = document.querySelector<HTMLFormElement>('#users-panel form')!;
  expect(form.noValidate).toBe(true);
  fill(`${mode}-email`, 'invalid-email');
  form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  await vi.waitFor(() => expect(form.querySelector('[role="alert"]')).not.toBeNull());
  const validation = form.querySelector('[role="alert"]')!;
  expect(validation.textContent?.trim().length).toBeGreaterThan(0);
  expect(validation.nextElementSibling?.classList.contains('admin-actions')).toBe(true);
  expect(document.querySelector('#users-panel > [role="alert"]')).toBeNull();
  expect(api.post).not.toHaveBeenCalled(); expect(api.put).not.toHaveBeenCalled();
  fill(`${mode}-email`, 'occupied@example.com');
  if (mode === 'create') fill('create-password', 'Ab1!xyzw');
  (mode === 'create' ? api.post : api.put).mockRejectedValueOnce(new Error('email unavailable'));
  button(mode === 'create' ? 'Create' : 'Save').click();
  await vi.waitFor(() => expect(form.querySelector('[role="alert"]')?.textContent).toBe('email unavailable'));
  expect(form.querySelector('[role="alert"]')?.nextElementSibling?.classList.contains('admin-actions')).toBe(true);
  expect(document.querySelectorAll('#users-panel [role="alert"]')).toHaveLength(1);
  expect((document.getElementById(`${mode}-email`) as HTMLInputElement).value).toBe('occupied@example.com');
});

it.each([['en', 'Inactive'], ['zh', '禁用']] as const)('places the inactive switch after edit and allows disable and restore (%s)', async (language,label) => {
  locale.set(language); await openAdmin();
  const toggle = document.querySelector<HTMLButtonElement>(`button[role="switch"][aria-label="${label} tester"]`)!;
  expect(toggle).not.toBeNull(); expect(toggle.previousElementSibling?.classList.contains('edit-user')).toBe(true);
  expect(toggle.getAttribute('aria-checked')).toBe('false');
  api.put.mockResolvedValueOnce({id:1,email:'user@example.com',username:'tester',display_name:'Tester Name',inactive:true}); toggle.click();
  await vi.waitFor(()=>expect(toggle.getAttribute('aria-checked')).toBe('true'));
  expect(api.put).toHaveBeenCalledWith('/admin/users/1/inactive',{inactive:true});
  expect(document.querySelector<HTMLButtonElement>('#users-panel .edit-user')?.disabled).toBe(true);
  expect(document.querySelector('#users-panel .inactive-label')?.textContent).toBe(label);
  api.put.mockResolvedValueOnce({id:1,email:'user@example.com',username:'tester',display_name:'Tester Name',inactive:false}); toggle.click();
  await vi.waitFor(()=>expect(toggle.getAttribute('aria-checked')).toBe('false'));
  expect(api.put).toHaveBeenLastCalledWith('/admin/users/1/inactive',{inactive:false});
  expect(document.querySelector<HTMLButtonElement>('#users-panel .edit-user')?.disabled).toBe(false);
});

it('retains inactivity state on failure and prevents duplicate or self toggles', async () => {
  session.set({user:{id:99,username:'operator',is_admin:true} as User,loaded:true,loading:false});
  const get=api.get.getMockImplementation()!;
  api.get.mockImplementation(async(path:string)=>path==='/admin/users' ? [
    {id:1,email:'user@example.com',username:'tester',display_name:'Tester Name',inactive:false},
    {id:99,email:'operator@example.com',username:'operator',display_name:'Operator',inactive:false,is_admin:true}
  ] : get(path));
  await openAdmin();
  expect(document.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Inactive operator"]')?.disabled).toBe(true);
  const toggle=document.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Inactive tester"]')!;
  let rejectToggle!:(error:Error)=>void;
  api.put.mockImplementationOnce(()=>new Promise((_resolve,reject)=>rejectToggle=reject)); toggle.click();
  await vi.waitFor(()=>expect(toggle.disabled).toBe(true));
  expect(toggle.getAttribute('aria-checked')).toBe('false'); toggle.click(); expect(api.put).toHaveBeenCalledTimes(1);
  rejectToggle(new Error('Status update failed'));
  await vi.waitFor(()=>expect(document.querySelector('#users-panel [role="alert"]')?.textContent).toBe('Status update failed'));
  expect(toggle.disabled).toBe(false); expect(toggle.getAttribute('aria-checked')).toBe('false');
  expect(document.querySelector<HTMLButtonElement>('#users-panel .edit-user')?.disabled).toBe(false);
});
