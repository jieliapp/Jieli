<script lang="ts">
  import { locale, t } from '$lib/stores/locale';

  type SectionId = string;

  let {
    activeSection,
    navItems,
    navigateTo
  }: {
    activeSection: SectionId;
    navItems: Array<{ id: SectionId; label: string; caption: string; icon: SectionId }>;
    navigateTo: (section: SectionId) => void;
  } = $props();

  const manualHref = $derived($locale === 'zh' ? 'https://jieli.app/zh/manual' : 'https://jieli.app/manual');
</script>

{#snippet NavGlyph(kind: SectionId)}
  {#if kind === 'account'}
    <svg viewBox="0 0 20 20" aria-hidden="true">
      <circle cx="10" cy="7" r="3.2" />
      <path d="M4.8 16.2c.9-3 2.6-4.5 5.2-4.5s4.3 1.5 5.2 4.5" />
    </svg>
  {:else if kind === 'extension'}
    <svg viewBox="0 0 20 20" aria-hidden="true">
      <path d="M6.5 8.2h7v7.3h-7z" />
      <path d="M5.4 8.2h9.2" />
      <path d="M10 8.2v7.3" />
      <path d="M7.4 5.2c1.5 0 2.6 3 2.6 3S7 8.3 7 6.2c0-.6.2-1 .4-1Z" />
      <path d="M12.6 5.2c-1.5 0-2.6 3-2.6 3s3 .1 3-2c0-.6-.2-1-.4-1Z" />
    </svg>
  {:else}
    <svg viewBox="0 0 20 20" aria-hidden="true">
      <path d="M10 2 3.5 4.5v5c0 4 3 6.8 6.5 8.5 3.5-1.7 6.5-4.5 6.5-8.5v-5L10 2Z" />
      <path d="m7 10 2 2 4-4" />
    </svg>
  {/if}
{/snippet}

<aside class="settings-nav" aria-label={$t('settings.navAria')}>
  <div class="nav-main">
    <div class="nav-title">
      <span>{$t('settings.title')}</span>
    </div>

    <nav class="nav-list">
      {#each navItems as item}
        <a
          class:active={activeSection === item.id}
          href={`#${item.id}`}
          aria-current={activeSection === item.id ? 'page' : undefined}
          onclick={(event) => {
            event.preventDefault();
            navigateTo(item.id);
          }}
        >
          <span class="nav-icon">{@render NavGlyph(item.icon)}</span>
          <span>
            <strong>{item.label}</strong>
          </span>
        </a>
      {/each}
    </nav>
  </div>

  <div class="help-card" aria-label={$t('settings.helpCenter')}>
    <h2>{$t('settings.helpCenter')}</h2>
    <a href={manualHref} target="_blank" rel="noreferrer">
      <span class="help-icon">
        <svg viewBox="0 0 20 20" aria-hidden="true">
          <path d="M4 4.5h4.3c1 0 1.7.7 1.7 1.7v9.3c0-1-.7-1.7-1.7-1.7H4V4.5Z" />
          <path d="M16 4.5h-4.3c-1 0-1.7.7-1.7 1.7v9.3c0-1 .7-1.7 1.7-1.7H16V4.5Z" />
        </svg>
      </span>
      <span>{$t('settings.helpManual')}</span>
    </a>
    <a href="mailto:admin@jieli.app">
      <span class="help-icon">
        <svg viewBox="0 0 20 20" aria-hidden="true">
          <circle cx="10" cy="10" r="7" />
          <path d="M10 5.8v5.1" />
          <path d="M10 14.2h.01" />
        </svg>
      </span>
      <span>{$t('settings.helpSelfHost')}</span>
    </a>
  </div>
</aside>

<style>
  .help-card {
    display: grid;
    gap: var(--space-2);
    margin-top: var(--space-8);
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: color-mix(in srgb, var(--color-bg-elevated) 72%, transparent);
  }

  .help-card h2 {
    margin: 0 0 var(--space-1);
    color: var(--color-text);
    font-size: var(--text-sm);
    font-weight: 650;
  }

  .help-card a {
    display: grid;
    grid-template-columns: 20px minmax(0, 1fr);
    gap: var(--space-2);
    align-items: center;
    min-height: 28px;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    font-weight: 600;
    text-decoration: none;
  }

  .help-card a:hover {
    color: var(--color-text);
  }

  .help-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
  }

  .help-icon svg {
    width: 18px;
    height: 18px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  @media (max-width: 900px) {
    :global(.settings-frame) .settings-nav {
      display: contents;
    }
    .help-card {
      order: 1;
      margin-top: 0;
    }
  }
</style>
