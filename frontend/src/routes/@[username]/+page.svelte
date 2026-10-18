<script lang="ts">
  import { page } from '$app/stores';
  import { t } from '$lib/stores/locale';
  import { session, updateSessionUser } from '$lib/stores/session';
  import { userApi } from '$lib/api/user';
  import type { ActivityDay, PublicProfile, User } from '$lib/api/types';
  import { copyToClipboard } from '$lib/utils';
  import { isSupportedAvatarMediaType, readFileAsDataURL } from '$lib/utils/avatar';
  import ActivityHeatmap from '$lib/components/profile/ActivityHeatmap.svelte';
  import Spinner from '$lib/components/ui/Spinner.svelte';
  import Button from '$lib/components/ui/Button.svelte';

  const avatarAccept = 'image/png,image/jpeg,image/gif,image/webp';

  let profile = $state<PublicProfile | null>(null);
  let activity = $state<ActivityDay[]>([]);
  let loading = $state(true);
  let error = $state(false);
  let loadedUsername = $state('');
  let uploadingAvatar = $state(false);
  let avatarErrorKey = $state('');
  let avatarPreview = $state('');
  let avatarInput = $state<HTMLInputElement | null>(null);
  let handleCopied = $state(false);
  let copyTimer: ReturnType<typeof setTimeout> | undefined;

  const username = $derived($page.params.username ?? '');
  const isOwnProfile = $derived(Boolean($session.user && profile?.user.username === $session.user.username));
  const currentInitial = $derived((profile?.user.username || '?').charAt(0).toUpperCase());

  function formatProfileStat(value: number) {
    if (Math.abs(value) < 1000) return value.toString();
    const rounded = Math.round((value / 1000) * 10) / 10;
    return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)}k`;
  }

  $effect(() => {
    if (username && username !== loadedUsername) {
      loadedUsername = username;
      load(username);
    }
  });

  async function load(targetUsername = username) {
    loading = true;
    error = false;
    avatarErrorKey = '';
    try {
      const [p, a] = await Promise.all([userApi.publicProfile(targetUsername), userApi.publicActivity(targetUsername)]);
      profile = p;
      activity = a ?? [];
      avatarPreview = p.user.avatar_url ?? '';
    } catch {
      error = true;
    } finally {
      loading = false;
    }
  }

  function mergeUploadedUser(user: User) {
    updateSessionUser(user);
    if (!profile) return;
    profile = {
      ...profile,
      user: {
        username: user.username,
        display_name: user.display_name ?? profile.user.display_name,
        avatar_url: user.avatar_url ?? profile.user.avatar_url
      }
    };
    avatarPreview = user.avatar_url ?? avatarPreview;
  }

  async function copyPublicProfileURL() {
    if (!profile) return;
    if (await copyToClipboard(`${location.origin}/@${profile.user.username}`)) {
      handleCopied = true;
      clearTimeout(copyTimer);
      copyTimer = setTimeout(() => (handleCopied = false), 1500);
    }
  }

  async function handleAvatarChange(event: Event) {
    if (!isOwnProfile) return;
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file || uploadingAvatar) return;
    avatarErrorKey = '';
    if (!isSupportedAvatarMediaType(file.type)) {
      avatarErrorKey = 'profile.unsupportedAvatarType';
      input.value = '';
      return;
    }
    const previousPreview = avatarPreview;
    uploadingAvatar = true;
    try {
      const dataURL = await readFileAsDataURL(file);
      avatarPreview = dataURL;
      const updated = await userApi.uploadAvatar({ data: dataURL, mediaType: file.type });
      mergeUploadedUser(updated);
    } catch {
      avatarPreview = previousPreview;
      avatarErrorKey = 'profile.avatarUploadFailed';
    } finally {
      uploadingAvatar = false;
      input.value = '';
    }
  }
</script>

<div class="app-container">
  {#if loading}
    <div class="state"><Spinner /></div>
  {:else if error || !profile}
    <div class="state">
      <p class="muted">{$t('common.error')}</p>
      <Button variant="secondary" onclick={() => load()}>{$t('common.retry')}</Button>
    </div>
  {:else}
    <div class="profile-shell">
      <div class="profile-card">
        <div class="brand-mark">
          <img src="/brand/logo-240.png" alt={$t('common.appName')} />
        </div>
        <div class="profile-art"></div>

        <section class="identity">
          {#if isOwnProfile}
            <button
              class="avatar-button"
              type="button"
              aria-label={$t('profile.avatarUploadHint')}
              title={$t('profile.avatarUploadHint')}
              onclick={() => avatarInput?.click()}
              disabled={uploadingAvatar}
            >
              {#if avatarPreview}
                <img class="avatar" src={avatarPreview} alt={profile.user.username} />
              {:else}
                <span class="avatar avatar-fallback">{currentInitial}</span>
              {/if}
            </button>
            <input
              bind:this={avatarInput}
              class="visually-hidden"
              type="file"
              accept={avatarAccept}
              onchange={handleAvatarChange}
            />
          {:else}
            <div class="avatar-static">
              {#if avatarPreview}
                <img class="avatar" src={avatarPreview} alt={profile.user.username} />
              {:else}
                <span class="avatar avatar-fallback">{currentInitial}</span>
              {/if}
            </div>
          {/if}
          {#if avatarErrorKey}
            <p class="avatar-status error-text">{$t(avatarErrorKey)}</p>
          {:else if uploadingAvatar}
            <p class="avatar-status">{$t('profile.uploadingAvatar')}</p>
          {/if}
          <div class="identity-text">
            <div class="name">{profile.user.display_name || profile.user.username}</div>
            <button
              class="handle"
              type="button"
              title={$t('profile.copyPublicUrl')}
              aria-label={$t('profile.copyPublicUrl')}
              onclick={copyPublicProfileURL}
            >
              @{profile.user.username}
            </button>
            {#if handleCopied}
              <span class="copy-status">{$t('profile.copiedPublicUrl')}</span>
            {/if}
          </div>
        </section>

        <section class="activity">
          <h3>{$t('profile.activity')}</h3>
          <ActivityHeatmap days={activity} />
        </section>

        <section class="stats">
          <div class="stat stat-messages"><span class="num">{formatProfileStat(profile.stats.total_messages)}</span><span class="lbl">{$t('profile.msgs')}</span></div>
          <div class="stat stat-added"><span class="num added">+{formatProfileStat(profile.stats.lines_added)}</span><span class="lbl">{$t('profile.added')}</span></div>
          <div class="stat stat-threads"><span class="num">{formatProfileStat(profile.stats.total_threads)}</span><span class="lbl">{$t('profile.threads')}</span></div>
          <div class="stat stat-removed"><span class="num removed">-{formatProfileStat(profile.stats.lines_removed)}</span><span class="lbl">{$t('profile.removed')}</span></div>
          <div class="stat stat-changed"><span class="num changed">~{formatProfileStat(profile.stats.files_changed)}</span><span class="lbl">{$t('profile.changed')}</span></div>
        </section>
      </div>
    </div>
  {/if}
</div>

<style>
  .profile-shell {
    max-width: 660px;
    margin: 0 auto;
  }

  .profile-card {
    position: relative;
    display: grid;
    grid-template-columns: 200px minmax(0, 1fr);
    grid-template-rows: 86px 198px 176px 150px;
    overflow: hidden;
    min-height: 610px;
    background:
      linear-gradient(rgba(132, 211, 162, 0.2) 1px, transparent 1px),
      linear-gradient(90deg, rgba(132, 211, 162, 0.2) 1px, transparent 1px),
      linear-gradient(135deg, rgba(255, 117, 93, 0.1), rgba(255, 253, 247, 0.72) 38%, rgba(129, 105, 214, 0.12)),
      var(--color-bg-elevated);
    background-size: 100% 150px, 200px 100%, auto, auto;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    box-shadow:
      0 18px 48px rgba(17, 17, 17, 0.12),
      inset 0 0 0 7px rgba(255, 199, 189, 0.34);
  }

  .profile-card::before {
    position: absolute;
    inset: 8px;
    content: '';
    pointer-events: none;
    border: 1px dashed rgba(201, 192, 170, 0.48);
    border-radius: var(--radius-lg);
  }

  .brand-mark {
    position: relative;
    z-index: 1;
    grid-column: 1 / -1;
    align-self: start;
    padding: var(--space-5) var(--space-5) 0;
  }

  .brand-mark img {
    display: block;
    width: auto;
    height: 26px;
    opacity: 0.42;
  }

  .profile-art {
    position: absolute;
    top: 24px;
    right: 14px;
    width: 250px;
    height: 250px;
    pointer-events: none;
    opacity: 0.55;
    background:
      repeating-radial-gradient(ellipse at center, rgba(255, 90, 69, 0.22) 0 1px, transparent 2px 7px),
      repeating-radial-gradient(ellipse at center, rgba(36, 130, 50, 0.16) 0 1px, transparent 2px 9px);
    border-radius: 46% 54% 48% 52%;
    transform: rotate(14deg);
    filter: blur(0.2px);
  }

  .identity {
    position: relative;
    z-index: 1;
    grid-column: 1 / 2;
    grid-row: 2 / 4;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    justify-content: center;
    gap: var(--space-3);
    padding-left: var(--space-6);
  }

  .avatar-button,
  .avatar-static {
    padding: 0;
    background: transparent;
    border: 0;
    border-radius: var(--radius-full);
  }

  .avatar-button {
    cursor: pointer;
  }

  .avatar-button:disabled {
    cursor: progress;
    opacity: 0.7;
  }

  .avatar {
    width: 88px;
    height: 88px;
    border-radius: var(--radius-full);
    object-fit: cover;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .avatar-fallback {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    font-size: var(--text-4xl);
    font-weight: 600;
  }

  .avatar-status {
    max-width: 140px;
    margin: 0;
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
  }

  .error-text {
    color: var(--color-danger);
  }

  .identity-text {
    position: relative;
    min-height: 4.6rem;
    transform: translateY(12px);
  }

  .name {
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    line-height: 1;
    white-space: nowrap;
  }

  .handle {
    display: block;
    margin-top: var(--space-2);
    padding: 0;
    color: var(--color-text-muted);
    background: transparent;
    border: 0;
    cursor: pointer;
    font-family: var(--font-mono);
    font-size: var(--text-lg);
  }

  .handle:hover {
    color: var(--color-text);
  }

  .copy-status {
    position: absolute;
    top: calc(100% + var(--space-1));
    left: 0;
    color: var(--color-success);
    font-size: var(--text-xs);
    white-space: nowrap;
  }

  .activity {
    position: relative;
    z-index: 1;
    grid-column: 2 / 3;
    grid-row: 3 / 4;
    display: grid;
    align-content: end;
    justify-content: end;
    padding: 0 var(--space-5) var(--space-4) 0;
  }

  .activity h3 {
    position: absolute;
    left: calc(-1 * var(--space-4));
    bottom: var(--space-4);
    writing-mode: vertical-rl;
    margin: 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    opacity: 0.45;
  }

  .stats {
    position: relative;
    z-index: 1;
    grid-column: 1 / -1;
    grid-row: 4 / 5;
    display: grid;
    grid-template-columns: 200px minmax(0, 1fr) minmax(0, 1fr);
    grid-template-rows: repeat(3, 1fr);
    align-items: center;
    padding: 0 var(--space-5);
    border-top: 1px dashed rgba(201, 192, 170, 0.42);
    pointer-events: none;
  }

  .stat {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    min-width: 0;
  }

  .stat-messages {
    grid-column: 1 / 2;
    grid-row: 1 / 2;
  }

  .stat-threads {
    grid-column: 1 / 2;
    grid-row: 2 / 3;
  }

  .stat-added {
    grid-column: 2 / 3;
    grid-row: 1 / 2;
  }

  .stat-removed {
    grid-column: 2 / 3;
    grid-row: 2 / 3;
    justify-self: end;
    padding-right: var(--space-5);
  }

  .stat-changed {
    grid-column: 3 / 4;
    grid-row: 3 / 4;
    justify-self: end;
  }

  .num {
    font-size: var(--text-2xl);
    font-weight: 600;
    line-height: 1;
  }

  .num.added {
    color: var(--color-added);
  }
  .num.removed {
    color: var(--color-removed);
  }
  .num.changed {
    color: var(--color-changed);
  }

  .lbl {
    color: var(--color-text-subtle);
    font-size: var(--text-xs);
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-10) 0;
  }

  .muted {
    color: var(--color-text-muted);
  }

  @media (max-width: 720px) {
    .profile-shell {
      max-width: 420px;
    }

    .profile-card {
      grid-template-columns: 1fr;
      grid-template-rows: 76px auto auto auto;
      min-height: auto;
      background-size: 100% 116px, 116px 100%, auto, auto;
    }

    .profile-art {
      width: 220px;
      height: 190px;
      opacity: 0.38;
    }

    .brand-mark {
      padding: var(--space-5) var(--space-5) 0;
    }

    .brand-mark img {
      height: 24px;
    }

    .identity {
      grid-column: 1;
      grid-row: 2;
      padding: var(--space-4) var(--space-5) var(--space-6);
    }

    .avatar {
      width: 80px;
      height: 80px;
    }

    .identity-text {
      transform: none;
    }

    .name {
      font-size: var(--text-2xl);
    }

    .activity {
      grid-column: 1;
      grid-row: 3;
      justify-content: start;
      padding: 0 var(--space-5) var(--space-4);
    }

    .activity h3 {
      position: static;
      writing-mode: horizontal-tb;
      margin-bottom: var(--space-3);
    }

    .stats {
      grid-column: 1;
      grid-row: 4;
      grid-template-columns: 1fr;
      grid-template-rows: none;
      gap: var(--space-3);
      padding: var(--space-4) var(--space-5) var(--space-5);
    }

    .stat,
    .stat-messages,
    .stat-threads,
    .stat-added,
    .stat-removed,
    .stat-changed {
      grid-column: auto;
      grid-row: auto;
      justify-self: start;
      padding-right: 0;
    }
  }
</style>
