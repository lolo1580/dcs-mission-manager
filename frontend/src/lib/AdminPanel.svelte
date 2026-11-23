<script>
  import { onMount } from 'svelte';
  import { sessionUser, refreshSession } from './session.js';
  import { t } from './i18n.js';
  import { showToast } from './toast.js';
  import Card from './ui/Card.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import SchoolAdmin from './SchoolAdmin.svelte';

  let users = [];
  let audit = [];
  let conflicts = [];
  let previews = {};
  let mergeChoices = {};
  let loading = true;
  let error = '';
  let saving = false;

  let draft = { username: '', displayName: '', email: '', password: '', role: 'member', dcsNames: '' };
  let editing = {};
  let passwords = {};

  async function getJSON(path) {
    const response = await fetch(path);
    if (!response.ok) throw new Error(`${response.status}`);
    return response.json();
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [userBody, auditBody] = await Promise.all([
        getJSON('/api/admin/users'),
        getJSON('/api/admin/audit?limit=80'),
      ]);
      users = userBody.users ?? [];
      audit = auditBody.entries ?? [];
      const conflictBody = await getJSON('/api/sync/conflicts').catch(() => ({ conflicts: [] }));
      conflicts = conflictBody.conflicts ?? [];
      for (const user of users) {
        editing[user.id] = {
          displayName: user.displayName ?? '',
          email: user.email ?? '',
          role: user.role,
          active: user.active,
          dcsNames: (user.dcsNames ?? []).join(', '),
        };
        passwords[user.id] = '';
      }
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  function names(text) {
    return text.split(/[\n,]/).map((value) => value.trim()).filter(Boolean);
  }

  async function createUser(event) {
    event.preventDefault();
    saving = true;
    error = '';
    try {
      const response = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: draft.username.trim(),
          displayName: draft.displayName.trim(),
          email: draft.email.trim(),
          password: draft.password,
          role: draft.role,
          dcsNames: names(draft.dcsNames),
        }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      draft = { username: '', displayName: '', email: '', password: '', role: 'member', dcsNames: '' };
      showToast($t('admin.created'), 'success');
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }

  async function saveUser(user) {
    const edit = editing[user.id];
    saving = true;
    error = '';
    try {
      const response = await fetch(`/api/admin/users/${user.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          displayName: edit.displayName,
          email: edit.email,
          role: edit.role,
          active: edit.active,
          dcsNames: names(edit.dcsNames),
          password: passwords[user.id] || '',
        }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      passwords[user.id] = '';
      showToast($t('admin.updated'), 'success');
      await load();
      await refreshSession();
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }

  async function deleteUser(user) {
    if (!confirm($t('admin.confirmDelete', { name: user.username }))) return;
    try {
      const response = await fetch(`/api/admin/users/${user.id}`, { method: 'DELETE' });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body?.error ?? `${response.status}`);
      }
      showToast($t('admin.deleted'), 'success');
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  async function dismissConflict(conflict) {
    try {
      const response = await fetch(`/api/sync/conflicts/${conflict.id}`, { method: 'DELETE' });
      if (!response.ok) throw new Error(`${response.status}`);
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  async function restoreConflict(conflict) {
    try {
      const response = await fetch(`/api/sync/conflicts/${conflict.id}/restore`, { method: 'POST' });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      showToast($t('admin.restored'), 'success');
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  async function loadPreview(conflict) {
    if (previews[conflict.id]) return;
    try {
      const response = await fetch(`/api/sync/conflicts/${conflict.id}/preview`);
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      previews = { ...previews, [conflict.id]: body };
      const choices = {};
      for (const field of body.fields ?? []) choices[field] = 'winner';
      mergeChoices = { ...mergeChoices, [conflict.id]: choices };
    } catch (err) {
      error = err.message;
    }
  }

  function display(value) {
    if (value === null || value === undefined) return '—';
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  }

  async function mergeConflict(conflict) {
    try {
      const response = await fetch(`/api/sync/conflicts/${conflict.id}/merge`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ choices: mergeChoices[conflict.id] ?? {} }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body?.error ?? `${response.status}`);
      showToast($t('admin.merged'), 'success');
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  function dateOf(ms) {
    return ms ? new Date(ms).toLocaleString() : $t('admin.never');
  }
</script>

<div class="admin">
  {#if error}<div class="error">{$t('admin.error', { detail: error })}</div>{/if}

  <Card title={$t('admin.new')}>
    <form class="create" on:submit={createUser}>
      <label class="field">
        <span>{$t('admin.username')}</span>
        <input bind:value={draft.username} required minlength="3" maxlength="32" autocapitalize="none" />
      </label>
      <label class="field">
        <span>{$t('admin.displayName')}</span>
        <input bind:value={draft.displayName} maxlength="80" />
      </label>
      <label class="field">
        <span>{$t('admin.email')}</span>
        <input type="email" bind:value={draft.email} maxlength="160" autocomplete="email" />
      </label>
      <label class="field">
        <span>{$t('admin.password')}</span>
        <input type="password" bind:value={draft.password} required minlength="10" autocomplete="new-password" />
      </label>
      <label class="field">
        <span>{$t('admin.role')}</span>
        <select bind:value={draft.role}>
          <option value="member">{$t('admin.role.member')}</option>
          <option value="admin">{$t('admin.role.admin')}</option>
        </select>
      </label>
      <label class="field wide">
        <span>{$t('admin.dcsNames')}</span>
        <input bind:value={draft.dcsNames} placeholder="Viper, Hornet" />
      </label>
      <div class="create-action">
        <button class="btn primary" type="submit" disabled={saving}>{$t('admin.create')}</button>
      </div>
    </form>
  </Card>

  <Card title={$t('admin.title')}>
    {#if loading}
      <p class="hint">…</p>
    {:else if users.length === 0}
      <EmptyState title={$t('admin.noUsers')} message="" />
    {:else}
      <div class="user-list">
        {#each users as user (user.id)}
          <article class="user" class:inactive={!editing[user.id].active}>
            <header>
              <div>
                <strong>{user.username}</strong>
                <span>{dateOf(user.lastLoginAt)}</span>
              </div>
              <Badge tone={editing[user.id].role === 'admin' ? 'blue' : 'neutral'}>
                {editing[user.id].role === 'admin' ? $t('admin.role.admin') : $t('admin.role.member')}
              </Badge>
              {#if !editing[user.id].active}<Badge tone="red">{$t('admin.active')}</Badge>{/if}
            </header>
            <div class="user-fields">
              <label class="field">
                <span>{$t('admin.displayName')}</span>
                <input bind:value={editing[user.id].displayName} maxlength="80" />
              </label>
              <label class="field">
                <span>{$t('admin.email')}</span>
                <input type="email" bind:value={editing[user.id].email} maxlength="160" autocomplete="email" />
              </label>
              <label class="field">
                <span>{$t('admin.role')}</span>
                <select bind:value={editing[user.id].role}>
                  <option value="member">{$t('admin.role.member')}</option>
                  <option value="admin">{$t('admin.role.admin')}</option>
                </select>
              </label>
              <label class="field">
                <span>{$t('admin.resetPassword')}</span>
                <input type="password" bind:value={passwords[user.id]} autocomplete="new-password" />
              </label>
              <label class="field wide">
                <span>{$t('admin.dcsNames')}</span>
                <input bind:value={editing[user.id].dcsNames} />
              </label>
            </div>
            <div class="user-actions">
              <label class="toggle">
                <input type="checkbox" bind:checked={editing[user.id].active} />
                {$t('admin.active')}
              </label>
              <button class="btn primary" on:click={() => saveUser(user)} disabled={saving}>{$t('admin.save')}</button>
              <button class="btn danger" on:click={() => deleteUser(user)} disabled={user.id === $sessionUser?.id}>
                {$t('admin.delete')}
              </button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </Card>

  <Card title={$t('admin.backup')} subtitle={$t('admin.backupHint')}>
    <a class="btn primary" href="/api/admin/backup" download>{$t('admin.downloadBackup')}</a>
  </Card>

  <Card title={$t('admin.audit')}>
    {#if audit.length === 0}
      <p class="hint">—</p>
    {:else}
      <ul class="audit">
        {#each audit as entry (entry.id)}
          <li>
            <span class="date">{dateOf(entry.realTs)}</span>
            <span class="action">{entry.action}</span>
            <span class="who">{entry.username || '—'}</span>
            <span class="detail">{entry.detail}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </Card>

  <Card title={$t('admin.conflicts')} subtitle={$t('admin.conflictOlder')}>
    {#if conflicts.length === 0}
      <p class="hint">{$t('admin.noConflicts')}</p>
    {:else}
      <ul class="conflicts">
        {#each conflicts as conflict (conflict.id)}
          <li>
            <div class="conflict-main">
              <strong>{conflict.kind} · {conflict.uid}</strong>
              <span>{dateOf(conflict.createdAt)} · {conflict.reason || '—'} · #{conflict.revision || '?'}</span>
            </div>
            <details>
              <summary>JSON</summary>
              <pre>{conflict.payload}</pre>
            </details>
            <details class="merge" on:toggle={() => loadPreview(conflict)}>
              <summary>{$t('admin.compare')}</summary>
              {#if previews[conflict.id]}
                <table class="compare">
                  <thead>
                    <tr><th>{$t('admin.field')}</th><th>{$t('admin.winner')}</th><th>{$t('admin.loser')}</th></tr>
                  </thead>
                  <tbody>
                    {#each previews[conflict.id].fields as field (field)}
                      <tr>
                        <td>{field}</td>
                        <td>
                          <label>
                            <input type="radio" value="winner" bind:group={mergeChoices[conflict.id][field]} />
                            <span>{display(previews[conflict.id].winner[field])}</span>
                          </label>
                        </td>
                        <td>
                          <label>
                            <input type="radio" value="loser" bind:group={mergeChoices[conflict.id][field]} />
                            <span>{display(previews[conflict.id].loser[field])}</span>
                          </label>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
                <button class="btn primary" on:click={() => mergeConflict(conflict)}>{$t('admin.merge')}</button>
              {:else}
                <p class="hint">…</p>
              {/if}
            </details>
            <button class="btn" on:click={() => restoreConflict(conflict)}>{$t('admin.restore')}</button>
            <button class="btn danger" on:click={() => dismissConflict(conflict)}>{$t('admin.conflictDismiss')}</button>
          </li>
        {/each}
      </ul>
    {/if}
  </Card>

  <SchoolAdmin />
</div>

<style>
  .admin {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    width: 100%;
    max-width: var(--content-max);
    margin: 0 auto;
    padding: 1rem;
  }

  .error {
    padding: 0.5rem 0.65rem;
    color: var(--red);
    background: var(--red-soft);
    border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
    border-radius: var(--radius);
    font-size: 0.8rem;
  }

  .create {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr 0.7fr;
    gap: 0.5rem;
    align-items: end;
  }

  .create .wide {
    grid-column: span 2;
  }

  .create-action {
    display: flex;
    justify-content: flex-end;
  }

  .user-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .user {
    padding: 0.6rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .user.inactive {
    opacity: 0.65;
  }

  .user header {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    margin-bottom: 0.45rem;
  }

  .user header div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    margin-right: auto;
  }

  .user header strong {
    font-size: 0.86rem;
  }

  .user header span {
    color: var(--muted);
    font-size: 0.72rem;
  }

  .user-fields {
    display: grid;
    grid-template-columns: 1fr 0.8fr 1fr;
    gap: 0.4rem;
  }

  .user-fields .wide {
    grid-column: span 3;
  }

  .user-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.4rem;
    margin-top: 0.45rem;
  }

  .toggle {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    margin-right: auto;
    color: var(--muted);
    font-size: 0.76rem;
  }

  .toggle input {
    accent-color: var(--green);
  }

  .audit {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .audit li {
    display: grid;
    grid-template-columns: 120px 130px 100px 1fr;
    gap: 0.5rem;
    padding: 0.3rem 0.4rem;
    background: var(--bg-elevated);
    border-radius: var(--radius-sm);
    font-size: 0.73rem;
  }

  .audit .date,
  .audit .who {
    color: var(--muted);
  }

  .audit .action {
    color: var(--blue);
    font-weight: 600;
  }

  .audit .detail {
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 0.78rem;
  }

  .conflicts {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .conflicts li {
    display: flex;
    align-items: flex-start;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding: 0.45rem 0.55rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    font-size: 0.75rem;
  }

  .conflict-main {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
    flex: 1;
  }

  .conflict-main strong {
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  .conflict-main span {
    color: var(--muted);
    font-size: 0.7rem;
  }

  .conflicts details {
    max-width: 45%;
  }

  .conflicts summary {
    color: var(--blue);
    cursor: pointer;
    font-size: 0.7rem;
  }

  .conflicts pre {
    max-height: 160px;
    overflow: auto;
    margin: 0.3rem 0 0;
    padding: 0.4rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.65rem;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .merge {
    flex-basis: 100%;
    max-width: 100% !important;
  }

  .compare {
    width: 100%;
    margin: 0.35rem 0;
    border-collapse: collapse;
    font-size: 0.7rem;
  }

  .compare th,
  .compare td {
    padding: 0.25rem 0.35rem;
    border-bottom: 1px solid var(--border);
    vertical-align: top;
    text-align: left;
  }

  .compare th {
    color: var(--muted);
    font-weight: 600;
  }

  .compare td:first-child {
    color: var(--muted);
    white-space: nowrap;
  }

  .compare label {
    display: flex;
    align-items: flex-start;
    gap: 0.25rem;
    cursor: pointer;
  }

  .compare span {
    overflow-wrap: anywhere;
  }

  .compare input {
    margin-top: 0.15rem;
    accent-color: var(--blue);
  }

  @media (max-width: 820px) {
    .create,
    .user-fields {
      grid-template-columns: 1fr;
    }

    .create .wide,
    .user-fields .wide {
      grid-column: auto;
    }

    .audit li {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>
