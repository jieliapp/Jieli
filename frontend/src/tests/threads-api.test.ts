import { afterEach, describe, expect, it, vi } from 'vitest';
import { threadsApi } from '../lib/api/threads';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('threadsApi.recent', () => {
  it('requests only unarchived threads before applying the recent limit', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ success: true, message: '', data: { threads: [] } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    await threadsApi.recent(9);

    const requestUrl = new URL(String(fetchMock.mock.calls[0]?.[0]), 'http://localhost');
    expect(requestUrl.pathname).toBe('/web/api/threads');
    expect(Object.fromEntries(requestUrl.searchParams)).toEqual({
      page_size: '9',
      page: '1',
      sort: 'updated',
      archived: 'false'
    });
  });
});
