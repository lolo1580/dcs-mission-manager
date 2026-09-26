<script>
  import { players, sideKey } from './session.js';
  import { coalitionColor } from './icons.js';
  import { t } from './i18n.js';

  function fmt(v) {
    return typeof v === 'number' ? v.toLocaleString() : '—';
  }
</script>

<section>
  <h2>
    {$t('players.title')} <span class="count">{$players.length}</span>
  </h2>

  {#if $players.length === 0}
    <p class="empty">{$t('players.none')}</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>{$t('players.pilot')}</th>
          <th title={$t('players.score')}>{$t('players.score')}</th>
          <th title={$t('players.killsTitle')}>{$t('players.kills')}</th>
          <th title={$t('players.landings')}>{$t('players.landings')}</th>
          <th title={$t('players.pingTitle')}>{$t('players.ping')}</th>
        </tr>
      </thead>
      <tbody>
        {#each $players as p (p.id)}
          <tr>
            <td class="name">
              <span class="dot" style="background:{coalitionColor(sideKey(p.side))}"></span>
              <span class="label" title={p.name}>{p.name}</span>
            </td>
            <td class="num">{fmt(p.score)}</td>
            <td class="num">{p.killsAir}/{p.killsCar}/{p.killsShip}</td>
            <td class="num">{p.landings}</td>
            <td class="num" class:warn={p.ping > 250}>{p.ping}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  section {
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

  .empty {
    margin: 0;
    color: var(--muted);
    font-size: 0.78rem;
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
    padding: 0 0.25rem 0.3rem;
    font-size: 0.7rem;
  }

  th:first-child,
  td:first-child {
    text-align: left;
  }

  td {
    padding: 0.25rem;
    border-top: 1px solid var(--border);
    font-variant-numeric: tabular-nums;
  }

  td.num {
    text-align: right;
  }

  td.warn {
    color: #f0b429;
  }

  .name {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    max-width: 0;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex: none;
  }

  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
