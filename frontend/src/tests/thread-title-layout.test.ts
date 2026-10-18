import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const source = readFileSync(new URL('../lib/components/thread/ThreadView.svelte', import.meta.url), 'utf8');

describe('thread detail title layout', () => {
  it('reserves symmetric space for mobile actions so long titles stay centered and truncated', () => {
    expect(source).toMatch(/@media \(max-width: 880px\)[\s\S]*?\.title-row\s*{[\s\S]*?grid-template-columns:\s*2rem minmax\(0, 1fr\) 2rem;/);
    expect(source).toMatch(/@media \(max-width: 880px\)[\s\S]*?\.thread-head h1\s*{[\s\S]*?grid-column:\s*2;/);
    expect(source).toMatch(/@media \(max-width: 880px\)[\s\S]*?\.title-action-slot\s*{[\s\S]*?grid-column:\s*3;/);
  });
});
