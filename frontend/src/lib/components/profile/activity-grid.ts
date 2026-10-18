import type { ActivityDay } from '$lib/api/types';

export interface ActivityCell {
  date: string;
  count: number;
  level: number;
}

export function localDateKey(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function activityScore(day: ActivityDay): number {
  return day.message_count || day.thread_count;
}

// levelFromRank buckets a day into 1..4 using its percentile rank among all
// non-zero days, rather than a linear fraction of the single max value. A linear
// scale (count / max) collapses every normal day to level 1 as soon as one
// outlier day exists (e.g. a one-time history backfill). Rank-based binning keeps
// the relative shape of the distribution: the outlier still tops out at level 4,
// but ordinary days spread across the 1..4 bands.
function buildLevelLookup(counts: number[]): Map<number, number> {
  const sorted = Array.from(new Set(counts.filter((c) => c > 0))).sort((a, b) => a - b);
  const lookup = new Map<number, number>();
  const n = sorted.length;
  if (n === 0) return lookup;
  sorted.forEach((value, index) => {
    // Percentile rank in (0, 1]; highest value maps to 1 -> level 4.
    const rank = (index + 1) / n;
    lookup.set(value, Math.min(4, Math.max(1, Math.ceil(rank * 4))));
  });
  return lookup;
}

export function buildActivityGrid(data: ActivityDay[], today = new Date(), weeks = 18): ActivityCell[][] {
  const map = new Map<string, number>();
  for (const d of data) {
    map.set(d.date, activityScore(d));
  }
  const levelLookup = buildLevelLookup(Array.from(map.values()));

  const end = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const start = new Date(end);
  start.setDate(end.getDate() - end.getDay() - (weeks - 1) * 7);

  const columns: ActivityCell[][] = [];
  const cursor = new Date(start);
  for (let week = 0; week < weeks; week++) {
    const col: ActivityCell[] = [];
    for (let row = 0; row < 7; row++) {
      const date = localDateKey(cursor);
      const count = map.get(date) ?? 0;
      const level = count === 0 ? 0 : (levelLookup.get(count) ?? 1);
      col.push({ date, count, level });
      cursor.setDate(cursor.getDate() + 1);
    }
    columns.push(col);
  }
  return columns;
}
