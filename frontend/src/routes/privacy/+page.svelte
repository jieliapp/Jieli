<script lang="ts">
  const effectiveDate = 'June 12, 2026';
  const contactEmail = 'admin@jieli.app';
  const issueURL = 'https://github.com/jieliapp/jieli/issues';
  const pluginURL = 'https://github.com/jieliapp/plugins';
</script>

<svelte:head>
  <title>Privacy Policy - Jieli</title>
  <meta
    name="description"
    content="Jieli privacy policy for OAuth sign-in, uploaded coding threads, sensitive data redaction, storage, sharing, and deletion."
  />
</svelte:head>

<article class="app-container policy">
  <header class="policy-header">
    <p class="eyebrow">Jieli</p>
    <h1>Privacy Policy</h1>
    <p class="effective">Effective date: {effectiveDate}</p>
  </header>

  <section>
    <h2>Overview</h2>
    <p>
      Jieli is a tool for syncing, browsing, searching, and sharing AI coding assistant threads. This Privacy
      Policy explains what information Jieli collects, how that information is used, and how you can contact us
      about your data.
    </p>
  </section>

  <section>
    <h2>Information We Collect</h2>
    <p>
      When you sign in with Google or GitHub, we collect basic account information needed to authenticate you,
      including your email address, display name, avatar image URL, and OAuth provider user ID.
    </p>
    <p>
      When you use Jieli, we may store content you choose to upload, including AI coding threads, messages,
      metadata, repository names, labels, and related attachments.
    </p>
  </section>

  <section>
    <h2>Sensitive Data Redaction</h2>
    <p>
      Sensitive information redaction runs inside the open-source Jieli plugin on your own machine, before a thread
      is uploaded. Detected sensitive information — including, but not limited to, passwords, API keys, access
      tokens, private keys, secrets, credit card numbers, and credit card CVV values — is redacted locally, so the
      detected values are not transmitted to or stored by Jieli's servers.
    </p>
    <p>
      Because the plugin is open source, you can review exactly how redaction works at
      <a href={pluginURL} target="_blank" rel="noreferrer">{pluginURL}</a>.
    </p>
    <p>
      Redaction is best-effort and pattern-based, so it may not catch every secret. You should still avoid
      intentionally uploading sensitive information and should rotate any credential that may have been exposed.
    </p>
  </section>

  <section>
    <h2>How We Use Information</h2>
    <p>We use the information we collect to:</p>
    <ul>
      <li>Create and manage your Jieli account.</li>
      <li>Authenticate your browser session.</li>
      <li>Associate uploaded threads and API keys with your account.</li>
      <li>Display your profile, avatar, and synced thread data.</li>
      <li>Provide search, browsing, export, and sharing features.</li>
    </ul>
    <p>
      Jieli does not use Google OAuth to access Gmail, Google Drive, Google Calendar, or other Google product
      data.
    </p>
  </section>

  <section>
    <h2>Use of Your Content for AI Training</h2>
    <p>
      Jieli does not use your uploaded content — including threads, messages, code, prompts, and attachments — to
      train, fine-tune, or improve any machine learning or AI models, whether our own or operated by third parties.
      We do not sell your uploaded content, and we do not use it for advertising.
    </p>
    <p>
      We access your uploaded content only as needed to operate the service — for example, to sync, store,
      display, search, export, or share your threads at your direction, or to provide support that you request.
    </p>
  </section>

  <section>
    <h2>Sharing</h2>
    <p>
      We do not sell your personal information. Your private threads are only visible to your account. If you
      choose to make a thread unlisted, anyone with the unlisted link may view that shared thread.
    </p>
    <p>
      We may disclose information if required by law or to protect the security and integrity of the service.
    </p>
  </section>

  <section>
    <h2>Data Storage and Security</h2>
    <p>
      Jieli stores account and thread data in its application database. API keys are stored as hashes or encrypted
      values where applicable.
    </p>
    <p>
      For Jieli Cloud, account and thread data is stored in infrastructure operated for the Jieli service.
      For enterprise self-hosted or on-premise deployments, customer data may be stored in infrastructure
      controlled by the customer.
    </p>
    <p>
      In enterprise deployments, the customer is responsible for infrastructure security, access control,
      backups, retention, and compliance obligations in its own environment. Jieli may process limited diagnostic,
      support, account, or contact information when providing technical support, maintenance, or deployment
      assistance, subject to the applicable agreement.
    </p>
  </section>

  <section>
    <h2>Data Retention</h2>
    <p>
      We retain your account information and uploaded content for as long as your account is active, so that you
      can browse, search, and share your threads. Storage limits for each plan are described on our pricing page.
      When a limit is reached, content is removed only through actions you choose, such as manual deletion or an
      optional automatic cleanup that is off by default and that you can enable.
    </p>
    <p>
      For most assistants, the original threads also remain on your local disk, and every thread can be exported
      as Markdown or JSON, so you can keep your own copy before deleting content from Jieli.
    </p>
  </section>

  <section>
    <h2>Data Deletion</h2>
    <p>
      You may delete uploaded threads or stop using the service at any time. To request deletion of your account
      data, contact us at <a href={`mailto:${contactEmail}`}>{contactEmail}</a>.
    </p>
  </section>

  <section>
    <h2>Support and Issue Reporting</h2>
    <p>
      Jieli may use GitHub issues for public bug reports, feature requests, and support discussions. Information
      you choose to post in GitHub issues may be publicly visible and is processed by GitHub according to its own
      policies. Do not include passwords, API keys, tokens, private keys, confidential customer data, or other
      sensitive information in public issues.
    </p>
    <p>
      You can report issues at <a href={issueURL} target="_blank" rel="noreferrer">{issueURL}</a>.
    </p>
  </section>

  <section>
    <h2>Third-Party Authentication</h2>
    <p>
      Jieli uses Google OAuth and GitHub OAuth for sign-in. These providers may process authentication data
      according to their own privacy policies.
    </p>
  </section>

  <section>
    <h2>Changes</h2>
    <p>
      We may update this Privacy Policy from time to time. Changes will be posted on this page with an updated
      effective date.
    </p>
  </section>

  <section>
    <h2>Contact</h2>
    <p>
      If you have questions about this Privacy Policy, contact us at
      <a href={`mailto:${contactEmail}`}>{contactEmail}</a>.
    </p>
  </section>
</article>

<style>
  .policy {
    max-width: 860px;
    padding-top: var(--space-8);
    padding-bottom: var(--space-12);
  }

  .policy-header {
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
