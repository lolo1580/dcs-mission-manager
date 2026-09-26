<script>
  import MapView from './lib/MapView.svelte';
  import Sidebar from './lib/Sidebar.svelte';
  import UnitDetails from './lib/UnitDetails.svelte';
  import { connected, lastUpdate, units, visibleUnits, basemaps, basemapId } from './lib/units.js';

  let mapView;
</script>

<div class="layout">
  <header>
    <strong>DCS Mission Manager</strong>
    <span class="live">
      <span class="dot" class:on={$connected}></span>
      {$connected ? 'connecté' : 'hors ligne'}
    </span>
    <span class="sep"></span>
    <span class="meta">{$units.length} unité{$units.length === 1 ? '' : 's'} suivie{$units.length === 1 ? '' : 's'}</span>
    <span class="meta">{$visibleUnits.length} affichée{$visibleUnits.length === 1 ? '' : 's'}</span>
    {#if $lastUpdate}
      <span class="meta">maj {$lastUpdate.toLocaleTimeString()}</span>
    {/if}
    {#if $basemaps.length}
      <label class="basemap">
        Fond
        <select bind:value={$basemapId}>
          {#each $basemaps as b (b.id)}
            <option value={b.id}>{b.name}</option>
          {/each}
        </select>
      </label>
    {/if}
    <button class="recenter" on:click={() => mapView?.recenter()} title="Recentrer la carte">
      Recentrer
    </button>
  </header>

  <main>
    <Sidebar />
    <div class="map-wrap">
      <MapView bind:this={mapView} />
      <UnitDetails />
    </div>
  </main>
</div>

<style>
  .layout {
    display: grid;
    grid-template-rows: auto 1fr;
    height: 100%;
  }

  header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 1rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    font-size: 0.88rem;
  }

  header strong {
    white-space: nowrap;
  }

  .live {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--muted);
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--red);
  }

  .dot.on {
    background: var(--green);
  }

  .sep {
    width: 1px;
    height: 18px;
    background: var(--border);
  }

  .meta {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .recenter {
    margin-left: auto;
    padding: 0.35rem 0.7rem;
    font-size: 0.8rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .basemap {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    margin-left: auto;
    color: var(--muted);
    font-size: 0.8rem;
  }

  .basemap select {
    padding: 0.3rem 0.45rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .basemap select:focus {
    outline: none;
    border-color: var(--blue);
  }

  .basemap + .recenter {
    margin-left: 0;
  }

  .recenter:hover {
    border-color: var(--blue);
  }

  main {
    display: flex;
    min-height: 0;
  }

  .map-wrap {
    position: relative;
    flex: 1;
    min-width: 0;
  }
</style>
