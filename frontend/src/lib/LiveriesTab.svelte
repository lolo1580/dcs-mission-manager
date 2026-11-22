<script>
  import { t } from './i18n.js';
  import { showToast } from './toast.js';
  import {
    contentBusy,
    contentLiveries,
    contentLiveryScan,
    dcsRunning,
    liveryAction,
  } from './content.js';
  import LiveryCard from './LiveryCard.svelte';
  import EmptyState from './ui/EmptyState.svelte';

  let query = '';
  let aircraft = '';
  let source = '';

  $: aircraftOptions = $contentLiveryScan?.aircraft ?? [];
  $: visible = $contentLiveries.filter((livery) => {
    const text = `${livery.name} ${livery.aircraft ?? ''} ${livery.countries?.join(' ') ?? ''}`.toLowerCase();
    if (query && !text.includes(query.trim().toLowerCase())) return false;
    if (aircraft === '__none__' && livery.aircraft) return false;
    if (aircraft && aircraft !== '__none__' && livery.aircraft !== aircraft) return false;
    if (source && livery.source !== source) return false;
    return true;
  });

  async function reveal(livery) {
    try {
      await liveryAction('reveal', { id: livery.id });
    } catch (error) {
      showToast(error.message, 'error');
    }
  }

  async function quarantine(event) {
    const livery = event.detail;
    const deps = livery.dependencies ?? [];
    let force = false;
    if (deps.length > 0) {
      if (!confirm($t('content.livery.confirmDeps', { name: livery.name, deps: deps.join(', ') }))) return;
      force = true;
    } else if (!confirm($t('content.livery.confirmRemove', { name: livery.name }))) {
      return;
    }
    try {
      await liveryAction('quarantine', { id: livery.id, force });
      showToast($t('content.livery.removed', { name: livery.name }));
    } catch (error) {
      showToast(error.message, 'error');
    }
  }

  async function classify(event) {
    const livery = event.detail;
    let targetAircraft = livery.aircraft;
    if (!targetAircraft) {
      targetAircraft = prompt($t('content.livery.chooseAircraft'), '');
      targetAircraft = targetAircraft?.trim() ?? '';
      if (!targetAircraft) return;
    }
    const ids = [livery.id];
    for (const dep of livery.dependencies ?? []) {
      const found = $contentLiveries.find(
        (item) =>
          item.source === 'user' &&
          (item.relPath === dep || item.relPath.endsWith(`/${dep}`) || item.name === dep)
      );
      if (found && !ids.includes(found.id)) ids.push(found.id);
    }
    if (!confirm($t('content.livery.confirmClassify', { name: livery.name, aircraft: targetAircraft }))) return;
    try {
      const result = await liveryAction('classify', { ids, aircraft: targetAircraft });
      showToast($t('content.livery.classified', { count: result.moved?.length ?? ids.length }));
    } catch (error) {
      showToast(error.message, 'error');
    }
  }
</script>

<div class="liveries">
  <div class="toolbar">
    <input class="search" type="search" bind:value={query} placeholder={$t('content.livery.search')} aria-label={$t('content.livery.search')} />
    <select bind:value={aircraft} aria-label={$t('content.livery.aircraft')}>
      <option value="">{$t('content.livery.allAircraft')}</option>
      {#each aircraftOptions as option (option.aircraft || '__none__')}
        <option value={option.aircraft || '__none__'}>
          {option.aircraft || $t('content.livery.unknownAircraft')} ({option.count})
        </option>
      {/each}
    </select>
    <select bind:value={source} aria-label={$t('content.livery.source')}>
      <option value="">{$t('content.livery.allSources')}</option>
      <option value="user">{$t('content.livery.userSource')}</option>
      <option value="mod">{$t('content.livery.modSource')}</option>
    </select>
    <span class="spacer"></span>
    <span class="muted">{$t('content.livery.count', { count: visible.length })}</span>
  </div>

  <p class="hint">{$t('content.livery.hint')}</p>

  {#if visible.length === 0}
    <EmptyState title={$t('content.livery.empty')} message={$t('content.livery.emptyHint')} />
  {:else}
    <div class="grid">
      {#each visible as livery (livery.id)}
        <LiveryCard
          {livery}
          busy={$contentBusy}
          dcsRunning={$dcsRunning}
          on:reveal={({ detail }) => reveal(detail)}
          on:quarantine={quarantine}
          on:classify={classify}
        />
      {/each}
    </div>
  {/if}
</div>

<style>
  .liveries {
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

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 0.72rem;
  }

  .muted {
    color: var(--muted);
    font-size: 0.74rem;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 0.55rem;
  }
</style>
