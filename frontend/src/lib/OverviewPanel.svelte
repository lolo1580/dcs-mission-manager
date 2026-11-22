<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { navigate } from './router.js';
  import { connected, feedStatus, mission } from './units.js';
  import { fmtTime } from './debriefs.js';
  import { checklists, loadChecklists } from './checklists.js';
  import Card from './ui/Card.svelte';
  import StatCard from './ui/StatCard.svelte';
  import Badge from './ui/Badge.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Skeleton from './ui/Skeleton.svelte';

  let overview = null;
  let career = null;
  let missions = [];
  let debriefs = [];
  let plans = [];
  let modules = { installed: 0, total: 0 };
  let health = null;
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
      const [ov, ca, mi, de, pl, mo, he] = await Promise.all([
        getJSON('/api/stats/overview'),
        getJSON('/api/career'),
        getJSON('/api/history/missions?limit=50'),
        getJSON('/api/debriefs'),
        getJSON('/api/flightplans'),
        getJSON('/api/modules'),
        getJSON('/api/health'),
      ]);
      overview = ov?.enabled === false ? null : ov;
      career = ca;
      missions = (mi.missions ?? []).filter((mission) => (mission.source ?? 'live') !== 'test');
      debriefs = (de.debriefs ?? []).slice().sort((a, b) => (b.createdAt ?? 0) - (a.createdAt ?? 0));
      plans = pl.plans ?? [];
      modules = { installed: mo.installed ?? 0, total: mo.total ?? 0 };
      health = he;
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

  $: player = (() => {
    const players = career?.players ?? [];
    return players.find((p) => p.name === career?.currentPlayer) ?? players[0] ?? null;
  })();
  $: hours = player?.totalFlightHours ?? 0;
  $: kills = overview?.kills ?? 0;
  $: deaths = overview?.deaths ?? 0;
  $: kd = deaths > 0 ? (kills / deaths).toFixed(2) : kills > 0 ? '∞' : '—';
  $: latestMission = missions[0] ?? null;
  $: latestDebrief = latestMission
    ? debriefs.find((item) => item.missionId === latestMission.id) ?? null
    : debriefs[0] ?? null;
  $: latest = latestMission ?? latestDebrief;
  $: latestTheatre = latestMission?.theatre || latestDebrief?.theatre || '';
  $: latestDate = latestMission?.startedAt || latestDebrief?.createdAt || 0;
  $: recentChecklists = $checklists.slice(0, 3);
  $: recentPlans = plans.slice(0, 3);

  function dateOf(ms) {
    if (!ms) return '—';
    return new Date(ms).toLocaleString(undefined, {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function summary(debrief) {
    return debrief?.parsed?.summary ?? {};
  }

  function feedKey() {
    if ($feedStatus === 'waiting') return 'overview.feedWaiting';
    if ($connected) return 'overview.feedLive';
    return 'overview.feedOffline';
  }
</script>

<div class="overview">
  {#if loading}
    <div class="stats-grid">
      {#each Array(4) as _}<div class="skeleton-card"><Skeleton lines={2} /></div>{/each}
    </div>
    <Card><Skeleton lines={5} /></Card>
  {:else if error}
    <Card><EmptyState title={$t('overview.error', { detail: '' })} message={error} /></Card>
  {:else}
    <section class="stats-grid">
      <StatCard label={$t('overview.hours')} value={hours ? `${Math.round(hours)} h` : '0 h'} hint={$t('overview.hoursHint')} tone="blue" icon="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 7v5l3 2" />
      <StatCard label={$t('overview.missions')} value={overview?.missions ?? 0} hint={$t('overview.missionsHint')} tone="green" icon="M3 12h18M12 3v18" />
      <StatCard label={$t('overview.kills')} value={kills} hint={$t('overview.killsHint')} tone="red" icon="M4 4l16 16M20 4L4 20" />
      <StatCard label={$t('overview.kd')} value={kd} hint={$t('overview.kdHint', { deaths })} tone="amber" icon="M4 20V10M10 20V4M16 20v-7M22 20H2" />
    </section>

    <div class="grid">
      <Card title={$t('overview.lastFlight')}>
        {#if latest}
          <div class="last">
            <div class="last-head">
              <strong>{latestMission?.name || latestDebrief?.mission || $t('debriefs.label', { id: latestDebrief?.id ?? '' })}</strong>
              {#if latestTheatre}<Badge tone="blue">{latestTheatre}</Badge>{/if}
            </div>
            <p class="meta">{dateOf(latestDate)}</p>
            {#if latestDebrief}
              <div class="last-stats">
                <span><b>{summary(latestDebrief).kills ?? 0}</b>{$t('debriefs.kills')}</span>
                <span><b>{summary(latestDebrief).landings ?? 0}</b>{$t('debriefs.landings')}</span>
                <span><b>{fmtTime(latestDebrief.parsed?.missionTime ?? 0)}</b>{$t('debriefs.duration')}</span>
              </div>
            {:else}
              <p class="meta">{$t('flights.noDebrief')}</p>
            {/if}
            <button class="btn" on:click={() => navigate('flights')}>{$t('overview.viewFlights')}</button>
          </div>
        {:else}
          <EmptyState title={$t('overview.noFlight')} message="" icon="M3 12h18M12 3v18" />
        {/if}
      </Card>

      <Card title={$t('overview.dcsStatus')}>
        <div class="status-list">
          <div class="status-row">
            <span class="dot" class:on={$connected}></span>
            <span>{$t(feedKey())}</span>
            <Badge tone={$connected ? 'green' : 'neutral'}>{$connected ? 'DCS' : '—'}</Badge>
          </div>
          <p class="meta">
            {$mission ? $t('overview.inMission', { name: $mission.name }) : $t('overview.noMission')}
          </p>
          <p class="meta">{$t('overview.modules', { installed: modules.installed, total: modules.total })}</p>
          <button class="btn ghost" on:click={() => navigate('settings')}>{$t('overview.openSettings')}</button>
        </div>
      </Card>

      <Card title={$t('overview.quickChecklist')} subtitle={$t('overview.quickChecklistHint')}>
        <button slot="actions" class="btn ghost" on:click={() => navigate('checklists')}>{$t('overview.openChecklists')}</button>
        {#if recentChecklists.length === 0}
          <p class="meta">{$t('overview.noChecklist')}</p>
        {:else}
          <ul class="mini-list">
            {#each recentChecklists as checklist (checklist.uid)}
              <li>
                <span>{checklist.name}</span>
                <Badge>{$t(`phase.${checklist.phase}`)}</Badge>
              </li>
            {/each}
          </ul>
        {/if}
      </Card>

      <Card title={$t('overview.plans')}>
        <button slot="actions" class="btn ghost" on:click={() => navigate('planner')}>{$t('overview.openPlanner')}</button>
        {#if recentPlans.length === 0}
          <p class="meta">{$t('overview.noPlans')}</p>
        {:else}
          <ul class="mini-list">
            {#each recentPlans as plan (plan.uid || plan.id)}
              <li><span>{plan.name}</span><Badge>{plan.departure} → {plan.arrival}</Badge></li>
            {/each}
          </ul>
        {/if}
      </Card>

      <Card title={$t('overview.storage')}>
        <div class="status-row">
          <Badge tone="amber">{$t('overview.storageLocal')}</Badge>
          {#if health?.version}<span class="meta">v{health.version}</span>{/if}
        </div>
        <p class="meta storage-hint">{$t('overview.storageHint')}</p>
      </Card>
    </div>
  {/if}
</div>

<style>
  .overview {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
    padding: 1rem;
    max-width: 1180px;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 0.6rem;
  }

  .skeleton-card {
    padding: 0.8rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 0.6rem;
    align-items: start;
  }

  .last {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    align-items: flex-start;
  }

  .last-head {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.45rem;
  }

  .last-head strong {
    font-size: 0.92rem;
  }

  .meta {
    margin: 0;
    color: var(--muted);
    font-size: 0.76rem;
    line-height: 1.45;
  }

  .last-stats {
    display: flex;
    flex-wrap: wrap;
    gap: 0.9rem;
    color: var(--muted);
    font-size: 0.72rem;
  }

  .last-stats span {
    display: flex;
    flex-direction: column;
    gap: 0.05rem;
  }

  .last-stats b {
    color: var(--text);
    font-size: 0.95rem;
    font-variant-numeric: tabular-nums;
  }

  .status-list {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    align-items: flex-start;
  }

  .status-row {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.8rem;
    width: 100%;
  }

  .status-row > :global(.badge) {
    margin-left: auto;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--red);
    flex: none;
  }

  .dot.on {
    background: var(--green);
    box-shadow: 0 0 0 3px var(--green-soft);
  }

  .mini-list {
    list-style: none;
    margin: 0.2rem 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .mini-list li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.35rem 0.5rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }

  .mini-list span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .storage-hint {
    margin-top: 0.5rem;
  }

  @media (max-width: 700px) {
    .overview {
      padding: 0.7rem;
    }
  }
</style>
