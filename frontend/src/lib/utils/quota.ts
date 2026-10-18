// Quota water-mark helpers shared by the settings usage UI. These are pure so
// they can be unit-tested without rendering the Svelte component.

export type QuotaLevel = 'neutral' | 'warning' | 'danger';

// quotaLevel maps current usage to a three-band severity used for colouring the
// progress bar: neutral below 90%, warning in [90%, 100%) (the auto-cleanup
// trigger band), and danger once the quota is full (>=100%).
export function quotaLevel(current: number, quota: number): QuotaLevel {
  if (quota <= 0) return 'neutral';
  const ratio = current / quota;
  if (ratio >= 1) return 'danger';
  if (ratio >= 0.9) return 'warning';
  return 'neutral';
}

// quotaPercent returns the fill percentage clamped to 0..100 for the bar width.
export function quotaPercent(current: number, quota: number): number {
  if (quota <= 0) return 0;
  const pct = (current / quota) * 100;
  if (pct < 0) return 0;
  if (pct > 100) return 100;
  return pct;
}
