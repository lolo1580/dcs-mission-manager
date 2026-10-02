<script>
  /**
   * Career: the player's own logbook, as DCS records it — rank, squadron,
   * awards, and a per-airframe breakdown of flight hours, landings, deaths
   * and kills.
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
  import { t } from './i18n.js';

  onMount(loadCareer);

  function fmtNum(v) {
    return typeof v === 'number' ? v.toLocaleString() : '—';
  }

  /** Reads a raw aggregate value, which may be a number or a string. */
  function agg(key) {
    const v = $careerPlayer?.aggregate?.[key];
    if (v == null) return null;
    const n = typeof v === 'number' ? v : Number(v);
    return Number.isFinite(n) ? n : null;
  }

  /** The largest flight-hours value, so a bar can be scaled to it. */
  $: maxHours = ($careerPlayer?.aircraft ?? []).reduce((m, a) => Math.max(m, a.flightHours ?? 0), 0) || 1;
</script>

<section class="career">
  <header>
    <h2>{$t('career.title')}</h2>
    <button class="refresh" on:click={loadCareer} disabled={$careerLoading}>{$t('stats.refresh')}</button>
  </header>

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
        {#if agg('missionsCount') != null}<div><dt>{$t('career.missions')}</dt><dd>{fmtNum(agg('missionsCount'))}</dd></div>{/if}
        {#if agg('landings') != null}<div><dt>{$t('career.landings')}</dt><dd>{fmtNum(agg('landings'))}</dd></div>{/if}
        {#if agg('totalScore') != null}<div><dt>{$t('career.score')}</dt><dd>{fmtNum(agg('totalScore'))}</dd></div>{/if}
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
        <table>
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
                <td class="name">{aircraftLabel(a.type)}</td>
                <td class="num hours">
                  <span class="bar" style="width:{Math.max(2, Math.round((a.flightHours / maxHours) * 100))}%"></span>
                  <span class="val">{fmtHours(a.flightHours)}</span>
                </td>
                <td class="num">{fmtNum(a.landings)}</td>
                <td class="num">{fmtNum(a.deaths)}</td>
                <td class="num">{fmtNum(a.ejections)}</td>
                <td class="num">{fmtNum(a.aaKills)}</td>
                <td class="num">{fmtNum(a.agKills)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
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
    margin-bottom: 0.7rem;
  }

  h2 {
    margin: 0;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
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

  .block h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
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
