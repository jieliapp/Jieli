import type { RenderUnit, Turn } from './normalize';

export type WorkUnit = Extract<RenderUnit, { kind: 'thinking' | 'tool' }>;

export type RenderGroup =
  | RenderUnit
  | {
      kind: 'workGroup';
      units: WorkUnit[];
    };

export type ThreadBlock =
  | { kind: 'turn'; id: string; turn: Turn }
  | { kind: 'workGroup'; id: string; units: WorkUnit[] };

export function isWorkUnit(unit: RenderUnit): unit is WorkUnit {
  return unit.kind === 'thinking' || unit.kind === 'tool';
}

export function summarizeWorkUnits(units: WorkUnit[]): string {
  const counts = new Map<string, number>();
  const order: string[] = [];

  for (const unit of units) {
    const label = unit.kind === 'thinking' ? 'Thought' : unit.name || 'Tool';
    if (!counts.has(label)) order.push(label);
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }

  return order
    .map((label) => {
      const count = counts.get(label) ?? 0;
      return count > 1 ? `${label} x${count}` : label;
    })
    .join(', ');
}

/** Group consecutive assistant thinking/tool units without reordering surrounding content. */
export function groupTurnUnits(units: RenderUnit[]): RenderGroup[] {
  const groups: RenderGroup[] = [];
  let pending: WorkUnit[] = [];

  function flushWork() {
    if (pending.length === 0) return;
    groups.push({ kind: 'workGroup', units: pending });
    pending = [];
  }

  for (const unit of units) {
    if (isWorkUnit(unit)) {
      pending.push(unit);
    } else {
      flushWork();
      groups.push(unit);
    }
  }

  flushWork();
  return groups;
}

function visibleUnits(units: RenderUnit[]): RenderUnit[] {
  return units.filter((unit) => isWorkUnit(unit) || unit.kind !== 'text' || unit.text.trim() !== '');
}

/** Merge adjacent assistant work across message boundaries without reordering visible content. */
export function groupThreadBlocks(turns: Turn[]): ThreadBlock[] {
  const blocks: ThreadBlock[] = [];
  let pending: WorkUnit[] = [];
  let pendingID: string | null = null;
  let contentID = 0;

  function flushWork() {
    if (pending.length === 0 || pendingID === null) return;
    blocks.push({ kind: 'workGroup', id: pendingID, units: pending });
    pending = [];
    pendingID = null;
  }

  function pushTurn(turn: Turn, units: RenderUnit[]) {
    if (units.length === 0) return;
    flushWork();
    blocks.push({
      kind: 'turn',
      id: `${turn.id}-content-${contentID++}`,
      turn: { ...turn, units }
    });
  }

  for (const turn of turns) {
    const units = visibleUnits(turn.units);
    if (units.length === 0) continue;

    if (turn.role !== 'assistant') {
      pushTurn(turn, units);
      continue;
    }

    let content: RenderUnit[] = [];
    for (const unit of units) {
      if (isWorkUnit(unit)) {
        pushTurn(turn, content);
        content = [];
        pendingID ??= `work-${blocks.length}`;
        pending.push(unit);
      } else {
        flushWork();
        content.push(unit);
      }
    }
    pushTurn(turn, content);
  }

  flushWork();
  return blocks;
}
