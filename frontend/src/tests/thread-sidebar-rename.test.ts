// @vitest-environment jsdom
// @ts-expect-error Vitest resolves `svelte` to the server entry; jsdom needs the client mount API.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ThreadSidebar from '../lib/components/thread/ThreadSidebar.svelte';
import type { ThreadDetail } from '../lib/api/types';

const apiMocks = vi.hoisted(() => ({
  updateMeta: vi.fn(),
  archive: vi.fn(),
  remove: vi.fn()
}));

vi.mock('$lib/api/threads', () => ({
  threadsApi: {
    updateMeta: apiMocks.updateMeta,
    archive: apiMocks.archive,
    remove: apiMocks.remove
  }
}));

function makeThread(): ThreadDetail {
  return {
    id: 12,
    provider: 'amp',
    provider_thread_id: 'T-rename',
    title: 'Original title',
    visibility: 'private',
    repo: 'jieliapp/plugins',
    branch: 'main',
    message_count: 1,
    lines_added: 0,
    lines_removed: 0,
    files_changed: 0,
    labels: [],
    archived: false,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    messages: []
  };
}

function buttonByText(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find((node) =>
    node.textContent?.includes(text)
  );
  expect(button).toBeTruthy();
  return button as HTMLButtonElement;
}

describe('ThreadSidebar mobile rename', () => {
  afterEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    apiMocks.updateMeta.mockReset();
    apiMocks.archive.mockReset();
    apiMocks.remove.mockReset();
  });

  it('opens the rename dialog from the mobile menu and saves the new title', async () => {
    const thread = makeThread();
    const onchange = vi.fn();
    const prompt = vi.spyOn(window, 'prompt');
    apiMocks.updateMeta.mockResolvedValue({ ...thread, title: 'Renamed title' });

    const component = mount(ThreadSidebar, {
      target: document.body,
      props: { thread, mobile: true, onchange }
    });

    document.body.querySelector<HTMLButtonElement>('[aria-label="More actions"]')?.click();
    await Promise.resolve();

    buttonByText('Rename').click();
    await Promise.resolve();

    expect(prompt).not.toHaveBeenCalled();
    const input = document.body.querySelector<HTMLInputElement>('input[aria-label="Thread title"]');
    expect(input).not.toBeNull();
    expect(input?.value).toBe('Original title');

    input!.value = 'Renamed title';
    input!.dispatchEvent(new Event('input', { bubbles: true }));
    buttonByText('Save').click();
    await Promise.resolve();
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(apiMocks.updateMeta).toHaveBeenCalledWith(12, { title: 'Renamed title' });
    expect(onchange).toHaveBeenCalledWith(expect.objectContaining({ title: 'Renamed title' }));
    expect(document.body.querySelector('input[aria-label="Thread title"]')).toBeNull();

    unmount(component);
  });

  it('shows only the current visibility state in the mobile visibility row', async () => {
    const component = mount(ThreadSidebar, {
      target: document.body,
      props: { thread: { ...makeThread(), visibility: 'unlisted' }, mobile: true }
    });

    document.body.querySelector<HTMLButtonElement>('[aria-label="More actions"]')?.click();
    await Promise.resolve();

    const visibilityButton = [...document.body.querySelectorAll<HTMLButtonElement>('button.mobile-menu-row.has-drawer')].find((button) =>
      button.textContent?.includes('Unlisted')
    );

    expect(visibilityButton).toBeTruthy();
    expect(visibilityButton?.textContent?.replace(/\s+/g, '')).toBe('Unlisted›');
    expect(visibilityButton?.textContent).not.toContain('Visibility');

    unmount(component);
  });

  it('shows only the final folder segment for Windows working directories', () => {
    const thread = {
      ...makeThread(),
      repo: '',
      branch: '',
      working_directory: 'C:\\Users\\Administrator\\.claude\\projects\\jieli'
    };

    const component = mount(ThreadSidebar, {
      target: document.body,
      props: { thread, mobile: false }
    });

    expect(document.body.textContent).toContain('jieli');
    expect(document.body.textContent).not.toContain('C:\\Users\\Administrator');

    unmount(component);
  });
});
