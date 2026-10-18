<script lang="ts">
  import { locale, localizedHref } from '$lib/stores/locale';

  type Block =
    | { t: 'p'; text: string; link?: { href: string; label: string; suffix?: string } }
    | { t: 'sub'; text: string }
    | { t: 'list'; items: string[] }
    | { t: 'code'; code: string }
    | { t: 'note'; text: string };

  type Section = { id: string; title: string; body: Block[] };
  type Copy = { hero: { eyebrow: string; title: string; subtitle: string }; sections: Section[] };

  const COPY: Record<'zh' | 'en', Copy> = {
    zh: {
      hero: {
        eyebrow: '用户手册',
        title: '欢迎使用 Jieli',
        subtitle:
          'Jieli 把你的 AI 编程对话实时同步到云端,变成可搜索、可分享、可跨 Agent 接力的工程上下文。本手册帮你用满它。'
      },
      sections: [
        {
          id: 'why',
          title: '为什么是 Jieli',
          body: [
            {
              t: 'p',
              text: 'Jieli 是「接力」的拼音。换工具、换会话时,上一棒的完整上下文交到下一棒手里,任务不掉链子地继续——这就是「接力」,也是 Jieli 名字的由来。'
            },
            {
              t: 'p',
              text: 'Jieli 是 AI 编程对话的云端同步 + 浏览 / 搜索 / 分享平台。装上插件后,你在 Claude Code、Codex 里的会话会自动同步上来,默认私密,只有你能看到。'
            },
            {
              t: 'list',
              items: [
                '多来源:Claude Code、Codex 统一汇聚在一个地方。',
                '可搜索:把散落在各处的对话变成可全文检索的工程记忆。',
                '可分享:一键生成只读链接,把某次排障过程分享给同事。',
                '可接力:换工具、换会话时,带着完整上下文无缝继续。'
              ]
            },
            {
              t: 'note',
              text: 'Jieli 与插件配合使用。Jieli 保存同步上来的会话记录,原始会话仍保留在你本地;每条对话都能导出 md / json,随时可自行留底。'
            }
          ]
        },
        {
          id: 'start',
          title: '快速开始',
          body: [
            {
              t: 'list',
              items: [
                '用 GitHub 登录 jieli.app,创建账户。',
                '在设置里复制你的访问令牌(API key)。',
                '在你的编程工具里安装 Jieli 插件(见下一节)。',
                '之后会话会通过插件 hooks 自动同步,无需手动操作。'
              ]
            }
          ]
        },
        {
          id: 'install',
          title: '安装插件',
          body: [
            { t: 'sub', text: 'Claude Code' },
            { t: 'p', text: '在终端里运行:' },
            { t: 'code', code: 'claude plugin marketplace add jieliapp/plugins\nclaude plugin install jieli@jieliapp' },
            { t: 'sub', text: 'Codex' },
            { t: 'p', text: '在终端里运行:' },
            { t: 'code', code: 'codex plugin marketplace add jieliapp/plugins\ncodex plugin add jieli@jieliapp' },
            {
              t: 'p',
              text: '安装后配置 API key:把密钥粘进对话让 agent 写入,写到 ~/.config/jieli/settings.json, 或导出环境变量。'
            },
            { t: 'code', code: 'export JIELI_API_KEY="<在 jieli.app 复制的密钥>"' }
          ]
        },
        {
          id: 'browse',
          title: '浏览与搜索',
          body: [
            {
              t: 'p',
              text: '在对话列表里可以全文搜索,并按来源、仓库、标签、可见性、是否归档过滤,快速定位某次对话。'
            },
            {
              t: 'p',
              text: '插件也能让 agent 直接搜索你的历史对话——给它一个关键词、文件名或仓库线索,它会调用 Jieli 的搜索接口召回相关线程作为上下文。'
            },
            { t: 'code', code: 'GET /plugin/threads?search=<关键词>&provider=claude_code&sort=updated' }
          ]
        },
        {
          id: 'handoff',
          title: '续接与跨 Agent 接力',
          body: [
            {
              t: 'p',
              text: '这是 Jieli 的核心:一次对话的上下文不必困在原会话里,可以带着它换会话、甚至换工具继续。'
            },
            { t: 'sub', text: '同工具续接' },
            { t: 'p', text: '在对话详情页点「在 CLI 中打开」,按来源生成续接命令,回到原工具接着干:' },
            { t: 'code', code: 'claude --resume <session>\ncodex resume <session>' },
            { t: 'sub', text: '跨 Agent 接力(handoff)' },
            {
              t: 'p',
              text: 'handoff skill 会从当前对话里抽取关键信息和相关文件,生成一段交接上下文,并附上 Jieli 线程 ID。把这段交接内容(或直接把线程链接)交给一个新的 agent——可以是另一个工具——它就能无缝接着干。'
            },
            {
              t: 'p',
              text: '新 agent 拿到 Jieli 线程链接后,jieli skill 会自动触发,读取该线程的 markdown 导出,把完整上下文加载进来。'
            },
            {
              t: 'note',
              text: '你只需要粘贴一条 /threads/T-… 链接,新会话就能拿到上一段工作的目标、决策、改过的文件和待办——不必把上下文重新讲一遍。'
            },
            { t: 'sub', text: 'Commit trailer(对 Code Review 尤其有用)' },
            {
              t: 'p',
              text: 'Claude Code 创建的提交会自动带上一行 trailer,指向产生这次改动的 Jieli 线程:'
            },
            { t: 'code', code: 'Jieli-Thread: https://jieli.app/threads/T-...' },
            {
              t: 'p',
              text: 'Code Review 时,审查者点开这行链接,就能看到这次提交背后的完整推理过程——为什么这么改、试过哪些方案、验证了什么——而不只是看到最终 diff。'
            }
          ]
        },
        {
          id: 'share',
          title: '分享对话',
          body: [
            {
              t: 'p',
              text: '每条对话默认私密,只有你能看到。需要分享时,把可见性改为「链接可见」,任何拿到链接的人都能只读查看;复制分享链接发出去即可。'
            }
          ]
        },
        {
          id: 'export',
          title: '导出与归档',
          body: [
            {
              t: 'p',
              text: '每条对话都能导出为 Markdown 或 JSON。不再需要的对话可以归档,从默认列表里收起来。'
            },
            {
              t: 'note',
              text: '删除是不可逆操作,Jieli 无法恢复。删除前请先用导出 md / json 留底。'
            }
          ]
        },
        {
          id: 'pricing',
          title: '订阅计划',
          body: [
            {
              t: 'p',
              text: 'Free 保留最多 3 万条消息,浏览 / 搜索 / 分享 / 导出全部可用;Pro 提供无限制消息存储。完整对比见',
              link: { href: '/pricing', label: '订阅计划页', suffix: '。' }
            }
          ]
        }
      ]
    },
    en: {
      hero: {
        eyebrow: "Owner's Manual",
        title: 'Welcome to Jieli',
        subtitle:
          'Jieli syncs your AI coding threads to the cloud and turns them into searchable, shareable engineering context you can hand off across agents. This manual helps you get the most out of it.'
      },
      sections: [
        {
          id: 'why',
          title: 'Why Jieli',
          body: [
            {
              t: 'p',
              text: 'Jieli (jiē lì) is the pinyin for 接力 — Chinese for “relay.” Switch tools or open a fresh session and the full context is passed to the next runner, so the task keeps going without dropping the baton. That hand-off is exactly what the name is about.'
            },
            {
              t: 'p',
              text: 'Jieli is a cloud sync plus browse / search / share platform for AI coding threads. Once the plugin is installed, your sessions in Claude Code and Codex sync automatically — private by default, visible only to you.'
            },
            {
              t: 'list',
              items: [
                'Multi-provider: Claude Code and Codex gathered in one place.',
                'Searchable: turn scattered conversations into full-text engineering memory.',
                'Shareable: generate a read-only link to share a debugging session with a teammate.',
                'Hand-off ready: continue with full context across sessions and tools.'
              ]
            },
            {
              t: 'note',
              text: 'Jieli works alongside the plugins. It stores the session records synced by plugins, your original sessions stay on your machine, and every thread exports to md / json so you can always keep your own copy.'
            }
          ]
        },
        {
          id: 'start',
          title: 'Get started',
          body: [
            {
              t: 'list',
              items: [
                'Sign in to jieli.app with GitHub to create an account.',
                'Copy your access token (API key) from Settings.',
                'Install the Jieli plugin in your coding tool (next section).',
                'From then on, sessions sync automatically through plugin hooks.'
              ]
            }
          ]
        },
        {
          id: 'install',
          title: 'Install the plugin',
          body: [
            { t: 'sub', text: 'Claude Code' },
            { t: 'p', text: 'Run these in your shell:' },
            { t: 'code', code: 'claude plugin marketplace add jieliapp/plugins\nclaude plugin install jieli@jieliapp' },
            { t: 'sub', text: 'Codex' },
            { t: 'p', text: 'Run these in your shell:' },
            { t: 'code', code: 'codex plugin marketplace add jieliapp/plugins\ncodex plugin add jieli@jieliapp' },
            {
              t: 'p',
              text: 'Then configure the API key: paste it into the chat for the agent to store, export it as an env var, or write it to ~/.config/jieli/settings.json.'
            },
            { t: 'code', code: 'export JIELI_API_KEY="<api-key-from-jieli.app>"' }
          ]
        },
        {
          id: 'browse',
          title: 'Browse & search',
          body: [
            {
              t: 'p',
              text: 'The threads list supports full-text search and filters by provider, repository, label, visibility, and archived state, so you can find a past thread fast.'
            },
            {
              t: 'p',
              text: 'The plugin also lets an agent search your history directly — give it a keyword, filename, or repo clue and it calls Jieli to pull relevant threads back as context.'
            },
            { t: 'code', code: 'GET /plugin/threads?search=<query>&provider=claude_code&sort=updated' }
          ]
        },
        {
          id: 'handoff',
          title: 'Resume & cross-agent hand-off',
          body: [
            {
              t: 'p',
              text: "This is the heart of Jieli: a thread's context doesn't have to stay trapped in its original session — carry it into a new session, or even a different tool, and keep going."
            },
            { t: 'sub', text: 'Resume in the same tool' },
            { t: 'p', text: 'On a thread page, click “Open in CLI” to get a resume command for its provider:' },
            { t: 'code', code: 'claude --resume <session>\ncodex resume <session>' },
            { t: 'sub', text: 'Cross-agent hand-off' },
            {
              t: 'p',
              text: 'The handoff skill extracts the key information and relevant files from the current thread, writes a hand-off context, and attaches the Jieli thread ID. Give that hand-off (or just the thread link) to a fresh agent — possibly in another tool — and it picks up seamlessly.'
            },
            {
              t: 'p',
              text: 'When the new agent receives a Jieli thread link, the jieli skill triggers automatically and reads the thread’s markdown export to load the full context.'
            },
            {
              t: 'note',
              text: 'Paste a single /threads/T-… link and the new session inherits the previous goal, decisions, touched files, and open tasks — no need to re-explain.'
            },
            { t: 'sub', text: 'Commit trailers (great for code review)' },
            {
              t: 'p',
              text: 'Commits created by Claude Code carry a trailer pointing back to the Jieli thread behind the change:'
            },
            { t: 'code', code: 'Jieli-Thread: https://jieli.app/threads/T-...' },
            {
              t: 'p',
              text: 'During code review, click that link to see the full reasoning behind the commit — why it was done this way, what was tried, what was verified — not just the final diff.'
            }
          ]
        },
        {
          id: 'share',
          title: 'Share threads',
          body: [
            {
              t: 'p',
              text: 'Every thread is private by default. To share, switch visibility to “unlisted” — anyone with the link can view it read-only — then copy the share link and send it.'
            }
          ]
        },
        {
          id: 'export',
          title: 'Export & archive',
          body: [
            {
              t: 'p',
              text: 'Every thread exports to Markdown or JSON. Archive threads you no longer need to tuck them out of the default list.'
            },
            {
              t: 'note',
              text: 'Deletion is irreversible and cannot be recovered from Jieli. Export md / json first to keep your own copy.'
            }
          ]
        },
        {
          id: 'pricing',
          title: 'Plans',
          body: [
            {
              t: 'p',
              text: 'Free keeps up to 30,000 messages with full browse / search / share / export; Pro includes unlimited message storage. See the full comparison at',
              link: { href: '/pricing', label: 'pricing', suffix: '.' }
            }
          ]
        }
      ]
    }
  };

  const copy = $derived(COPY[$locale === 'en' ? 'en' : 'zh']);
  const pageTitle = $derived($locale === 'en' ? "Owner's Manual - Jieli" : '用户手册 - Jieli');
</script>

<svelte:head>
  <title>{pageTitle}</title>
  <meta name="description" content={copy.hero.subtitle} />
  <link rel="alternate" hreflang="en" href="https://jieli.app/manual" />
  <link rel="alternate" hreflang="zh" href="https://jieli.app/zh/manual" />
  <link rel="alternate" hreflang="x-default" href="https://jieli.app/manual" />
</svelte:head>

<div class="manual app-container">
  <aside class="toc">
    <nav aria-label={copy.hero.eyebrow}>
      <ol>
        {#each copy.sections as section, i (section.id)}
          <li>
            <a href={`#${section.id}`}><span class="toc-num">{i + 1}</span>{section.title}</a>
          </li>
        {/each}
      </ol>
    </nav>
  </aside>

  <article class="content">
    <header class="hero">
      <p class="eyebrow">{copy.hero.eyebrow}</p>
      <h1>{copy.hero.title}</h1>
      <p class="subtitle">{copy.hero.subtitle}</p>
    </header>

    {#each copy.sections as section, i (section.id)}
      <section id={section.id} class="block">
        <h2><span class="sec-num">{i + 1}</span>{section.title}</h2>
        {#each section.body as block}
          {#if block.t === 'p'}
            <p>
              {block.text}{#if block.link} <a href={$localizedHref(block.link.href)}>{block.link.label}</a>{block.link.suffix ?? ''}{/if}
            </p>
          {:else if block.t === 'sub'}
            <h3>{block.text}</h3>
          {:else if block.t === 'list'}
            <ul>
              {#each block.items as item}<li>{item}</li>{/each}
            </ul>
          {:else if block.t === 'code'}
            <pre><code>{block.code}</code></pre>
          {:else if block.t === 'note'}
            <div class="note">{block.text}</div>
          {/if}
        {/each}
      </section>
    {/each}
  </article>
</div>

<style>
  .manual {
    display: grid;
    grid-template-columns: 220px minmax(0, 1fr);
    gap: var(--space-10);
    max-width: 1080px;
    padding-top: var(--space-8);
    padding-bottom: var(--space-12);
    align-items: start;
  }

  .toc {
    position: sticky;
    top: calc(var(--topbar-height) + var(--space-6));
  }

  .toc ol {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .toc a {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-1) 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    text-decoration: none;
    line-height: 1.4;
  }

  .toc a:hover {
    color: var(--color-text);
  }

  .toc-num {
    color: var(--color-accent);
    font-weight: 600;
    flex: 0 0 auto;
  }

  .content {
    min-width: 0;
  }

  .hero {
    padding-bottom: var(--space-6);
    border-bottom: 1px solid var(--color-border);
    margin-bottom: var(--space-2);
  }

  .eyebrow {
    margin: 0 0 var(--space-2);
    font-size: var(--text-xs);
    letter-spacing: 0.18em;
    text-transform: uppercase;
    color: var(--color-text-subtle);
  }

  .hero h1 {
    margin: 0;
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    font-weight: 500;
    line-height: 1.08;
  }

  .subtitle {
    margin: var(--space-4) 0 0;
    max-width: 640px;
    color: var(--color-text-muted);
    font-size: var(--text-lg);
    line-height: 1.6;
  }

  .block {
    margin-top: var(--space-8);
    scroll-margin-top: calc(var(--topbar-height) + var(--space-6));
  }

  h2 {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    margin: 0 0 var(--space-4);
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: 500;
  }

  .sec-num {
    color: var(--color-accent);
    font-size: var(--text-lg);
    font-weight: 600;
  }

  h3 {
    margin: var(--space-5) 0 var(--space-2);
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--color-text);
  }

  p,
  li {
    color: var(--color-text-muted);
    font-size: var(--text-base);
    line-height: 1.72;
  }

  p {
    margin: 0 0 var(--space-3);
  }

  ul {
    margin: 0 0 var(--space-3);
    padding-left: var(--space-5);
  }

  li {
    margin-bottom: var(--space-1);
  }

  pre {
    margin: 0 0 var(--space-4);
    padding: var(--space-3) var(--space-4);
    background: #111111;
    border-radius: var(--radius-md);
    overflow-x: auto;
  }

  pre code {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    color: #d7ddd5;
    white-space: pre;
  }

  .note {
    margin: 0 0 var(--space-4);
    padding: var(--space-3) var(--space-4);
    border-left: 3px solid var(--color-accent);
    background: var(--color-bg-inset);
    border-radius: var(--radius-sm);
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 1.6;
  }

  @media (max-width: 768px) {
    .manual {
      grid-template-columns: 1fr;
      gap: var(--space-6);
    }

    .toc {
      position: static;
    }

    .toc ol {
      flex-direction: row;
      flex-wrap: wrap;
      gap: var(--space-2) var(--space-4);
      padding-bottom: var(--space-4);
      border-bottom: 1px solid var(--color-border);
    }

    .hero h1 {
      font-size: var(--text-2xl);
    }
  }
</style>
