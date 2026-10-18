<script lang="ts">
  let { title = undefined }: { title?: string } = $props();
</script>

<svg viewBox="0 0 20 20" role={title ? 'img' : undefined} aria-hidden={title ? undefined : 'true'}>
  {#if title}
    <title>{title}</title>
  {/if}
  <path d="M8.45 4.25c-3.05 0-5.5 1.9-5.5 4.25 0 1.2.65 2.26 1.7 3l-.58 1.88 2.2-1.12c.66.3 1.4.47 2.18.47 3.05 0 5.5-1.9 5.5-4.23 0-2.35-2.45-4.25-5.5-4.25Z" />
  <path d="M12.25 7.5c2.55.25 4.5 1.88 4.5 3.85 0 1.08-.58 2.03-1.52 2.72l.48 1.68-1.98-1.05c-.56.22-1.2.34-1.88.34-1.63 0-3.05-.7-3.78-1.75" opacity=".58" />
</svg>

<style>
  svg {
    display: block;
    width: 100%;
    height: 100%;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
