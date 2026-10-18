// @vitest-environment jsdom
// @ts-expect-error Vitest resolves `svelte` to the server entry; jsdom needs the client mount API.
import { mount, unmount } from '../../node_modules/svelte/src/index-client.js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ThreadView from '../lib/components/thread/ThreadView.svelte';
import type { ThreadDetail } from '../lib/api/types';

function makeThread(): ThreadDetail {
  return {
    id: 1,
    provider: 'amp',
    provider_thread_id: 'T-scroll',
    title: 'Long Thread',
    visibility: 'private',
    message_count: 2,
    lines_added: 0,
    lines_removed: 0,
    files_changed: 0,
    labels: [],
    archived: false,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    messages: [
      { role: 'user', content: 'first message' },
      { role: 'assistant', content: 'latest message' }
    ]
  };
}

function makeLongThread(turnPairs: number): ThreadDetail {
  const messages = [];
  for (let i = 0; i < turnPairs; i++) {
    messages.push({ role: 'user', content: `user question ${i}` });
    messages.push({ role: 'assistant', content: `assistant answer ${i}` });
  }
  return { ...makeThread(), message_count: messages.length, messages };
}

function flushFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}

function setPageMetrics(scrollY: number, innerHeight: number, scrollHeight: number) {
  Object.defineProperty(window, 'scrollY', { configurable: true, value: scrollY });
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: innerHeight });
  Object.defineProperty(document.documentElement, 'scrollHeight', {
    configurable: true,
    value: scrollHeight
  });
}

describe('ThreadView scroll to latest message', () => {
  afterEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('shows a bottom jump button after scrolling down and scrolls smoothly to the latest message', async () => {
    vi.stubGlobal(
      'IntersectionObserver',
      vi.fn(() => ({
        observe: vi.fn(),
        disconnect: vi.fn()
      }))
    );
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});

    setPageMetrics(500, 800, 2400);
    const component = mount(ThreadView, {
      target: document.body,
      props: { thread: makeThread() }
    });

    window.dispatchEvent(new Event('scroll'));
    await Promise.resolve();

    const button = document.body.querySelector<HTMLButtonElement>('[aria-label="Scroll to latest message"]');
    expect(button).not.toBeNull();

    button?.click();

    await flushFrame();
    expect(scrollTo).toHaveBeenCalledWith({ top: 2400, behavior: 'smooth' });
    unmount(component);
  });

  it('renders the full thread before scrolling to the bottom on long threads', async () => {
    vi.stubGlobal(
      'IntersectionObserver',
      vi.fn(() => ({
        observe: vi.fn(),
        disconnect: vi.fn()
      }))
    );
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});

    // 60 turn pairs => 120 blocks, far beyond the initial 40-block window.
    setPageMetrics(500, 800, 2400);
    const component = mount(ThreadView, {
      target: document.body,
      props: { thread: makeLongThread(60) }
    });
    await Promise.resolve();

    // Only the initial slice is mounted right after first paint.
    const initialMessages = document.body.querySelectorAll('.msg').length;
    expect(initialMessages).toBeLessThan(120);

    window.dispatchEvent(new Event('scroll'));
    await Promise.resolve();

    const button = document.body.querySelector<HTMLButtonElement>('[aria-label="Scroll to latest message"]');
    button?.click();
    await flushFrame();

    // Clicking forces the whole thread to render, then scrolls to the bottom.
    expect(document.body.querySelectorAll('.msg').length).toBe(120);
    expect(scrollTo).toHaveBeenCalledWith({ top: 2400, behavior: 'smooth' });
    unmount(component);
  });

  it('hides the bottom jump button when the latest message is within the final screen', async () => {
    vi.stubGlobal(
      'IntersectionObserver',
      vi.fn(() => ({
        observe: vi.fn(),
        disconnect: vi.fn()
      }))
    );

    setPageMetrics(900, 800, 2400);
    const component = mount(ThreadView, {
      target: document.body,
      props: { thread: makeThread() }
    });

    window.dispatchEvent(new Event('scroll'));
    await Promise.resolve();

    expect(document.body.querySelector('[aria-label="Scroll to latest message"]')).toBeNull();
    unmount(component);
  });
});
