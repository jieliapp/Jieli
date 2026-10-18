import { describe, expect, it } from 'vitest';
import { groupThreadBlocks, groupTurnUnits, summarizeWorkUnits } from '../lib/components/thread/work-groups';
import type { RenderUnit, Turn } from '../lib/components/thread/normalize';
import type { WorkUnit } from '../lib/components/thread/work-groups';

describe('thread work grouping', () => {
  it('groups consecutive thinking and tool units into one work block', () => {
    const units: RenderUnit[] = [
      { kind: 'text', text: 'Before' },
      { kind: 'thinking', text: 'Plan' },
      { kind: 'tool', name: 'Bash', input: { cmd: 'ls' }, output: 'file.txt' },
      { kind: 'text', text: 'After' }
    ];

    expect(groupTurnUnits(units)).toEqual([
      { kind: 'text', text: 'Before' },
      {
        kind: 'workGroup',
        units: [
          { kind: 'thinking', text: 'Plan' },
          { kind: 'tool', name: 'Bash', input: { cmd: 'ls' }, output: 'file.txt' }
        ]
      },
      { kind: 'text', text: 'After' }
    ]);
  });

  it('keeps separated work blocks when normal content appears between tools', () => {
    const units: RenderUnit[] = [
      { kind: 'tool', name: 'Read', input: { path: 'a.ts' } },
      { kind: 'text', text: 'Explanation' },
      { kind: 'tool', name: 'Bash', input: { cmd: 'npm test' } }
    ];

    expect(groupTurnUnits(units)).toEqual([
      { kind: 'workGroup', units: [{ kind: 'tool', name: 'Read', input: { path: 'a.ts' } }] },
      { kind: 'text', text: 'Explanation' },
      { kind: 'workGroup', units: [{ kind: 'tool', name: 'Bash', input: { cmd: 'npm test' } }] }
    ]);
  });

  it('merges adjacent assistant work-only turns into one work block', () => {
    const turns: Turn[] = [
      {
        id: 'turn-0',
        role: 'assistant',
        userIndex: -1,
        units: [{ kind: 'tool', name: 'Read', input: { path: 'a.ts' } }]
      },
      {
        id: 'turn-1',
        role: 'assistant',
        userIndex: -1,
        units: [{ kind: 'tool', name: 'Bash', input: { cmd: 'npm test' } }]
      }
    ];

    expect(groupThreadBlocks(turns)).toEqual([
      {
        kind: 'workGroup',
        id: 'work-0',
        units: [
          { kind: 'tool', name: 'Read', input: { path: 'a.ts' } },
          { kind: 'tool', name: 'Bash', input: { cmd: 'npm test' } }
        ]
      }
    ]);
  });

  it('merges work at the end of a mixed assistant turn with following assistant work turns', () => {
    const turns: Turn[] = [
      {
        id: 'turn-0',
        role: 'assistant',
        userIndex: -1,
        units: [
          { kind: 'text', text: 'Let me first check the state.' },
          { kind: 'tool', name: 'Bash', input: { cmd: 'git status' } }
        ]
      },
      {
        id: 'turn-1',
        role: 'assistant',
        userIndex: -1,
        units: [
          { kind: 'text', text: '   ' },
          { kind: 'tool', name: 'Bash', input: { cmd: 'git diff' } },
          { kind: 'tool', name: 'Read', input: { path: 'AGENTS.md' } }
        ]
      },
      {
        id: 'turn-2',
        role: 'assistant',
        userIndex: -1,
        units: [{ kind: 'text', text: 'Let me try reading the git state differently.' }]
      }
    ];

    expect(groupThreadBlocks(turns)).toEqual([
      {
        kind: 'turn',
        id: 'turn-0-content-0',
        turn: {
          id: 'turn-0',
          role: 'assistant',
          userIndex: -1,
          units: [{ kind: 'text', text: 'Let me first check the state.' }]
        }
      },
      {
        kind: 'workGroup',
        id: 'work-1',
        units: [
          { kind: 'tool', name: 'Bash', input: { cmd: 'git status' } },
          { kind: 'tool', name: 'Bash', input: { cmd: 'git diff' } },
          { kind: 'tool', name: 'Read', input: { path: 'AGENTS.md' } }
        ]
      },
      {
        kind: 'turn',
        id: 'turn-2-content-1',
        turn: {
          id: 'turn-2',
          role: 'assistant',
          userIndex: -1,
          units: [{ kind: 'text', text: 'Let me try reading the git state differently.' }]
        }
      }
    ]);
  });
});

describe('work summary', () => {
  it('summarizes thinking and tool names without reordering units', () => {
    const units: WorkUnit[] = [
      { kind: 'thinking', text: 'Plan' },
      { kind: 'tool', name: 'Read', input: { path: 'a.ts' } },
      { kind: 'tool', name: 'Bash', input: { cmd: 'npm test' } },
      { kind: 'tool', name: 'Bash', input: { cmd: 'npm run build' } }
    ];

    expect(summarizeWorkUnits(units)).toBe('Thought, Read, Bash x2');
  });
});
