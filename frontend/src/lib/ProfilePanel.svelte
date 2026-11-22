<script>
  import { onMount } from 'svelte';
  import { sessionUser, isAdmin } from './session.js';
  import { t } from './i18n.js';
  import { showToast } from './toast.js';
  import Card from './ui/Card.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';

  let displayName = '';
  let email = '';
  let dcsNames = '';
  let savingProfile = false;
  let profileError = '';

  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';
  let savingPassword = false;
  let passwordError = '';

  let tokens = [];
  let tokenName = '';
  let createdToken = '';
  let tokensLoading = true;
  let tokenError = '';

  $: if ($sessionUser) {
    if (!displayName) displayName = $sessionUser.displayName ?? '';
    if (!email) email = $sessionUser.email ?? '';
    if (!dcsNames) dcsNames = ($sessionUser.dcsNames ?? []).join(', ');
  }

  async function getJSON(path) {
    const response = await fetch(path);
    if (!response.ok) throw new Error(`${response.status}`);
    return response.json();
  }

  async function loadTokens() {
    tokensLoading = true;
    tokenError = '';
    try {
      const body = await getJSON('/api/auth/tokens');
      tokens = body.tokens ?? [];
    } catch (error) {
      tokenError = error.message;
    } finally {
      tokensLoading = false;
    }
  }

  onMount(loadTokens);

  async function saveProfile(event) {
    event.preventDefault();
    savingProfile = true;
    profileError = '';
    try {
      const names = dcsNames.split(/[\n,]/).map((value) => value.trim()).filter(Boolean);
      const response = await fetch('/api/auth/profile', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ displayName: displayName.trim(), email: email.trim(), dcsNames: names }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      sessionUser.set(body.user);
      showToast($t('profile.saved'), 'success');
    } catch (error) {
      profileError = $t('profile.error', { detail: error.message });
    } finally {
      savingProfile = false;
    }
  }

  async function changePassword(event) {
    event.preventDefault();
    passwordError = '';
    if (newPassword !== confirmPassword) {
      passwordError = $t('profile.passwordMismatch');
      return;
    }
    savingPassword = true;
    try {
      const response = await fetch('/api/auth/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ currentPassword, newPassword }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
      showToast($t('profile.passwordChanged'), 'success');
    } catch (error) {
      passwordError = $t('profile.error', { detail: error.message });
    } finally {
      savingPassword = false;
    }
  }

  async function createToken(event) {
    event.preventDefault();
    tokenError = '';
    try {
      const response = await fetch('/api/auth/tokens', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: tokenName.trim() }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      createdToken = body.token;
      tokenName = '';
      await loadTokens();
    } catch (error) {
      tokenError = error.message;
    }
  }

  async function revokeToken(token) {
    if (!confirm(`${$t('profile.deleteToken')}: ${token.name}?`)) return;
    try {
      const response = await fetch(`/api/auth/tokens/${token.id}`, { method: 'DELETE' });
      if (!response.ok) throw new Error(`${response.status}`);
      await loadTokens();
    } catch (error) {
      tokenError = error.message;
    }
  }

  function dateOf(ms) {
    return ms ? new Date(ms).toLocaleString() : $t('admin.never');
  }
</script>

<div class="profile">
  <header class="identity">
    <div class="avatar">{$sessionUser?.username?.slice(0, 2).toUpperCase() ?? '?'}</div>
    <div>
      <h2>{$sessionUser?.displayName || $sessionUser?.username}</h2>
      <p>{$sessionUser?.username} · <Badge tone={$isAdmin ? 'blue' : 'neutral'}>{$isAdmin ? $t('admin.role.admin') : $t('admin.role.member')}</Badge></p>
    </div>
  </header>

  <div class="columns">
    <Card title={$t('profile.title')}>
      <form class="form" on:submit={saveProfile}>
        <label class="field">
          <span>{$t('profile.displayName')}</span>
          <input bind:value={displayName} maxlength="80" />
        </label>
        <label class="field">
          <span>{$t('profile.email')}</span>
          <input type="email" bind:value={email} maxlength="160" autocomplete="email" />
        </label>
        <p class="hint">{$t('profile.emailHint')}</p>
        <label class="field">
          <span>{$t('profile.dcsNames')}</span>
          <textarea bind:value={dcsNames} rows="3" placeholder="Viper, Hornet"></textarea>
        </label>
        <p class="hint">{$t('profile.dcsNamesHint')}</p>
        {#if profileError}<p class="error">{profileError}</p>{/if}
        <div>
          <button class="btn primary" type="submit" disabled={savingProfile}>{$t('profile.saveProfile')}</button>
        </div>
      </form>
    </Card>

    <Card title={$t('profile.password')}>
      <form class="form" on:submit={changePassword}>
        <label class="field">
          <span>{$t('profile.currentPassword')}</span>
          <input type="password" bind:value={currentPassword} autocomplete="current-password" required />
        </label>
        <label class="field">
          <span>{$t('profile.newPassword')}</span>
          <input type="password" bind:value={newPassword} autocomplete="new-password" minlength="10" required />
        </label>
        <label class="field">
          <span>{$t('profile.confirmPassword')}</span>
          <input type="password" bind:value={confirmPassword} autocomplete="new-password" minlength="10" required />
        </label>
        {#if passwordError}<p class="error">{passwordError}</p>{/if}
        <div>
          <button class="btn primary" type="submit" disabled={savingPassword}>{$t('profile.changePassword')}</button>
        </div>
      </form>
    </Card>
  </div>

  <Card title={$t('profile.tokens')} subtitle={$t('profile.tokensHint')}>
    <form class="token-form" on:submit={createToken}>
      <input bind:value={tokenName} placeholder={$t('profile.tokenName')} maxlength="80" required />
      <button class="btn primary" type="submit">{$t('profile.createToken')}</button>
    </form>

    {#if createdToken}
      <div class="token-once">
        <p>{$t('profile.tokenOnce')}</p>
        <code>{createdToken}</code>
      </div>
    {/if}
    {#if tokenError}<p class="error">{tokenError}</p>{/if}

    {#if tokensLoading}
      <p class="hint">…</p>
    {:else if tokens.length === 0}
      <EmptyState title={$t('profile.noTokens')} message="" icon="M4 8h16v10H4zM8 8V6a4 4 0 0 1 8 0v2" />
    {:else}
      <ul class="token-list">
        {#each tokens as token (token.id)}
          <li>
            <div>
              <strong>{token.name}</strong>
              <span>{dateOf(token.createdAt)} · {dateOf(token.lastUsedAt)}</span>
            </div>
            <button class="btn danger" on:click={() => revokeToken(token)}>{$t('profile.deleteToken')}</button>
          </li>
        {/each}
      </ul>
    {/if}
  </Card>
</div>

<style>
  .profile {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    padding: 1rem;
    max-width: 1000px;
  }

  .identity {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .avatar {
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    background: var(--blue-soft);
    color: var(--blue);
    font-weight: 700;
  }

  .identity h2 {
    margin: 0;
    font-size: 1.05rem;
  }

  .identity p {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    margin: 0.15rem 0 0;
    color: var(--muted);
    font-size: 0.78rem;
  }

  .columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem;
    align-items: start;
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 0.74rem;
    line-height: 1.4;
  }

  .error {
    margin: 0;
    color: var(--red);
    font-size: 0.78rem;
  }

  .token-form {
    display: flex;
    gap: 0.4rem;
    margin-bottom: 0.6rem;
  }

  .token-form input {
    flex: 1;
    min-width: 0;
    padding: 0.4rem 0.55rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
  }

  .token-once {
    padding: 0.55rem 0.65rem;
    margin-bottom: 0.6rem;
    background: var(--amber-soft);
    border: 1px solid color-mix(in srgb, var(--amber) 40%, transparent);
    border-radius: var(--radius);
  }

  .token-once p {
    margin: 0 0 0.3rem;
    color: var(--amber);
    font-size: 0.76rem;
  }

  .token-once code {
    display: block;
    overflow-wrap: anywhere;
    font-size: 0.76rem;
    color: var(--text);
  }

  .token-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .token-list li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    padding: 0.45rem 0.55rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .token-list div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .token-list strong {
    font-size: 0.82rem;
  }

  .token-list span {
    color: var(--muted);
    font-size: 0.72rem;
  }

  @media (max-width: 760px) {
    .columns {
      grid-template-columns: 1fr;
    }

    .profile {
      padding: 0.7rem;
    }

    .token-form {
      flex-direction: column;
    }
  }
</style>
