<script lang="ts">
  import { t } from '$lib/stores/locale';
  import { session } from '$lib/stores/session';
  import { providerMeta } from '$lib/providers/registry';
  import { timeAgo } from '$lib/utils';
  import type { ThreadSummary } from '$lib/api/types';

  let { thread, search = '' }: { thread: ThreadSummary; search?: string } = $props();

  const provider = $derived(providerMeta(thread.provider));
  const user = $derived($session.user);
  const initial = $derived((user?.username || '?').charAt(0).toUpperCase());
  const searchParts = $derived(search.trim().split(/\s+/).filter(Boolean));

  function formatFileURI(value: string): string {
    if (!value) return '';
    const withoutScheme = value.startsWith('file://') ? value.slice('file://'.length) : value;
    try {
      return decodeURIComponent(withoutScheme);
    } catch {
      return withoutScheme;
    }
  }

  function basenamePath(value: string): string {
    if (!value) return '';
    const trimmed = value.replace(/[\\/]+$/, '');
    const parts = trimmed.split(/[\\/]+/).filter(Boolean);
    return parts.at(-1) ?? trimmed;
  }

  const repoLabel = $derived(
    thread.repo ? (thread.branch ? `${thread.repo}:${thread.branch}` : thread.repo) : ''
  );
  const folderLabel = $derived(repoLabel ? '' : basenamePath(formatFileURI(thread.working_directory || '')));

  type HighlightSegment = { text: string; hit: boolean };

  function matchAt(text: string, index: number): string {
    const lower = text.toLocaleLowerCase();
    for (const part of searchParts) {
      const needle = part.toLocaleLowerCase();
      if (needle && lower.startsWith(needle, index)) return text.slice(index, index + part.length);
    }
    return '';
  }

  function highlightSegments(text: string): HighlightSegment[] {
    if (!searchParts.length || !text) return [{ text, hit: false }];
    const segments: HighlightSegment[] = [];
    let buffer = '';
    let index = 0;
    while (index < text.length) {
      const hit = matchAt(text, index);
      if (hit) {
        if (buffer) {
          segments.push({ text: buffer, hit: false });
          buffer = '';
        }
        segments.push({ text: hit, hit: true });
        index += hit.length;
      } else {
        buffer += text[index];
        index += 1;
      }
    }
    if (buffer) segments.push({ text: buffer, hit: false });
    return segments;
  }
</script>

{#snippet Highlight(text: string)}
  {#each highlightSegments(text) as segment, index (`${index}-${segment.text}`)}
    {#if segment.hit}
      <mark>{segment.text}</mark>
    {:else}
      {segment.text}
    {/if}
  {/each}
{/snippet}

<a class="card" href={`/threads/${thread.provider_thread_id}`}>
  {#if user?.avatar_url}
    <img class="avatar" src={user.avatar_url} alt={user.username} />
  {:else}
    <span class="avatar" aria-hidden="true">{initial}</span>
  {/if}

  <div class="body">
    <div class="head">
      <span class="title">{@render Highlight(thread.title || thread.provider_thread_id)}</span>
      {#if thread.archived}
        <span class="archived-badge" title={$t('threads.archived')} aria-label={$t('threads.archived')}>
          <svg class="archived-icon" viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
            <path
              fill="currentColor"
              d="M2 2.5A1.5 1.5 0 0 1 3.5 1h9A1.5 1.5 0 0 1 14 2.5v1A1.5 1.5 0 0 1 13 4.91V13a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4.91A1.5 1.5 0 0 1 2 3.5v-1ZM3.5 2a.5.5 0 0 0-.5.5v1a.5.5 0 0 0 .5.5h9a.5.5 0 0 0 .5-.5v-1a.5.5 0 0 0-.5-.5h-9ZM4 5v8a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V5H4Zm2 2h4v1H6V7Z"
            />
          </svg>
          <span class="archived-text">{$t('threads.archived')}</span>
        </span>
      {/if}
      <span
        class={`visibility-badge ${thread.visibility === 'unlisted' ? 'visibility-badge-unlisted' : 'visibility-badge-private'}`}
        title={thread.visibility === 'unlisted' ? $t('threads.unlisted') : $t('threads.private')}
        aria-label={thread.visibility === 'unlisted' ? $t('threads.unlisted') : $t('threads.private')}
      >
        {#if thread.visibility === 'unlisted'}
          <svg class="visibility-icon" viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
            <path
              fill="currentColor"
              d="M7.775 3.275a.75.75 0 0 0 1.06 1.06l1.25-1.25a2 2 0 1 1 2.83 2.83l-2.5 2.5a2 2 0 0 1-2.83 0 .75.75 0 0 0-1.06 1.06 3.5 3.5 0 0 0 4.95 0l2.5-2.5a3.5 3.5 0 0 0-4.95-4.95l-1.25 1.25Zm-4.69 9.64a2 2 0 0 1 0-2.83l2.5-2.5a2 2 0 0 1 2.83 0 .75.75 0 0 0 1.06-1.06 3.5 3.5 0 0 0-4.95 0l-2.5 2.5a3.5 3.5 0 0 0 4.95 4.95l1.25-1.25a.75.75 0 0 0-1.06-1.06l-1.25 1.25a2 2 0 0 1-2.83 0Z"
            />
          </svg>
          <span class="visibility-text">{$t('threads.unlisted')}</span>
        {:else}
          <svg class="visibility-icon" viewBox="0 0 12 12" width="10" height="10" aria-hidden="true">
            <path
              fill="currentColor"
              d="M6 0a2.5 2.5 0 0 0-2.5 2.5V4H3a1 1 0 0 0-1 1v5a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V5a1 1 0 0 0-1-1h-.5V2.5A2.5 2.5 0 0 0 6 0Zm1.5 4h-3V2.5a1.5 1.5 0 0 1 3 0V4Z"
            />
          </svg>
          <span class="visibility-text">{$t('threads.private')}</span>
        {/if}
      </span>
    </div>

    <div class="meta">
      <span class="provider">{provider.label}</span>
      <span class="sep">·</span>
      <span>{timeAgo(thread.updated_at)}</span>
      <span class="sep">·</span>
      <span>{thread.message_count} {$t('threads.messages')}</span>
      {#if repoLabel}
        <span class="sep">·</span>
        <span class="repo">
          <svg class="repo-icon" viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
            <path
              fill="currentColor"
              d="M2 1.75A1.75 1.75 0 0 1 3.75 0h9.5a.75.75 0 0 1 .75.75v11.5a.75.75 0 0 1-.75.75H3.75a.25.25 0 0 0-.25.25v.5c0 .14.11.25.25.25h9.5a.75.75 0 0 1 0 1.5h-9.5A1.75 1.75 0 0 1 2 13.75Zm1.5 10.02c.08-.01.17-.02.25-.02h8.75V1.5H3.75a.25.25 0 0 0-.25.25Z"
            />
          </svg>
          {@render Highlight(repoLabel)}
        </span>
      {:else if folderLabel}
        <span class="sep">·</span>
        <span class="repo">
          <svg class="repo-icon" viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
            <path
              fill="none"
              stroke="currentColor"
              stroke-width="1.2"
              stroke-linejoin="round"
              d="M1.5 4.25a1.25 1.25 0 0 1 1.25-1.25h3.3a1 1 0 0 1 .8.4l.6.8a1 1 0 0 0 .8.4h4.7a1.25 1.25 0 0 1 1.25 1.25v7.25a1 1 0 0 1-1 1H2.5a1 1 0 0 1-1-1Z"
            />
          </svg>
          {@render Highlight(folderLabel)}
        </span>
      {/if}
    </div>

    {#if thread.preview}
      <p class="preview"><span class="preview-text">{@render Highlight(thread.preview)}</span></p>
    {/if}

    {#if thread.labels && thread.labels.length}
      <div class="labels">
        {#each thread.labels as label (label)}
          <span class="label">{@render Highlight(label)}</span>
        {/each}
      </div>
    {/if}
  </div>
</a>

<style>
  .card {
    display: flex;
    gap: var(--space-3);
    background: transparent;
    border-radius: var(--radius-md);
    padding: var(--space-4) var(--space-3);
    color: var(--color-text);
    transition: background 0.15s ease;
  }

  .card:hover {
    background: var(--color-bg-hover);
  }

  .avatar {
    flex-shrink: 0;
    width: 32px;
    height: 32px;
    border-radius: var(--radius-full);
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: var(--text-sm);
    font-weight: 600;
    margin-top: 1px;
    object-fit: cover;
  }

  .body {
    flex: 1;
    min-width: 0;
  }

  .head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-1);
  }

  .title {
    flex: 1 1 auto;
    min-width: 0;
    font-weight: 600;
    font-size: var(--text-base);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .visibility-badge {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-height: 24px;
    padding: 2px 10px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: 600;
    line-height: 1.2;
    white-space: nowrap;
  }

  .archived-badge {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1);
    min-height: 24px;
    padding: 2px var(--space-2);
    border: 1px solid rgba(166, 124, 82, 0.3);
    border-radius: var(--radius-full);
    background: rgba(166, 124, 82, 0.14);
    color: var(--color-warning);
    font-size: var(--text-xs);
    font-weight: 500;
    line-height: 1.4;
    white-space: nowrap;
  }

  .archived-icon {
    flex: 0 0 auto;
  }

  .visibility-badge-private {
    background: var(--color-bg-inset);
    color: var(--color-text-muted);
  }

  .visibility-badge-unlisted {
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
    border-color: color-mix(in srgb, var(--color-accent) 35%, var(--color-border));
    color: var(--color-accent);
  }

  .visibility-icon {
    flex: 0 0 auto;
  }

  .meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
  }

  .provider {
    color: var(--color-text-muted);
  }

  .sep {
    opacity: 0.5;
  }

  .repo {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }

  .repo-icon {
    flex: 0 0 auto;
  }

  .preview {
    margin: var(--space-3) 0 0;
    padding: 0.6rem 0.85rem;
    background: #e3e3e1;
    border-radius: var(--radius-lg);
    color: #0b0c0b;
    font-size: var(--text-sm);
    line-height: 1.5;
  }

  .preview-text {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .labels {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }

  .label {
    font-size: var(--text-xs);
    padding: 1px var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--color-bg-inset);
    color: var(--color-text-muted);
  }

  mark {
    padding: 0 1px;
    border-radius: 2px;
    background: #fff2a8;
    color: inherit;
  }

  @media (max-width: 520px) {
    .card {
      padding: var(--space-3);
    }

    .head {
      align-items: flex-start;
    }

    .visibility-badge {
      width: 24px;
      height: 24px;
      min-height: 24px;
      padding: 0;
      align-self: flex-start;
    }

    .archived-badge {
      width: 24px;
      height: 24px;
      min-height: 24px;
      padding: 0;
      align-self: flex-start;
    }

    .archived-icon {
      width: 11px;
      height: 11px;
    }

    .visibility-badge .visibility-icon {
      width: 10px;
      height: 10px;
    }

    .visibility-text,
    .archived-text {
      position: absolute;
      width: 1px;
      height: 1px;
      padding: 0;
      margin: -1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
      border: 0;
    }
  }
</style>
