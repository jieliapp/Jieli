import type { WorkUnit } from './work-groups';

export interface EditFileInfo {
  path: string;
  file: string;
  verb: string;
  added: number;
  removed: number;
  rows: DiffRow[];
}

export interface DiffRow {
  kind: 'same' | 'added' | 'removed';
  oldLine: number | null;
  newLine: number | null;
  text: string;
}

export function editFileInfo(unit: WorkUnit): EditFileInfo | null {
  if (unit.kind !== 'tool') return null;
  const input = inputRecord(unit.input);
  if (!input) return null;

  const name = normalizeToolName(unit.name);
  const change = fileChangeInfo(name, input);
  if (!change) {
    return null;
  }
  const rows =
    change.rows ??
    diffRows(
      change.kind !== 'edit' && change.oldText === '' ? [] : splitLines(change.oldText),
      change.kind !== 'edit' && change.newText === '' ? [] : splitLines(change.newText)
    );
  return {
    path: change.path,
    file: change.fileLabel ?? basename(change.path),
    verb: verbForChange(change.kind),
    added: rows.filter((row) => row.kind === 'added').length,
    removed: rows.filter((row) => row.kind === 'removed').length,
    rows
  };
}

export function stringInput(input: unknown, ...names: string[]): string {
  return stringInputValue(input, ...names).value;
}

type FileChangeInfo =
  | { kind: 'edit' | 'write' | 'create'; path: string; oldText: string; newText: string; rows?: never; fileLabel?: string }
  | { kind: 'patch'; path: string; oldText?: never; newText?: never; rows: DiffRow[]; fileLabel?: string };

function fileChangeInfo(
  name: string,
  input: Record<string, unknown>
): FileChangeInfo | null {
  if (name === 'apply_patch') {
    const patch = stringInput(input, 'patchtext', 'patch_text', 'patchText');
    return patchFileChange(patch);
  }

  const path = stringInput(input, 'path', 'file_path', 'filePath');
  if (!path) return null;

  if (name === 'edit_file' || name === 'edit') {
    const oldText = stringInput(input, 'old_str', 'oldStr', 'old_string', 'oldString');
    const newText = stringInput(input, 'new_str', 'newStr', 'new_string', 'newString');
    return { kind: 'edit', path, oldText, newText };
  }

  if (name === 'write' || name === 'write_file') {
    const content = stringInputValue(input, 'content');
    if (!content.found) return null;
    return { kind: 'write', path, oldText: '', newText: content.value };
  }

  if (name === 'create_file') {
    const content = stringInputValue(input, 'content');
    if (!content.found) return null;
    return { kind: 'create', path, oldText: '', newText: content.value };
  }

  return null;
}

function patchFileChange(patch: string): FileChangeInfo | null {
  if (!patch) return null;
  const rows: DiffRow[] = [];
  const paths: string[] = [];
  let inFile = false;

  for (const line of patch.replace(/\r\n/g, '\n').split('\n')) {
    const path = patchPath(line);
    if (path) {
      paths.push(path);
      inFile = true;
      continue;
    }
    if (!inFile || line === '*** End Patch' || line.startsWith('*** ')) continue;
    if (line.startsWith('@@')) continue;
    if (line.startsWith('+')) {
      rows.push({ kind: 'added', oldLine: null, newLine: null, text: line.slice(1) });
    } else if (line.startsWith('-')) {
      rows.push({ kind: 'removed', oldLine: null, newLine: null, text: line.slice(1) });
    } else if (line.startsWith(' ')) {
      rows.push({ kind: 'same', oldLine: null, newLine: null, text: line.slice(1) });
    }
  }

  const uniquePaths = [...new Set(paths)];
  const firstPath = uniquePaths[0];
  if (!firstPath || rows.length === 0) return null;
  return {
    kind: 'patch',
    path: firstPath,
    fileLabel: uniquePaths.length > 1 ? `${uniquePaths.length} files` : basename(firstPath),
    rows
  };
}

function patchPath(line: string): string {
  const match = line.match(/^\*\*\* (?:Add|Update|Delete) File:\s*(.+)$/);
  return match?.[1]?.trim() ?? '';
}

function verbForChange(kind: FileChangeInfo['kind']): string {
  if (kind === 'create') return 'Created';
  if (kind === 'patch') return 'Patched';
  return kind === 'write' ? 'Wrote' : 'Edited';
}

function normalizeToolName(name?: string): string {
  return (name ?? '').trim().toLowerCase();
}

function inputRecord(input: unknown): Record<string, unknown> | null {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return null;
  return input as Record<string, unknown>;
}

function stringInputValue(input: unknown, ...names: string[]): { value: string; found: boolean } {
  const record = inputRecord(input);
  if (!record) return { value: '', found: false };
  for (const name of names) {
    const value = record[name];
    if (typeof value === 'string') return { value, found: true };
  }
  return { value: '', found: false };
}

function basename(path: string): string {
  const parts = path.replace(/\/+$/, '').split('/').filter(Boolean);
  return parts.at(-1) ?? path;
}

function splitLines(text: string): string[] {
  if (text === '') return [''];
  return text.replace(/\n$/, '').split('\n');
}

function diffRows(oldLines: string[], newLines: string[]): DiffRow[] {
  let prefix = 0;
  while (prefix < oldLines.length && prefix < newLines.length && oldLines[prefix] === newLines[prefix]) {
    prefix += 1;
  }

  let suffix = 0;
  while (
    suffix < oldLines.length - prefix &&
    suffix < newLines.length - prefix &&
    oldLines[oldLines.length - 1 - suffix] === newLines[newLines.length - 1 - suffix]
  ) {
    suffix += 1;
  }

  const rows: DiffRow[] = [];
  let oldLine = 1;
  let newLine = 1;

  for (let i = 0; i < prefix; i += 1) {
    rows.push({ kind: 'same', oldLine: oldLine++, newLine: newLine++, text: oldLines[i] });
  }
  for (let i = prefix; i < oldLines.length - suffix; i += 1) {
    rows.push({ kind: 'removed', oldLine: oldLine++, newLine: null, text: oldLines[i] });
  }
  for (let i = prefix; i < newLines.length - suffix; i += 1) {
    rows.push({ kind: 'added', oldLine: null, newLine: newLine++, text: newLines[i] });
  }
  for (let i = oldLines.length - suffix; i < oldLines.length; i += 1) {
    rows.push({ kind: 'same', oldLine: oldLine++, newLine: newLine++, text: oldLines[i] });
  }

  return rows;
}
