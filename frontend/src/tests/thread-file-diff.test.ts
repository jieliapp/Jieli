import { describe, expect, it } from 'vitest';
import { editFileInfo } from '../lib/components/thread/file-diff';

describe('thread file diff', () => {
  it('recognizes Claude Code Write tool calls as added file content', () => {
    const edit = editFileInfo({
      kind: 'tool',
      name: 'Write',
      input: {
        file_path: '/repo/docs/test.md',
        content: 'alpha\nbeta\n'
      }
    });

    expect(edit).toMatchObject({
      path: '/repo/docs/test.md',
      file: 'test.md',
      verb: 'Wrote',
      added: 2,
      removed: 0
    });
    expect(edit?.rows.map((row) => [row.kind, row.text])).toEqual([
      ['added', 'alpha'],
      ['added', 'beta']
    ]);
  });

  it('recognizes Claude Code Edit tool calls as file edits', () => {
    const edit = editFileInfo({
      kind: 'tool',
      name: 'Edit',
      input: {
        file_path: '/repo/docs/test.md',
        old_string: 'alpha\nbeta\n',
        new_string: 'alpha\nbeta changed\ngamma\n',
        replace_all: false
      }
    });

    expect(edit).toMatchObject({
      path: '/repo/docs/test.md',
      file: 'test.md',
      verb: 'Edited',
      added: 2,
      removed: 1
    });
    expect(edit?.rows.map((row) => [row.kind, row.text])).toEqual([
      ['same', 'alpha'],
      ['removed', 'beta'],
      ['added', 'beta changed'],
      ['added', 'gamma']
    ]);
  });

  it('keeps recognizing Amp edit_file tool calls', () => {
    const edit = editFileInfo({
      kind: 'tool',
      name: 'edit_file',
      input: {
        path: '/repo/src/app.go',
        old_str: 'one',
        new_str: 'two'
      }
    });

    expect(edit).toMatchObject({
      path: '/repo/src/app.go',
      file: 'app.go',
      verb: 'Edited',
      added: 1,
      removed: 1
    });
  });

  it('recognizes Amp create_file tool calls as added file content', () => {
    const edit = editFileInfo({
      kind: 'tool',
      name: 'create_file',
      input: {
        path: '/repo/index.go',
        content: 'package thing\n\nconst Name = "jieli"\n'
      }
    });

    expect(edit).toMatchObject({
      path: '/repo/index.go',
      file: 'index.go',
      verb: 'Created',
      added: 3,
      removed: 0
    });
    expect(edit?.rows.map((row) => [row.kind, row.text])).toEqual([
      ['added', 'package thing'],
      ['added', ''],
      ['added', 'const Name = "jieli"']
    ]);
  });

  it('recognizes Amp apply_patch tool calls as patch diffs', () => {
    const edit = editFileInfo({
      kind: 'tool',
      name: 'apply_patch',
      input: {
        patchtext: [
          '*** Begin Patch',
          '*** Update File: /repo/internal/schema/metadata.go',
          '@@',
          ' type IndexInfo struct {',
          '-    Unique bool',
          '+    Unique bool // Is unique index',
          '+    Where string',
          ' }',
          '*** End Patch'
        ].join('\n')
      }
    });

    expect(edit).toMatchObject({
      path: '/repo/internal/schema/metadata.go',
      file: 'metadata.go',
      verb: 'Patched',
      added: 2,
      removed: 1
    });
    expect(edit?.rows.map((row) => [row.kind, row.text])).toEqual([
      ['same', 'type IndexInfo struct {'],
      ['removed', '    Unique bool'],
      ['added', '    Unique bool // Is unique index'],
      ['added', '    Where string'],
      ['same', '}']
    ]);
  });
});
