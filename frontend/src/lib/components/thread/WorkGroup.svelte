<script lang="ts">
  import { t } from '$lib/stores/locale';
  import { editFileInfo, stringInput } from './file-diff';
  import MarkdownView from './MarkdownView.svelte';
  import { formatToolOutputBlock } from './tool-output-format';
  import type { WorkUnit } from './work-groups';

  let { units, bodyFor }: { units: WorkUnit[]; bodyFor: (unit: WorkUnit) => string } = $props();

  let open = $state(false);
  let openItems = $state(new Set<number>());

  function toggleItem(index: number) {
    const next = new Set(openItems);
    if (next.has(index)) {
      next.delete(index);
    } else {
      next.add(index);
    }
    openItems = next;
  }

  function unitLabel(unit: WorkUnit): string {
    if (unit.kind === 'thinking') return $t('detail.thinking');
    const command = bashCommand(unit);
    if (command) return `$ ${command}`;
    const name = normalizeToolName(unit.name);
    const input = unit.input as Record<string, unknown> | null;
    const edit = editFileInfo(unit);
    if (edit) {
      return `${edit.verb} ${edit.file || 'file'}`;
    }
    if (name === 'read' || name === 'view_media') {
      const path = stringInput(input, 'path', 'file_path', 'filePath');
      const range = lineRangeLabel(input);
      return `Read ${compactPath(path) || 'file'}${range ? ` ${range}` : ''}`;
    }
    if (name === 'grep') {
      const path = typeof input?.path === 'string' ? compactPath(input.path) : 'files';
      const pattern = typeof input?.pattern === 'string' ? input.pattern : '';
      return pattern ? `Searched ${path}: ${pattern}` : `Searched ${path}`;
    }
    if (name === 'glob') {
      const pattern = typeof input?.filePattern === 'string' ? input.filePattern : '';
      return pattern ? `Listed ${pattern}` : 'Listed files';
    }
    if (name === 'search_github') {
      const pattern = typeof input?.pattern === 'string' ? input.pattern : '';
      return pattern ? `Searched GitHub: ${pattern}` : 'Searched GitHub';
    }
    if (name === 'commit_search') return 'Searched commits';
    if (name === 'diff') return 'Compared changes';
    if (name === 'code_review') return 'Ran code review';
    if (name === 'skill') {
      const skill = typeof input?.name === 'string' ? input.name : '';
      return skill ? `Used skill ${skill}` : 'Used skill';
    }
    return unit.name ? `Ran tool ${unit.name}` : $t('detail.toolCall');
  }

  function isBashTool(unit: WorkUnit): boolean {
    if (unit.kind !== 'tool') return false;
    const name = normalizeToolName(unit.name);
    return name === 'bash' || name === 'shell_command';
  }

  function normalizeToolName(name?: string): string {
    return (name ?? '').trim().toLowerCase();
  }

  function bashCommand(unit: WorkUnit): string {
    if (unit.kind !== 'tool' || !isBashTool(unit)) return '';
    const input = unit.input as { cmd?: unknown; command?: unknown } | null;
    if (input && typeof input.cmd === 'string') return input.cmd;
    if (input && typeof input.command === 'string') return input.command;
    return '';
  }

  function isCommandLabel(unit: WorkUnit): boolean {
    return Boolean(bashCommand(unit));
  }

  function commandFailed(unit: WorkUnit): boolean {
    if (unit.kind !== 'tool' || !isBashTool(unit)) return false;
    if (typeof unit.exitCode === 'number') return unit.exitCode !== 0;
    const status = unit.status?.toLowerCase();
    return status === 'failed' || status === 'failure' || status === 'error' || status === 'errored';
  }

  function isExpandable(unit: WorkUnit): boolean {
    const edit = editFileInfo(unit);
    if (unit.kind === 'thinking') return Boolean(unit.text?.trim());
    if (edit) return true;
    if (unit.kind === 'tool' && isBashTool(unit)) return Boolean(unit.output);
    if (unit.kind === 'tool') return hasToolInput(unit) || Boolean(unit.output);
    return true;
  }

  function compactPath(path: string): string {
    if (!path) return '';
    const decoded = path.startsWith('file://') ? path.slice('file://'.length) : path;
    const parts = decoded.replace(/\/+$/, '').split('/').filter(Boolean);
    if (parts.length <= 2) return decoded;
    return parts.slice(-2).join('/');
  }

  function lineRangeLabel(input: Record<string, unknown> | null): string {
    if (!input) return '';
    const readRange = input.read_range ?? input.readRange ?? input.range;
    if (Array.isArray(readRange)) {
      const start = lineNumber(readRange[0]);
      const end = lineNumber(readRange[1]);
      if (start !== null && end !== null) return `L${start}-${end}`;
      if (start !== null) return `L${start}`;
    }

    const start =
      lineNumber(input.start_line) ??
      lineNumber(input.startLine) ??
      lineNumber(input.line_start) ??
      lineNumber(input.lineStart) ??
      lineNumber(input.offset);
    const end =
      lineNumber(input.end_line) ??
      lineNumber(input.endLine) ??
      lineNumber(input.line_end) ??
      lineNumber(input.lineEnd);
    if (start !== null && end !== null) return `L${start}-${end}`;
    if (start !== null) return `L${start}`;
    return '';
  }

  function lineNumber(value: unknown): number | null {
    if (typeof value === 'number' && Number.isFinite(value)) return Math.trunc(value);
    if (typeof value === 'string' && /^\d+$/.test(value.trim())) return Number(value.trim());
    return null;
  }

  function hasToolInput(unit: WorkUnit): boolean {
    if (unit.kind !== 'tool' || unit.input === undefined || unit.input === null) return false;
    if (typeof unit.input === 'object' && Object.keys(unit.input).length === 0) return false;
    return true;
  }

  function visibleInput(unit: WorkUnit): unknown {
    if (unit.kind !== 'tool' || !isReadLikeTool(unit)) return unit.kind === 'tool' ? unit.input : undefined;
    if (!unit.input || typeof unit.input !== 'object' || Array.isArray(unit.input)) return unit.input;

    const next = { ...(unit.input as Record<string, unknown>) };
    for (const key of lineRangeKeys) {
      delete next[key];
    }
    return next;
  }

  const lineRangeKeys = [
    'read_range',
    'readRange',
    'range',
    'start_line',
    'startLine',
    'line_start',
    'lineStart',
    'offset',
    'end_line',
    'endLine',
    'line_end',
    'lineEnd'
  ];

  function isReadLikeTool(unit: WorkUnit): boolean {
    if (unit.kind !== 'tool') return false;
    const name = normalizeToolName(unit.name);
    return name === 'read' || name === 'view_media';
  }

  function inputRows(input: unknown): Array<{ key: string; value: string; multiline: boolean }> {
    if (!input || typeof input !== 'object' || Array.isArray(input)) return [];
    return Object.entries(input).map(([key, value]) => {
      const formatted = formatToolValue(value);
      return { key, value: formatted, multiline: formatted.includes('\n') };
    });
  }

  function formatToolValue(value: unknown): string {
    if (typeof value === 'string') return value;
    if (typeof value === 'number' || typeof value === 'boolean') return String(value);
    if (value === null) return 'null';
    if (value === undefined) return '';
    return JSON.stringify(value, null, 2);
  }

  function rawInput(input: unknown): string {
    return typeof input === 'string' ? input : JSON.stringify(input, null, 2);
  }

</script>

<div class="work-group">
  <div class="outer-line">
    <span class="rule"></span>
    <button class="outer-toggle" onclick={() => (open = !open)} aria-expanded={open}>
      {open ? $t('detail.hideWork') : $t('detail.showWork')}
      <span class="chev" class:open>›</span>
    </button>
    <span class="rule"></span>
  </div>

  {#if open}
    <div class="items">
      {#each units as unit, index (index)}
        {@const expandable = isExpandable(unit)}
        <section class="item">
          <button
            class="item-toggle"
            class:command-item={isCommandLabel(unit)}
            class:static-item={!expandable}
            onclick={() => {
              if (expandable) toggleItem(index);
            }}
            aria-expanded={expandable ? openItems.has(index) : undefined}
          >
            {#if expandable}
              <span class="item-chev" class:open={openItems.has(index)}>›</span>
            {/if}
            {#if isCommandLabel(unit)}
              <span class="command-prompt" class:failure={commandFailed(unit)}>$</span>
              <span class="item-label command-label">{bashCommand(unit)}</span>
            {:else}
              <span class="item-label">{unitLabel(unit)}</span>
            {/if}
          </button>
          {#if expandable && openItems.has(index)}
            {@const edit = editFileInfo(unit)}
            {#if edit || unit.kind === 'thinking' || (unit.kind === 'tool' && isBashTool(unit) && unit.output) || (unit.kind === 'tool' && !isBashTool(unit)) || unit.kind !== 'tool'}
              <div class="item-body" class:command-body={isCommandLabel(unit)}>
                {#if unit.kind === 'thinking'}
                <MarkdownView source={unit.text} />
                {:else if edit}
                <div class="edit-file">
                  <div class="edit-head">
                    <span class="edit-title">{edit.verb} <span class="file-name" title={edit.path}>{edit.file}</span></span>
                    <span class="edit-stats"><span class="added">+{edit.added}</span> <span class="removed">-{edit.removed}</span></span>
                  </div>
                  <div class="diff-panel">
                    <div class="diff-lines">
                      {#each edit.rows as row, rowIndex (rowIndex)}
                        <div class="diff-row {row.kind}">
                          <span class="diff-mark">{row.kind === 'added' ? '+' : row.kind === 'removed' ? '-' : ' '}</span>
                          <pre>{row.text || ' '}</pre>
                        </div>
                      {/each}
                    </div>
                  </div>
                </div>
                {:else if unit.kind === 'tool' && isBashTool(unit)}
                <div class="terminal-call">
                  {#if unit.output}
                    <pre class="terminal-output">{unit.output}</pre>
                  {/if}
                </div>
                {:else if unit.kind === 'tool'}
                {@const input = visibleInput(unit)}
                <div class="tool-detail">
                  {#if input !== undefined && input !== null && !(typeof input === 'object' && !Array.isArray(input) && Object.keys(input).length === 0)}
                    {@const rows = inputRows(input)}
                    {#if rows.length > 0}
                      <div class="input-fields">
                        {#each rows as row (row.key)}
                          <div class="input-field" class:multiline={row.multiline}>
                            <span class="input-key">{row.key}</span>
                            <pre class="input-value">{row.value}</pre>
                          </div>
                        {/each}
                      </div>
                    {:else}
                      <pre class="json-block">{rawInput(input)}</pre>
                    {/if}
                  {/if}
                  {#if unit.output}
                    {@const output = formatToolOutputBlock(unit.output)}
                    <pre class="tool-output">{output.value}</pre>
                  {/if}
                </div>
                {:else}
                <MarkdownView source={bodyFor(unit)} />
                {/if}
              </div>
            {/if}
          {/if}
        </section>
      {/each}
    </div>
  {/if}
</div>

<style>
  .work-group {
    margin: 1.2rem 0 1.7rem;
    max-width: 100%;
    margin-left: auto;
    margin-right: auto;
  }

  .outer-line {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
  }

  .rule {
    height: 1px;
    width: min(120px, 18vw);
    flex: 0 1 120px;
    background: color-mix(in srgb, var(--color-border) 55%, transparent);
  }

  .outer-toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    max-width: min(520px, 100%);
    padding: 0.08rem var(--space-2);
    background: var(--color-bg);
    border: none;
    border-radius: var(--radius-full);
    color: var(--color-text-muted);
    font-size: 0.78rem;
    font-weight: 500;
    cursor: pointer;
  }

  .outer-toggle:hover {
    color: var(--color-text);
  }

  .chev,
  .item-chev {
    display: inline-block;
    transition: transform 0.15s;
  }

  .chev.open,
  .item-chev.open {
    transform: rotate(90deg);
  }

  .items {
    margin: var(--space-3) 0 0;
    padding-left: 0;
    border-left: none;
  }

  .item + .item {
    margin-top: 0.55rem;
  }

  .item-toggle {
    display: grid;
    grid-template-columns: 1rem minmax(0, 1fr);
    align-items: center;
    column-gap: var(--space-2);
    width: 100%;
    margin: 0 calc(var(--space-1) * -1);
    padding: 0.08rem var(--space-1);
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    font-size: 0.8rem;
    line-height: 1.45;
    text-align: left;
    cursor: pointer;
  }

  .item-toggle.command-item {
    grid-template-columns: 1rem 1rem minmax(0, 1fr);
  }

  .item-toggle.static-item {
    grid-template-columns: minmax(0, 1fr);
    cursor: default;
  }

  .item-toggle.static-item.command-item {
    grid-template-columns: 1rem minmax(0, 1fr);
  }

  .item-toggle:hover {
    background: var(--color-bg-hover);
    color: var(--color-text);
  }

  .item-toggle.static-item:hover {
    background: transparent;
    color: var(--color-text-muted);
  }

  .item-chev {
    width: 1rem;
    color: var(--color-text-subtle);
    text-align: left;
  }

  .item-label {
    font-family: inherit;
    font-size: 0.8rem;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-label.command-label {
    font-family: var(--font-code);
    overflow: visible;
    text-overflow: clip;
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .command-prompt {
    width: 1rem;
    color: var(--color-text-subtle);
    font-family: var(--font-code);
    font-size: 0.8rem;
    text-align: left;
  }

  .command-prompt.failure {
    color: #b85c50;
  }

  .item-body {
    margin: 0.35rem 0 0;
    padding: 0 0 0 calc(1rem + var(--space-2));
    max-width: 100%;
    min-width: 0;
    background: transparent;
    border: none;
    border-radius: 0;
    font-size: 0.8rem;
    overflow-x: auto;
  }

  .item-body.command-body {
    padding-left: calc(2rem + (var(--space-2) * 2));
  }

  .terminal-call,
  .tool-detail {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
  }

  .terminal-line,
  .terminal-output,
  .tool-output,
  .json-block,
  .input-value {
    margin: 0;
    font-family: var(--font-code);
    white-space: pre;
  }

  .terminal-line,
  .terminal-output,
  .tool-output,
  .json-block {
    min-width: max-content;
    padding: 0;
    border: none;
    border-radius: 0;
    background: transparent;
    color: var(--color-text);
  }

  .terminal-line {
    color: var(--color-text);
    font-size: 1rem;
  }

  .terminal-output {
    color: var(--color-text-muted);
    font-size: 0.88rem;
    line-height: 1.38;
  }

  .tool-output,
  .json-block {
    color: var(--color-text-muted);
    font-size: 0.88rem;
    line-height: 1.38;
  }

  .terminal-line + .terminal-output {
    margin-top: var(--space-1);
  }

  .prompt {
    color: var(--color-removed);
  }

  .input-fields {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    width: 100%;
    max-width: 100%;
    min-width: 0;
  }

  .input-field {
    display: grid;
    grid-template-columns: minmax(0, 6rem) minmax(0, 1fr);
    align-items: baseline;
    column-gap: var(--space-3);
    width: 100%;
    min-width: 0;
  }

  .input-key {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text-subtle);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    text-transform: lowercase;
    white-space: nowrap;
  }

  .input-value {
    display: block;
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    color: var(--color-text);
    line-height: 1.45;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .input-field.multiline {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .input-field.multiline .input-value {
    overflow: visible;
    white-space: pre-wrap;
  }

  .edit-file {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .edit-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    min-width: 0;
    color: var(--color-text);
    font-size: var(--text-sm);
  }

  .edit-title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-name {
    color: var(--color-link);
  }

  .edit-stats {
    flex: 0 0 auto;
    font-family: var(--font-mono);
  }

  .added {
    color: var(--color-added);
  }

  .removed {
    color: var(--color-removed);
  }

  .diff-panel {
    max-width: 100%;
    overflow-x: auto;
    border: none;
    border-radius: 0;
    background: transparent;
    font-family: var(--font-code);
  }

  .diff-lines {
    width: max-content;
    min-width: 100%;
  }

  .diff-row {
    display: grid;
    grid-template-columns: 1.2rem minmax(0, 1fr);
    line-height: 1.34;
  }

  .diff-row.added {
    background: color-mix(in srgb, var(--color-added) 6%, transparent);
  }

  .diff-row.removed {
    background: color-mix(in srgb, var(--color-removed) 6%, transparent);
  }

  .diff-mark {
    padding: 0;
    color: var(--color-text-subtle);
    text-align: center;
    user-select: none;
  }

  .diff-row.added .diff-mark {
    color: var(--color-added);
  }

  .diff-row.removed .diff-mark {
    color: var(--color-removed);
  }

  .diff-row pre {
    margin: 0;
    padding: 0 var(--space-1);
    min-width: 60ch;
    white-space: pre;
    color: var(--color-text);
    font-size: 0.88rem;
  }
</style>
