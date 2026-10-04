<script>
  /**
   * Modules: what DCS itself reports (MissionEditor/modules.lua) — terrains,
   * aircraft, campaigns and tech packs — with two distinct facts per row:
   * "owned" (bought, the store's have="1") and "installed" (actually present in
   * the installation, from autoupdate.cfg). They differ when a purchased module
   * has been uninstalled.
   */
  import { onMount } from 'svelte';
  import {
    modules,
    moduleTotals,
    modulesError,
    modulesLoading,
    moduleSearch,
    ownedOnly,
    installedOnly,
    moduleCategory,
    moduleCategories,
    visibleModules,
    loadModules,
  } from './modules.js';
  import { t } from './i18n.js';

  onMount(loadModules);

  const CATEGORY_KEYS = {
    terrains: 'modules.cat.terrains',
    moduls: 'modules.cat.moduls',
    campaigns: 'modules.cat.campaigns',
    bundles: 'modules.cat.bundles',
  };

  function categoryLabel(id) {
    return $t(CATEGORY_KEYS[id] ?? id);
  }

  /** DCS type -> a label key, falling back to the raw type. */
  const TYPE_KEYS = {
    Terrain: 'modules.type.terrain',
    Campaign: 'modules.type.campaign',
    'Jet engine plane': 'modules.type.jet',
    'Piston engine plane': 'modules.type.piston',
    Helicopter: 'modules.type.helicopter',
    'Navigation system': 'modules.type.nav',
    'Tech pack': 'modules.type.tech',
    Bundle: 'modules.type.bundle',
  };

  function typeLabel(type) {
    return TYPE_KEYS[type] ? $t(TYPE_KEYS[type]) : type || '—';
  }

  /** Badge state for a module: installed / owned-not-installed / not owned. */
  function installState(m) {
    if (m.installKnown && m.installed) return 'installed';
    if (!m.installKnown) return 'unknown';
    return m.owned ? 'owned' : 'notOwned';
  }

  /**
   * The version column is only useful when something is actually installed with
   * a version; showing a column of "—" for a list of campaigns is noise.
   */
  $: hasVersions = $visibleModules.some(
    (m) => m.installKnown && m.installed && m.versions?.length
  );

  function version(m) {
    if (!m.installed || !m.versions?.length) return '—';
    return m.versions[m.versions.length - 1];
  }

  const STATE_KEYS = {
    installed: 'modules.state.installed',
    owned: 'modules.state.owned',
    notOwned: 'modules.state.notOwned',
    unknown: 'modules.state.unknown',
  };
</script>

<section class="modules">
  <header>
    <h2>
      {$t('modules.title')}
      <span class="count">
        {$visibleModules.length} / {$moduleTotals.total}
        · {$moduleTotals.owned} {$t('modules.owned')}
        · {$moduleTotals.installed} {$t('modules.installedCount')}
      </span>
    </h2>
    <button class="refresh" on:click={loadModules} disabled={$modulesLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  {#if $modulesError}
    <p class="error">{$modulesError}</p>
  {/if}

  <div class="controls">
    <input
      type="search"
      placeholder={$t('modules.search')}
      value={$moduleSearch}
      on:input={(e) => moduleSearch.set(e.currentTarget.value)}
    />
    <label class="toggle" title={$t('modules.ownedHint')}>
      <input
        type="checkbox"
        checked={$ownedOnly}
        on:change={(e) => ownedOnly.set(e.currentTarget.checked)}
      />
      {$t('modules.ownedOnly')}
    </label>
    <label class="toggle" title={$t('modules.installedHint')}>
      <input
        type="checkbox"
        checked={$installedOnly}
        on:change={(e) => installedOnly.set(e.currentTarget.checked)}
      />
      {$t('modules.installedOnly')}
    </label>
    <div class="cats">
      <button class:active={$moduleCategory === ''} on:click={() => moduleCategory.set('')}>
        {$t('modules.cat.all')}
      </button>
      {#each $moduleCategories as c (c)}
        <button class:active={$moduleCategory === c} on:click={() => moduleCategory.set(c)}>
          {categoryLabel(c)}
        </button>
      {/each}
    </div>
  </div>

  {#if $modules.length === 0 && !$modulesError}
    <p class="empty">{$t('modules.none')}</p>
  {:else if $visibleModules.length === 0}
    <p class="empty">{$t('modules.noMatch')}</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>{$t('modules.col.module')}</th>
          <th>{$t('modules.col.type')}</th>
          <th>{$t('modules.col.developer')}</th>
          <th class="badge-col">{$t('modules.col.owned')}</th>
          <th class="badge-col">{$t('modules.col.installed')}</th>
          {#if hasVersions}<th class="ver-col">{$t('modules.col.version')}</th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each $visibleModules as m (m.category + '/' + (m.id || m.title))}
          {@const state = installState(m)}
          <tr class:installed={state === 'installed'}>
            <td class="name">
              <span class="dot" class:on={state === 'installed'}></span>
              <span class="label" title={m.title}>{m.title || m.id}</span>
              {#if m.id && m.id !== m.title}<span class="id">{m.id}</span>{/if}
            </td>
            <td>{typeLabel(m.type)}</td>
            <td class="dev">{m.developer || '—'}</td>
            <td class="ver">
              <span class="badge" class:yes={m.owned} class:no={!m.owned}
                title={m.owned ? $t('modules.state.owned') : $t('modules.state.notOwned')}>
                {m.owned ? $t('modules.yes') : $t('modules.no')}
              </span>
            </td>
            <td class="ver">
              {#if m.installKnown}
                <span class="badge" class:yes={m.installed} class:no={!m.installed}
                  title={m.installed ? $t('modules.state.installed') : $t('modules.state.owned')}>
                  {m.installed ? $t('modules.yes') : $t('modules.no')}
                </span>
              {:else}
                <span class="badge unknown" title={$t('modules.state.unknown')}>{$t('modules.col.na')}</span>
              {/if}
            </td>
            {#if hasVersions}
              <td class="ver-col ver">{version(m)}</td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .modules {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    overflow: hidden;
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
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
  }

  .count {
    color: var(--text);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
    text-transform: none;
    letter-spacing: 0;
    font-size: 0.76rem;
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

  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.8rem;
    margin-bottom: 0.6rem;
  }

  .controls input[type='search'] {
    width: 260px;
    padding: 0.35rem 0.5rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.8rem;
  }

  .controls input:focus {
    outline: none;
    border-color: var(--blue);
  }

  .toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.76rem;
    color: var(--muted);
    cursor: pointer;
    white-space: nowrap;
  }

  .cats {
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
  }

  .cats button {
    padding: 0.22rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    cursor: pointer;
  }

  .cats button.active {
    color: var(--text);
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
    overflow-y: auto;
  }

  th {
    position: sticky;
    top: 0;
    text-align: left;
    font-weight: 500;
    color: var(--muted);
    padding: 0.3rem 0.4rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    font-size: 0.7rem;
  }

  /* The badge columns must line up with their right-aligned values. */
  th.badge-col,
  th.ver-col {
    text-align: right;
  }

  .ver-col {
    width: 7rem;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  td {
    padding: 0.28rem 0.4rem;
    border-top: 1px solid var(--border);
    vertical-align: baseline;
  }

  tr.installed .name .label {
    color: var(--text);
  }

  td.name {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    min-width: 0;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--border);
    flex: none;
  }

  .dot.on {
    background: var(--green);
  }

  .label {
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .id {
    color: var(--muted);
    font-size: 0.7rem;
    opacity: 0.7;
    flex: none;
  }

  .dev {
    color: var(--muted);
  }

  td.ver {
    text-align: right;
    white-space: nowrap;
  }

  .badge {
    padding: 0.05rem 0.4rem;
    border-radius: 999px;
    font-size: 0.68rem;
    border: 1px solid var(--border);
  }

  .badge.yes {
    color: var(--green);
    border-color: color-mix(in srgb, var(--green) 45%, var(--border));
  }

  .badge.no {
    color: var(--muted);
  }

  .badge.unknown {
    color: var(--muted);
    opacity: 0.6;
    border-style: dashed;
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
