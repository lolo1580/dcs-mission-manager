<script>
  /**
   * DCS install: the mods under Saved Games\DCS\Mods, and the state of the
   * scripts — the manager's own files, the tools sharing Export.lua, the
   * pre-rename leftovers to clean up, and other tools' hooks.
   */
  import { onMount } from 'svelte';
  import {
    mods,
    scriptStatus,
    installError,
    installLoading,
    modsTotalBytes,
    installNeedsAttention,
    loadInstall,
    fmtSize,
  } from './install.js';
  import { t } from './i18n.js';

  onMount(loadInstall);

  /** Label for a managed file's state. */
  const STATE_KEYS = {
    installed: 'install.state.installed',
    outdated: 'install.state.outdated',
    missing: 'install.state.missing',
    unknown: 'install.state.unknown',
    merged: 'install.state.merged',
    created: 'install.state.created',
    unchanged: 'install.state.unchanged',
  };

  function stateLabel(state) {
    return $t(STATE_KEYS[state] ?? state);
  }
</script>

<section class="install">
  <header>
    <h2>{$t('install.title')}</h2>
    <button class="refresh" on:click={loadInstall} disabled={$installLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  {#if $installError}
    <p class="error">{$installError}</p>
  {/if}

  {#if $scriptStatus}
    {#if $installNeedsAttention}
      <p class="attention">⚠ {$t('install.attention')}</p>
    {/if}

    <div class="block">
      <h3>{$t('install.scripts')}</h3>
      <table class="managed">
        <tbody>
          {#each $scriptStatus.managed ?? [] as m (m.destRel)}
            <tr>
              <td class="path">{m.destRel}</td>
              <td class="state">
                <span class="badge {m.state}">{stateLabel(m.state)}</span>
                {#if m.note}<span class="note">{m.note}</span>{/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    {#if ($scriptStatus.exports ?? []).length || ($scriptStatus.legacy ?? []).length}
      <div class="block">
        <h3>{$t('install.export')}</h3>
        {#if ($scriptStatus.exports ?? []).length}
          <p class="line">
            {$t('install.sharedWith')}
            {#each $scriptStatus.exports as e, i (e)}
              {#if i}<span class="sep">·</span>{/if}<span class="tool">{e}</span>
            {/each}
          </p>
        {:else}
          <p class="line muted">{$t('install.notShared')}</p>
        {/if}
        {#if ($scriptStatus.legacy ?? []).length}
          <p class="line warn">
            {$t('install.legacy')}
            {#each $scriptStatus.legacy as l (l)}<code>{l}</code>{/each}
          </p>
        {/if}
      </div>
    {/if}

    {#if ($scriptStatus.thirdParty ?? []).length}
      <div class="block">
        <h3>
          {$t('install.thirdParty')}
          <span class="count">{$scriptStatus.thirdParty.length}</span>
        </h3>
        <table>
          <thead>
            <tr><th>{$t('install.col.file')}</th><th>{$t('install.col.tool')}</th><th>{$t('install.col.dir')}</th><th>{$t('install.col.size')}</th></tr>
          </thead>
          <tbody>
            {#each $scriptStatus.thirdParty as h (h.dir + '/' + h.name)}
              <tr>
                <td class="path">{h.name}{#if h.backup}<span class="tag">{$t('install.backup')}</span>{/if}</td>
                <td>{h.tool || '—'}</td>
                <td class="muted">{h.dir}</td>
                <td class="num">{fmtSize(h.sizeBytes)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <div class="block">
    <h3>
      {$t('install.mods')}
      <span class="count">{$mods.length} · {fmtSize($modsTotalBytes)}</span>
    </h3>
    {#if $mods.length === 0}
      <p class="empty">{$t('install.noMods')}</p>
    {:else}
      <table>
        <thead>
          <tr><th>{$t('install.col.mod')}</th><th>{$t('install.col.category')}</th><th>{$t('install.col.files')}</th><th>{$t('install.col.size')}</th></tr>
        </thead>
        <tbody>
          {#each $mods as m (m.category + '/' + m.name)}
            <tr>
              <td class="path">
                {m.name}
                {#if !m.hasEntryLua}<span class="tag warn" title={$t('install.noEntry')}>⚠</span>{/if}
              </td>
              <td class="muted">{m.category}</td>
              <td class="num">{m.files.toLocaleString()}</td>
              <td class="num">{fmtSize(m.sizeBytes)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</section>

<style>
  .install {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
  }

  h2 {
    margin: 0;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  .refresh {
    padding: 0.25rem 0.55rem;
    font-size: 0.74rem;
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

  .attention {
    margin: 0 0 0.8rem;
    padding: 0.4rem 0.6rem;
    font-size: 0.78rem;
    color: #f0b429;
    border: 1px solid color-mix(in srgb, #f0b429 40%, var(--border));
    border-radius: 6px;
    background: color-mix(in srgb, #f0b429 10%, var(--bg));
  }

  .block {
    margin-bottom: 1.1rem;
  }

  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
  }

  .count {
    color: var(--muted);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
  }

  table.managed {
    max-width: 640px;
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
    padding: 0.28rem 0.4rem;
    border-top: 1px solid var(--border);
    vertical-align: baseline;
  }

  .path {
    color: var(--text);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.74rem;
  }

  td.num {
    text-align: right;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .muted {
    color: var(--muted);
  }

  .state {
    text-align: right;
    white-space: nowrap;
  }

  .badge {
    padding: 0.05rem 0.45rem;
    border-radius: 999px;
    font-size: 0.68rem;
    border: 1px solid var(--border);
    color: var(--muted);
  }

  .badge.installed {
    color: var(--green);
    border-color: color-mix(in srgb, var(--green) 45%, var(--border));
  }

  .badge.outdated {
    color: #f0b429;
    border-color: color-mix(in srgb, #f0b429 45%, var(--border));
  }

  .badge.missing {
    color: var(--muted);
  }

  .note {
    margin-left: 0.4rem;
    font-size: 0.7rem;
    color: var(--muted);
  }

  .line {
    margin: 0.2rem 0;
    font-size: 0.78rem;
    color: var(--text);
  }

  .line.muted {
    color: var(--muted);
  }

  .line.warn {
    color: #f0b429;
  }

  .sep {
    margin: 0 0.35rem;
    color: var(--muted);
  }

  .tool {
    color: var(--blue);
  }

  code {
    margin-left: 0.4rem;
    padding: 0.05rem 0.35rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: 0.72rem;
  }

  .tag {
    margin-left: 0.4rem;
    padding: 0 0.3rem;
    font-size: 0.62rem;
    text-transform: uppercase;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 999px;
  }

  .tag.warn {
    color: #f0b429;
    border-color: color-mix(in srgb, #f0b429 45%, var(--border));
  }

  .empty {
    color: var(--muted);
    font-size: 0.8rem;
    line-height: 1.5;
  }

  .error {
    color: #f0b429;
    font-size: 0.78rem;
  }
</style>
