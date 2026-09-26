<script>
  import { onMount } from 'svelte';
  import {
    debriefList,
    debrief,
    debriefError,
    debriefLoading,
    debriefEvents,
    debriefKills,
    loadDebriefList,
    loadDebrief,
    fmtTime,
  } from './debriefs.js';
  import { t } from './i18n.js';

  onMount(loadDebriefList);

  function dateOf(ms) {
    return ms ? new Date(ms).toLocaleString() : '—';
  }

  function eventColor(type) {
    switch (type) {
      case 'kill':
      case 'shot down':
        return '#3d7dff';
      case 'crash':
      case 'pilot dead':
        return '#ff4d4d';
      case 'eject':
        return '#f0883e';
      case 'land':
      case 'landing':
        return '#3fb950';
      case 'takeoff':
        return '#58a6ff';
      default:
        return '#9aa4b2';
    }
  }

  function describe(tr, e) {
    const who = e.initiatorPilot || '?';
    switch (e.type) {
      case 'takeoff':
        return tr('events.tookOff', { pilot: who, place: e.placeDisplayName || e.place || '?' });
      case 'land':
      case 'landing':
        return tr('events.landed', { pilot: who, place: e.placeDisplayName || e.place || '?' });
      case 'kill':
      case 'shot down':
        return tr('debriefs.killed', { pilot: who, target: e.target || '?', weapon: e.weapon || '?' });
      case 'crash':
        return tr('debriefs.crashed', { pilot: who });
      case 'eject':
        return tr('debriefs.ejected', { pilot: who });
      case 'pilot dead':
        return tr('debriefs.died', { pilot: who });
      case 'engine shutdown':
        return tr('debriefs.engineShutdown', { pilot: who, place: e.placeDisplayName || e.place || '?' });
      case 'mission end':
        return tr('debriefs.missionEnd', { comment: e.comment || '' });
      default:
        return who !== '?' ? `${who}` : '';
    }
  }
</script>

<section class="debriefs">
  <header>
    <h2>
      {$t('debriefs.title')} <span class="count">{$debriefList.length}</span>
    </h2>
    <button class="refresh" on:click={loadDebriefList}>{$t('debriefs.refresh')}</button>
  </header>

  {#if $debriefError}
    <p class="error">{$debriefError}</p>
  {/if}

  {#if $debriefList.length === 0}
    <p class="empty">
      {$t('debriefs.none')}
    </p>
  {:else}
    <div class="split">
      <ul class="list">
        {#each $debriefList as d (d.id)}
          <li>
            <button class:selected={$debrief?.id === d.id} on:click={() => loadDebrief(d.id)}>
              <span class="name">{d.mission || d.parsed?.callsign || $t('debriefs.label', { id: d.id })}</span>
              <span class="sub">
                {dateOf(d.createdAt)} · {d.parsed?.summary?.kills ?? 0} {$t('debriefs.kills')} ·
                {fmtTime(d.parsed?.missionTime ?? 0)}
              </span>
            </button>
          </li>
        {/each}
      </ul>

      <div class="detail">
        {#if $debriefLoading}
          <p class="empty">{$t('debriefs.loading')}</p>
        {:else if $debrief}
          {@const s = $debrief.parsed?.summary}
          <h3>{$debrief.mission || $debrief.parsed?.callsign || $t('debriefs.label', { id: $debrief.id })}</h3>

          <div class="stats">
            <div><span>{s?.takeoffs ?? 0}</span>{$t('debriefs.takeoffs')}</div>
            <div><span>{s?.landings ?? 0}</span>{$t('debriefs.landings')}</div>
            <div><span>{s?.kills ?? 0}</span>{$t('debriefs.kills')}</div>
            <div><span>{s?.crashes ?? 0}</span>{$t('debriefs.crashes')}</div>
            <div><span>{s?.ejections ?? 0}</span>{$t('debriefs.ejections')}</div>
            <div><span>{fmtTime($debrief.parsed?.missionTime ?? 0)}</span>{$t('debriefs.duration')}</div>
          </div>

          {#if s?.pilots?.length}
            <p class="pilots">{$t('debriefs.pilots')}: {s.pilots.join(', ')}</p>
          {/if}

          <h4>
            {$t('debriefs.timeline')}
            <span class="count">{$debriefEvents.length}</span>
          </h4>
          <ol>
            {#each $debriefEvents as e, i (`${e.eventId}-${i}`)}
              <li>
                <span class="t">{fmtTime(e.t)}</span>
                <span class="bar" style="background:{eventColor(e.type)}"></span>
                <span class="kind">{e.type}</span>
                <span class="desc">{describe($t, e)}</span>
              </li>
            {/each}
          </ol>
        {:else}
          <p class="empty">{$t('debriefs.select')}</p>
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .debriefs {
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
    gap: 0.4rem;
  }

  .count {
    color: var(--text);
    font-variant-numeric: tabular-nums;
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

  .refresh:hover {
    color: var(--text);
    border-color: var(--blue);
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
    width: 280px;
    min-width: 280px;
    overflow-y: auto;
  }

  .list button {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    width: 100%;
    padding: 0.45rem 0.55rem;
    margin-bottom: 0.25rem;
    text-align: left;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
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
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sub {
    font-size: 0.72rem;
    color: var(--muted);
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

  .detail h4 {
    margin: 1rem 0 0.5rem;
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    display: flex;
    gap: 0.4rem;
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(90px, 1fr));
    gap: 0.5rem;
  }

  .stats div {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem 0.6rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 0.7rem;
    color: var(--muted);
  }

  .stats span {
    font-size: 1.1rem;
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .pilots {
    margin: 0.7rem 0 0;
    font-size: 0.78rem;
    color: var(--muted);
  }

  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    font-size: 0.78rem;
  }

  ol li {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    padding: 0.22rem 0;
    border-top: 1px solid var(--border);
  }

  .t {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    flex: none;
    width: 52px;
  }

  .bar {
    width: 3px;
    align-self: stretch;
    border-radius: 2px;
    flex: none;
  }

  .kind {
    flex: none;
    width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .desc {
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty {
    color: var(--muted);
    font-size: 0.82rem;
    line-height: 1.5;
  }

  .error {
    color: #f0b429;
    font-size: 0.78rem;
  }

  code {
    background: var(--bg);
    padding: 0.05rem 0.3rem;
    border-radius: 4px;
  }
</style>
