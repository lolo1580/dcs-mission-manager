<script>
  import { visibleEvents, eventFilter, eventFilter as filterStore, eventCounts, EVENT_FILTERS } from './session.js';
  import { coalitionColor } from './icons.js';

  function timeOf(e) {
    return new Date(e.realTs).toLocaleTimeString();
  }

  function eventColor(e) {
    switch (e.event) {
      case 'kill':
        return coalitionColor(e.args?.[2] === 1 ? 'red' : e.args?.[2] === 2 ? 'blue' : 'neutral');
      case 'friendly_fire':
        return '#f0b429';
      case 'crash':
      case 'pilot_death':
        return '#ff4d4d';
      case 'eject':
        return '#f0883e';
      case 'landing':
        return '#3fb950';
      default:
        return '#9aa4b2';
    }
  }

  /** Build a readable one-line summary from the raw DCS event arguments. */
  function describe(e) {
    const a = e.args ?? [];
    switch (e.event) {
      case 'kill':
        return `#${a[0]} a détruit #${a[3]} (${a[6] ?? '?'})`;
      case 'friendly_fire':
        // DCS: "friendly_fire", playerID, weaponName, victimPlayerID
        return `#${a[0]} → allié #${a[2]} (${a[1] ?? '?'})`;
      case 'crash':
        return `#${a[0]} a crashé (unité ${a[1]})`;
      case 'eject':
        return `#${a[0]} s'est éjecté (unité ${a[1]})`;
      case 'pilot_death':
        return `#${a[0]} est mort (unité ${a[1]})`;
      case 'takeoff':
        return `#${a[0]} a décollé de ${a[2] ?? '?'}`;
      case 'landing':
        return `#${a[0]} a atterri à ${a[2] ?? '?'}`;
      case 'change_slot':
        return `#${a[0]} a changé de slot`;
      case 'connect':
        return `${a[1] ?? '#' + a[0]} connecté`;
      case 'disconnect':
        return `${a[1] ?? '#' + a[0]} déconnecté`;
      case 'mission_end':
        return `Mission terminée — gagnant : ${a[0] ?? '?'}`;
      default:
        return a.length ? a.join(' · ') : '';
    }
  }
</script>

<section class="events">
  <h2>
    Événements <span class="count">{$visibleEvents.length}</span>
  </h2>

  <div class="chips">
    {#each EVENT_FILTERS as f (f.id)}
      <button
        class="chip"
        class:active={$eventFilter === f.id}
        on:click={() => filterStore.set(f.id)}
      >
        {f.label}
        {#if f.id !== 'all' && ($eventCounts[f.id] ?? 0) > 0}
          <span class="n">{$eventCounts[f.id]}</span>
        {/if}
      </button>
    {/each}
  </div>

  <ul>
    {#each $visibleEvents.slice().reverse().slice(0, 200) as e (`${e.id}`)}
      <li>
        <span class="t">{timeOf(e)}</span>
        <span class="bar" style="background:{eventColor(e)}"></span>
        <span class="kind">{e.event}</span>
        <span class="desc">{describe(e)}</span>
      </li>
    {/each}
    {#if $visibleEvents.length === 0}
      <li class="empty">Aucun événement</li>
    {/if}
  </ul>
</section>

<style>
  .events {
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: 0.9rem 1rem;
    border-bottom: 1px solid var(--border);
  }

  h2 {
    margin: 0 0 0.6rem;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    display: flex;
    justify-content: space-between;
  }

  .count {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin-bottom: 0.5rem;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.15rem 0.45rem;
    font-size: 0.7rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    cursor: pointer;
  }

  .chip:hover {
    color: var(--text);
  }

  .chip.active {
    color: var(--text);
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
  }

  .chip .n {
    opacity: 0.6;
    font-variant-numeric: tabular-nums;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    max-height: 240px;
    font-size: 0.76rem;
  }

  li {
    display: flex;
    align-items: baseline;
    gap: 0.4rem;
    padding: 0.2rem 0;
    border-top: 1px solid var(--border);
  }

  li:first-child {
    border-top: none;
  }

  .t {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    flex: none;
  }

  .bar {
    width: 3px;
    align-self: stretch;
    border-radius: 2px;
    flex: none;
  }

  .kind {
    flex: none;
    color: var(--text);
    font-weight: 500;
  }

  .desc {
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty {
    color: var(--muted);
    border-top: none;
  }
</style>
