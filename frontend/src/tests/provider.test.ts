import { describe, expect, it } from 'vitest';
import { cliCommand, modeLabel, providerThreadUrl } from '../lib/providers/registry';
import type { ThreadDetail } from '../lib/api/types';

function makeThreadDetail(overrides: Partial<ThreadDetail>): ThreadDetail {
  return {
    id: 1,
    provider: 'claude_code',
    provider_thread_id: 'T-test',
    title: 'Test',
    visibility: 'private',
    message_count: 0,
    lines_added: 0,
    lines_removed: 0,
    files_changed: 0,
    labels: [],
    archived: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    messages: [],
    ...overrides
  };
}

describe('cliCommand', () => {
  it('offers no CLI write action for retired Amp sessions', () => {
    expect(cliCommand({ provider: 'amp', provider_thread_id: 'T-ac11e473-f6ec-4508-a231-2fa494a4acd3' })).toBe(
      null
    );
  });

  it('builds the Claude Code resume command from the provider thread id without T prefix', () => {
    expect(
      cliCommand({
        provider: 'claude_code',
        provider_thread_id: 'T-4e5bb892-5198-4556-bdf0-b9fddc069976'
      })
    ).toBe('claude --resume 4e5bb892-5198-4556-bdf0-b9fddc069976');
  });

  it('builds the Codex resume command from the provider thread id without T prefix', () => {
    expect(
      cliCommand({
        provider: 'codex',
        provider_thread_id: 'T-019ea976-2b0d-77d1-b2fd-210aee939e4e'
      })
    ).toBe('codex resume 019ea976-2b0d-77d1-b2fd-210aee939e4e');
  });

  it('builds the OpenCode session command from the provider thread id without T prefix', () => {
    expect(
      cliCommand({
        provider: 'opencode',
        provider_thread_id: 'T-ses_abc123'
      })
    ).toBe('opencode --session ses_abc123');
  });

  it('builds the Pi session command from the provider thread id without T prefix', () => {
    expect(
      cliCommand({
        provider: 'pi',
        provider_thread_id: 'T-019ea976-2b0d-77d1-b2fd-210aee939e4e'
      })
    ).toBe('pi --session 019ea976-2b0d-77d1-b2fd-210aee939e4e');
  });
});

describe('modeLabel', () => {
  it('falls back to model when a thread has no provider mode', () => {
    expect(modeLabel(makeThreadDetail({ mode: '', model: 'gpt-5.5' }))).toBe('gpt-5.5');
  });
});

describe('providerThreadUrl', () => {
  it('uses the uploaded source URL when available', () => {
    expect(
      providerThreadUrl({
        provider: 'amp',
        provider_thread_id: 'T-local',
        source_url: 'https://ampcode.com/threads/T-uploaded'
      })
    ).toBe('https://ampcode.com/threads/T-uploaded');
  });

  it('does not fabricate an external URL for historical Amp sessions', () => {
    expect(
      providerThreadUrl({
        provider: 'amp',
        provider_thread_id: 'T-5a46e197-ecb1-46a8-b5c7-5ea4ef985bad'
      })
    ).toBe(null);
  });
});
