<script>
  import { onMount } from 'svelte';
  import {
    filteredAerodromes,
    aerodromeError,
    search,
    showOnMap,
    loadAerodromes,
    loadNearest,
    fmtMHz,
    fmtCoords,
  } from './aerodromes.js';
  import { t } from './i18n.js';

  let selected = null;
  let nearestFirst = false;

  onMount(loadAerodromes);

  async function useNearest() {
    nearestFirst = await loadNearest();
  }
</script>

<section class="aerodromes">
  <header>
    <h2>{$t('aerodromes.title')} <span class="count">{$filteredAerodromes.length}</span></h2>
    <button class="refresh" on:click={loadAerodromes}>{$t('aerodromes.refresh')}</button>
  </header>

  {#if $aerodromeError}
    <p class="error">{$aerodromeError}</p>
  {/if}

  <div class="controls">
    <input
      type="search"
      placeholder={$t('aerodromes.search')}
      value={$search}
      on:input={(e) => search.set(e.currentTarget.value)}
    />
    <button class="nearest" on:click={useNearest} title={$t('aerodromes.byDistance')}>
      {nearestFirst ? $t('aerodromes.byDistance') : $t('aerodromes.nearest')}
    </button>
    <label class="onmap">
      <input type="checkbox" bind:checked={$showOnMap} />
      {$t('aerodromes.onMap')}
    </label>
  </div>

  <div class="split">
    <ul class="list">
      {#each $filteredAerodromes as a (a.id)}
        <li>
          <button class:selected={selected?.id === a.id} on:click={() => (selected = a)}>
            <span class="name">{a.name}</span>
            <span class="sub">
              {a.id}
              {#if a.distanceKm != null}<span class="dist">{a.distanceKm.toFixed(1)} km</span>{/if}
            </span>
          </button>
        </li>
      {/each}
      {#if $filteredAerodromes.length === 0}
        <li class="empty">{$t('aerodromes.none')}</li>
      {/if}
    </ul>

    <div class="detail">
      {#if selected}
        <h3>{selected.name} <span class="icao">{selected.id}</span></h3>
        <dl>
          <div><dt>{$t('aerodromes.coalition')}</dt><dd>{$t('coalition.' + selected.coalition)}</dd></div>
          <div><dt>{$t('aerodromes.coordinates')}</dt><dd>{fmtCoords(selected)}</dd></div>
          <div><dt>{$t('aerodromes.elevation')}</dt><dd>{selected.elevationM} m</dd></div>
          <div><dt>{$t('aerodromes.runway')}</dt><dd>{selected.runway}</dd></div>
          <div class="hl"><dt>{$t('aerodromes.tower')}</dt><dd>{fmtMHz(selected.tower)}</dd></div>
          {#if selected.tacan}
            <div class="hl"><dt>TACAN</dt><dd>{selected.tacan}</dd></div>
          {/if}
          {#if selected.ils?.length}
            {#each selected.ils as ils (ils.runway)}
              <div class="hl"><dt>ILS {ils.runway}</dt><dd>{fmtMHz(ils.mhz)}</dd></div>
            {/each}
          {/if}
        </dl>

        {#if selected.charts?.length}
          <h4>{$t('aerodromes.charts')}</h4>
          <ul class="charts">
            {#each selected.charts as c (c)}
              <li>{c}</li>
            {/each}
          </ul>
          <p class="hint">
            {$t('aerodromes.chartsHint')}
          </p>
        {/if}
      {:else}
        <p class="empty">{$t('aerodromes.select')}</p>
      {/if}
    </div>
  </div>
</section>

<style>
  .aerodromes {
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
  }

  .count {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .refresh,
  .nearest {
    padding: 0.25rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .refresh:hover,
  .nearest:hover {
    color: var(--text);
    border-color: var(--blue);
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    margin-bottom: 0.6rem;
  }

  .controls input[type='search'] {
    flex: 1;
    min-width: 0;
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

  .onmap {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.76rem;
    color: var(--muted);
    white-space: nowrap;
    cursor: pointer;
  }

  .split {
    display: flex;
    gap: 1rem;
    min-height: 0;
    flex: 1;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    width: 260px;
    min-width: 260px;
    overflow-y: auto;
  }

  .list button {
    display: flex;
    flex-direction: column;
    gap: 0.05rem;
    width: 100%;
    padding: 0.4rem 0.55rem;
    margin-bottom: 0.22rem;
    text-align: left;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 7px;
    color: var(--text);
    cursor: pointer;
  }

  .list button:hover {
    border-color: var(--muted);
  }

  .list button.selected {
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
  }

  .name {
    font-size: 0.84rem;
  }

  .sub {
    display: flex;
    gap: 0.5rem;
    font-size: 0.72rem;
    color: var(--muted);
  }

  .dist {
    color: var(--blue);
    font-variant-numeric: tabular-nums;
  }

  .detail {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
  }

  .detail h3 {
    margin: 0 0 0.7rem;
    font-size: 1rem;
  }

  .icao {
    margin-left: 0.4rem;
    font-size: 0.78rem;
    color: var(--muted);
    font-weight: 400;
  }

  .detail h4 {
    margin: 1rem 0 0.4rem;
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  dl {
    margin: 0;
    display: grid;
    gap: 0.3rem;
    font-size: 0.82rem;
    max-width: 420px;
  }

  dl div {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.25rem 0.45rem;
    border-radius: 6px;
  }

  dl div.hl {
    background: var(--bg);
    border: 1px solid var(--border);
  }

  dt {
    color: var(--muted);
  }

  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }

  .charts {
    list-style: none;
    margin: 0;
    padding: 0;
    font-size: 0.76rem;
    color: var(--muted);
  }

  .charts li {
    padding: 0.15rem 0;
  }

  .hint {
    margin: 0.5rem 0 0;
    font-size: 0.74rem;
    color: var(--muted);
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
</style>
