<script lang="ts">
  import { locale } from '$lib/stores/locale';
  import { authApi } from '$lib/api/auth';
  import Button from '$lib/components/ui/Button.svelte';

  type Plan = {
    name: string;
    tagline: string;
    badge?: string;
    price: string;
    priceFull?: string;
    period: string;
    yearly?: string;
    features: string[];
    cta: string;
    ctaDisabled: boolean;
    featured: boolean;
  };

  type Copy = {
    head: { title: string; eyebrow: string; subtitle: string };
    plans: Plan[];
  };

  const COPY: Record<'zh' | 'en', Copy> = {
    zh: {
      head: {
        eyebrow: '订阅计划',
        title: '选择适合你的方案',
        subtitle:
          'Jieli 把每一条 AI 编程对话沉淀为可搜索、可分享的工程上下文。免费开始,需要更多时再升级 Pro。'
      },
      plans: [
        {
          name: 'Free',
          tagline: '个人入门',
          price: '$0',
          period: '/月',
          features: [
            '保留最多 3 万条消息',
            '浏览、搜索、分享、导出',
            '同步全部来源(Claude Code / Codex)',
            '默认私密,可生成分享链接'
          ],
          cta: '免费开始使用',
          ctaDisabled: false,
          featured: false
        },
        {
          name: 'Pro',
          tagline: '个人重度使用',
          price: '$5',
          period: '/月',
          features: [
            '无限制消息存储',
            '包含 Free 的全部能力',
          ],
          cta: '立即订阅',
          ctaDisabled: false,
          featured: true
        }
      ]
    },
    en: {
      head: {
        eyebrow: 'Pricing',
        title: 'Pick the plan that fits',
        subtitle:
          'Jieli turns every AI coding thread into searchable, shareable engineering context. Start free, upgrade to Pro when you need more.'
      },
      plans: [
        {
          name: 'Free',
          tagline: 'Starter',
          price: '$0',
          period: '/mo',
          features: [
            'Keep up to 30,000 messages',
            'Browse, search, share, export',
            'Sync every provider (Claude Code / Codex)',
            'Private by default, shareable by link'
          ],
          cta: 'Get started for free',
          ctaDisabled: false,
          featured: false
        },
        {
          name: 'Pro',
          tagline: 'Individual power users',
          price: '$5',
          period: '/mo',
          features: [
            'Unlimited message storage',
            'Everything in Free',
          ],
          cta: 'Subscribe now',
          ctaDisabled: false,
          featured: true
        }
      ]
    }
  };

  const copy = $derived(COPY[$locale === 'en' ? 'en' : 'zh']);
  const pageTitle = $derived($locale === 'en' ? 'Pricing - Jieli' : '订阅计划 - Jieli');
</script>

<svelte:head>
  <title>{pageTitle}</title>
  <meta name="description" content={copy.head.subtitle} />
  <link rel="alternate" hreflang="en" href="https://jieli.app/pricing" />
  <link rel="alternate" hreflang="zh" href="https://jieli.app/zh/pricing" />
  <link rel="alternate" hreflang="x-default" href="https://jieli.app/pricing" />
</svelte:head>

<div class="pricing app-container">
  <header class="head">
    <p class="eyebrow">{copy.head.eyebrow}</p>
    <h1>{copy.head.title}</h1>
    <p class="subtitle">{copy.head.subtitle}</p>
  </header>

  <div class="plans">
    {#each copy.plans as plan (plan.name)}
      <section class="plan" class:featured={plan.featured}>
        <div class="plan-head">
          <span class="plan-name">{plan.name}</span>
          {#if plan.badge}<span class="badge">{plan.badge}</span>{/if}
        </div>
        <p class="plan-tagline">{plan.tagline}</p>

        <div class="price-row">
          <span class="price">{plan.price}</span>
          <span class="period">{plan.period}</span>
          {#if plan.priceFull}<span class="price-full">{plan.priceFull}</span>{/if}
        </div>
        {#if plan.yearly}<p class="yearly">{plan.yearly}</p>{/if}

        <div class="cta">
          {#if plan.ctaDisabled}
            <Button variant="secondary" disabled>{plan.cta}</Button>
          {:else}
            <Button href={authApi.githubLoginUrl()} variant="primary">{plan.cta} →</Button>
          {/if}
        </div>

        <ul class="features">
          {#each plan.features as feature}
            <li>
              <svg class="check" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
                <path
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M3 8.5l3.2 3.2L13 5"
                />
              </svg>
              <span>{feature}</span>
            </li>
          {/each}
        </ul>
      </section>
    {/each}
  </div>
</div>

<style>
  .pricing {
    max-width: 920px;
    padding-top: var(--space-10);
    padding-bottom: var(--space-12);
  }

  .head {
    text-align: center;
    margin-bottom: var(--space-10);
  }

  .eyebrow {
    margin: 0 0 var(--space-3);
    font-size: var(--text-xs);
    letter-spacing: 0.18em;
    text-transform: uppercase;
    color: var(--color-text-subtle);
  }

  h1 {
    margin: 0;
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    font-weight: 500;
    line-height: 1.1;
  }

  .subtitle {
    margin: var(--space-4) auto 0;
    max-width: 560px;
    color: var(--color-text-muted);
    font-size: var(--text-lg);
    line-height: 1.6;
  }

  .plans {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-5);
    align-items: stretch;
  }

  .plan {
    display: flex;
    flex-direction: column;
    padding: var(--space-6);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    background: var(--color-bg-elevated);
  }

  .plan.featured {
    border-color: var(--color-accent);
    box-shadow: var(--shadow-md);
  }

  .plan-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .plan-name {
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: 500;
  }

  .badge {
    padding: 2px 8px;
    border-radius: var(--radius-full);
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    font-size: var(--text-xs);
    font-weight: 600;
  }

  .plan-tagline {
    margin: var(--space-2) 0 var(--space-5);
    color: var(--color-text-muted);
    font-size: var(--text-sm);
  }

  .price-row {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
  }

  .price {
    font-size: var(--text-3xl);
    font-weight: 600;
    letter-spacing: -0.01em;
  }

  .period {
    color: var(--color-text-muted);
    font-size: var(--text-base);
  }

  .price-full {
    color: var(--color-text-subtle);
    font-size: var(--text-base);
    text-decoration: line-through;
  }

  .yearly {
    margin: var(--space-2) 0 0;
    color: var(--color-text-subtle);
    font-size: var(--text-sm);
  }

  .cta {
    margin: var(--space-5) 0;
  }

  .cta :global(.btn) {
    width: 100%;
  }

  .features {
    margin: var(--space-2) 0 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-5);
  }

  .features li {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
    color: var(--color-text);
    font-size: var(--text-base);
    line-height: 1.5;
  }

  .check {
    flex: 0 0 auto;
    margin-top: 3px;
    color: var(--color-accent);
  }

  @media (max-width: 640px) {
    .plans {
      grid-template-columns: 1fr;
    }

    h1 {
      font-size: var(--text-2xl);
    }
  }
</style>
