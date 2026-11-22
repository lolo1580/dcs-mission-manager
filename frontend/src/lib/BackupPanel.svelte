<script>
  /**
   * Player-profile backup: archive the parts of DCS's Saved Games folder that are
   * painful to lose (logbook, bindings, options…), list what has been saved, and
   * restore an archive back. Categories are selectable; restore can be dry-run so
   * the user sees what it would touch before committing.
   */
  import { onMount } from 'svelte';
  import { t } from './i18n.js';

  let categories = [];
  let selected = new Set();
  let backups = [];
  let generalBackups = [];
  let mappings = null;
  let mappingArchives = [];
  let directory = '';
  let savedGames = '';
  let loading = true;
  let busy = false;
  let mappingBusy = false;
  let error = '';
  let notice = '';

  async function getJSON(url) {
    const r = await fetch(url, { cache: 'no-store' });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || url + ' -> ' + r.status);
    return r.json();
  }

  async function postJSON(url, body) {
    const r = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || url + ' -> ' + r.status);
    return r.json();
  }

  async function reload() {
    loading = true;
    error = '';
    try {
      const [cfg, list, mappingData] = await Promise.all([
        getJSON('/api/backup/categories'),
        getJSON('/api/backup'),
        getJSON('/api/backup/mappings'),
      ]);
      categories = cfg.categories ?? [];
      directory = cfg.directory ?? '';
      savedGames = cfg.savedGames ?? '';
      if (selected.size === 0) selected = new Set(cfg.default ?? []);
      backups = list.backups ?? [];
      generalBackups = backups.filter(
        (a) => !(a.categories?.length === 1 && a.categories[0] === 'input')
      );
      mappings = mappingData.mappings ?? null;
      mappingArchives = mappingData.archives ?? [];
    } catch (e) {
      error = String(e.message ?? e);
    } finally {
      loading = false;
    }
  }

  function toggle(id) {
    const next = new Set(selected);
    next.has(id) ? next.delete(id) : next.add(id);
    selected = next;
  }

  async function createBackup() {
    busy = true;
    error = '';
    notice = '';
    try {
      const res = await postJSON('/api/backup', { categories: [...selected] });
      notice = `Saved ${res.backup.entries} file(s) to ${res.backup.name}`;
      await reload();
    } catch (e) {
      error = String(e.message ?? e);
    } finally {
      busy = false;
    }
  }

  async function createMappings() {
    mappingBusy = true;
    error = '';
    notice = '';
    try {
      const res = await postJSON('/api/backup/mappings', {});
      notice = `Mappings sauvegardés dans ${res.backup.name}`;
      await reload();
    } catch (e) {
      error = String(e.message ?? e);
    } finally {
      mappingBusy = false;
    }
  }

  async function restoreMappings(a, dryRun) {
    mappingBusy = true;
    error = '';
    notice = '';
    try {
      const res = await postJSON('/api/backup/mappings/restore', { name: a.name, dryRun });
      const r = res.result;
      notice = dryRun
        ? `Simulation : ${r.files} fichier(s) seraient restaurés depuis ${a.name}`
        : `Mappings restaurés depuis ${a.name}${r.safetyBackup ? ' (copie de sécurité conservée)' : ''}`;
    } catch (e) {
      error = String(e.message ?? e);
    } finally {
      mappingBusy = false;
    }
  }


  async function restore(a, dryRun) {
    busy = true;
    error = '';
    notice = '';
    try {
      const res = await postJSON('/api/backup/restore', { name: a.name, dryRun });
      const r = res.result;
      notice = dryRun
        ? `Would restore ${r.files} file(s) from ${a.name}`
        : `Restored ${r.files} file(s) from ${a.name}${r.safetyBackup ? ' (safety copy kept)' : ''}`;
    } catch (e) {
      error = String(e.message ?? e);
    } finally {
      busy = false;
    }
  }

  function fmtDate(s) {
    return s ? new Date(s).toLocaleString() : '—';
  }

  function fmtSize(n) {
    if (!n) return '0 B';
    const u = ['B', 'KiB', 'MiB', 'GiB'];
    let i = 0;
    while (n >= 1024 && i < u.length - 1) {
      n /= 1024;
      i++;
    }
    return `${n.toFixed(i === 0 ? 0 : 1)} ${u[i]}`;
  }

  onMount(reload);
</script>

<section class="backup">
  <header>
    <h2>{$t('backup.title')}</h2>
    <button class="refresh" on:click={reload} disabled={busy || loading}>{$t('stats.refresh')}</button>
  </header>

  {#if error}<p class="error">{error}</p>{/if}
  {#if notice}<p class="notice">{notice}</p>{/if}

  {#if !savedGames && !loading}
    <p class="empty">{$t('backup.noSavedGames')}</p>
  {/if}

  <div class="block mappings">
    <h3>
      {$t('backup.mappings.title')}
      {#if mappings?.files}<span class="count">{mappings.files}</span>{/if}
    </h3>
    <p class="hint">{$t('backup.mappings.hint')}</p>
    {#if mappings?.files}
      <p class="mapping-summary">
        {$t('backup.mappings.summary', {
          aircraft: mappings.aircraft?.length ?? 0,
          files: mappings.files,
          size: fmtSize(mappings.bytes),
        })}
      </p>
      {#if mappings.aircraft?.length}
        <div class="aircraft-list">
          {#each mappings.aircraft.slice(0, 12) as aircraft (aircraft.name)}
            <span class="tag">{aircraft.name} <b>{aircraft.files}</b></span>
          {/each}
          {#if mappings.aircraft.length > 12}
            <span class="tag">+{mappings.aircraft.length - 12}</span>
          {/if}
        </div>
      {/if}
    {:else}
      <p class="empty">{$t('backup.mappings.noneFound')}</p>
    {/if}
    <button class="primary" on:click={createMappings} disabled={mappingBusy || !mappings?.files}>
      {$t(mappingBusy ? 'backup.mappings.saving' : 'backup.mappings.save')}
    </button>

    {#if mappingArchives.length}
      <h4>{$t('backup.mappings.archives')}</h4>
      <table>
        <thead>
          <tr>
            <th>{$t('backup.col.name')}</th>
            <th>{$t('backup.col.date')}</th>
            <th>{$t('backup.col.size')}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each mappingArchives as a (a.name)}
            <tr>
              <td class="name" title={a.name}>{a.name}</td>
              <td class="num">{fmtDate(a.createdAt)}</td>
              <td class="num">{fmtSize(a.bytes)}</td>
              <td class="actions">
                <a class="link" href={'/api/backup/download/' + encodeURIComponent(a.name)} download>{$t('backup.download')}</a>
                <button on:click={() => restoreMappings(a, true)} disabled={mappingBusy}>{$t('backup.dryRun')}</button>
                <button class="danger" on:click={() => restoreMappings(a, false)} disabled={mappingBusy}>{$t('backup.restore')}</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <div class="block">
    <h3>{$t('backup.create')}</h3>
    <p class="hint">{$t('backup.hint')}</p>
    <div class="cats">
      {#each categories as c (c.id)}
        <label class="cat" class:large={c.large}>
          <input type="checkbox" checked={selected.has(c.id)} on:change={() => toggle(c.id)} />
          <span class="label">{$t('backup.cat.' + c.id)}</span>
          {#if c.large}<span class="tag">{$t('backup.large')}</span>{/if}
        </label>
      {/each}
    </div>
    <button class="primary" on:click={createBackup} disabled={busy || selected.size === 0}>
      {$t('backup.run')}
    </button>
  </div>

  <div class="block">
    <h3>{$t('backup.archives')} {#if generalBackups.length}<span class="count">{generalBackups.length}</span>{/if}</h3>
    {#if loading}
      <p class="empty">{$t('debriefs.loading')}</p>
    {:else if generalBackups.length === 0}
      <p class="empty">{$t('backup.none')}</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>{$t('backup.col.name')}</th>
            <th>{$t('backup.col.date')}</th>
            <th>{$t('backup.col.size')}</th>
            <th>{$t('backup.col.categories')}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each generalBackups as a (a.name)}
            <tr>
              <td class="name" title={a.name}>{a.name}</td>
              <td class="num">{fmtDate(a.createdAt)}</td>
              <td class="num">{fmtSize(a.bytes)}</td>
              <td class="cats-cell">
                {#each a.categories ?? [] as c (c)}<span class="tag">{c}</span>{/each}
              </td>
              <td class="actions">
                <a class="link" href={'/api/backup/download/' + encodeURIComponent(a.name)} download>{$t('backup.download')}</a>
                <button on:click={() => restore(a, true)} disabled={busy}>{$t('backup.dryRun')}</button>
                <button class="danger" on:click={() => restore(a, false)} disabled={busy}>{$t('backup.restore')}</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
    {#if directory}<p class="hint">{$t('backup.dir')}: <code>{directory}</code></p>{/if}
  </div>
</section>

<style>
  .backup {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    padding: 0.9rem 1rem;
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    margin-bottom: 0.7rem;
  }

  h2 {
    margin: 0;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.82rem;
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
  }

  .refresh {
    padding: 0.28rem 0.6rem;
    font-size: 0.76rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .refresh:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--blue);
  }

  .block {
    margin-bottom: 1.1rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--border);
  }

  .hint {
    margin: 0.2rem 0 0.6rem;
    color: var(--muted);
    font-size: 0.76rem;
  }

  .mapping-summary {
    margin: 0.1rem 0 0.45rem;
    color: var(--text);
    font-size: 0.78rem;
  }

  .aircraft-list {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    margin-bottom: 0.6rem;
  }

  .aircraft-list .tag {
    color: var(--muted);
    font-size: 0.68rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.05rem 0.4rem;
  }

  .aircraft-list .tag b {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .mappings h4 {
    margin: 0.8rem 0 0.35rem;
    font-size: 0.76rem;
    color: var(--muted);
    font-weight: 600;
  }

  .count {
    color: var(--muted);
    font-weight: 400;
  }

  .cats {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-bottom: 0.7rem;
  }

  .cat {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0.6rem;
    font-size: 0.78rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    cursor: pointer;
  }

  .cat input {
    accent-color: var(--blue);
  }

  .cat .tag {
    color: var(--muted);
    font-size: 0.66rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0 0.35rem;
  }

  .primary {
    padding: 0.4rem 0.9rem;
    font-size: 0.8rem;
    color: var(--text);
    background: color-mix(in srgb, var(--blue) 30%, var(--bg));
    border: 1px solid var(--blue);
    border-radius: 7px;
    cursor: pointer;
  }

  .primary:disabled {
    opacity: 0.5;
    cursor: default;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
  }

  th {
    text-align: left;
    font-weight: 500;
    color: var(--muted);
    padding: 0.3rem 0.4rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.7rem;
  }

  td {
    padding: 0.32rem 0.4rem;
    border-top: 1px solid var(--border);
    vertical-align: middle;
  }

  td.num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .name {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cats-cell .tag {
    display: inline-block;
    margin: 0 0.2rem 0.15rem 0;
    padding: 0.02rem 0.35rem;
    font-size: 0.66rem;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 999px;
  }

  .actions {
    text-align: right;
    white-space: nowrap;
  }

  .actions button,
  .actions .link {
    margin-left: 0.3rem;
    padding: 0.2rem 0.5rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
    text-decoration: none;
  }

  .actions button:hover,
  .actions .link:hover {
    color: var(--text);
    border-color: var(--blue);
  }

  .actions .danger:hover {
    color: #ff8a8a;
    border-color: #a5342f;
  }

  code {
    background: var(--bg);
    padding: 0.05rem 0.3rem;
    border-radius: 4px;
  }

  .empty {
    color: var(--muted);
    font-size: 0.8rem;
  }

  .error {
    color: #f0b429;
    font-size: 0.78rem;
  }

  .notice {
    color: var(--green);
    font-size: 0.78rem;
  }
</style>
