<script>
  import ChartViewer from './lib/ChartViewer.svelte';
  import PlayerPanel from './lib/PlayerPanel.svelte';
  import EventPanel from './lib/EventPanel.svelte';
  import ChatPanel from './lib/ChatPanel.svelte';
  import DebriefPanel from './lib/DebriefPanel.svelte';
  import StatsPanel from './lib/StatsPanel.svelte';
  import AnalyticsPanel from './lib/AnalyticsPanel.svelte';
  import AerodromePanel from './lib/AerodromePanel.svelte';
  import ModulesPanel from './lib/ModulesPanel.svelte';
  import CareerPanel from './lib/CareerPanel.svelte';
  import MissionsPanel from './lib/MissionsPanel.svelte';
  import { connected, paused, fetchTheatres } from './lib/units.js';
  import { events, players, mission } from './lib/session.js';
  import { t, lang, LANGUAGES, setLang } from './lib/i18n.js';

  let tab = 'session';

  function goTo(id) {
    tab = id;
  }

  // Theatres drive the airfields tab: load them once at startup.
  fetchTheatres().catch(() => {});
</script>

<div class="layout">
  <header>
    <strong>{$t('app.title')}</strong>
    <span class="live">
      <span class="dot" class:on={$connected}></span>
      {$connected ? $t('app.connected') : $t('app.offline')}
    </span>

    <nav class="tabs">
      <button class:active={tab === 'session'} on:click={() => goTo('session')}>
        {$t('tab.session')}
        {#if $players.length}<span class="badge">{$players.length}</span>{/if}
      </button>
      <button class:active={tab === 'debriefs'} on:click={() => goTo('debriefs')}>{$t('tab.debriefs')}</button>
      <button class:active={tab === 'missions'} on:click={() => goTo('missions')}>{$t('tab.library')}</button>
      <button class:active={tab === 'stats'} on:click={() => goTo('stats')}>{$t('tab.stats')}</button>
      <button class:active={tab === 'career'} on:click={() => goTo('career')}>{$t('tab.career')}</button>
      <button class:active={tab === 'analytics'} on:click={() => goTo('analytics')}>{$t('tab.analytics')}</button>
      <button class:active={tab === 'aerodromes'} on:click={() => goTo('aerodromes')}>{$t('tab.aerodromes')}</button>
      <button class:active={tab === 'modules'} on:click={() => goTo('modules')}>{$t('tab.modules')}</button>
    </nav>

    {#if $mission}
      <span class="mission" title={$mission.name}>
        <span class="mission-dot"></span>{$mission.name}
      </span>
    {/if}

    {#if tab === 'session'}
      <span class="meta">
        {$events.length}
        {$events.length === 1 ? ($lang === 'fr' ? 'événement' : 'event') : ($lang === 'fr' ? 'événements' : 'events')}
      </span>
    {:else if tab === 'debriefs'}
      <span class="meta">{$t('tab.missionHistory')}</span>
    {/if}

    <label class="lang">
      <select value={$lang} on:change={(e) => setLang(e.currentTarget.value)} aria-label="Language">
        {#each LANGUAGES as l (l.id)}
          <option value={l.id}>{l.label}</option>
        {/each}
      </select>
    </label>
  </header>

  {#if $paused && tab === 'session'}
    <div class="pause-banner" title={$t('app.pausedNote')}>
      <span class="pause-icon">⏸</span>
      {$t('app.paused')} — <span class="pause-note">{$t('app.pausedNote')}</span>
    </div>
  {/if}

  <main>
    {#if tab === 'session'}
      <div class="session">
        <PlayerPanel />
        <EventPanel />
      </div>
      <div class="session side">
        <ChatPanel />
      </div>
    {:else if tab === 'debriefs'}
      <div class="session wide">
        <DebriefPanel />
      </div>
    {:else if tab === 'stats'}
      <div class="session wide">
        <StatsPanel />
      </div>
    {:else if tab === 'career'}
      <div class="session wide">
        <CareerPanel />
      </div>
    {:else if tab === 'missions'}
      <div class="session wide">
        <MissionsPanel />
      </div>
    {:else if tab === 'analytics'}
      <div class="session wide">
        <AnalyticsPanel />
      </div>
    {:else if tab === 'modules'}
      <div class="session wide">
        <ModulesPanel />
      </div>
    {:else}
      <div class="session wide">
        <AerodromePanel />
      </div>
    {/if}
  </main>
</div>

<!-- Modal chart viewer: a scan is displayed whole, on top of everything. -->
<ChartViewer />

<style>
  .layout {
    /* A flex column rather than a grid: the banners are optional, so the number
       of rows varies. A grid with a fixed row count put a fourth child on an
       implicit row, which sized the main area to its content and gave a banner
       the whole height. */
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
  }

  header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
    row-gap: 0.35rem;
    padding: 0.5rem 1rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    font-size: 0.88rem;
  }

  header strong {
    white-space: nowrap;
  }

  .live {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--muted);
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--red);
  }

  .dot.on {
    background: var(--green);
  }

  .tabs {
    display: inline-flex;
    gap: 0.2rem;
    padding: 0.15rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  .tabs button {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.3rem 0.7rem;
    font-size: 0.82rem;
    color: var(--muted);
    background: transparent;
    border: none;
    border-radius: 6px;
    cursor: pointer;
  }

  .tabs button:hover {
    color: var(--text);
  }

  .tabs button.active {
    color: var(--text);
    background: var(--panel);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  }

  .badge {
    padding: 0 0.35rem;
    font-size: 0.7rem;
    color: var(--bg);
    background: var(--blue);
    border-radius: 999px;
    font-variant-numeric: tabular-nums;
  }

  .mission {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    max-width: 240px;
    padding: 0.2rem 0.5rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.78rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mission-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--green);
    flex: none;
  }

  .meta {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .lang {
    margin-left: auto;
  }

  .lang select {
    padding: 0.3rem 0.45rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .lang select:focus {
    outline: none;
    border-color: var(--blue);
  }

  main {
    display: flex;
    /* Take all the height the header and the banners leave, and never shrink
       below the viewport: min-height: 0 is what allows the inner panels to
       scroll instead of stretching the page. */
    flex: 1 1 auto;
    min-height: 0;
    min-width: 0;
  }

  .pause-banner {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.9rem;
    font-size: 0.78rem;
    color: #9aa4b2;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
  }

  .pause-icon {
    font-size: 0.9rem;
  }

  .pause-note {
    color: var(--muted);
  }

  .session {
    width: 460px;
    min-width: 460px;
    overflow-y: auto;
    background: var(--panel);
    border-right: 1px solid var(--border);
  }

  .session.side {
    flex: 1;
    width: auto;
    min-width: 0;
    border-right: none;
  }

  .session.wide {
    flex: 1;
    width: auto;
    min-width: 0;
    border-right: none;
    overflow: hidden;
  }
</style>
