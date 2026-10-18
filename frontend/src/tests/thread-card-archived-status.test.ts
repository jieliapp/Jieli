import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const source = readFileSync(new URL('../lib/components/thread/ThreadCard.svelte', import.meta.url), 'utf8');

describe('thread card archived status', () => {
  it('keeps the label accessible while showing only an archive icon on mobile', () => {
    expect(source).toContain('aria-label={$t(\'threads.archived\')}');
    expect(source).toContain('class="archived-icon"');
    expect(source).toMatch(/@media \(max-width: 520px\)[\s\S]*?\.visibility-text,\s*\.archived-text\s*{[\s\S]*?position:\s*absolute;/);
  });
});
