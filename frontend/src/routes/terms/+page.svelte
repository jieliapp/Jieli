<script lang="ts">
  const effectiveDate = 'June 8, 2026';
  const contactEmail = 'admin@jieli.app';
  const issueURL = 'https://github.com/jieliapp/jieli/issues';
</script>

<svelte:head>
  <title>Terms of Service - Jieli</title>
  <meta
    name="description"
    content="Jieli terms of service for account access, uploaded content, acceptable use, service availability, and contact information."
  />
</svelte:head>

<article class="app-container terms">
  <header class="terms-header">
    <p class="eyebrow">Jieli</p>
    <h1>Terms of Service</h1>
    <p class="effective">Effective date: {effectiveDate}</p>
  </header>

  <section>
    <h2>Overview</h2>
    <p>
      These Terms of Service govern your use of Jieli, a tool for syncing, browsing, searching, and sharing AI
      coding assistant threads. By using Jieli, you agree to these terms.
    </p>
  </section>

  <section>
    <h2>Accounts and Authentication</h2>
    <p>
      Jieli uses Google OAuth and GitHub OAuth for sign-in. You are responsible for keeping your account,
      browser session, deployment credentials, and upload API keys secure.
    </p>
  </section>

  <section>
    <h2>Uploaded Content</h2>
    <p>
      You are responsible for the AI coding threads, messages, metadata, repository names, labels, attachments,
      and other content you upload to Jieli. You should only upload content that you have the right to store and
      process.
    </p>
    <p>
      Jieli may process uploaded content to redact detected sensitive information, including but not limited to
      passwords, API keys, access tokens, private keys, secrets, credit card numbers, and credit card CVV values.
      Redaction is a protective measure and is not a guarantee that all sensitive information will be detected.
    </p>
  </section>

  <section>
    <h2>Service Ownership</h2>
    <p>
      Jieli, including its service, software, user interface, brand, documentation, and related technology, is
      owned by Jieli or its licensors. These Terms do not grant you source code access, ownership rights, or any
      right to copy, modify, redistribute, reverse engineer, or create derivative works from Jieli, except where
      expressly permitted in a separate written agreement.
    </p>
    <p>
      You retain ownership of the content you upload. You grant Jieli the limited rights needed to host, process,
      redact, index, search, display, export, and share that content as necessary to provide the service.
    </p>
  </section>

  <section>
    <h2>Acceptable Use</h2>
    <p>You agree not to use Jieli to:</p>
    <ul>
      <li>Upload content that you do not have permission to store or process.</li>
      <li>Violate applicable laws or regulations.</li>
      <li>Attempt to access another user's private threads, account, or API keys.</li>
      <li>Interfere with the security, availability, or integrity of the service.</li>
      <li>Use shared links to publish unlawful, abusive, or harmful content.</li>
    </ul>
  </section>

  <section>
    <h2>Sharing</h2>
    <p>
      Threads are private by default. If you choose to make a thread unlisted, anyone with the unlisted link may
      view that shared thread. You are responsible for deciding what content is appropriate to share.
    </p>
  </section>

  <section>
    <h2>Enterprise Deployments</h2>
    <p>
      Jieli may offer enterprise self-hosted or on-premise deployments. These deployments may be provided as
      compiled binaries, deployment packages, configuration files, documentation, and support services. Unless
      otherwise agreed in writing, enterprise deployments do not include source code access, source code licenses,
      or rights to modify, redistribute, reverse engineer, or create derivative works of Jieli.
    </p>
    <p>
      Enterprise customers are responsible for their own infrastructure, access control, backups, network
      security, and compliance obligations in their deployment environment. Jieli may provide technical support,
      maintenance, and deployment assistance according to the applicable order form, service agreement, or support
      plan.
    </p>
  </section>

  <section>
    <h2>Support and Issue Reporting</h2>
    <p>
      Jieli may use GitHub issues for public bug reports, feature requests, and support discussions. GitHub issues
      are not a source code license and do not imply that Jieli is open source. Do not post passwords, API keys,
      tokens, private keys, confidential customer data, or other sensitive information in public issues.
    </p>
    <p>
      You can report issues at <a href={issueURL} target="_blank" rel="noreferrer">{issueURL}</a>.
    </p>
  </section>

  <section>
    <h2>Service Availability</h2>
    <p>
      Jieli is provided without a guarantee of uninterrupted availability. Features may change over time as the
      product evolves.
    </p>
  </section>

  <section>
    <h2>Disclaimer</h2>
    <p>
      Jieli is provided as is, without warranties of any kind to the fullest extent permitted by law. We are not
      liable for indirect, incidental, special, consequential, or punitive damages arising from your use of the
      service.
    </p>
  </section>

  <section>
    <h2>Changes</h2>
    <p>
      We may update these Terms from time to time. Changes will be posted on this page with an updated effective
      date.
    </p>
  </section>

  <section>
    <h2>Contact</h2>
    <p>
      If you have questions about these Terms, contact us at
      <a href={`mailto:${contactEmail}`}>{contactEmail}</a>.
    </p>
  </section>
</article>

<style>
  .terms {
    max-width: 860px;
    padding-top: var(--space-8);
    padding-bottom: var(--space-12);
  }

  .terms-header {
    padding-bottom: var(--space-5);
    border-bottom: 1px solid var(--color-border);
    margin-bottom: var(--space-6);
  }

  .eyebrow {
    margin: 0 0 var(--space-2);
    color: var(--color-text-subtle);
    font-size: var(--text-sm);
    font-weight: 700;
    text-transform: uppercase;
  }

  h1 {
    margin: 0;
    font-size: clamp(2rem, 4vw, 3.5rem);
    line-height: 1.05;
    letter-spacing: 0;
  }

  .effective {
    margin: var(--space-3) 0 0;
    color: var(--color-text-muted);
  }

  section {
    margin-top: var(--space-6);
  }

  h2 {
    margin: 0 0 var(--space-3);
    font-size: var(--text-xl);
    letter-spacing: 0;
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

  a {
    color: var(--color-link);
  }
</style>
