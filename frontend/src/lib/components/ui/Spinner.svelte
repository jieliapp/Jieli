<script lang="ts">
  let { size = 20 }: { size?: number } = $props();
</script>

<span
  class="spinner"
  style="width: {size}px; height: {size}px;"
  role="status"
  aria-label="Loading"
></span>

<style>
  .spinner {
    display: inline-block;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
