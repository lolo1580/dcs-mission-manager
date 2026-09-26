<script>
  import { onMount } from 'svelte';
  import {
    heatSource,
    heatTotal,
    sortieStats,
    trails,
    analyticsError,
    loadAnalytics,
    reloadHeat,
    fmtKm,
    fmtSpeed,
    fmtDuration,
  } from './analytics.js';

  onMount(loadAnalytics);

  async function changeSource(id) {
    heatSource.set(id);
    await reloadHeat();
  }
</script>

<section class="analytics">
  <header>
    <h2>Analyse</h2>
    <button class="refresh" on:click={loadAnalytics}>Rafraîchir</button>
  </header>

  {#if $analyticsError}
    <p class="error">{$analyticsError}</p>
  {/if}

  <div class="block">
    <h3>
      Carte de chaleur
      <span class="count">{$heatTotal} pts ({$heatSource === 'positions' ? 'trafic' : 'pertes'})</span>
    </h3>
    <p class="hint">
      Agrégation des positions suivies. « Pertes » met en évidence les zones où
      des unités ont disparu ; « Trafic » montre où l'activité se concentre.
      Utilise le bouton <strong>Historique</strong> de l'en-tête pour superposer
      la carte de chaleur et les traces sur la carte.
    </p>
    <div class="sources">
      <button class:active={$heatSource === 'losses'} on:click={() => changeSource('losses')}>
        Pertes
      </button>
      <button class:active={$heatSource === 'positions'} on:click={() => changeSource('positions')}>
        Trafic
      </button>
    </div>
  </div>

  <div class="block">
    <h3>
      Analyse de sortie
      <span class="count">{$sortieStats.length}</span>
    </h3>
    {#if $sortieStats.length === 0}
      <p class="empty">
        Aucune trace enregistrée pour l'instant. Les positions sont échantillonnées
        pendant la mission.
      </p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>Unité</th><th>Durée</th><th>Distance</th>
            <th>Alt. max</th><th>Vit. max</th><th>G max</th><th>Points</th>
          </tr>
        </thead>
        <tbody>
          {#each $sortieStats as s (s.unitId)}
            {@const trail = $trails[s.unitId] ?? []}
            {@const label = trail[0] ? s.unitId : s.unitId}
            <tr>
              <td class="name">{s.type || s.unitId}</td>
              <td class="num">{fmtDuration(s.durationSec)}</td>
              <td class="num">{fmtKm(s.distanceKm)}</td>
              <td class="num">{Math.round(s.maxAlt)} m</td>
              <td class="num">{fmtSpeed(s.maxSpeed)}</td>
              <td class="num">{s.maxG > 0 ? s.maxG.toFixed(1) : '—'}</td>
              <td class="num">{s.points}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</section>

<style>
  .analytics {
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

  .refresh:hover {
    color: var(--text);
    border-color: var(--blue);
  }

  .block {
    margin-bottom: 1.2rem;
  }

  h3 {
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

  .hint {
    margin: 0 0 0.5rem;
    font-size: 0.74rem;
    color: var(--muted);
    line-height: 1.5;
    max-width: 60ch;
  }

  .sources {
    display: flex;
    gap: 0.3rem;
  }

  .sources button {
    padding: 0.25rem 0.6rem;
    font-size: 0.76rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .sources button.active {
    color: var(--text);
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
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
