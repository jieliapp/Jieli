import { describe, expect, it } from 'vitest';
import { formatToolOutputBlock } from '../lib/components/thread/tool-output-format';

describe('tool output formatting', () => {
  it('expands JSON tool output strings with nested markdown string values', () => {
    const output = JSON.stringify({
      previous_status: {
        completed: '### Strengths\n- Provider-id login remains first-class'
      },
      completed: '子代理发现一个有效问题：重复邮箱检查是大小写精确匹配。'
    });

    expect(formatToolOutputBlock(output)).toEqual({
      kind: 'text-block',
      value:
        'previous_status.completed:\n### Strengths\n- Provider-id login remains first-class\n\ncompleted:\n子代理发现一个有效问题：重复邮箱检查是大小写精确匹配。'
    });
  });

  it('leaves ordinary tool output unchanged', () => {
    expect(formatToolOutputBlock('plain output')).toEqual({
      kind: 'text-block',
      value: 'plain output'
    });
  });
});
