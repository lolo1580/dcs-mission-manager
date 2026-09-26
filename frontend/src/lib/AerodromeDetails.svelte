<script>
  import { selectedUnit } from './units.js';
  import {
    selectedAerodrome,
    selectAerodrome,
    fmtMHz,
    fmtCoords,
    aerodromeCharts,
    chartsError,
    viewingChart,
  } from './aerodromes.js';
  import { revealAerodrome } from './ui.js';
  import { coalitionColor } from './icons.js';
  import { t } from './i18n.js';

  function fmt(v, digits = 0) {
    return typeof v === 'number' ? v.toLocaleString(undefined, { maximumFractionDigits: digits }) : '—';
  }

  // A unit click clears the airfield (see MapView), and vice versa, so only one
  // card is ever shown in this corner.
  function close() {
    selectAerodrome(null);
  }

  function reveal() {
    revealAerodrome($selectedAerodrome);
  }
</script>

{#if $selectedAerodrome && !$selectedUnit}
  {@const a = $selectedAerodrome}
  <div class="card">
    <header>
      <span class="dot" style="background:{coalitionColor(a.coalition)}"></span>
      <strong>{a.name}</strong>
      <button class="close" title={$t('unit.close')} on:click={close}>×</button>
    </header>
    <dl>
      <div><dt>{$t('aerodromes.icao')}</dt><dd>{a.icaoCode && a.icaoCode.length === 4 ? a.icaoCode : a.id}</dd></div>
      {#if a.coalition}
        <div><dt>{$t('aerodromes.coalition')}</dt><dd>{$t('coalition.' + a.coalition)}</dd></div>
      {/if}
      <div><dt>{$t('aerodromes.coordinates')}</dt><dd>{fmtCoords(a)}</dd></div>
      {#if a.elevationM}<div><dt>{$t('aerodromes.elevation')}</dt><dd>{fmt(a.elevationM)} m</dd></div>{/if}
      {#if a.runway}<div><dt>{$t('aerodromes.runway')}</dt><dd>{a.runway}</dd></div>{/if}
      {#if a.tower}
        <div class="hl"><dt>{$t('aerodromes.tower')}</dt><dd>{fmtMHz(a.tower)}</dd></div>
      {/if}
      {#if a.tacan}
        <div class="hl"><dt>TACAN</dt><dd>{a.tacan}</dd></div>
      {/if}
      {#if a.vor}
        <div class="hl"><dt>{$t('aerodromes.vor')}</dt><dd>{a.vor}{a.vorMhz ? ` (${fmtMHz(a.vorMhz)})` : ''}</dd></div>
      {/if}
      {#if a.rsbn}
        <div class="hl"><dt>{$t('aerodromes.rsbn')}</dt><dd>{a.rsbn}</dd></div>
      {/if}
      {#each a.ils ?? [] as ils, i (i)}
        <div class="hl"><dt>ILS{ils.runway ? ` ${ils.runway}` : ''}</dt><dd>{fmtMHz(ils.mhz)}</dd></div>
      {/each}
      {#each a.prmg ?? [] as prmg, i (i)}
        <div class="hl"><dt>{$t('aerodromes.prmg')}{prmg.runway ? ` ${prmg.runway}` : ''}</dt><dd>{fmtMHz(prmg.mhz)}</dd></div>
      {/each}
      {#each a.ndb ?? [] as ndb, i (i)}
        <div class="hl"><dt>{$t('aerodromes.ndb')}{ndb.name ? ` ${ndb.name}` : ''}</dt><dd>{ndb.khz ? `${ndb.khz} kHz` : '—'}</dd></div>
      {/each}
    </dl>

    {#if $chartsError}
      <p class="charts muted">{$chartsError}</p>
    {:else if $aerodromeCharts.length}
      <h4>{$t('aerodromes.charts')}</h4>
      <ul class="charts">
        {#each $aerodromeCharts as c (c.path)}
          <li>
            <button class="chart" on:click={() => viewingChart.set(c)}>
              <span class="kind">{$t('charts.kind.' + c.kind)}</span>
              <span class="cname">{c.name}</span>
            </button>
          </li>
        {/each}
      </ul>
    {:else}
      <p class="charts muted">{$t('aerodromes.noChart')}</p>
    {/if}

    <p class="src" title={a.source === 'dcs' ? $t('aerodromes.source.dcsHint') : $t('aerodromes.source.embeddedHint')}>
      {a.source === 'dcs' ? $t('aerodromes.source.dcs') : $t('aerodromes.source.embedded')}
    </p>

    <button class="reveal" on:click={reveal}>{$t('aerodromes.showOnMap')}</button>
  </div>
{/if}

<style>
  /* Shares the bottom-right corner with the unit card; only one is shown. */
  .card {
    position: absolute;
    right: 1rem;
    bottom: 1rem;
    z-index: 1000;
    /* Never wider than the map it sits on, so it cannot spill off screen on a
       narrow window. */
    width: min(280px, calc(100% - 2rem));
    padding: 0.8rem 0.9rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }

  header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
  }

  header strong {
    flex: 1;
    font-size: 0.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    flex: none;
  }

  .close {
    background: transparent;
    border: none;
    color: var(--muted);
    font-size: 1.1rem;
    line-height: 1;
    cursor: pointer;
  }

  .close:hover {
    color: var(--text);
  }

  dl {
    margin: 0;
    display: grid;
    gap: 0.25rem;
    font-size: 0.8rem;
  }

  dl div {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
  }

  dl div.hl {
    padding: 0.15rem 0.35rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
  }

  dt {
    color: var(--muted);
  }

  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
    text-align: right;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .charts {
    list-style: none;
    margin: 0.5rem 0 0;
    padding: 0;
    display: grid;
    /* minmax(0, 1fr) plus min-width on the items: a grid item defaults to
       min-width: auto, so a long file name makes it wider than its container
       instead of being truncated. */
    grid-template-columns: minmax(0, 1fr);
    gap: 0.2rem;
  }

  .charts li {
    min-width: 0;
  }

  .charts.muted {
    font-size: 0.72rem;
    color: var(--muted);
  }

  h4 {
    margin: 0.7rem 0 0;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  .chart {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    /* min-width lets the children shrink, so a long file name is truncated
       rather than overflowing the card. */
    min-width: 0;
    padding: 0.25rem 0.4rem;
    font-size: 0.74rem;
    text-align: left;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .chart:hover {
    border-color: var(--blue);
  }

  .kind {
    flex: none;
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--muted);
  }

  .cname {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .src {
    margin: 0.35rem 0 0;
    font-size: 0.7rem;
    color: var(--muted);
    cursor: help;
  }

  .reveal {
    width: 100%;
    margin-top: 0.6rem;
    padding: 0.3rem 0.55rem;
    font-size: 0.76rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .reveal:hover {
    border-color: var(--blue);
  }
</style>
