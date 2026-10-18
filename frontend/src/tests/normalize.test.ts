import { describe, expect, it } from 'vitest';
import { normalizeMessages, userAnchors } from '../lib/components/thread/normalize';
import type { ThreadDetail } from '../lib/api/types';

function makeThread(messages: unknown[]): ThreadDetail {
  return {
    id: 1,
    provider: 'amp',
    provider_thread_id: 'T-abc',
    title: 'Test',
    visibility: 'private',
    message_count: messages.length,
    lines_added: 0,
    lines_removed: 0,
    files_changed: 0,
    labels: [],
    archived: false,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    data_json: {},
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    messages: messages as any
  };
}

describe('normalizeMessages', () => {
  it('maps roles and assigns stable anchor ids', () => {
    const turns = normalizeMessages(
      makeThread([
        { role: 'user', content: 'hi' },
        { role: 'assistant', content: 'hello' },
        { role: 'user', content: 'again' }
      ])
    );
    expect(turns.map((t) => t.role)).toEqual(['user', 'assistant', 'user']);
    expect(turns.map((t) => t.id)).toEqual(['turn-0', 'turn-1', 'turn-2']);
  });

  it('indexes user question turns sequentially', () => {
    const turns = normalizeMessages(
      makeThread([
        { role: 'user', content: 'a' },
        { role: 'assistant', content: 'b' },
        { role: 'user', content: 'c' }
      ])
    );
    expect(turns[0].userIndex).toBe(0);
    expect(turns[1].userIndex).toBe(-1);
    expect(turns[2].userIndex).toBe(1);
  });

  it('treats human as user and string content as a text unit', () => {
    const turns = normalizeMessages(
      makeThread([{ role: 'human', content: 'x' }])
    );
    expect(turns[0].role).toBe('user');
    expect(turns[0].units).toEqual([{ kind: 'text', text: 'x' }]);
  });

  it('emits text and thinking units from content blocks', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [
            { type: 'thinking', thinking: 'pondering' },
            { type: 'text', text: 'answer' }
          ]
        }
      ])
    );
    expect(turns[0].units).toEqual([
      { kind: 'thinking', text: 'pondering' },
      { kind: 'text', text: 'answer' }
    ]);
  });

  it('pairs tool_use with its tool_result by id', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [{ type: 'tool_use', id: 'tu-1', name: 'Bash', input: { cmd: 'ls' } }]
        },
        {
          role: 'user',
          content: [
            {
              type: 'tool_result',
              toolUseID: 'tu-1',
              run: { result: { output: 'file.txt', exitCode: 0 }, status: 'success' }
            }
          ]
        }
      ])
    );
    expect(turns).toHaveLength(1);
    expect(turns[0].units[0]).toEqual({
      kind: 'tool',
      name: 'Bash',
      input: { cmd: 'ls' },
      output: 'file.txt',
      exitCode: 0,
      status: 'success'
    });
  });

  it('reads Amp and Claude Code tool_result content as tool output', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [{ type: 'tool_use', id: 'tool-1', name: 'Glob', input: { filepattern: '**/*_test.go', limit: 20 } }]
        },
        {
          role: 'tool',
          content: [{ type: 'tool_result', tool_use_id: 'tool-1', content: 'backend/service/thread/upload_test.go' }]
        }
      ])
    );

    expect(turns).toHaveLength(1);
    expect(turns[0].units[0]).toEqual({
      kind: 'tool',
      name: 'Glob',
      input: { filepattern: '**/*_test.go', limit: 20 },
      output: 'backend/service/thread/upload_test.go',
      exitCode: undefined,
      status: undefined
    });
  });

  it('reads native Amp shell result output, exit code, and status', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [{ type: 'tool_use', id: 'tool-1', name: 'Bash', input: { cmd: 'npm test' } }]
        },
        {
          role: 'user',
          content: [
            {
              type: 'tool_result',
              toolUseID: 'tool-1',
              run: { status: 'done', result: { output: 'tests failed', exitCode: 1 } }
            }
          ]
        }
      ])
    );

    expect(turns[0].units[0]).toEqual({
      kind: 'tool',
      name: 'Bash',
      input: { cmd: 'npm test' },
      output: 'tests failed',
      exitCode: 1,
      status: 'done'
    });
  });

  it('maps Claude Code is_error tool results to error status', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [{ type: 'tool_use', id: 'tool-1', name: 'Bash', input: { command: 'npm test' } }]
        },
        {
          role: 'tool',
          content: [{ type: 'tool_result', tool_use_id: 'tool-1', content: 'failed', is_error: true }]
        }
      ])
    );

    expect(turns[0].units[0]).toMatchObject({
      kind: 'tool',
      output: 'failed',
      status: 'error'
    });
  });

  it('reads native Amp tool result arrays as output', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [{ type: 'tool_use', id: 'tool-1', name: 'Grep', input: { pattern: 'TODO' } }]
        },
        {
          role: 'user',
          content: [
            {
              type: 'tool_result',
              toolUseID: 'tool-1',
              run: { status: 'done', result: [{ type: 'text', text: 'src/a.ts: TODO' }] }
            }
          ]
        }
      ])
    );

    expect(turns[0].units[0]).toMatchObject({
      kind: 'tool',
      output: 'src/a.ts: TODO',
      status: 'done'
    });
  });

  it('reads text blocks inside tool_result content arrays', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'assistant',
          content: [{ type: 'tool_use', id: 'tool-1', name: 'Read', input: { path: 'README.md' } }]
        },
        {
          role: 'tool',
          content: [
            {
              type: 'tool_result',
              tool_use_id: 'tool-1',
              content: [{ type: 'text', text: 'file body' }]
            }
          ]
        }
      ])
    );

    expect(turns[0].units[0]).toMatchObject({
      kind: 'tool',
      output: 'file body'
    });
  });

  it('emits image units from image blocks', () => {
    const turns = normalizeMessages(
      makeThread([
        {
          role: 'user',
          content: [
            { type: 'text', text: 'look' },
            { type: 'image', source: { url: '/mock-attachments/a.png', type: 'url' } }
          ]
        }
      ])
    );
    expect(turns[0].units).toEqual([
      { kind: 'text', text: 'look' },
      { kind: 'image', url: '/mock-attachments/a.png' }
    ]);
    expect(turns[0].userIndex).toBe(0);
  });
});

describe('userAnchors', () => {
  it('returns only user question turns with their index', () => {
    const anchors = userAnchors(
      normalizeMessages(
        makeThread([
          { role: 'user', content: 'a' },
          { role: 'assistant', content: 'b' },
          { role: 'user', content: 'c' }
        ])
      )
    );
    expect(anchors).toEqual([
      { id: 'turn-0', index: 0, preview: 'a' },
      { id: 'turn-2', index: 1, preview: 'c' }
    ]);
  });

  it('uses normalized prompt preview text for left navigation', () => {
    const anchors = userAnchors(
      normalizeMessages(
        makeThread([
          {
            role: 'user',
            content: [
              { type: 'text', text: '  First line\n\nSecond   line with extra space  ' },
              { type: 'image', source: { url: '/mock-attachments/a.png', type: 'url' } }
            ]
          },
          {
            role: 'user',
            content: [{ type: 'image', source: { url: '/mock-attachments/b.png', type: 'url' } }]
          }
        ])
      )
    );

    expect(anchors).toEqual([
      { id: 'turn-0', index: 0, preview: 'First line Second line with extra space' },
      { id: 'turn-1', index: 1, preview: 'Image attachment' }
    ]);
  });
});
