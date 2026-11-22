<script>
  import { t } from './i18n.js';
  import { showToast } from './toast.js';
  import {
    contentBusy,
    contentMods,
    dcsRunning,
    modAction,
  } from './content.js';
  import { fmtSize } from './install.js';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';

  let query = '';
  let category = '';
  let state = '';

  $: categories = [...new Set($contentMods.map((mod) => mod.category).filter(Boolean))].sort();
  $: visible = $contentMods.filter((mod) => {
    const text = `${mod.name} ${mod.category} ${mod.version ?? ''}`.toLowerCase();
    if (query && !text.includes(query.trim().toLowerCase())) return false;
    if (category && mod.category !== category) return false;
    if (state === 'enabled' && !mod.enabled) return false;
    if (state === 'disabled' && mod.enabled) return false;
    return true;
  });

  async function toggle(mod) {
    const action = mod.enabled ? 'disable' : 'enable';
    if (mod.enabled && !confirm($t('content.mods.confirmDisable', { name: mod.name }))) return;
    try {
      await modAction(action, { category: mod.category, name: mod.name });
      showToast($t(mod.enabled ? 'content.mods.disabled' : 'content.mods.enabled', { name: mod.name }));
    } catch (error) {
      showToast(error.message, 'error');
    }
  }

  async function quarantine(mod) {
    if (!confirm($t('content.mods.confirmRemove', { name: mod.name }))) return;
    try {
      await modAction('quarantine', { category: mod.category, name: mod.name });
      showToast($t('content.mods.removed', { name: mod.name }));
    } catch (error) {
      showToast(error.message, 'error');
    }
  }

  async function reveal(mod) {
    try {
      await modAction('reveal', { category: mod.category, name: mod.name });
    } catch (error) {
      showToast(error.message, 'error');
    }
  }

  function warningLabel(code) {
    return $t(`content.warn.${code}`);
  }
</script>

<div class="mods">
  <div class="toolbar">
    <input class="search" type="search" bind:value={query} placeholder={$t('content.mods.search')} aria-label={$t('content.mods.search')} />
    <select bind:value={category} aria-label={$t('content.mods.category')}>
      <option value="">{$t('content.mods.allCategories')}</option>
      {#each categories as value (value)}
        <option value={value}>{value}</option>
      {/each}
    </select>
    <select bind:value={state} aria-label={$t('content.mods.state')}>
      <option value="">{$t('content.mods.allStates')}</option>
      <option value="enabled">{$t('content.mods.enabled')}</option>
      <option value="disabled">{$t('content.mods.disabledState')}</option>
    </select>
    <span class="spacer"></span>
    <span class="muted">{$t('content.mods.count', { count: visible.length })}</span>
  </div>

  {#if visible.length === 0}
    <EmptyState title={$t('content.mods.empty')} message={$t('content.mods.emptyHint')} />
  {:else}
    <div class="mod-list">
      {#each visible as mod (mod.category + '/' + mod.name)}
        <article class="mod-row" class:disabled={!mod.enabled}>
          <div class="mod-main">
            <div class="mod-title">
              <strong>{mod.name}</strong>
              <Badge tone={mod.enabled ? 'green' : 'neutral'}>
                {mod.enabled ? $t('content.mods.enabled') : $t('content.mods.disabledState')}
              </Badge>
              {#if mod.version}<span class="version">v{mod.version}</span>{/if}
            </div>
            <div class="meta">
              <span>{mod.category}</span>
              <span>·</span>
              <span>{fmtSize(mod.sizeBytes)}</span>
              <span>·</span>
              <span>{$t('content.mods.files', { count: mod.files })}</span>
              {#if mod.author}<span>·</span><span>{mod.author}</span>{/if}
            </div>
            {#if mod.warnings?.length}
              <div class="warnings">
                {#each mod.warnings as warning (warning)}
                  <span class="warn">⚠ {warningLabel(warning)}</span>
                {/each}
              </div>
            {/if}
          </div>
          <div class="mod-actions">
            <button class="btn" type="button" on:click={() => reveal(mod)} title={$t('content.mods.reveal')}>📂</button>
            <button class="btn" type="button" on:click={() => toggle(mod)} disabled={$contentBusy || $dcsRunning}>
              {mod.enabled ? $t('content.mods.disable') : $t('content.mods.enable')}
            </button>
            <button class="btn danger" type="button" on:click={() => quarantine(mod)} disabled={$contentBusy || $dcsRunning}>
              {$t('content.mods.remove')}
            </button>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</div>

<style>
  .mods {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.45rem;
  }

  .search {
    flex: 1;
    min-width: 180px;
    max-width: 340px;
    padding: 0.4rem 0.55rem;
    color: var(--text);
    background: var(--bg-elevated);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
  }

  select {
    padding: 0.38rem 0.45rem;
    color: var(--text);
    background: var(--bg-elevated);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .spacer {
    flex: 1;
  }

  .mod-list {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .mod-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.8rem;
    padding: 0.65rem 0.75rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-soft);
  }

  .mod-row.disabled {
    opacity: 0.72;
  }

  .mod-main {
    display: flex;
    flex-direction: column;
    gap: 0.16rem;
    min-width: 0;
  }

  .mod-title {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .mod-title strong {
    font-size: 0.88rem;
  }

  .version {
    color: var(--faint);
    font-size: 0.7rem;
  }

  .meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.25rem;
    color: var(--muted);
    font-size: 0.72rem;
  }

  .warnings {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    margin-top: 0.1rem;
  }

  .warn {
    color: var(--amber);
    font-size: 0.7rem;
  }

  .mod-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.35rem;
    flex: none;
  }

  .muted {
    color: var(--muted);
    font-size: 0.74rem;
  }

  @media (max-width: 760px) {
    .mod-row {
      align-items: flex-start;
      flex-direction: column;
    }
  }
</style>
