<script>
  import { onMount } from 'svelte';
  import {
    statsOverview,
    statsPilots,
    statsWeapons,
    statsEngines,
    statsNetwork,
    enginesByCategory,
    engineCategory,
    statTab,
    STAT_TABS,
    scopeMode,
    statsError,
    statsLoading,
    loadStats,
    fmtNum,
  } from './stats.js';
  import { CATEGORY_LABELS } from './icons.js';

  onMount(loadStats);

  function onScopeChange(mode) {
    scopeMode.set(mode);
    loadStats();
  }

  function medal(i) {
    return i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : '';
  }
</script>

<section class="stats">
  <header>
    <h2>Statistiques</h2>
    <div class="scope">
      <button class:active={$scopeMode === 'career'} on:click={() => onScopeChange('career')}>
        Carrière
      </button>
      <button class:active={$scopeMode === 'mission'} on:click={() => onScopeChange('mission')}>
        Mission
      </button>
    </div>
    <button class="refresh" on:click={loadStats} disabled={$statsLoading}>Rafraîchir</button>
  </header>

  {#if $statsError}
    <p class="error">{$statsError}</p>
  {/if}

  {#if $statsOverview}
    {@const o = $statsOverview}
    <div class="cards">
      <div><span>{o.missions}</span>Missions</div>
      <div><span>{o.players}</span>Pilotes</div>
      <div><span>{o.kills}</span>Kills</div>
      <div><span>{o.deaths}</span>Morts</div>
      <div><span>{o.crashes}</span>Crashes</div>
      <div><span>{o.ejections}</span>Éjections</div>
      <div class:warn={o.friendlyFire > 0}><span>{o.friendlyFire}</span>Friendly fire</div>
    </div>
  {/if}

  <nav class="subtabs">
    {#each STAT_TABS as t (t.id)}
      <button class:active={$statTab === t.id} on:click={() => statTab.set(t.id)}>{t.label}</button>
    {/each}
  </nav>

  <div class="panel">
    {#if $statTab === 'pilots'}
      {#if $statsPilots.length === 0}
        <p class="empty">Aucune donnée de pilote.</p>
      {:else}
        <table>
          <thead>
            <tr>
              <th></th><th>Pilote</th><th>Score</th><th>Kills</th><th>Morts</th>
              <th>K/D</th><th>Att.</th><th>Éject.</th><th>Crash</th><th>FF</th><th>Ping</th>
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
        <p class="empty">Aucune donnée d'arme. Les armes proviennent des événements de kill.</p>
      {:else}
        <table>
          <thead>
            <tr><th>Arme</th><th>Kills</th><th>FF</th><th>Cibles</th></tr>
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
            {CATEGORY_LABELS[c] ?? c}
          </button>
        {/each}
      </div>
      {#if $enginesByCategory.length === 0}
        <p class="empty">Aucun engin dans cette catégorie.</p>
      {:else}
        <table>
          <thead>
            <tr><th>Type DCS</th><th>Kills</th><th>Pertes</th><th>Sorties</th><th>K/D</th></tr>
          </thead>
          <tbody>
            {#each $enginesByCategory as e (e.typeId)}
              <tr>
                <td class="name">{e.typeId}</td>
                <td class="num">{e.kills}</td>
                <td class="num">{e.deaths}</td>
                <td class="num">{e.sorties}</td>
                <td class="num">{fmtNum(e.kd, 2)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    {:else if $statTab === 'balance'}
      {#if !$statsOverview?.coalitions?.length}
        <p class="empty">Aucune donnée de coalition.</p>
      {:else}
        {#each $statsOverview.coalitions as c (c.coalition)}
          {@const total = Math.max(...$statsOverview.coalitions.map((x) => x.score), 1)}
          <div class="balance-row">
            <span class="side {c.coalition}">{c.coalition}</span>
            <div class="bar">
              <div
                class="fill {c.coalition}"
                style="width:{(c.score / total) * 100}%"
              ></div>
            </div>
            <span class="num">{c.score} pts</span>
            <span class="num">{c.kills} kills</span>
            <span class="num">{c.players} joueurs</span>
          </div>
        {/each}
      {/if}
    {:else if $statTab === 'network'}
      {#if $statsNetwork.length === 0}
        <p class="empty">Aucune donnée réseau.</p>
      {:else}
        <table>
          <thead>
            <tr><th>Pilote</th><th>Échantillons</th><th>Ping moyen</th><th>Ping max</th></tr>
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
</section>

<style>
  .stats {
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

  .refresh {
    margin-left: auto;
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

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(88px, 1fr));
    gap: 0.5rem;
    margin-bottom: 0.8rem;
  }

  .cards div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    padding: 0.5rem 0.6rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 0.68rem;
    color: var(--muted);
  }

  .cards span {
    font-size: 1.15rem;
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .cards div.warn span {
    color: #f0b429;
  }

  .subtabs {
    display: flex;
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
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
  }

  th {
    position: sticky;
    top: 0;
    text-align: right;
    font-weight: 500;
    color: var(--muted);
    padding: 0.35rem 0.4rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    font-size: 0.7rem;
  }

  th:nth-child(1),
  th:nth-child(2),
  td:nth-child(1),
  td:nth-child(2) {
    text-align: left;
  }

  td {
    padding: 0.3rem 0.4rem;
    border-top: 1px solid var(--border);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  td.num,
  th:not(:nth-child(1)):not(:nth-child(2)) {
    text-align: right;
  }

  td.warn {
    color: #f0b429;
  }

  .medal {
    width: 22px;
  }

  .name {
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .targets {
    text-align: left;
  }

  .tag {
    display: inline-block;
    margin: 0 0.2rem 0.15rem 0;
    padding: 0.05rem 0.35rem;
    font-size: 0.68rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
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
    font-size: 0.82rem;
  }

  .error {
    color: #f0b429;
    font-size: 0.78rem;
  }
</style>
