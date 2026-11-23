<script>
  import { onMount } from 'svelte';
  import { navigate } from './router.js';
  import { t } from './i18n.js';
  import { loadChecklists } from './checklists.js';
  import Card from './ui/Card.svelte';
  import StatCard from './ui/StatCard.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Skeleton from './ui/Skeleton.svelte';

  let overview = null;
  let pilots = [];
  let missions = [];
  let debriefs = [];
  let checklists = 0;
  let loading = true;
  let error = '';

  async function getJSON(path) {
    const response = await fetch(path);
    if (!response.ok) throw new Error(`${response.status}`);
    return response.json();
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [ov, pi, mi, de, cl] = await Promise.all([
        getJSON('/api/stats/overview'),
        getJSON('/api/stats/pilots'),
        getJSON('/api/history/missions?limit=100&withData=1&source=live'),
        getJSON('/api/debriefs?limit=100'),
        getJSON('/api/checklists'),
      ]);
      overview = ov?.enabled === false ? null : ov;
      pilots = (pi.pilots ?? []).slice().sort((a, b) => b.kills - a.kills).slice(0, 8);
      missions = (mi.missions ?? []).slice().sort((a, b) => (b.startedAt ?? 0) - (a.startedAt ?? 0));
      debriefs = de.debriefs ?? [];
      checklists = (cl.checklists ?? []).length;
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    load();
    loadChecklists();
  });

  $: byMission = new Map(debriefs.filter((d) => d.missionId).map((d) => [d.missionId, d]));
  $: recent = missions.slice(0, 8).map((mission) => ({
    ...mission,
    debrief: byMission.get(mission.id) ?? null,
  }));
  // Flight time comes from the statistics overview, which uses the same mission
  // scope as the mission counter. Summing the history list here used to mix
  // "missions with data" and every empty session, showing thousands of hours.
  $: hours = (() => {
    const h = (overview?.flightSeconds ?? 0) / 3600;
    return h >= 10 ? Math.round(h) : Math.round(h * 10) / 10;
  })();

  function dateOf(ms) {
    if (!ms) return '—';
    return new Date(ms).toLocaleString(undefined, {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function duration(mission) {
    if (!mission?.startedAt) return '—';
    const end = mission.endedAt || mission.lastActivityAt || mission.startedAt;
    const total = Math.max(0, Math.round((end - mission.startedAt) / 60000));
    return total >= 60 ? `${Math.floor(total / 60)} h ${String(total % 60).padStart(2, '0')}` : `${total} min`;
  }
</script>

<div class="squadron">
  {#if loading}
    <div class="grid">
      {#each Array(4) as _}<div class="skeleton"><Skeleton lines={2} /></div>{/each}
    </div>
    <Card><Skeleton lines={8} /></Card>
  {:else if error}
    <Card><EmptyState title={$t('squadron.error', { detail: '' })} message={error} /></Card>
  {:else}
    <section class="grid">
      <StatCard label={$t('squadron.missions')} value={overview?.missions ?? 0} hint={`${hours} h`} tone="blue" icon="M3 12h18M12 3v18" />
      <StatCard label={$t('squadron.pilots')} value={overview?.players ?? 0} hint={`${checklists} checklists`} tone="green" icon="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" />
      <StatCard label={$t('squadron.kills')} value={overview?.kills ?? 0} hint={`${overview?.crashes ?? 0} crashes`} tone="red" icon="M4 4l16 16M20 4L4 20" />
      <StatCard label={$t('squadron.deaths')} value={overview?.deaths ?? 0} hint={`${overview?.ejections ?? 0} ejections`} tone="amber" icon="M12 3v18M5 10l7 7 7-7" />
    </section>

    <div class="columns">
      <Card title={$t('squadron.recentFlights')}>
        <button slot="actions" class="btn ghost" on:click={() => navigate('flights')}>{$t('squadron.viewAll')}</button>
        {#if recent.length === 0}
          <EmptyState title={$t('squadron.noFlights')} message="" icon="M3 12h18M12 3v18" />
        {:else}
          <ul class="flight-list">
            {#each recent as mission (mission.id)}
              <li>
                <div>
                  <strong>{mission.name}</strong>
                  <span>{dateOf(mission.startedAt)} · {mission.theatre || '—'} · {duration(mission)}</span>
                </div>
                <span class="badges">
                  {#if mission.abandoned}<Badge tone="amber">{$t('flights.abandoned')}</Badge>{/if}
                  {#if mission.debrief}
                    <Badge tone="blue">{$t('flights.debrief')}</Badge>
                  {:else}
                    <Badge>{$t('flights.noDebrief')}</Badge>
                  {/if}
                </span>
              </li>
            {/each}
          </ul>
        {/if}
      </Card>

      <Card title={$t('squadron.topPilots')}>
        {#if pilots.length === 0}
          <EmptyState title={$t('squadron.noFlights')} message="" />
        {:else}
          <ul class="pilot-list">
            {#each pilots as pilot (pilot.ucid + pilot.name)}
              <li>
                <span class="rank">{pilot.kills}</span>
                <div>
                  <strong>{pilot.name}</strong>
                  <span>{pilot.missions} missions · {pilot.landings} landings</span>
                </div>
                <span class="kd">{pilot.kd?.toFixed?.(2) ?? pilot.kd}</span>
              </li>
            {/each}
          </ul>
        {/if}
      </Card>
    </div>
  {/if}
</div>

<style>
  .squadron {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
    width: 100%;
    max-width: var(--content-max);
    margin: 0 auto;
    padding: 1rem;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 0.6rem;
  }

  .skeleton {
    padding: 0.8rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }

  .columns {
    display: grid;
    grid-template-columns: 1.4fr 1fr;
    gap: 0.6rem;
    align-items: start;
  }

  .flight-list,
  .pilot-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .flight-list li,
  .pilot-list li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    padding: 0.45rem 0.55rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .badges {
    display: inline-flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 0.35rem;
  }

  .flight-list div,
  .pilot-list div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
  }

  .flight-list strong,
  .pilot-list strong {
    font-size: 0.82rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .flight-list span,
  .pilot-list span {
    color: var(--muted);
    font-size: 0.72rem;
  }

  .rank {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    flex: none;
    border-radius: 50%;
    background: var(--red-soft);
    color: var(--red);
    font-weight: 700;
    font-size: 0.78rem;
  }

  .kd {
    color: var(--blue) !important;
    font-variant-numeric: tabular-nums;
    font-weight: 650;
  }

  @media (max-width: 820px) {
    .columns {
      grid-template-columns: 1fr;
    }

    .squadron {
      padding: 0.7rem;
    }
  }
</style>
