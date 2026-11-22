<script>
  import { fmtTime } from './debriefs.js';
  import { t } from './i18n.js';

  export let debrief;

  $: summary = debrief?.parsed?.summary ?? {};
  $: events = debrief?.parsed?.events ?? [];

  function eventColor(type) {
    switch (type) {
      case 'kill':
      case 'shot down':
        return 'var(--blue)';
      case 'crash':
      case 'pilot dead':
        return 'var(--red)';
      case 'eject':
        return 'var(--amber)';
      case 'land':
      case 'landing':
        return 'var(--green)';
      case 'takeoff':
        return 'var(--blue)';
      default:
        return 'var(--faint)';
    }
  }

  function describe(tr, event) {
    const who = event.initiatorPilot || '?';
    const place = event.placeDisplayName || event.place || '?';
    switch (event.type) {
      case 'takeoff':
        return tr('events.tookOff', { pilot: who, place });
      case 'land':
      case 'landing':
        return tr('events.landed', { pilot: who, place });
      case 'kill':
      case 'shot down':
        return tr('debriefs.killed', {
          pilot: who,
          target: event.targetPilot || event.target || '?',
          weapon: event.weapon || '?',
        });
      case 'crash':
        return tr('debriefs.crashed', { pilot: who });
      case 'eject':
        return tr('debriefs.ejected', { pilot: who });
      case 'pilot dead':
        return tr('debriefs.died', { pilot: who });
      case 'engine shutdown':
        return tr('debriefs.engineShutdown', { pilot: who, place });
      case 'mission end':
        return tr('debriefs.missionEnd', { comment: event.comment || '' });
      default:
        return who !== '?' ? who : '';
    }
  }

</script>

<div class="detail">
  <div class="stats">
    <div><span>{summary.takeoffs ?? 0}</span>{$t('debriefs.takeoffs')}</div>
    <div><span>{summary.landings ?? 0}</span>{$t('debriefs.landings')}</div>
    <div><span>{summary.kills ?? 0}</span>{$t('debriefs.kills')}</div>
    <div><span>{summary.crashes ?? 0}</span>{$t('debriefs.crashes')}</div>
    <div><span>{summary.ejections ?? 0}</span>{$t('debriefs.ejections')}</div>
    <div><span>{fmtTime(debrief.parsed?.missionTime ?? 0)}</span>{$t('debriefs.duration')}</div>
  </div>

  {#if summary.pilots?.length}
    <p class="pilots">{$t('debriefs.pilots')}: {summary.pilots.join(', ')}</p>
  {/if}

  <h4>
    {$t('debriefs.timeline')}
    <span class="count">{events.length}</span>
  </h4>
  <ol>
    {#each events as event, index (`${event.eventId}-${index}`)}
      <li>
        <span class="t">{fmtTime(event.t)}</span>
        <span class="bar" style="background:{eventColor(event.type)}"></span>
        <span class="kind">{event.type}</span>
        <span class="desc">{describe($t, event)}</span>
      </li>
    {/each}
  </ol>
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 0.8rem;
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(92px, 1fr));
    gap: 0.45rem;
  }

  .stats div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    padding: 0.5rem 0.6rem;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .stats span {
    font-size: 1.05rem;
    font-weight: 650;
    color: var(--text);
    text-transform: none;
    letter-spacing: 0;
    font-variant-numeric: tabular-nums;
  }

  .pilots {
    margin: 0;
    font-size: 0.78rem;
    color: var(--muted);
  }

  h4 {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: 0;
    font-size: 0.74rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--muted);
  }

  .count {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }

  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  li {
    display: grid;
    grid-template-columns: 48px 3px 92px 1fr;
    align-items: center;
    gap: 0.55rem;
    padding: 0.32rem 0.5rem;
    background: var(--bg-elevated);
    border-radius: var(--radius-sm);
    font-size: 0.76rem;
  }

  .t {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .bar {
    width: 3px;
    height: 16px;
    border-radius: 2px;
  }

  .kind {
    color: var(--muted);
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .desc {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  @media (max-width: 640px) {
    li {
      grid-template-columns: 44px 3px 1fr;
    }

    .kind {
      display: none;
    }
  }
</style>
