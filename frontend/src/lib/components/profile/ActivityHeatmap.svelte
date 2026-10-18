<script lang="ts">
  import type { ActivityDay } from '$lib/api/types';
  import { buildActivityGrid } from './activity-grid';

  let { days }: { days: ActivityDay[] } = $props();
  const grid = $derived(buildActivityGrid(days));
</script>

<div class="heatmap">
  {#each grid as col (col[0].date)}
    <div class="col">
      {#each col as cell (cell.date)}
        <span class="cell level-{cell.level}" title={cell.count > 0 ? `${cell.date}: ${cell.count}` : undefined} aria-label={`${cell.date}: ${cell.count}`}></span>
      {/each}
    </div>
  {/each}
</div>

<style>
  .heatmap {
    display: grid;
    grid-template-columns: repeat(18, 14px);
    gap: 2px;
    padding-bottom: var(--space-2);
  }

  .col {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .cell {
    position: relative;
    width: 100%;
    aspect-ratio: 1;
    border-radius: 2px;
    border: 1px solid rgba(232, 225, 210, 0.65);
    background: var(--color-heatmap-empty);
    box-sizing: border-box;
    transition: transform 0.08s ease, border-color 0.08s ease, box-shadow 0.08s ease;
  }

  .level-1:hover,
  .level-2:hover,
  .level-3:hover,
  .level-4:hover {
    z-index: 2;
    transform: scale(1.18);
    border-color: rgba(84, 76, 66, 0.35);
    box-shadow: 0 2px 8px rgba(35, 31, 27, 0.14);
  }

  .level-1 {
    background: var(--color-heatmap-1);
    border-color: transparent;
  }
  .level-2 {
    background: var(--color-heatmap-2);
    border-color: transparent;
  }
  .level-3 {
    background: var(--color-heatmap-3);
    border-color: transparent;
  }
  .level-4 {
    background: var(--color-heatmap-4);
    border-color: transparent;
  }
</style>
