<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { sessionUser, isServer } from './session.js';
  import DebriefDetail from './DebriefDetail.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Skeleton from './ui/Skeleton.svelte';
  import { fmtTime } from './debriefs.js';

  let missions = [];
  let debriefs = [];
  let loading = true;
  let error = '';
  let query = '';
  let debriefFilter = 'all';
  let sourceFilter = 'all';
  let uploaderFilter = '';
  let pilotFilter = '';
  let mineOnly = false;
  let selectedKey = '';
  let pilots = [];

  async function getJSON(path) {
    const response = await fetch(path);
    if (!response.ok) throw new Error(`${response.status}`);
    return response.json();
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const missionPath = pilotFilter
        ? `/api/history/missions?limit=500&pilot=${encodeURIComponent(pilotFilter)}`
        : '/api/history/missions?limit=500';
      const [missionBody, debriefBody, pilotBody] = await Promise.all([
        getJSON(missionPath),
        getJSON('/api/debriefs?limit=200'),
        getJSON('/api/stats/pilots').catch(() => ({ pilots: [] })),
      ]);
      missions = missionBody.missions ?? [];
      debriefs = debriefBody.debriefs ?? [];
      pilots = [...new Set((pilotBody.pilots ?? []).map((pilot) => pilot.name).filter(Boolean))].sort();
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  function buildFlights(missionList, debriefList, includeOrphans = true) {
    const byMission = new Map();
    for (const debrief of debriefList) {
      if (!debrief.missionId) continue;
      const current = byMission.get(debrief.missionId);
      if (!current || (debrief.createdAt ?? 0) > (current.createdAt ?? 0)) {
        byMission.set(debrief.missionId, debrief);
      }
    }

    const flights = missionList.map((mission) => ({
      key: `m${mission.id}`,
      mission,
      debrief: byMission.get(mission.id) ?? null,
      at: mission.endedAt || mission.startedAt || 0,
    }));

    if (includeOrphans) {
      const known = new Set(missionList.map((mission) => mission.id));
      for (const debrief of debriefList) {
        if (debrief.missionId && known.has(debrief.missionId)) continue;
        flights.push({
          key: `d${debrief.id}`,
          mission: null,
          debrief,
          at: debrief.createdAt ?? 0,
        });
      }
    }

    return flights.sort((a, b) => b.at - a.at);
  }

  $: allFlights = buildFlights(missions, debriefs, !pilotFilter);
  $: uploaders = [...new Set(missions.map((mission) => mission.uploader).filter(Boolean))].sort();
  $: flights = allFlights.filter((flight) => {
    const name = `${flight.mission?.name ?? ''} ${flight.debrief?.mission ?? ''} ${flight.mission?.theatre ?? flight.debrief?.theatre ?? ''}`.toLowerCase();
    if (query && !name.includes(query.trim().toLowerCase())) return false;
    if (debriefFilter === 'with' && !flight.debrief) return false;
    if (debriefFilter === 'without' && flight.debrief) return false;
    if (sourceFilter !== 'all' && (flight.mission?.source ?? 'live') !== sourceFilter) return false;
    if (uploaderFilter && flight.mission?.uploader !== uploaderFilter) return false;
    if (mineOnly && $isServer && flight.mission?.ownerId !== $sessionUser?.id) return false;
    return true;
  });

  $: if (flights.length > 0 && !flights.some((flight) => flight.key === selectedKey)) {
    selectedKey = flights[0].key;
  }
  $: selected = flights.find((flight) => flight.key === selectedKey) ?? null;

  function dateOf(ms) {
    if (!ms) return '—';
    return new Date(ms).toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function duration(mission) {
    if (!mission?.startedAt) return '—';
    const end = mission.endedAt || mission.lastActivityAt || mission.startedAt;
    const total = Math.max(0, Math.round((end - mission.startedAt) / 1000));
    if (total >= 3600) {
      const hours = Math.floor(total / 3600);
      const minutes = Math.floor((total % 3600) / 60);
      return `${hours} h ${String(minutes).padStart(2, '0')}`;
    }
    return `${Math.floor(total / 60)} min`;
  }

  function summary(flight) {
    return flight?.debrief?.parsed?.summary ?? {};
  }

  function title(flight) {
    return flight.mission?.name || flight.debrief?.mission || `#${flight.mission?.id ?? flight.debrief?.id}`;
  }
</script>

<section class="flights">
  <div class="toolbar">
    <input class="search" type="search" bind:value={query} placeholder={$t('flights.search')} aria-label={$t('flights.search')} />
    <select bind:value={debriefFilter} aria-label={$t('flights.debrief')}>
      <option value="all">{$t('flights.all')}</option>
      <option value="with">{$t('flights.withDebrief')}</option>
      <option value="without">{$t('flights.withoutDebrief')}</option>
    </select>
    <select bind:value={sourceFilter} aria-label={$t('flights.source')}>
      <option value="all">{$t('flights.sourceAll')}</option>
      <option value="live">{$t('flights.sourceLive')}</option>
      <option value="test">{$t('flights.sourceTest')}</option>
    </select>
    {#if pilots.length > 0}
      <select value={pilotFilter} aria-label={$t('flights.pilot')}
        on:change={(event) => { pilotFilter = event.currentTarget.value; load(); }}>
        <option value="">{$t('flights.allPilots')}</option>
        {#each pilots as pilot (pilot)}
          <option value={pilot}>{pilot}</option>
        {/each}
      </select>
    {/if}
    {#if $isServer}
      <select bind:value={uploaderFilter} aria-label={$t('flights.uploader')}>
        <option value="">{$t('flights.allUploaders')}</option>
        {#each uploaders as uploader (uploader)}
          <option value={uploader}>{uploader}</option>
        {/each}
      </select>
      <label class="mine">
        <input type="checkbox" bind:checked={mineOnly} />
        {$t('flights.mine')}
      </label>
    {/if}
    <span class="count">{$t(flights.length === 1 ? 'flights.countOne' : 'flights.count', { count: flights.length })}</span>
    <button class="btn ghost" on:click={load}>{$t('flights.refresh')}</button>
  </div>

  {#if loading}
    <div class="panel"><Skeleton lines={8} /></div>
  {:else if error}
    <EmptyState title={$t('flights.error', { detail: '' })} message={error} />
  {:else if flights.length === 0}
    <EmptyState title={$t('flights.none')} message="" icon="M3 12h18M12 3v18" />
  {:else}
    <div class="split">
      <ul class="list">
        {#each flights as flight (flight.key)}
          <li>
            <button class:selected={flight.key === selectedKey} on:click={() => (selectedKey = flight.key)}>
              <span class="row-head">
                <span class="name">{title(flight)}</span>
                {#if flight.mission?.source === 'test'}<Badge tone="amber">{$t('flights.sourceTest')}</Badge>{/if}
                {#if flight.mission?.abandoned}<Badge tone="amber">{$t('flights.abandoned')}</Badge>{/if}
                {#if flight.debrief}<Badge tone="blue">{$t('flights.debrief')}</Badge>{/if}
              </span>
              <span class="sub">
                {dateOf(flight.at)}
                {#if flight.mission?.theatre ?? flight.debrief?.theatre}· {flight.mission?.theatre ?? flight.debrief?.theatre}{/if}
                {#if flight.mission?.uploader}· {flight.mission.uploader}{/if}
              </span>
              <span class="sub">
                {duration(flight.mission)}
                {#if flight.debrief}· {summary(flight).kills ?? 0} {$t('debriefs.kills')} · {fmtTime(flight.debrief.parsed?.missionTime ?? 0)}{/if}
              </span>
            </button>
          </li>
        {/each}
      </ul>

      <div class="detail panel">
        {#if selected}
          <header class="detail-head">
            <div>
              <h2>{title(selected)}</h2>
              <p class="meta">
                {dateOf(selected.at)}
                {#if selected.mission?.theatre ?? selected.debrief?.theatre}· {selected.mission?.theatre ?? selected.debrief?.theatre}{/if}
                {#if selected.mission?.uploader}· {$t('flights.uploader')}: {selected.mission.uploader}{/if}
              </p>
            </div>
          </header>

          {#if selected.mission}
            <div class="meta-grid">
              <div><span>{$t('flights.started')}</span>{dateOf(selected.mission.startedAt)}</div>
              <div><span>{$t('flights.ended')}</span>{selected.mission.endedAt ? dateOf(selected.mission.endedAt) : $t('flights.inProgress')}</div>
              <div><span>{$t('flights.duration')}</span>{duration(selected.mission)}</div>
              <div><span>{$t('flights.winner')}</span>{selected.mission.winner || '—'}</div>
              {#if selected.mission.abandoned}
                <div><span>{$t('flights.status')}</span>{$t('flights.abandoned')}</div>
              {/if}
            </div>
          {/if}

          {#if selected.debrief}
            <DebriefDetail debrief={selected.debrief} />
          {:else}
            <EmptyState title={$t('flights.noDebrief')} message="" icon="M6 2h9l5 5v15H6z" />
          {/if}
        {:else}
          <EmptyState title={$t('flights.select')} message="" />
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .flights {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    height: 100%;
    min-height: 0;
    padding: 1rem;
  }

  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.45rem;
    flex: none;
  }

  .search {
    flex: 1;
    min-width: 160px;
    max-width: 320px;
    padding: 0.38rem 0.55rem;
    color: var(--text);
    background: var(--bg-elevated);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
  }

  .search:focus {
    outline: none;
    border-color: var(--blue);
  }

  select {
    padding: 0.36rem 0.45rem;
    color: var(--text);
    background: var(--bg-elevated);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .count {
    color: var(--muted);
    font-size: 0.76rem;
    font-variant-numeric: tabular-nums;
  }

  .mine {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    color: var(--muted);
    font-size: 0.76rem;
    white-space: nowrap;
  }

  .mine input {
    accent-color: var(--blue);
  }

  .split {
    display: grid;
    grid-template-columns: minmax(240px, 320px) 1fr;
    gap: 0.6rem;
    flex: 1;
    min-height: 0;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0.15rem;
    overflow-y: auto;
    min-height: 0;
  }

  .list li {
    margin-bottom: 0.3rem;
  }

  .list button {
    display: flex;
    flex-direction: column;
    gap: 0.12rem;
    width: 100%;
    padding: 0.5rem 0.6rem;
    text-align: left;
    color: var(--text);
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    cursor: pointer;
  }

  .list button:hover {
    border-color: var(--border-strong);
  }

  .list button.selected {
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 12%, var(--panel));
  }

  .row-head {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.3rem;
    min-width: 0;
  }

  .name {
    font-size: 0.84rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    flex: 1;
  }

  .sub {
    color: var(--muted);
    font-size: 0.72rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .panel {
    min-height: 0;
    padding: 0.8rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }

  .detail {
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }

  .detail-head {
    display: flex;
    justify-content: space-between;
    gap: 0.6rem;
  }

  h2 {
    margin: 0;
    font-size: 1rem;
  }

  .meta {
    margin: 0.15rem 0 0;
    color: var(--muted);
    font-size: 0.75rem;
  }

  .meta-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
    gap: 0.4rem;
  }

  .meta-grid div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    padding: 0.45rem 0.55rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .meta-grid span {
    color: var(--muted);
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  @media (max-width: 760px) {
    .flights {
      height: auto;
      padding: 0.7rem;
    }

    .split {
      grid-template-columns: 1fr;
      flex: none;
    }

    .list {
      max-height: 42vh;
    }

    .detail {
      overflow: visible;
    }
  }
</style>
