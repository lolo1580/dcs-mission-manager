<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { showToast } from './toast.js';
  import Card from './ui/Card.svelte';
  import Badge from './ui/Badge.svelte';

  let settings = { serverUrl: '', hasToken: false, clientId: '', lastSync: 0, lastResult: '' };
  let serverUrl = '';
  let token = '';
  let loading = true;
  let saving = false;
  let testing = false;
  let syncing = false;
  let error = '';
  let info = '';

  async function getJSON(path) {
    const response = await fetch(path);
    if (!response.ok) throw new Error(`${response.status}`);
    return response.json();
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const body = await getJSON('/api/sync/settings');
      settings = body;
      serverUrl = body.serverUrl ?? '';
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function saveSettings(event) {
    event.preventDefault();
    saving = true;
    error = '';
    info = '';
    try {
      const response = await fetch('/api/sync/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ serverUrl: serverUrl.trim(), token: token.trim() }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      settings = body;
      token = '';
      showToast($t('sync.saved'), 'success');
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }

  async function testConnection() {
    testing = true;
    error = '';
    info = '';
    try {
      const response = await fetch('/api/sync/test', { method: 'POST' });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      info = `DCS Manager ${body.version ?? ''} · ${body.mode ?? ''}`;
    } catch (err) {
      error = err.message;
    } finally {
      testing = false;
    }
  }

  async function syncNow() {
    syncing = true;
    error = '';
    info = '';
    try {
      const response = await fetch('/api/sync/run', { method: 'POST' });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      info = $t('sync.result', { count: body.imported ?? 0 });
      info += ` · ${body.checklists ?? 0} ${$t('checklist.title')}`;
      if (body.conflicts) info += ` · ${body.conflicts} ${$t('admin.conflicts')}`;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      syncing = false;
    }
  }

  function dateOf(ms) {
    return ms ? new Date(ms).toLocaleString() : $t('sync.never');
  }
</script>

<Card title={$t('sync.title')} subtitle={$t('sync.hint')}>
  {#if loading}
    <p class="hint">…</p>
  {:else}
    <form class="form" on:submit={saveSettings}>
      <label class="field">
        <span>{$t('sync.serverUrl')}</span>
        <input bind:value={serverUrl} placeholder="http://192.168.1.20:8080" autocapitalize="none" spellcheck="false" />
      </label>
      <label class="field">
        <span>{$t('sync.token')}</span>
        <input type="password" bind:value={token} placeholder={settings.hasToken ? $t('sync.tokenSaved') : ''} autocomplete="off" />
      </label>
      <label class="field">
        <span>{$t('sync.clientId')}</span>
        <input value={settings.clientId} readonly />
      </label>
      <p class="hint">{$t('sync.clientIdHint')}</p>

      <div class="actions">
        <Badge tone={settings.hasToken && settings.serverUrl ? 'green' : 'amber'}>
          {settings.hasToken && settings.serverUrl ? $t('sync.configured') : $t('sync.notConfigured')}
        </Badge>
        <span class="spacer"></span>
        <button class="btn" type="submit" disabled={saving}>{$t('sync.save')}</button>
        <button class="btn" type="button" on:click={testConnection} disabled={testing || !settings.hasToken}>
          {testing ? $t('sync.running') : $t('sync.test')}
        </button>
        <button class="btn primary" type="button" on:click={syncNow} disabled={syncing || !settings.hasToken}>
          {syncing ? $t('sync.running') : $t('sync.run')}
        </button>
      </div>
    </form>

    {#if error}<p class="error">{$t('sync.error', { detail: error })}</p>{/if}
    {#if info}<p class="info">{info}</p>{/if}
    <p class="hint">{$t('sync.lastSync', { date: dateOf(settings.lastSync) })}{#if settings.lastResult} · {settings.lastResult}{/if}</p>
  {/if}
</Card>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .spacer {
    flex: 1;
  }

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 0.74rem;
    line-height: 1.4;
  }

  .error {
    margin: 0.5rem 0 0;
    color: var(--red);
    font-size: 0.78rem;
  }

  .info {
    margin: 0.5rem 0 0;
    color: var(--green);
    font-size: 0.78rem;
  }
</style>
