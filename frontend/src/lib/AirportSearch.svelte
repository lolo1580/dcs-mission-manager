<script>
  /**
   * Airport autocomplete backed by /api/airports. The embedded dataset covers
   * the world; DCS airfields of the selected theatre are merged in by the API
   * and shown with their tower/TACAN frequencies.
   */
  import { t } from './i18n.js';
  import { searchAirports } from './flightplans.js';

  export let placeholder = '';
  export let theatre = '';
  export let disabled = false;
  export let onSelect = () => {};

  let query = '';
  let results = [];
  let open = false;
  let loading = false;
  let timer;
  let seq = 0;

  function input(event) {
    query = event.currentTarget.value;
    clearTimeout(timer);
    if (!query.trim()) {
      results = [];
      open = false;
      return;
    }
    timer = setTimeout(run, 250);
  }

  async function run() {
    const current = ++seq;
    loading = true;
    const list = await searchAirports(query, theatre);
    if (current !== seq) return;
    results = list;
    loading = false;
    open = true;
  }

  function choose(airport) {
    onSelect(airport);
    query = '';
    results = [];
    open = false;
  }

  function blur() {
    // Let a click on a result land before closing the list.
    setTimeout(() => (open = false), 160);
  }

  function keydown(event) {
    if (event.key === 'Escape') {
      open = false;
    }
    if (event.key === 'Enter' && results.length > 0) {
      event.preventDefault();
      choose(results[0]);
    }
  }
</script>

<div class="airport-search">
  <input
    type="text"
    value={query}
    {placeholder}
    {disabled}
    on:input={input}
    on:keydown={keydown}
    on:focus={() => { if (results.length > 0) open = true; }}
    on:blur={blur}
    autocomplete="off"
    spellcheck="false"
  />
  {#if loading}<span class="loading">…</span>{/if}
  {#if open}
    <ul class="results">
      {#if results.length === 0}
        <li class="empty">{$t('planner.noAirport')}</li>
      {:else}
        {#each results as airport (airport.ident + ':' + (airport.source ?? '') + ':' + airport.name)}
          <li>
            <button type="button" on:click={() => choose(airport)}>
              <span class="ident">{airport.ident}</span>
              {#if airport.icao && airport.icao !== airport.ident}<span class="icao">{airport.icao}</span>{/if}
              <span class="name">{airport.name}</span>
              <span class="city">{airport.municipality}{airport.country ? `, ${airport.country}` : ''}</span>
              {#if airport.source === 'dcs'}<span class="badge">{$t('planner.sourceDCS')}</span>{/if}
              {#if airport.tower}<span class="freq">TWR {airport.tower}</span>{/if}
              {#if airport.tacan}<span class="freq">TCN {airport.tacan}</span>{/if}
            </button>
          </li>
        {/each}
      {/if}
    </ul>
  {/if}
</div>

<style>
  .airport-search {
    position: relative;
    min-width: 0;
  }

  input {
    width: 100%;
    padding: 0.34rem 0.5rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.78rem;
  }

  .loading {
    position: absolute;
    right: 0.5rem;
    top: 0.28rem;
    color: var(--muted);
    font-size: 0.8rem;
  }

  .results {
    position: absolute;
    z-index: 40;
    left: 0;
    right: 0;
    top: calc(100% + 4px);
    margin: 0;
    padding: 0.25rem;
    list-style: none;
    max-height: 320px;
    overflow-y: auto;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 12px 30px rgba(0, 0, 0, 0.35);
  }

  .results li + li {
    border-top: 1px solid var(--border);
  }

  .results button {
    display: grid;
    grid-template-columns: auto auto 1fr auto;
    gap: 0.15rem 0.45rem;
    align-items: baseline;
    width: 100%;
    padding: 0.4rem 0.45rem;
    text-align: left;
    color: var(--text);
    background: transparent;
    border: none;
    border-radius: 5px;
    cursor: pointer;
  }

  .results button:hover {
    background: var(--bg);
  }

  .ident {
    font-weight: 650;
    font-size: 0.82rem;
  }

  .icao {
    color: var(--muted);
    font-size: 0.68rem;
  }

  .name {
    grid-column: 1 / 4;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.78rem;
  }

  .city {
    grid-column: 1 / 4;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-size: 0.7rem;
  }

  .badge {
    justify-self: end;
    padding: 0.05rem 0.3rem;
    border-radius: 4px;
    background: rgba(88, 166, 255, 0.15);
    color: #58a6ff;
    font-size: 0.62rem;
  }

  .freq {
    justify-self: end;
    color: var(--muted);
    font-size: 0.66rem;
    font-variant-numeric: tabular-nums;
  }

  .empty {
    padding: 0.5rem;
    color: var(--muted);
    font-size: 0.74rem;
  }
</style>
