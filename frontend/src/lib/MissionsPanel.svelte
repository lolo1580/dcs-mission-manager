<script>
  /**
   * Mission library: the .miz saved in DCS's Saved Games folder, described by the
   * metadata each one carries — theatre, in-game date and time, weather and size.
   */
  import { onMount } from 'svelte';
  import {
    missions,
    missionTotals,
    missionsError,
    missionsLoading,
    missionSearch,
    visibleMissions,
    loadMissions,
    fmtSize,
    fmtClock,
    fmtDate,
  } from './missions.js';
  import { t } from './i18n.js';

  import { theatres } from './units.js';

  onMount(loadMissions);

  /** Human name of a theatre id, from the theatre list when it is known. */
  function theatreName(id) {
    if (!id) return '—';
    return $theatres.find((x) => x.id === id)?.name ?? id;
  }
</script>

<section class="missions">
  <header>
    <h2>
      {$t('missions.title')}
      <span class="count">{$missionTotals.total}</span>
    </h2>
    <button class="refresh" on:click={loadMissions} disabled={$missionsLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  {#if $missionsError}
    <p class="error">{$missionsError}</p>
  {/if}

  <div class="controls">
    <input
      type="search"
      placeholder={$t('missions.search')}
      value={$missionSearch}
      on:input={(e) => missionSearch.set(e.currentTarget.value)}
    />
  </div>

  {#if $missions.length === 0 && !$missionsError}
    <p class="empty">{$t('missions.none')}</p>
  {:else if $visibleMissions.length === 0}
    <p class="empty">{$t('missions.noMatch')}</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>{$t('missions.col.name')}</th>
          <th>{$t('missions.col.theatre')}</th>
          <th>{$t('missions.col.date')}</th>
          <th>{$t('missions.col.time')}</th>
          <th>{$t('missions.col.weather')}</th>
          <th>{$t('missions.col.size')}</th>
          <th>{$t('missions.col.modified')}</th>
        </tr>
      </thead>
      <tbody>
        {#each $visibleMissions as m (m.path)}
          <tr>
            <td class="name" title={m.path}>{m.name}</td>
            <td>{theatreName(m.theatre)}</td>
            <td class="num">{m.date || '—'}</td>
            <td class="num">{fmtClock(m.startTime)}</td>
            <td class="weather">
              {#if m.weather}
                {m.weather}{#if m.temperatureC != null}<span class="temp">{Math.round(m.temperatureC)}°C</span>{/if}
              {:else}—{/if}
            </td>
            <td class="num">{fmtSize(m.sizeBytes)}</td>
            <td class="num muted">{fmtDate(m.modTime)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .missions {
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
    margin-bottom: 0.6rem;
  }

  .controls input[type='search'] {
    width: 280px;
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

  td {
    padding: 0.28rem 0.4rem;
    border-top: 1px solid var(--border);
    vertical-align: baseline;
  }

  td.name {
    color: var(--text);
    max-width: 22rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  td.num {
    text-align: right;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  td.muted {
    color: var(--muted);
  }

  td.weather {
    color: var(--muted);
  }

  .temp {
    margin-left: 0.4rem;
    color: var(--blue);
    font-variant-numeric: tabular-nums;
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
