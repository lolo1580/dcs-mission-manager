<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { showToast } from './toast.js';
  import {
    contentError,
    contentLoading,
    contentMods,
    contentModsScan,
    contentLiveries,
    contentLiveryScan,
    contentQuarantine,
    dcsRunning,
    loadContent,
    liveryAction,
    modAction,
  } from './content.js';
  import { fmtSize } from './install.js';
  import ModsTab from './ModsTab.svelte';
  import LiveriesTab from './LiveriesTab.svelte';
  import ContentImportDialog from './ContentImportDialog.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Skeleton from './ui/Skeleton.svelte';
  import Badge from './ui/Badge.svelte';

  let tab = 'mods';
  let importOpen = false;

  onMount(loadContent);

  $: activeMods = $contentModsScan?.active ?? $contentMods.filter((m) => m.enabled).length;
  $: disabledMods = $contentModsScan?.disabled ?? $contentMods.filter((m) => !m.enabled).length;
  $: warnings =
    $contentMods.reduce((sum, mod) => sum + (mod.warnings?.length ?? 0), 0) +
    ($contentLiveryScan?.liveries ?? $contentLiveries).reduce(
      (sum, livery) => sum + (livery.warnings?.length ?? 0),
      0
    );
  $: totalLiveryBytes = $contentLiveryScan?.totalBytes ?? 0;

  async function restore(item) {
    try {
      if (item.kind === 'mod') {
        await modAction('restore', { quarantineId: item.id });
      } else {
        await liveryAction('restore', { quarantineId: item.id });
      }
      showToast($t('content.trash.restored', { name: item.name }));
    } catch (error) {
      showToast(error.message, 'error');
    }
  }
</script>

<section class="content-page">
  <header class="page-head">
    <div class="heading">
      <h2>{$t('content.title')}</h2>
      <p class="muted">{$t('content.subtitle')}</p>
    </div>
    <div class="head-actions">
      <button class="btn" type="button" on:click={loadContent} disabled={$contentLoading}>↻ {$t('content.refresh')}</button>
      <button class="btn primary" type="button" on:click={() => (importOpen = true)} disabled={$dcsRunning}>+ {$t('content.import')}</button>
    </div>
  </header>

  {#if $contentError}
    <p class="error">{$contentError}</p>
  {/if}
  {#if $dcsRunning}
    <p class="banner">⚠ {$t('content.dcsRunning')} <span class="muted">{$t('content.dcsRunningHint')}</span></p>
  {/if}

  <div class="summary">
    <div class="summary-card">
      <span class="k">{$t('content.activeMods')}</span>
      <strong>{activeMods}</strong>
      <span class="muted">{fmtSize(($contentModsScan?.totalBytes ?? 0) - ($contentMods.filter((m) => !m.enabled).reduce((s, m) => s + (m.sizeBytes ?? 0), 0)))}</span>
    </div>
    <div class="summary-card">
      <span class="k">{$t('content.disabledMods')}</span>
      <strong>{disabledMods}</strong>
      <span class="muted">{$t('content.disabledHint')}</span>
    </div>
    <div class="summary-card">
      <span class="k">{$t('content.liveries')}</span>
      <strong>{$contentLiveries.length}</strong>
      <span class="muted">{fmtSize(totalLiveryBytes)}</span>
    </div>
    <div class="summary-card">
      <span class="k">{$t('content.warnings')}</span>
      <strong>{warnings}</strong>
      <span class="muted">{$t('content.warningsHint')}</span>
    </div>
  </div>

  <nav class="tabs" aria-label={$t('content.title')}>
    <button class:active={tab === 'mods'} type="button" on:click={() => (tab = 'mods')}>
      {$t('content.tab.mods')} <span class="count">{$contentMods.length}</span>
    </button>
    <button class:active={tab === 'liveries'} type="button" on:click={() => (tab = 'liveries')}>
      {$t('content.tab.liveries')} <span class="count">{$contentLiveries.length}</span>
    </button>
    <button class:active={tab === 'trash'} type="button" on:click={() => (tab = 'trash')}>
      {$t('content.tab.trash')} <span class="count">{$contentQuarantine.length}</span>
    </button>
  </nav>

  {#if $contentLoading && !$contentModsScan}
    <div class="loading"><Skeleton lines={5} /></div>
  {:else if tab === 'mods'}
    <ModsTab />
  {:else if tab === 'liveries'}
    <LiveriesTab />
  {:else}
    <div class="trash">
      {#if $contentQuarantine.length === 0}
        <EmptyState title={$t('content.trash.empty')} message={$t('content.trash.emptyHint')} />
      {:else}
        <p class="muted">{$t('content.trash.hint')}</p>
        <div class="trash-list">
          {#each $contentQuarantine as item (item.id)}
            <div class="trash-item">
              <div class="trash-main">
                <strong>{item.name}</strong>
                <span class="muted">
                  {item.kind === 'mod' ? $t('content.kind.mod') : $t('content.kind.livery')}
                  {#if item.aircraft} · {item.aircraft}{/if}
                  {#if item.category} · {item.category}{/if}
                  · {fmtSize(item.sizeBytes)}
                </span>
                <span class="path">{item.relPath || item.path}</span>
              </div>
              <div class="trash-actions">
                <Badge>{item.createdAt?.slice(0, 16).replace('T', ' ') ?? ''}</Badge>
                <button class="btn" type="button" on:click={() => restore(item)} disabled={$dcsRunning}>
                  {$t('content.trash.restore')}
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  <ContentImportDialog
    bind:open={importOpen}
    on:imported={() => {
      importOpen = false;
      showToast($t('content.import.done'));
      loadContent();
    }}
  />
</section>

<style>
  .content-page {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    width: 100%;
    max-width: var(--content-max);
    margin: 0 auto;
    padding: 1rem;
  }

  .page-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.7rem;
  }

  .heading h2 {
    margin: 0;
    font-size: 1.02rem;
  }

  .heading p {
    margin: 0.15rem 0 0;
    font-size: 0.78rem;
  }

  .head-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 0.55rem;
  }

  .summary-card {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
    padding: 0.75rem 0.85rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-soft);
  }

  .summary-card .k {
    font-size: 0.66rem;
    font-weight: 650;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--faint);
  }

  .summary-card strong {
    font-size: 1.4rem;
    font-variant-numeric: tabular-nums;
  }

  .summary-card .muted {
    font-size: 0.72rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tabs {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-wrap: wrap;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.35rem;
  }

  .tabs button {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.38rem 0.7rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
    cursor: pointer;
  }

  .tabs button:hover {
    color: var(--text);
    border-color: var(--border);
  }

  .tabs button.active {
    color: var(--text);
    background: var(--panel);
    border-color: var(--border);
  }

  .count {
    min-width: 1.2rem;
    padding: 0.02rem 0.3rem;
    border-radius: 999px;
    background: var(--bg);
    color: var(--muted);
    font-size: 0.68rem;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }

  .loading {
    padding: 1rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }

  .trash {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .trash-list {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .trash-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.7rem;
    padding: 0.6rem 0.75rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .trash-main {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
  }

  .trash-main strong {
    font-size: 0.84rem;
  }

  .trash-main .muted {
    font-size: 0.72rem;
  }

  .trash-actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex: none;
  }

  .path {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.68rem;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .error {
    margin: 0;
    padding: 0.5rem 0.65rem;
    color: var(--red);
    background: var(--red-soft);
    border: 1px solid color-mix(in srgb, var(--red) 40%, transparent);
    border-radius: var(--radius);
    font-size: 0.78rem;
  }

  .banner {
    margin: 0;
    padding: 0.5rem 0.65rem;
    color: var(--amber);
    background: var(--amber-soft);
    border: 1px solid color-mix(in srgb, var(--amber) 40%, transparent);
    border-radius: var(--radius);
    font-size: 0.78rem;
  }

  .muted {
    color: var(--muted);
  }

  @media (max-width: 700px) {
    .content-page {
      padding: 0.7rem;
    }

    .trash-item {
      align-items: flex-start;
      flex-direction: column;
    }
  }
</style>
