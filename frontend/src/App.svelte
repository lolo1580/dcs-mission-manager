<script>
  import ChartViewer from './lib/ChartViewer.svelte';
  import logo from './assets/logo.png';
  import DebriefPanel from './lib/DebriefPanel.svelte';
  import CareerPanel from './lib/CareerPanel.svelte';
  import AerodromePanel from './lib/AerodromePanel.svelte';
  import ModulesPanel from './lib/ModulesPanel.svelte';
  import MissionsPanel from './lib/MissionsPanel.svelte';
  import InstallPanel from './lib/InstallPanel.svelte';
  import ConfigPanel from './lib/ConfigPanel.svelte';
  import PanelsPanel from './lib/PanelsPanel.svelte';
  import { connected, paused, mission, fetchTheatres, connect } from './lib/units.js';
  import { t, lang, LANGUAGES, setLang } from './lib/i18n.js';
  import { onMount } from 'svelte';

  // The Session tab (players, events, chat) was removed: the manager now opens
  // on the debriefs, which are the durable record of a flight.
  let tab = 'debriefs';

  function goTo(id) {
    tab = id;
  }

  // Open the live stream once, for the whole application: the mission name in
  // the header, the cockpit-hardware updates and the pause indicator all depend
  // on it.
  onMount(() => {
    const stopStream = connect();
    return stopStream;
  });

  // Theatres drive the airfields tab: load them once at startup.
  fetchTheatres().catch(() => {});
</script>

<div class="layout">
  <header>
    <span class="brand">
      <img class="logo" src={logo} alt="" aria-hidden="true" />
      <strong>{$t('app.title')}</strong>
    </span>
    <span class="live">
      <span class="dot" class:on={$connected}></span>
      {$connected ? $t('app.connected') : $t('app.offline')}
    </span>

    <nav class="tabs">
      <button class:active={tab === 'debriefs'} on:click={() => goTo('debriefs')}>{$t('tab.debriefs')}</button>
      <button class:active={tab === 'missions'} on:click={() => goTo('missions')}>{$t('tab.library')}</button>
      <button class:active={tab === 'stats'} on:click={() => goTo('stats')}>{$t('tab.stats')}</button>
      <button class:active={tab === 'aerodromes'} on:click={() => goTo('aerodromes')}>{$t('tab.aerodromes')}</button>
      <button class:active={tab === 'modules'} on:click={() => goTo('modules')}>{$t('tab.modules')}</button>
      <button class:active={tab === 'install'} on:click={() => goTo('install')}>{$t('tab.install')}</button>
      <button class:active={tab === 'config'} on:click={() => goTo('config')}>{$t('tab.config')}</button>
      <button class:active={tab === 'panels'} on:click={() => goTo('panels')}>{$t('tab.panels')}</button>
    </nav>

    {#if $mission}
      <span class="mission" title={$mission.name}>
        <span class="mission-dot"></span>{$mission.name}
      </span>
    {/if}

    {#if tab === 'debriefs'}
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

  {#if $paused}
    <div class="pause-banner" title={$t('app.pausedNote')}>
      <span class="pause-icon">⏸</span>
      {$t('app.paused')} — <span class="pause-note">{$t('app.pausedNote')}</span>
    </div>
  {/if}

  <main>
    {#if tab === 'debriefs'}
      <div class="session wide">
        <DebriefPanel />
      </div>
    {:else if tab === 'stats'}
      <div class="session wide">
        <CareerPanel />
      </div>
    {:else if tab === 'missions'}
      <div class="session wide">
        <MissionsPanel />
      </div>
    {:else if tab === 'modules'}
      <div class="session wide">
        <ModulesPanel />
      </div>
    {:else if tab === 'install'}
      <div class="session wide">
        <InstallPanel />
      </div>
    {:else if tab === 'config'}
      <div class="session wide">
        <ConfigPanel />
      </div>
    {:else if tab === 'panels'}
      <div class="session wide">
        <PanelsPanel />
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

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }

  .logo {
    width: 28px;
    height: 28px;
    flex: none;
    /* The emblem is a circle on a transparent background, so it sits cleanly on
       the header regardless of the theme. */
    border-radius: 50%;
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
