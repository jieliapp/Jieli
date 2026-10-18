// @vitest-environment jsdom
// @ts-expect-error Vitest resolves `svelte` to the server entry; jsdom needs the client mount API.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ThreadPage from '../routes/threads/[id]/+page.svelte';
import type { ThreadDetail } from '../lib/api/types';

const apiMocks = vi.hoisted(() => ({
  detail: vi.fn()
}));

// @ts-expect-error Vitest resolves `svelte` to the server entry; page onMount needs the client runtime.
vi.mock('svelte', async () => await import('../../node_modules/svelte/src/index-client.js'));

vi.mock('$app/stores', () => ({
  page: {
    subscribe(run: (value: { params: { id: string } }) => void) {
      run({ params: { id: 'T-browser-title' } });
      return () => {};
    }
  }
}));

vi.mock('$lib/api/threads', () => ({
  threadsApi: {
    detail: apiMocks.detail
  }
}));

function makeThread(): ThreadDetail {
  return {
    id: 1,
    provider: 'amp',
    provider_thread_id: 'T-browser-title',
    title: 'Fix browser tab title',
    visibility: 'private',
    message_count: 0,
    lines_added: 0,
    lines_removed: 0,
    files_changed: 0,
    labels: [],
    archived: false,
    created_at: '2026-09-06T00:00:00Z',
    updated_at: '2026-09-06T00:00:00Z',
    messages: []
  };
}

describe('thread detail browser title', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        observe() {}
        disconnect() {}
      }
    );
  });

  afterEach(() => {
    document.body.innerHTML = '';
    document.title = '';
    apiMocks.detail.mockReset();
    vi.unstubAllGlobals();
  });

  it('uses the loaded thread title in the browser tab', async () => {
    apiMocks.detail.mockResolvedValue(makeThread());
    const component = mount(ThreadPage, { target: document.body });

    await vi.waitFor(() => {
      expect(apiMocks.detail).toHaveBeenCalledWith('T-browser-title');
      expect(document.body.textContent).toContain('Fix browser tab title');
      expect(document.title).toBe('Fix browser tab title · Jieli');
    });

    unmount(component);
  });
});
