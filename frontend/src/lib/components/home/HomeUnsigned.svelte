<script lang="ts">
  import { t } from '$lib/stores/locale';
  import { authApi } from '$lib/api/auth';
  import { copyToClipboard } from '$lib/utils';
  import Button from '$lib/components/ui/Button.svelte';

  type InstallOption = {
    key: 'claudeCode' | 'codex';
    titleKey: string;
    hintKey: string;
    prompt: string;
    commands: string[];
    disabled?: boolean;
  };

  const installOptions: InstallOption[] = [
    {
      key: 'claudeCode',
      titleKey: 'home.pluginClaudeCode',
      hintKey: 'home.installInShell',
      prompt: '$',
      commands: [
        'claude plugin marketplace add jieliapp/plugins',
        'claude plugin install jieli@jieliapp'
      ]
    },
    {
      key: 'codex',
      titleKey: 'home.pluginCodex',
      hintKey: 'home.installInShell',
      prompt: '$',
      commands: [
        'codex plugin marketplace add jieliapp/plugins',
        'codex plugin add jieli@jieliapp'
      ]
    },

  ];

  let activeInstall = $state(0);
  let copiedInstall = $state(false);
  let copyTimer: ReturnType<typeof setTimeout> | undefined;
  const selectedInstall = $derived(installOptions[activeInstall] ?? installOptions[0]);

  async function copyInstallCommands() {
    if (await copyToClipboard(selectedInstall.commands.join('\n'))) {
      copiedInstall = true;
      clearTimeout(copyTimer);
      copyTimer = setTimeout(() => (copiedInstall = false), 1500);
    }
  }
</script>

<div class="marketing">
  <section class="hero app-container">
    <div class="hero-text">
      <h1 class="headline">{$t('home.tagline')}</h1>
      <p class="subtitle">{$t('home.subtitle')}</p>
      <div class="hero-cta">
        <Button href={authApi.githubLoginUrl()} variant="primary">
          {$t('home.ctaGetStarted')} →
        </Button>
      </div>
    </div>
  </section>

  <section class="announce">
    <div class="app-container announce-inner">
      <span class="announce-text">{$t('home.announce')}</span>
      <a class="announce-link" href="https://github.com/jieliapp/plugins" target="_blank" rel="noopener">
        {$t('home.announceLink')} →
      </a>
    </div>
  </section>

  <section id="install" class="cli app-container">
    <div class="cli-head">
      <span class="eyebrow">{$t('home.cliEyebrow')}</span>
      <h2 class="cli-title">{$t('home.cliTitle')}</h2>
    </div>

    <div class="terminal">
      <div class="terminal-bar">
        <div class="terminal-dots" aria-hidden="true">
          <span class="dot dot-red"></span>
          <span class="dot dot-yellow"></span>
          <span class="dot dot-green"></span>
        </div>
        <button class="copy-command" type="button" onclick={copyInstallCommands}>
          {copiedInstall ? $t('common.copied') : $t('common.copy')}
        </button>
      </div>
      <div class="terminal-body">
        <div class="orb" aria-hidden="true"></div>
        <p class="welcome">{$t(selectedInstall.titleKey)}</p>
        <p class="install-hint">{$t(selectedInstall.hintKey)}</p>
        <div class="cmd-list" aria-label={$t('home.cliTitle')}>
          {#each selectedInstall.commands as command}
            <div class="cmd-line"><span class="prompt">{selectedInstall.prompt}</span><span>{command}</span></div>
          {/each}
        </div>
      </div>
    </div>

    <div class="segmented" role="tablist" aria-label={$t('home.cliTitle')}>
      {#each installOptions as option, i (option.key)}
        <button
          class="seg"
          class:active={activeInstall === i}
          class:disabled={option.disabled}
          role="tab"
          aria-selected={activeInstall === i}
          onclick={() => (activeInstall = i)}
        >
          {$t(option.titleKey)}
        </button>
      {/each}
    </div>
  </section>
</div>

<style>
  .marketing {
    margin-top: calc(-1 * var(--space-6));
    margin-bottom: calc(-1 * var(--space-10));
    padding-top: var(--space-6);
    padding-bottom: calc(var(--space-12) + var(--space-10));
    background-color: #e1e2c7;
    background-image: radial-gradient(rgba(119, 119, 92, 0.26) 1px, transparent 1px);
    background-size: 28px 28px;
  }

  /* Hero: single column, CTA follows text */
  .hero {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding-top: var(--space-12);
    padding-bottom: var(--space-10);
  }

  .hero-text {
    max-width: 800px;
  }

  .headline {
    font-family: var(--font-serif);
    font-size: var(--text-4xl);
    font-weight: 500;
    line-height: 1.08;
    letter-spacing: -0.015em;
    margin-bottom: var(--space-4);
  }

  .subtitle {
    font-size: var(--text-lg);
    color: var(--color-text-muted);
    margin: 0;
    max-width: 640px;
  }

  .hero-cta {
    margin-top: var(--space-5);
  }

  /* Announcement bar: full-width light band */
  .announce {
    background: rgba(255, 253, 247, 0.38);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
    margin: var(--space-6) 0 var(--space-12);
  }

  .announce-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding-top: var(--space-4);
    padding-bottom: var(--space-4);
    flex-wrap: wrap;
  }

  .announce-text {
    font-family: var(--font-serif);
    font-size: var(--text-lg);
    color: var(--color-text);
  }

  .announce-link {
    font-size: var(--text-sm);
    color: var(--color-text-muted);
    white-space: nowrap;
  }
  .announce-link:hover {
    color: var(--color-text);
  }

  /* CLI section */
  .cli {
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .cli-head {
    width: 100%;
    max-width: 760px;
    margin-bottom: var(--space-5);
  }

  .eyebrow {
    display: block;
    font-size: var(--text-xs);
    letter-spacing: 0.18em;
    text-transform: uppercase;
    color: var(--color-text-subtle);
    margin-bottom: var(--space-2);
  }

  .cli-title {
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: 500;
    margin: 0;
  }

  .terminal {
    width: 100%;
    max-width: 760px;
    background: #111111;
    border: 1px solid rgba(17, 17, 17, 0.22);
    border-radius: var(--radius-xl);
    overflow: hidden;
    box-shadow: 0 18px 40px rgba(17, 17, 17, 0.18);
  }

  .terminal-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .terminal-dots {
    display: flex;
    gap: var(--space-2);
  }

  .copy-command {
    padding: 0.2rem 0.5rem;
    color: rgba(255, 255, 255, 0.64);
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    cursor: pointer;
  }

  .copy-command:hover {
    color: rgba(255, 255, 255, 0.9);
    background: rgba(255, 255, 255, 0.1);
  }

  .dot {
    width: 11px;
    height: 11px;
    border-radius: var(--radius-full);
    background: rgba(255, 255, 255, 0.24);
  }
  .dot-red {
    background: #e0625a;
  }
  .dot-yellow {
    background: #d6a429;
  }
  .dot-green {
    background: #4fc48a;
  }

  .terminal-body {
    padding: var(--space-8) var(--space-6) var(--space-6);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    min-height: 240px;
  }

  .orb {
    width: 96px;
    height: 96px;
    border-radius: var(--radius-full);
    background:
      radial-gradient(
          circle at 35% 30%,
          rgba(79, 196, 138, 0.9),
          rgba(79, 196, 138, 0) 60%
        ),
      radial-gradient(circle at 70% 75%, rgba(127, 180, 240, 0.6), rgba(127, 180, 240, 0) 55%),
      #0c0f0c;
    box-shadow:
      inset 0 0 30px rgba(79, 196, 138, 0.25),
      0 0 40px rgba(79, 196, 138, 0.12);
  }

  .welcome {
    font-family: var(--font-serif);
    font-size: var(--text-lg);
    color: #f6fff5;
    margin: 0;
  }

  .install-hint {
    margin: var(--space-1) 0 0;
    color: var(--color-text-subtle);
    font-size: var(--text-sm);
    text-align: center;
  }

  .cmd-list {
    width: 100%;
    display: grid;
    padding: var(--space-3) var(--space-4);
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: var(--radius-md);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    color: #d7ddd5;
    overflow-x: auto;
  }

  .cmd-line {
    display: flex;
    gap: var(--space-3);
    min-width: max-content;
    padding: 0.3rem 0;
    white-space: pre;
  }

  .prompt {
    color: #63d199;
    flex: 0 0 auto;
    user-select: none;
  }

  .segmented {
    display: inline-flex;
    gap: 2px;
    margin-top: var(--space-5);
    padding: 3px;
    background: var(--color-bg-inset);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-full);
  }

  .seg {
    padding: var(--space-2) var(--space-4);
    border: none;
    border-radius: var(--radius-full);
    background: transparent;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }

  .seg:hover {
    color: var(--color-text);
  }

  .seg.active {
    background: var(--color-secondary);
    color: var(--color-text);
  }

  .seg.disabled:not(.active) {
    color: var(--color-text-subtle);
  }

  @media (max-width: 640px) {
    .hero {
      align-items: flex-start;
    }
    .headline {
      font-size: var(--text-3xl);
    }

    .segmented {
      width: 100%;
      overflow-x: auto;
      justify-content: flex-start;
    }

    .seg {
      flex: 1 0 auto;
    }
  }
</style>
