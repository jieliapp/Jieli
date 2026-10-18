import { describe, expect, it } from 'vitest';
import { buildActivityGrid, localDateKey } from '../lib/components/profile/activity-grid';

describe('activity heatmap grid', () => {
  it('uses local date keys instead of UTC ISO dates', () => {
    expect(localDateKey(new Date(2026, 5, 3))).toBe('2026-06-03');
  });

  it('maps recent activity onto the matching local day cell', () => {
    const grid = buildActivityGrid(
      [{ date: '2026-06-03', thread_count: 2, message_count: 5, lines_added: 0, lines_removed: 0, lines_changed: 0 }],
      new Date(2026, 5, 3),
      1
    );
    const cells = grid.flat();
    expect(cells.find((cell) => cell.date === '2026-06-03')).toEqual({
      date: '2026-06-03',
      count: 5,
      level: 4
    });
  });

  it('builds a fixed profile activity grid by default', () => {
    const grid = buildActivityGrid([], new Date(2026, 5, 3));
    expect(grid).toHaveLength(18);
    expect(grid.every((col) => col.length === 7)).toBe(true);
    expect(grid[17][3].date).toBe('2026-06-03');
  });

  it('uses message count for heatmap intensity when thread count is low', () => {
    const grid = buildActivityGrid(
      [
        { date: '2026-06-02', thread_count: 1, message_count: 10, lines_added: 0, lines_removed: 0, lines_changed: 0 },
        { date: '2026-06-03', thread_count: 1, message_count: 530, lines_added: 0, lines_removed: 0, lines_changed: 0 }
      ],
      new Date(2026, 5, 3),
      1
    );
    const cells = grid.flat();
    expect(cells.find((cell) => cell.date === '2026-06-03')?.level).toBe(4);
    expect(cells.find((cell) => cell.date === '2026-06-02')?.count).toBe(10);
  });

  it('does not let a single outlier day wash out the rest of the grid', () => {
    const days = [
      { date: '2026-06-01', thread_count: 121, message_count: 22222, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-02', thread_count: 1, message_count: 622, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-03', thread_count: 1, message_count: 318, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-04', thread_count: 1, message_count: 564, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-05', thread_count: 1, message_count: 194, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-06', thread_count: 1, message_count: 443, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-07', thread_count: 1, message_count: 242, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-08', thread_count: 1, message_count: 84, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-09', thread_count: 1, message_count: 890, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-10', thread_count: 1, message_count: 43, lines_added: 0, lines_removed: 0, lines_changed: 0 },
      { date: '2026-06-11', thread_count: 1, message_count: 937, lines_added: 0, lines_removed: 0, lines_changed: 0 }
    ];
    const cells = buildActivityGrid(days, new Date(2026, 5, 11), 18).flat();
    const levelFor = (date: string) => cells.find((cell) => cell.date === date)?.level;
    // The outlier still tops out, but normal days are not all crushed to level 1.
    expect(levelFor('2026-06-01')).toBe(4);
    expect(levelFor('2026-06-11')).toBeGreaterThanOrEqual(3);
    expect(levelFor('2026-06-09')).toBeGreaterThanOrEqual(3);
    // The smallest non-zero day stays low.
    expect(levelFor('2026-06-10')).toBe(1);
    // The mid-range days land in the middle bands, not all at level 1.
    expect(levelFor('2026-06-06')).toBeGreaterThanOrEqual(2);
    const usedLevels = new Set(days.map((d) => levelFor(d.date)));
    expect(usedLevels.size).toBeGreaterThanOrEqual(3);
  });
});
