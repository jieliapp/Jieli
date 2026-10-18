import type { ThreadDetail, ThreadMessage, ContentBlock } from '$lib/api/types';

/** A single rendered unit within a turn. */
export type RenderUnit =
  | { kind: 'text'; text: string }
  | { kind: 'thinking'; text: string }
  | {
      kind: 'tool';
      name?: string;
      input: unknown;
      output?: string;
      exitCode?: number;
      status?: string;
    }
  | { kind: 'image'; url: string };

/** A turn corresponds to one source message and its ordered render units. */
export interface Turn {
  /** Stable anchor id for navigation. */
  id: string;
  role: 'user' | 'assistant' | 'tool';
  /** Index among user-question turns (text/image), -1 otherwise. */
  userIndex: number;
  units: RenderUnit[];
}

function roleOf(raw: ThreadMessage): 'user' | 'assistant' | 'tool' {
  const r = String(raw.role ?? '').toLowerCase();
  if (r === 'user' || r === 'human') return 'user';
  if (r === 'tool') return 'tool';
  return 'assistant';
}

function asBlocks(content: ThreadMessage['content']): ContentBlock[] {
  if (typeof content === 'string') {
    return content ? [{ type: 'text', text: content }] : [];
  }
  if (Array.isArray(content)) {
    return content.map((b) =>
      typeof b === 'string' ? ({ type: 'text', text: b } as ContentBlock) : b
    );
  }
  return [];
}

function blockText(block: ContentBlock): string {
  const b = block as { text?: unknown; thinking?: unknown };
  if (typeof b.text === 'string') return b.text;
  if (typeof b.thinking === 'string') return b.thinking;
  return '';
}

function resultID(block: ContentBlock): string | undefined {
  const b = block as { toolUseID?: unknown; tool_use_id?: unknown; id?: unknown };
  if (typeof b.toolUseID === 'string') return b.toolUseID;
  if (typeof b.tool_use_id === 'string') return b.tool_use_id;
  if (typeof b.id === 'string') return b.id;
  return undefined;
}

interface ToolResultData {
  output?: string;
  exitCode?: number;
  status?: string;
}

function resultContent(content: unknown): string | undefined {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return undefined;

  const parts = content
    .map((item) => {
      if (typeof item === 'string') return item;
      if (!item || typeof item !== 'object') return '';
      const block = item as ContentBlock;
      if (block.type === 'text' || block.type === 'thinking') return blockText(block);
      return JSON.stringify(block);
    })
    .filter((part) => part.trim() !== '');

  return parts.length ? parts.join('\n') : undefined;
}

function resultOutput(result: unknown, fallbackContent: unknown): string | undefined {
  if (typeof result === 'string') return result;
  if (Array.isArray(result)) return resultContent(result);
  if (result && typeof result === 'object') {
    const obj = result as { output?: unknown; content?: unknown };
    if (typeof obj.output === 'string') return obj.output;
    return resultContent(obj.content);
  }
  return resultContent(fallbackContent);
}

function resultExitCode(result: unknown): number | undefined {
  if (!result || typeof result !== 'object' || Array.isArray(result)) return undefined;
  const exitCode = (result as { exitCode?: unknown; exit_code?: unknown }).exitCode ?? (result as { exit_code?: unknown }).exit_code;
  return typeof exitCode === 'number' ? exitCode : undefined;
}

function resultData(block: ContentBlock): ToolResultData {
  const b = block as {
    content?: unknown;
    is_error?: unknown;
    run?: { result?: unknown; status?: string };
  };
  const run = b.run;
  return {
    output: resultOutput(run?.result, b.content),
    exitCode: resultExitCode(run?.result),
    status: run?.status ?? (b.is_error === true ? 'error' : undefined)
  };
}

/** Normalize provider messages into renderable, anchor-stable turns. */
export function normalizeMessages(thread: ThreadDetail): Turn[] {
  const messages = thread.messages ?? [];

  // First pass: index tool_result blocks by the tool_use id they answer.
  const resultsByID = new Map<string, ToolResultData>();
  for (const raw of messages) {
    for (const block of asBlocks(raw.content)) {
      if (block.type === 'tool_result') {
        const id = resultID(block);
        if (id) resultsByID.set(id, resultData(block));
      }
    }
  }

  let userCount = 0;
  const turns: Turn[] = [];

  messages.forEach((raw, i) => {
    const role = roleOf(raw);
    const units: RenderUnit[] = [];

    for (const block of asBlocks(raw.content)) {
      switch (block.type) {
        case 'text': {
          const text = blockText(block);
          if (text) units.push({ kind: 'text', text });
          break;
        }
        case 'thinking': {
          const text = blockText(block);
          if (text) units.push({ kind: 'thinking', text });
          break;
        }
        case 'tool_use': {
          const b = block as { id?: string; name?: string; input?: unknown };
          const result = b.id ? resultsByID.get(b.id) : undefined;
          units.push({
            kind: 'tool',
            name: b.name,
            input: b.input,
            output: result?.output,
            exitCode: result?.exitCode,
            status: result?.status
          });
          break;
        }
        case 'image': {
          const url = (block as { source?: { url?: string } }).source?.url;
          if (url) units.push({ kind: 'image', url });
          break;
        }
        // tool_result blocks are consumed via pairing above.
      }
    }

    if (units.length === 0) return;

    const isQuestion =
      role === 'user' && units.some((u) => u.kind === 'text' || u.kind === 'image');
    const userIndex = isQuestion ? userCount++ : -1;

    turns.push({ id: `turn-${i}`, role, userIndex, units });
  });

  return turns;
}

export interface UserAnchor {
  id: string;
  index: number;
  preview: string;
}

function promptPreview(units: RenderUnit[]): string {
  const text = units
    .filter((unit): unit is Extract<RenderUnit, { kind: 'text' }> => unit.kind === 'text')
    .map((unit) => unit.text)
    .join(' ')
    .replace(/\s+/g, ' ')
    .trim();

  if (text) return text;
  if (units.some((unit) => unit.kind === 'image')) return 'Image attachment';
  return '';
}

/** Extract user-question anchors for the left navigation rail. */
export function userAnchors(turns: Turn[]): UserAnchor[] {
  return turns
    .filter((turn) => turn.userIndex >= 0)
    .map((turn) => ({ id: turn.id, index: turn.userIndex, preview: promptPreview(turn.units) }));
}
