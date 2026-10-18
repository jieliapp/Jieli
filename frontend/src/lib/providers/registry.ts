import type { Provider, ThreadDetail, ThreadSummary } from '../api/types';

export interface ProviderMeta {
  id: Provider;
  label: string;
}

const PROVIDERS: Record<Provider, ProviderMeta> = {
  amp: { id: 'amp', label: 'Amp' },
  claude_code: { id: 'claude_code', label: 'Claude Code' },
  codex: { id: 'codex', label: 'Codex' },
  opencode: { id: 'opencode', label: 'OpenCode' },
  pi: { id: 'pi', label: 'Pi' },
  gemini: { id: 'gemini', label: 'Gemini CLI' },
  github_copilot: { id: 'github_copilot', label: 'GitHub Copilot' },
  antigravity: { id: 'antigravity', label: 'Antigravity' },
  qwen_code: { id: 'qwen_code', label: 'Qwen Code' },
  cursor: { id: 'cursor', label: 'Cursor' },
  windsurf: { id: 'windsurf', label: 'Windsurf' },
  droid: { id: 'droid', label: 'Droid' }
};

export function providerMeta(provider: Provider): ProviderMeta {
  return PROVIDERS[provider];
}

/** Generate the provider-aware "open in CLI" command. */
export function cliCommand(thread: Pick<ThreadSummary, 'provider' | 'provider_thread_id'>): string | null {
  if (thread.provider === 'claude_code' && thread.provider_thread_id) {
    return `claude --resume ${providerSessionID(thread.provider_thread_id)}`;
  }
  if (thread.provider === 'codex' && thread.provider_thread_id) {
    return `codex resume ${providerSessionID(thread.provider_thread_id)}`;
  }
  if (thread.provider === 'opencode' && thread.provider_thread_id) {
    return `opencode --session ${providerSessionID(thread.provider_thread_id)}`;
  }
  if (thread.provider === 'pi' && thread.provider_thread_id) {
    return `pi --session ${providerSessionID(thread.provider_thread_id)}`;
  }
  return null;
}

/** Generate the canonical provider web URL for a synced thread. */
export function providerThreadUrl(
  thread: Pick<ThreadSummary, 'provider' | 'provider_thread_id' | 'source_url'>
): string | null {
  if (thread.source_url) return thread.source_url;
  return null;
}

/** Optional mode/context extraction for the detail sidebar. */
export function modeLabel(thread: ThreadDetail): string | null {
  return thread.mode || thread.model || null;
}

export function contextPercent(thread: ThreadDetail): number | null {
  return typeof thread.context_percent === 'number'
    ? thread.context_percent
    : null;
}

function providerSessionID(providerThreadID: string): string {
  return providerThreadID.startsWith('T-') ? providerThreadID.slice(2) : providerThreadID;
}
