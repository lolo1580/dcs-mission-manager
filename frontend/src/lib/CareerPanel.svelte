<script>
  /**
   * Career & statistics, merged into one tab:
   *
   *  - on top, the player's own logbook as DCS records it (MissionEditor/
   *    logbook.lua): rank, squadron, awards, flight hours and the per-airframe
   *    breakdown;
   *  - below, the statistics the manager aggregates from its recorded sessions:
   *    overview, pilots, weapons, airframes, balance and network, scoped to the
   *    whole career or a single mission.
   *
   * The two answer different questions — "what does DCS say I have done?" and
   * "what did the manager record?" — so they are stacked rather than blended,
   * the logbook first.
   */
  import { onMount } from 'svelte';
  import {
    careerPlayers,
    careerPlayer,
    careerCurrent,
    careerError,
    careerLoading,
    careerIndex,
    careerTotalHours,
    loadCareer,
    fmtHours,
    aircraftLabel,
  } from './career.js';
  import {
    statsOverview,
    statsPilots,
    statsWeapons,
    enginesByCategory,
    engineCategory,
    statTab,
    STAT_TABS,
    scopeMode,
    selectedMissionID,
    statsMissions,
    statsMissionsError,
    statsTrend,
    trendPilotUCID,
    trendLoading,
    trendError,
    statsError,
    statsLoading,
    statsNetwork,
    statsEnabled,
    loadStats,
    loadStatsMissions,
    loadTrend,
    fmtNum,
  } from './stats.js';
  import { t } from './i18n.js';
  import StatsTrend from './StatsTrend.svelte';

  const STAT_TAB_KEYS = {
    pilots: 'stats.pilots',
    weapons: 'stats.weapons',
    engines: 'stats.engines',
    balance: 'stats.balance',
    network: 'stats.network',
  };

  function statTabKey(id) {
    return STAT_TAB_KEYS[id] ?? id;
  }

  onMount(() => {
    loadCareer();
    loadStatsMissions();
    loadStats();
    loadTrend();
  });

  // The header's Refresh reloads both halves: they come from different files
  // (the logbook and the database) but belong to the same view.
  function refresh() {
    loadCareer();
    loadStatsMissions();
    loadStats();
    loadTrend();
  }

  async function onScopeChange(mode) {
    scopeMode.set(mode);
    if (mode === 'mission') await loadStatsMissions();
    loadStats();
  }

  function onMissionChange(event) {
    selectedMissionID.set(Number(event.currentTarget.value));
    loadStats();
  }

  function onTrendPilotChange(event) {
    trendPilotUCID.set(event.currentTarget.value);
    loadTrend();
  }

  function missionLabel(mission) {
    return `${new Date(mission.startedAt).toLocaleDateString()} · ${mission.name}`;
  }

  function trendPilots(pilots) {
    const seen = new Set();
    return pilots.filter((pilot) => {
      if (!pilot.ucid || seen.has(pilot.ucid)) return false;
      seen.add(pilot.ucid);
      return true;
    });
  }

  /** Reads a raw logbook aggregate value, which may be a number or a string. */
  function agg(key) {
    const v = $careerPlayer?.aggregate?.[key];
    if (v == null) return null;
    const n = typeof v === 'number' ? v : Number(v);
    return Number.isFinite(n) ? n : null;
  }

  /** The largest flight-hours value, so a bar can be scaled to it. */
  $: maxHours = ($careerPlayer?.aircraft ?? []).reduce((m, a) => Math.max(m, a.flightHours ?? 0), 0) || 1;

  function medal(i) {
    return i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : '';
  }
</script>

<section class="career">
  <header>
    <h2>{$t('tab.stats')}</h2>
    <button class="refresh" on:click={refresh} disabled={$careerLoading || $statsLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  <!-- ------------------------------------------------------------------ -->
  <!-- Logbook (DCS's own record)                                          -->
  <!-- ------------------------------------------------------------------ -->
  {#if $careerError}
    <p class="error">{$careerError}</p>
  {/if}

  {#if $careerPlayers.length === 0 && !$careerError}
    <p class="empty">{$t('career.none')}</p>
  {:else if $careerPlayer}
    {#if $careerPlayers.length > 1}
      <div class="profiles">
        {#each $careerPlayers as p, i (p.name + i)}
          <button class:active={$careerIndex === i} on:click={() => careerIndex.set(i)}>
            {p.name}
            {#if p.name === $careerCurrent}<span class="cur">★</span>{/if}
          </button>
        {/each}
      </div>
    {/if}

    <div class="identity">
      <h3>
        {$careerPlayer.name}
        {#if $careerPlayer.name === $careerCurrent}<span class="tag">{$t('career.current')}</span>{/if}
      </h3>
      <dl>
        {#if $careerPlayer.rank}<div><dt>{$t('career.rank')}</dt><dd>{$careerPlayer.rank}</dd></div>{/if}
        {#if $careerPlayer.squadron}<div><dt>{$t('career.squadron')}</dt><dd>{$careerPlayer.squadron}</dd></div>{/if}
        <div><dt>{$t('career.flightHours')}</dt><dd>{fmtHours($careerTotalHours)}</dd></div>
        {#if agg('missionsCount') != null}<div><dt>{$t('career.missions')}</dt><dd>{fmtNum(agg('missionsCount'), 0)}</dd></div>{/if}
        {#if agg('landings') != null}<div><dt>{$t('career.landings')}</dt><dd>{fmtNum(agg('landings'), 0)}</dd></div>{/if}
        {#if agg('totalScore') != null}<div><dt>{$t('career.score')}</dt><dd>{fmtNum(agg('totalScore'), 0)}</dd></div>{/if}
        {#if $careerPlayer.awards?.length}
          <div><dt>{$t('career.awards')}</dt><dd>{$careerPlayer.awards.length}</dd></div>
        {/if}
        {#if $careerPlayer.invulnerable}<div><dt>{$t('career.mode')}</dt><dd>{$t('career.invulnerable')}</dd></div>{/if}
      </dl>
    </div>

    <div class="block">
      <h3>
        {$t('career.airframes')}
        <span class="count">{$careerPlayer.aircraft.length}</span>
      </h3>
      {#if $careerPlayer.aircraft.length === 0}
        <p class="empty">{$t('career.noAirframe')}</p>
      {:else}
        <table class="airframes">
          <thead>
            <tr>
              <th>{$t('career.col.aircraft')}</th>
              <th>{$t('career.col.hours')}</th>
              <th>{$t('career.col.landings')}</th>
              <th>{$t('career.col.deaths')}</th>
              <th>{$t('career.col.ejections')}</th>
              <th>{$t('career.col.aa')}</th>
              <th>{$t('career.col.ag')}</th>
            </tr>
          </thead>
          <tbody>
            {#each $careerPlayer.aircraft as a (a.type)}
              <tr>
                <td class="name" title={a.type}>{aircraftLabel(a.type)}</td>
                <td class="num hours">
                  <span class="bar" style="width:{Math.max(2, Math.round((a.flightHours / maxHours) * 100))}%"></span>
                  <span class="val">{fmtHours(a.flightHours)}</span>
                </td>
                <td class="num">{fmtNum(a.landings, 0)}</td>
                <td class="num">{fmtNum(a.deaths, 0)}</td>
                <td class="num">{fmtNum(a.ejections, 0)}</td>
                <td class="num">{fmtNum(a.aaKills, 0)}</td>
                <td class="num">{fmtNum(a.agKills, 0)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}

  <!-- ------------------------------------------------------------------ -->
  <!-- Statistics (the manager's own record)                               -->
  <!-- ------------------------------------------------------------------ -->
  <div class="stats">
    <div class="stats-head">
      <h3>{$t('stats.title')}</h3>
      <div class="scope">
        <button class:active={$scopeMode === 'career'} on:click={() => onScopeChange('career')}>
          {$t('stats.career')}
        </button>
        <button class:active={$scopeMode === 'mission'} on:click={() => onScopeChange('mission')}>
          {$t('stats.mission')}
        </button>
      </div>
    </div>

    {#if $scopeMode === 'mission'}
      <div class="mission-picker">
        <label for="stats-mission">{$t('stats.chooseMission')}</label>
        <select id="stats-mission" value={$selectedMissionID} on:change={onMissionChange} disabled={$statsMissions.length === 0}>
          {#each $statsMissions as mission (mission.id)}
            <option value={mission.id}>{missionLabel(mission)}</option>
          {/each}
        </select>
        {#if $statsMissions.length === 0}<span class="muted">{$t('stats.noMission')}</span>{/if}
        {#if $statsMissionsError}<span class="error">{$statsMissionsError}</span>{/if}
      </div>
    {/if}

    {#if $statsError}
      <p class="error">{$statsError}</p>
    {/if}

    {#if !$statsEnabled}
      <p class="empty">{$t('stats.disabled')}</p>
    {:else}
      {#if $statsOverview}
        {@const o = $statsOverview}
        <div class="cards">
          <div class="kpi k-missions"><span class="v">{o.missions}</span><span class="k">{$t('stats.missions')}</span></div>
          <div class="kpi k-pilots"><span class="v">{o.players}</span><span class="k">{$t('stats.pilots')}</span></div>
          <div class="kpi k-kills"><span class="v">{o.kills}</span><span class="k">{$t('events.kills')}</span></div>
          <div class="kpi k-deaths"><span class="v">{o.deaths}</span><span class="k">{$t('stats.deaths')}</span></div>
          <div class="kpi k-crash"><span class="v">{o.crashes}</span><span class="k">{$t('stats.crashes')}</span></div>
          <div class="kpi k-eject"><span class="v">{o.ejections}</span><span class="k">{$t('stats.ejections')}</span></div>
          <div class="kpi k-ff" class:warn={o.friendlyFire > 0}><span class="v">{o.friendlyFire}</span><span class="k">{$t('events.friendlyFire')}</span></div>
        </div>
      {/if}

      {#if $scopeMode === 'career'}
        <div class="trend-block">
          <div class="trend-head">
            <h3>{$t('stats.trendTitle')}</h3>
            <label>{$t('stats.trendPilot')}
              <select value={$trendPilotUCID} on:change={onTrendPilotChange}>
                <option value="">{$t('stats.allPilots')}</option>
                {#each trendPilots($statsPilots) as pilot (pilot.ucid)}
                  <option value={pilot.ucid}>{pilot.name}</option>
                {/each}
              </select>
            </label>
          </div>
          {#if $trendError}<p class="error">{$trendError}</p>{/if}
          {#if $trendLoading}<p class="muted">{$t('stats.loadingTrend')}</p>{:else}<StatsTrend points={$statsTrend} />{/if}
        </div>
      {/if}

      <nav class="subtabs">
        {#each STAT_TABS as tab (tab.id)}
          <button class:active={$statTab === tab.id} on:click={() => statTab.set(tab.id)}>{$t(statTabKey(tab.id))}</button>
        {/each}
      </nav>

      <div class="panel">
      {#if $statTab === 'pilots'}
        {#if $statsPilots.length === 0}
          <p class="empty">{$t('stats.noPilot')}</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th></th><th>{$t('players.pilot')}</th><th>{$t('players.score')}</th><th>{$t('stats.killsCol')}</th><th>{$t('stats.deaths')}</th>
                <th>K/D</th><th>{$t('stats.landings')}</th><th>{$t('stats.ejections')}</th><th>{$t('stats.crashes')}</th><th>{$t('stats.friendlyFire')}</th><th>{$t('stats.ping')}</th>
              </tr>
            </thead>
            <tbody>
              {#each $statsPilots as p, i}
                <tr>
                  <td class="medal">{medal(i)}</td>
                  <td class="name" title={p.ucid}>{p.name}</td>
                  <td class="num">{p.score}</td>
                  <td class="num">{p.killsAir}/{p.killsCar}/{p.killsShip}</td>
                  <td class="num">{p.deaths}</td>
                  <td class="num">{fmtNum(p.kd, 2)}</td>
                  <td class="num">{p.landings}</td>
                  <td class="num">{p.ejections}</td>
                  <td class="num">{p.crashes}</td>
                  <td class="num" class:warn={p.friendlyFire > 0}>{p.friendlyFire}</td>
                  <td class="num">{Math.round(p.avgPing)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {:else if $statTab === 'weapons'}
        {#if $statsWeapons.length === 0}
          <p class="empty">{$t('stats.noWeapon')}</p>
        {:else}
          <table>
            <thead>
              <tr><th>{$t('stats.weapon')}</th><th>{$t('stats.killsCol')}</th><th>{$t('stats.friendlyFire')}</th><th>{$t('stats.targets')}</th></tr>
            </thead>
            <tbody>
              {#each $statsWeapons as w (w.weapon)}
                <tr>
                  <td class="name">{w.weapon}</td>
                  <td class="num">{w.kills}</td>
                  <td class="num" class:warn={w.friendlyFire > 0}>{w.friendlyFire}</td>
                  <td class="targets">
                    {#each Object.entries(w.victimsByType ?? {}).slice(0, 4) as [type, n] (type)}
                      <span class="tag">{type} ×{n}</span>
                    {/each}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {:else if $statTab === 'engines'}
        <div class="cats">
          {#each ['plane', 'heli', 'ground', 'ship', 'structure', 'other'] as c (c)}
            <button class:active={$engineCategory === c} on:click={() => engineCategory.set(c)}>
              {$t('category.' + c)}
            </button>
          {/each}
        </div>
        {#if $enginesByCategory.length === 0}
          <p class="empty">{$t('stats.noEngine')}</p>
        {:else}
          <table>
            <thead>
              <tr><th>{$t('stats.type')}</th><th>{$t('stats.killsCol')}</th><th>{$t('stats.losses')}</th><th>{$t('stats.missions')}</th><th>K/D</th></tr>
            </thead>
            <tbody>
              {#each $enginesByCategory as e (e.typeId)}
                <tr>
                  <td class="name">{e.typeId}</td>
                  <td class="num">{e.kills}</td>
                  <td class="num">{e.deaths}</td>
                  <td class="num">{e.missions}</td>
                  <td class="num">{fmtNum(e.kd, 2)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {:else if $statTab === 'balance'}
        {#if !$statsOverview?.coalitions?.length}
          <p class="empty">{$t('stats.noCoalition')}</p>
        {:else}
          {#each $statsOverview.coalitions as c (c.coalition)}
            {@const total = Math.max(...$statsOverview.coalitions.map((x) => x.score), 1)}
            <div class="balance-row">
              <span class="side {c.coalition}">{$t('coalition.' + c.coalition)}</span>
              <div class="bar">
                <div
                  class="fill {c.coalition}"
                  style="width:{(c.score / total) * 100}%"
                ></div>
              </div>
              <span class="num">{c.score} {$t('stats.points')}</span>
              <span class="num">{c.kills} {$t('stats.killsCol')}</span>
              <span class="num">{c.players} {$t('players.title')}</span>
            </div>
          {/each}
        {/if}
      {:else if $statTab === 'network'}
        {#if $statsNetwork.length === 0}
          <p class="empty">{$t('stats.noNetwork')}</p>
        {:else}
          <table>
            <thead>
              <tr><th>{$t('players.pilot')}</th><th>{$t('stats.samples')}</th><th>{$t('stats.avgPing')}</th><th>{$t('stats.maxPing')}</th></tr>
            </thead>
            <tbody>
              {#each $statsNetwork as n (n.name)}
                <tr>
                  <td class="name">{n.name}</td>
                  <td class="num">{n.samples}</td>
                  <td class="num" class:warn={n.avgPing > 250}>{Math.round(n.avgPing)} ms</td>
                  <td class="num" class:warn={n.maxPing > 400}>{n.maxPing} ms</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
      </div>
    {/if}
  </div>
</section>

<style>
  .career {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    /* A single scroll for the whole view: the logbook grows to its content and
       the statistics tables follow, instead of each half scrolling on its own. */
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    margin-bottom: 0.7rem;
  }

  h2 {
    margin: 0;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
  }

  .refresh {
    padding: 0.28rem 0.6rem;
    font-size: 0.76rem;
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

  .refresh:disabled {
    opacity: 0.5;
  }

  /* ---- Logbook ---- */

  .profiles {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    margin-bottom: 0.7rem;
  }

  .profiles button {
    padding: 0.25rem 0.6rem;
    font-size: 0.76rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    cursor: pointer;
  }

  .profiles button.active {
    color: var(--text);
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
  }

  .cur {
    color: var(--green);
  }

  .identity {
    margin-bottom: 1.1rem;
  }

  .identity h3 {
    margin: 0 0 0.5rem;
    font-size: 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .tag {
    padding: 0.05rem 0.4rem;
    font-size: 0.66rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--green);
    border: 1px solid color-mix(in srgb, var(--green) 45%, var(--border));
    border-radius: 999px;
  }

  dl {
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 0.3rem 1.2rem;
    font-size: 0.82rem;
    max-width: 640px;
  }

  dl div {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.2rem 0.45rem;
    border-radius: 6px;
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

  .block {
    margin-bottom: 1rem;
  }

  .count {
    color: var(--muted);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
  }

  th {
    text-align: right;
    font-weight: 500;
    color: var(--muted);
    padding: 0.3rem 0.4rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.7rem;
  }

  th:nth-child(1),
  td:nth-child(1) {
    text-align: left;
  }

  td {
    padding: 0.28rem 0.4rem;
    border-top: 1px solid var(--border);
    font-variant-numeric: tabular-nums;
  }

  td.num {
    text-align: right;
  }

  .name {
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Hours column: a bar behind the value, scaled to the player's top airframe. */
  td.hours {
    position: relative;
    min-width: 9rem;
  }

  td.hours .bar {
    position: absolute;
    left: 0;
    top: 50%;
    transform: translateY(-50%);
    height: 1.05rem;
    background: color-mix(in srgb, var(--blue) 30%, transparent);
    border-radius: 4px;
  }

  td.hours .val {
    position: relative;
  }

  /* ---- Statistics ---- */

  .stats {
    border-top: 1px solid var(--border);
    padding-top: 0.9rem;
  }

  .stats-head {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    margin-bottom: 0.7rem;
  }

  .stats-head h3 {
    margin: 0;
  }

  .scope {
    display: inline-flex;
    gap: 0.15rem;
    padding: 0.12rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 7px;
  }

  .scope button {
    padding: 0.25rem 0.6rem;
    font-size: 0.76rem;
    color: var(--muted);
    background: transparent;
    border: none;
    border-radius: 5px;
    cursor: pointer;
  }

  .scope button.active {
    color: var(--text);
    background: var(--panel);
  }

  .mission-picker, .trend-head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.8rem;
    font-size: 0.78rem;
  }

  .mission-picker select, .trend-head select {
    max-width: min(100%, 420px);
    padding: 0.3rem 0.5rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
  }

  .trend-block { margin: 0 0 1rem; }
  .trend-head { justify-content: space-between; margin-bottom: 0.45rem; }
  .trend-head h3 { margin: 0; }
  .trend-head label { display: flex; align-items: center; gap: 0.5rem; }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(104px, 1fr));
    gap: 0.55rem;
    margin-bottom: 0.85rem;
  }

  /* KPI cards, per the redesign mockup: a coloured accent stripe, a large value
     and a small uppercase label. The stripe colour distinguishes the metrics at a
     glance (missions, pilots, kills…) without a legend. */
  .cards .kpi {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.55rem 0.7rem 0.55rem 0.85rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 9px;
    overflow: hidden;
  }

  .cards .kpi::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 3px;
    background: var(--accent, var(--blue));
  }

  .cards .kpi .k {
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .cards .kpi .v {
    font-size: 1.35rem;
    font-weight: 650;
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .cards .k-missions { --accent: var(--blue); }
  .cards .k-pilots { --accent: #a371f7; }
  .cards .k-kills { --accent: var(--green); }
  .cards .k-deaths { --accent: #f0883e; }
  .cards .k-crash { --accent: #db6d28; }
  .cards .k-eject { --accent: #58a6ff; }
  .cards .k-ff { --accent: #f0b429; }

  .cards .kpi.warn {
    border-color: color-mix(in srgb, #f0b429 55%, var(--border));
  }

  .cards .kpi.warn .v {
    color: #f0b429;
  }

  .subtabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin-bottom: 0.6rem;
  }

  .subtabs button {
    padding: 0.28rem 0.65rem;
    font-size: 0.78rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
  }

  .subtabs button:hover {
    color: var(--text);
  }

  .subtabs button.active {
    color: var(--text);
    background: var(--bg);
    border-color: var(--border);
  }

  .panel {
    min-height: 0;
  }

  .panel th:not(:nth-child(1)):not(:nth-child(2)),
  .panel td.num {
    text-align: right;
  }

  .panel th:nth-child(1),
  .panel th:nth-child(2),
  .panel td:nth-child(1),
  .panel td:nth-child(2) {
    text-align: left;
  }

  .panel td.warn {
    color: #f0b429;
  }

  .medal {
    width: 22px;
  }

  .targets {
    text-align: left;
  }

  .tag-chip {
    display: inline-block;
  }

  .panel .tag {
    display: inline-block;
    margin: 0 0.2rem 0.15rem 0;
    padding: 0.05rem 0.35rem;
    font-size: 0.68rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    text-transform: none;
    letter-spacing: 0;
  }

  .cats {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin-bottom: 0.5rem;
  }

  .cats button {
    padding: 0.22rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    cursor: pointer;
  }

  .cats button.active {
    color: var(--text);
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
  }

  .balance-row {
    display: grid;
    grid-template-columns: 70px 1fr auto auto auto;
    align-items: center;
    gap: 0.7rem;
    padding: 0.45rem 0;
    border-top: 1px solid var(--border);
    font-size: 0.8rem;
  }

  .side {
    font-weight: 600;
  }

  .side.red {
    color: #ff4d4d;
  }

  .side.blue {
    color: #3d7dff;
  }

  .bar {
    height: 10px;
    background: var(--bg);
    border-radius: 999px;
    overflow: hidden;
  }

  .fill {
    height: 100%;
    border-radius: 999px;
  }

  .fill.red {
    background: #ff4d4d;
  }

  .fill.blue {
    background: #3d7dff;
  }

  .fill.spectator {
    background: #9aa4b2;
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
