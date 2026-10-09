<script>
  import { onMount } from 'svelte';
  import {
    careerPlayers, careerPlayer, careerCurrent, careerError, careerLoading,
    careerIndex, careerTotalHours, careerPeriod, careerDateFrom, careerDateTo,
    loadCareer, fmtHours, aircraftLabel,
  } from './career.js';
  import { periodParams, fmtNum } from './stats.js';
  import { t } from './i18n.js';

  let pilots = [];
  let pilotLoading = false;
  let pilotError = '';
  let pilotDisabled = false;
  let selectedPilot = '';
  let pilotRequest = 0;
  let insights = null;
  let insightsLoading = false;
  let insightsError = '';
  let insightsRequest = 0;

  function pilotKey(pilot) { return JSON.stringify([pilot.ucid, pilot.name]); }
  $: periodPilot = pilots.find((pilot) => pilotKey(pilot) === selectedPilot)
    ?? pilots.find((pilot) => pilot.name === $careerPlayer?.name)
    ?? null;
  $: invalidDates = $careerPeriod === 'custom'
    && (!$careerDateFrom || !$careerDateTo || $careerDateFrom > $careerDateTo);

  async function loadPilots() {
    const request = ++pilotRequest;
    pilotLoading = true;
    pilotError = '';
    pilots = [];
    if (invalidDates) {
      pilotLoading = false;
      return;
    }
    try {
      const params = new URLSearchParams({ scope: 'career', ...periodParams($careerPeriod, $careerDateFrom, $careerDateTo) });
      const response = await fetch(`/api/stats/pilots?${params}`);
      if (!response.ok) throw new Error(`${response.status}`);
      const body = await response.json();
      if (request !== pilotRequest) return;
      pilotDisabled = body.enabled === false;
      pilots = pilotDisabled ? [] : (body.pilots ?? []);
    } catch (error) {
      if (request === pilotRequest) pilotError = error.message;
    } finally {
      if (request === pilotRequest) pilotLoading = false;
    }
  }

  async function loadInsights(query) {
    const request = ++insightsRequest;
    insightsLoading = true;
    insightsError = '';
    insights = null;
    try {
      const response = await fetch(`/api/career/insights?${query}`);
      if (!response.ok) throw new Error(`${response.status}`);
      const body = await response.json();
      if (request !== insightsRequest) return;
      insights = body.enabled === false ? null : body;
    } catch (error) {
      if (request === insightsRequest) insightsError = error.message;
    } finally {
      if (request === insightsRequest) insightsLoading = false;
    }
  }

  $: insightQuery = periodPilot && !invalidDates
    ? new URLSearchParams({ ucid: periodPilot.ucid || '', name: periodPilot.name,
      ...periodParams($careerPeriod, $careerDateFrom, $careerDateTo) }).toString()
    : '';
  $: if (insightQuery) loadInsights(insightQuery);
  else { insightsRequest += 1; insights = null; insightsLoading = false; }

  onMount(() => { loadCareer(); loadPilots(); });

  function refresh() { loadCareer(); loadPilots(); }

  function todayLocal() {
    const now = new Date();
    const two = (number) => String(number).padStart(2, '0');
    return `${now.getFullYear()}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
  }

  function onPeriodChange(event) {
    const period = event.currentTarget.value;
    if (period === 'custom' && (!$careerDateFrom || !$careerDateTo)) {
      careerDateFrom.set(todayLocal());
      careerDateTo.set(todayLocal());
    }
    careerPeriod.set(period);
    selectedPilot = '';
    loadPilots();
  }

  function onDateChange() { loadPilots(); }

  function agg(key) {
    const value = $careerPlayer?.aggregate?.[key];
    if (value == null) return null;
    const number = typeof value === 'number' ? value : Number(value);
    return Number.isFinite(number) ? number : null;
  }

  $: maxHours = ($careerPlayer?.aircraft ?? []).reduce((max, aircraft) => Math.max(max, aircraft.flightHours ?? 0), 0) || 1;
  $: mapMax = Math.max(1, ...(insights?.maps ?? []).map((place) => place.missions));
  $: countryMax = Math.max(1, ...(insights?.countries ?? []).map((place) => place.missions));
  $: playMinutes = (insights?.days ?? []).reduce((total, day) => total + day.minutes, 0);

  function heatLevel(minutes) {
    if (!minutes) return 0;
    if (minutes < 60) return 1;
    if (minutes < 120) return 2;
    if (minutes < 240) return 3;
    return 4;
  }

  function formatMinutes(minutes) { return minutes === 0 ? '0 h' : minutes < 60 ? `${minutes} min` : `${(minutes / 60).toFixed(1)} h`; }
</script>

<section class="career">
  <header>
    <h2>{$t('tab.career')}</h2>
    <button class="refresh" on:click={refresh} disabled={$careerLoading || pilotLoading || insightsLoading}>{$t('stats.refresh')}</button>
  </header>

  <div class="period-picker">
    <label for="career-period">{$t('stats.period')}</label>
    <select id="career-period" value={$careerPeriod} on:change={onPeriodChange}>
      {#each ['day', 'week', 'month', 'sixMonths', 'year', 'total', 'custom'] as period}
        <option value={period}>{$t('stats.period.' + period)}</option>
      {/each}
    </select>
    {#if $careerPeriod === 'custom'}
      <label for="career-from">{$t('stats.from')}</label>
      <input id="career-from" type="date" bind:value={$careerDateFrom} on:change={onDateChange} />
      <label for="career-to">{$t('stats.to')}</label>
      <input id="career-to" type="date" bind:value={$careerDateTo} on:change={onDateChange} />
      {#if invalidDates}<span class="error">{$t('stats.invalidPeriod')}</span>{/if}
    {/if}
  </div>
  {#if $careerPeriod !== 'total'}<p class="muted note">{$t('career.periodSource')}</p>{/if}
  {#if pilotError}<p class="error">{pilotError}</p>{/if}
  {#if pilotDisabled}<p class="empty">{$t('stats.disabled')}</p>{/if}
  {#if !pilotLoading && pilots.length > 0}
    <div class="period-picker">
      <label for="career-pilot">{$t('career.periodPilot')}</label>
      <select id="career-pilot" value={periodPilot ? pilotKey(periodPilot) : ''} on:change={(event) => selectedPilot = event.currentTarget.value}>
        {#if !periodPilot}<option value="">{$t('career.choosePilot')}</option>{/if}
        {#each pilots as pilot, index (`${pilot.ucid}:${pilot.name}:${index}`)}
          <option value={pilotKey(pilot)}>{pilot.name}</option>
        {/each}
      </select>
    </div>
  {/if}
  <!-- ------------------------------------------------------------------ -->
  <!-- Logbook (DCS's own record)                                          -->
  <!-- ------------------------------------------------------------------ -->
  {#if $careerPeriod === 'total' && $careerError}
    <p class="error">{$careerError}</p>
  {/if}

  {#if $careerPeriod !== 'total'}
    {#if invalidDates}<p class="error">{$t('stats.invalidPeriod')}</p>
    {:else if pilotLoading}<p class="muted">{$t('career.loading')}</p>
    {:else if periodPilot}
      <div class="identity"><h3>{periodPilot.name}</h3><dl>
        <div><dt>{$t('career.missions')}</dt><dd>{fmtNum(periodPilot.missions, 0)}</dd></div>
        <div><dt>{$t('career.landings')}</dt><dd>{fmtNum(periodPilot.landings, 0)}</dd></div>
        <div><dt>{$t('career.score')}</dt><dd>{fmtNum(periodPilot.score, 0)}</dd></div>
        <div><dt>{$t('stats.killsCol')}</dt><dd>{fmtNum(periodPilot.kills, 0)}</dd></div>
        <div><dt>{$t('stats.deaths')}</dt><dd>{fmtNum(periodPilot.deaths, 0)}</dd></div>
        <div><dt>{$t('stats.crashes')}</dt><dd>{fmtNum(periodPilot.crashes, 0)}</dd></div>
        <div><dt>{$t('stats.ejections')}</dt><dd>{fmtNum(periodPilot.ejections, 0)}</dd></div>
      </dl></div>
    {:else}<p class="empty">{$t('career.periodEmpty')}</p>{/if}
  {:else if $careerPlayers.length === 0 && !$careerError}
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

  <div class="advanced">
    <h3>{$t('career.insights')}</h3>
    <p class="muted note">{$t('career.insightsSource')}</p>
    {#if insightsError}<p class="error">{insightsError}</p>{/if}
    {#if insightsLoading}<p class="muted">{$t('career.loading')}</p>{/if}
    {#if !periodPilot && !pilotLoading && !pilotDisabled}<p class="empty">{$t('career.choosePilot')}</p>{/if}
    {#if insights}
      <div class="breakdowns">
        <div class="breakdown">
          <h4>{$t('career.favoriteMaps')}</h4>
          {#if insights.maps.length === 0}<p class="empty">{$t('career.noMapData')}</p>{/if}
          {#each insights.maps as place (place.name)}
            <div class="place"><span>{place.name}</span><b>{place.missions}</b></div>
            <div class="place-track"><span style={`width:${Math.max(3, place.missions / mapMax * 100)}%`}></span></div>
          {/each}
        </div>
        <div class="breakdown">
          <h4>{$t('career.favoriteCountries')}</h4>
          {#if insights.countries.length === 0}<p class="empty">{$t('career.noCountryData')}</p>{/if}
          {#each insights.countries as place (place.name)}
            <div class="place"><span>{place.name}</span><b>{place.missions}</b></div>
            <div class="place-track"><span style={`width:${Math.max(3, place.missions / countryMax * 100)}%`}></span></div>
          {/each}
        </div>
      </div>
      <div class="heatmap">
        <div class="heat-head"><h4>{$t('career.heatmap')}</h4><strong>{formatMinutes(playMinutes)}</strong></div>
        <p class="muted note">{$t('career.heatmapHint')}</p>
        <div class="heat-grid" role="img" aria-label={$t('career.heatmap')}>
          {#each insights.days as day (day.date)}
            <div class={`heat-cell level-${heatLevel(day.minutes)}`} title={`${day.date} · ${formatMinutes(day.minutes)}`}>
              <span>{day.date.slice(5)}</span><strong>{formatMinutes(day.minutes)}</strong>
            </div>
          {/each}
        </div>
        <div class="heat-legend"><span>0</span><i class="level-0"></i><i class="level-1"></i><i class="level-2"></i><i class="level-3"></i><i class="level-4"></i><span>4 h+</span></div>
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


  .period-picker { display: flex; flex-wrap: wrap; align-items: center; gap: .5rem; margin-bottom: .8rem; font-size: .78rem; }
  .period-picker select, .period-picker input { max-width: min(100%, 420px); padding: .3rem .5rem; color: var(--text); background: var(--bg); border: 1px solid var(--border); border-radius: 6px; }
  .advanced { border-top: 1px solid var(--border); padding-top: 1rem; margin-top: .5rem; }
  .advanced h3 { font-size: .9rem; margin: 0 0 .4rem; }
  .note { font-size: .78rem; line-height: 1.45; margin: 0 0 .9rem; }
  .breakdowns { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
  .breakdown, .heatmap { padding: .8rem; background: var(--bg); border: 1px solid var(--border); border-radius: 9px; }
  .breakdown h4, .heatmap h4 { margin: 0 0 .7rem; font-size: .82rem; }
  .place { display: flex; justify-content: space-between; gap: .7rem; font-size: .78rem; margin-top: .5rem; }
  .place b { font-variant-numeric: tabular-nums; }
  .place-track { height: 5px; border-radius: 5px; background: var(--panel); margin-top: .2rem; overflow: hidden; }
  .place-track span { display: block; height: 100%; background: var(--blue); border-radius: inherit; }
  .heatmap { margin-top: 1rem; }
  .heat-head { display: flex; justify-content: space-between; gap: .7rem; align-items: baseline; }
  .heat-head strong { font-size: .9rem; font-variant-numeric: tabular-nums; }
  .heat-grid { display: grid; grid-template-columns: repeat(10, minmax(0, 1fr)); gap: .35rem; }
  .heat-cell { min-height: 3.2rem; padding: .25rem; border-radius: 5px; display: flex; flex-direction: column; justify-content: space-between; font-size: .68rem; font-variant-numeric: tabular-nums; }
  .heat-cell strong { font-size: .72rem; align-self: flex-end; }
  .level-0 { background: #263342; color: #a9b8c8; }
  .level-1 { background: #287c78; color: #f0fafa; }
  .level-2 { background: #4ab080; color: #071b16; }
  .level-3 { background: #d6bf64; color: #272009; }
  .level-4 { background: #e7824c; color: #2b1006; }
  .heat-legend { display: flex; justify-content: flex-end; align-items: center; gap: .2rem; margin-top: .6rem; font-size: .69rem; color: var(--muted); }
  .heat-legend i { display: inline-block; width: .7rem; height: .7rem; border-radius: 2px; }
  .empty { color: var(--muted); font-size: .8rem; line-height: 1.5; }
  .error { color: #f0b429; font-size: .78rem; }
  @media (max-width: 720px) { .breakdowns { grid-template-columns: 1fr; } .heat-grid { grid-template-columns: repeat(5, minmax(0, 1fr)); } }
</style>
