// @vitest-environment jsdom
// @ts-expect-error jsdom needs Svelte's client mount API.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, expect, it, vi } from 'vitest';
import { configApi } from '$lib/api/auth';
import { userApi } from '$lib/api/user';
import { locale } from '$lib/stores/locale';
import Settings from '../routes/web/settings/+page.svelte';
import { ApiError } from '$lib/api/client';

// @ts-expect-error onMount must use the client runtime in jsdom.
vi.mock('svelte', async () => await import('../../node_modules/svelte/src/index-client.js'));
vi.mock('$lib/api/auth', () => ({ configApi: { get: vi.fn(async () => ({ cloud_billing: false })) } }));
vi.mock('$lib/api/user', () => ({ userApi: {
  profile: vi.fn(async () => ({ user: { username: 'tester' }, current_messages: 42, message_quota: -1, stats: { total_threads: 3, lines_added: 0, lines_removed: 0 }, plan_level: 'free' })),
  updateProfile: vi.fn(async (value) => ({ username: value.username, display_name: value.display_name })),
  apiKey: vi.fn(async () => ({ has_key: true, prefix: 'jl_', api_key: 'test-sync-key' }))
} }));
let component: ReturnType<typeof mount>;
afterEach(async () => {
  if (component) await unmount(component);
  document.body.innerHTML = '';
  history.replaceState(null, '', '/');
});
it('keeps storage in account preferences and switches panels without exposing hosted billing', async () => {
  history.replaceState(null, '', '/web/settings#account');
  component = mount(Settings, { target: document.body });
  await vi.waitFor(() => expect(document.querySelector('a[aria-current="page"]')?.getAttribute('href')).toBe('#account'));
  await vi.waitFor(() => expect(document.querySelector('.preferences-surface')?.textContent).toContain('42'));
  expect(document.querySelector('a[href="#storage"]')).toBeNull();
  expect(document.querySelector('a[href="#billing"]')).toBeNull();
  (document.querySelector('a[href="#security"]') as HTMLAnchorElement).click();
  await vi.waitFor(() => expect(location.hash).toBe('#security'));
  await vi.waitFor(() => expect(document.querySelector('main code')?.textContent).toContain('jl_'));
  expect(document.querySelector('a[href="/web/auth/password"]')).not.toBeNull();
  location.hash = 'account';
  await vi.waitFor(() => expect(document.querySelector('a[aria-current="page"]')?.getAttribute('href')).toBe('#account'));
  expect(document.querySelector('main code')).toBeNull();
  expect(document.querySelector('a[href="/web/auth/password"]')).toBeNull();
  expect(document.querySelector('a[href="/@tester"]')).not.toBeNull();
});

it('uses compact k notation for large usage statistics while keeping smaller counts exact', async () => {
  vi.mocked(userApi.profile).mockResolvedValueOnce({
    user: { id: 1, username: 'tester', github_id: '' }, current_messages: 42, message_quota: -1, plan_level: 'free', auto_cleanup_enabled: false,
    stats: { total_threads: 1141, total_messages: 339310, lines_added: 4620238, lines_removed: 567095, files_changed: 14141 }
  });
  component = mount(Settings, { target: document.body });
  await vi.waitFor(() => expect(document.querySelector('.lines-code')?.textContent).toContain('+4620k'));
  const usage = document.querySelector('.usage-overview')!;
  expect(usage.querySelector('.lines-code')?.textContent).toContain('+4620k');
  expect(usage.querySelector('.lines-code')?.textContent).toContain('−567k');
  expect(usage.querySelector('.lines-code')?.textContent).toContain('~4053k');
  expect(usage.querySelector('small')?.textContent).toContain('14k');
  expect(usage.querySelectorAll('strong')[1]?.textContent).toBe('1,141');
  expect(usage.querySelectorAll('strong')[2]?.textContent).toBe('339k');
});

it('keeps profile read-only until editing, cancels without saving, and saves changed details', async () => {
  locale.set('en');
  component = mount(Settings, { target: document.body });
  const button = (label: string) => Array.from(document.querySelectorAll('button')).find(b => b.textContent?.trim() === label)!;
  await vi.waitFor(() => expect(button('Edit Details')).toBeDefined());
  button('Edit Details').click();
  await vi.waitFor(() => expect(document.querySelectorAll('.account-form input').length).toBe(2));
  const input = document.querySelector<HTMLInputElement>('.account-form input')!;
  input.value = 'New name';
  input.dispatchEvent(new Event('input', { bubbles: true }));
  button('Cancel').click();
  await vi.waitFor(() => expect(document.querySelector('.account-form')).toBeNull());
  expect(userApi.updateProfile).not.toHaveBeenCalled();
  button('Edit Details').click();
  await vi.waitFor(() => expect(document.querySelector<HTMLInputElement>('.account-form input')?.value).toBe(''));
  const next = document.querySelector<HTMLInputElement>('.account-form input')!;
  next.value = 'Saved name'; next.dispatchEvent(new Event('input', { bubbles: true }));
  await vi.waitFor(() => expect(button('Save changes').disabled).toBe(false));
  const saved = await userApi.profile();
  vi.mocked(userApi.profile).mockResolvedValueOnce({ ...saved, user: { ...saved.user, display_name: 'Saved name' } });
  button('Save changes').click();
  await vi.waitFor(() => expect(userApi.updateProfile).toHaveBeenCalledWith({ username: 'tester', display_name: 'Saved name' }));
  await vi.waitFor(() => expect(document.querySelector('.account-form')).toBeNull());
  expect(document.querySelector('.identity-grid')?.textContent).toContain('Saved name');
});

it.skipIf(process.env.JIELI_EDITION !== 'cloud')('shows membership inside the account page only when hosted billing is enabled', async () => {
  vi.mocked(configApi.get).mockResolvedValueOnce({ cloud_billing: true } as Awaited<ReturnType<typeof configApi.get>>);
  component = mount(Settings, { target: document.body });
  await vi.waitFor(() => expect(document.querySelector('a[href="#billing"]')).not.toBeNull());
  (document.querySelector('a[href="#billing"]') as HTMLAnchorElement).click();
  await vi.waitFor(() => expect(document.querySelector('.settings-content .billing-surface')).not.toBeNull());
});
it.each([
 ['en','Use only lowercase English letters or digits for your username.'],
 ['zh','用户名只能使用小写英文字母和数字。']
] as const)('keeps uppercase input visible, explains it, and saves after correction (%s)',async(language,expected)=>{
 locale.set('en');vi.mocked(userApi.updateProfile).mockClear();
 component=mount(Settings,{target:document.body});
 const button=(label:string)=>Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()===label)!;
 await vi.waitFor(()=>expect(button('Edit Details')).toBeDefined());button('Edit Details').click();
 await vi.waitFor(()=>expect(document.querySelectorAll('.account-form input').length).toBe(2));
 const username=document.querySelectorAll<HTMLInputElement>('.account-form input')[1];
 const save=button('Save changes');locale.set(language);
 username.value='Aa';username.dispatchEvent(new Event('input',{bubbles:true}));
 await vi.waitFor(()=>expect(document.querySelector('.account-form [role="alert"]')?.textContent).toBe(expected));
 expect(username.value).toBe('Aa');
 username.value='AaTest';username.dispatchEvent(new Event('input',{bubbles:true}));
 await vi.waitFor(()=>expect(username.value).toBe('AaTest'));
 await vi.waitFor(()=>expect(save.disabled).toBe(false));save.click();
 expect(userApi.updateProfile).not.toHaveBeenCalled();
 username.value='aatest';username.dispatchEvent(new Event('input',{bubbles:true}));
 await vi.waitFor(()=>expect(document.querySelector('.account-form [role="alert"]')).toBeNull());
 await vi.waitFor(()=>expect(save.disabled).toBe(false));save.click();
 await vi.waitFor(()=>expect(userApi.updateProfile).toHaveBeenCalledWith({username:'aatest',display_name:''}));
 await vi.waitFor(()=>expect(document.querySelector('.account-form')).toBeNull());
});
it.each([
 ['zh','aa','用户名需为 3–32 个小写英文字母或数字。'],
 ['en','aa','Use 3–32 lowercase English letters or digits for your username.'],
 ['zh','ab_cd','用户名需为 3–32 个小写英文字母或数字。']
] as const)('explains invalid username locally (%s/%s)',async(language,value,expected)=>{
 locale.set('en');vi.mocked(userApi.updateProfile).mockClear();
 component=mount(Settings,{target:document.body});
 const button=(label:string)=>Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()===label)!;
 await vi.waitFor(()=>expect(button('Edit Details')).toBeDefined());button('Edit Details').click();
 await vi.waitFor(()=>expect(document.querySelectorAll('.account-form input').length).toBe(2));
 const username=document.querySelectorAll<HTMLInputElement>('.account-form input')[1];
 username.value=value;username.dispatchEvent(new Event('input',{bubbles:true}));
 await vi.waitFor(()=>expect(button('Save changes').disabled).toBe(false));
 const save=button('Save changes');locale.set(language);save.click();
 await vi.waitFor(()=>expect(document.querySelector('.account-form [role="alert"]')?.textContent).toBe(expected));
 expect(userApi.updateProfile).not.toHaveBeenCalled();
 expect(document.querySelector('.settings-content > [role="alert"]')).toBeNull();
});
it.each([
 ['zh',400,'invalid profile','账户保存失败'],
 ['en',400,'invalid profile','Could not save account'],
 ['zh',409,'username unavailable','该用户名不可用，请换一个'],
 ['en',409,'username unavailable','This username is unavailable. Please choose another one.']
] as const)('keeps localized profile errors inside the edit form (%s/%s)',async(language,status,message,expected)=>{
 locale.set('en');vi.mocked(userApi.updateProfile).mockClear();
 component=mount(Settings,{target:document.body});
 const button=(label:string)=>Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()===label)!;
 await vi.waitFor(()=>expect(button('Edit Details')).toBeDefined());button('Edit Details').click();
 await vi.waitFor(()=>expect(document.querySelectorAll('.account-form input').length).toBe(2));
 const displayName=document.querySelector<HTMLInputElement>('.account-form input')!;
 displayName.value='Draft name';displayName.dispatchEvent(new Event('input',{bubbles:true}));
 const username=document.querySelectorAll<HTMLInputElement>('.account-form input')[1];
 username.value='testernew';username.dispatchEvent(new Event('input',{bubbles:true}));
 await vi.waitFor(()=>expect(button('Save changes').disabled).toBe(false));
 const save=button('Save changes');locale.set(language);
 vi.mocked(userApi.updateProfile).mockRejectedValueOnce(new ApiError(message,status));save.click();
 await vi.waitFor(()=>expect(document.querySelector('.account-form [role="alert"]')?.textContent).toBe(expected));
 expect(document.querySelector('.settings-content > [role="alert"]')).toBeNull();
 expect(username.value).toBe('testernew');
 expect(displayName.value).toBe('Draft name');
 expect(userApi.updateProfile).toHaveBeenCalledWith({username:'testernew',display_name:'Draft name'});
 const cancel=document.querySelector<HTMLButtonElement>('.profile-actions button')!;cancel.click();
 await vi.waitFor(()=>expect(document.querySelector('.account-form')).toBeNull());
 expect(document.querySelector('[role="alert"]')).toBeNull();
 document.querySelector<HTMLButtonElement>('.profile-actions button')!.click();
 await vi.waitFor(()=>expect(document.querySelector('.account-form')).not.toBeNull());
 expect(document.querySelector('.account-form [role="alert"]')).toBeNull();
 expect(document.querySelectorAll<HTMLInputElement>('.account-form input')[1].value).toBe('tester');
});
