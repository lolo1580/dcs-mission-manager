<script>
  import MapView from './lib/MapView.svelte';
  import Sidebar from './lib/Sidebar.svelte';
  import UnitDetails from './lib/UnitDetails.svelte';
  import PlayerPanel from './lib/PlayerPanel.svelte';
  import EventPanel from './lib/EventPanel.svelte';
  import ChatPanel from './lib/ChatPanel.svelte';
  import DebriefPanel from './lib/DebriefPanel.svelte';
  import StatsPanel from './lib/StatsPanel.svelte';
  import AnalyticsPanel from './lib/AnalyticsPanel.svelte';
  import AerodromePanel from './lib/AerodromePanel.svelte';
  import {
    connected,
    lastUpdate,
    units,
    visibleUnits,
    basemaps,
    basemapId,
    visibility,
  } from './lib/units.js';
  import { players, events, mission } from './lib/session.js';
  import { loadAnalytics } from './lib/analytics.js';
  import { t, lang, LANGUAGES, setLang } from './lib/i18n.js';

  let mapView;
  let tab = 'map';
  let history = false;

  function toggleHistory() {
    history = !history;
    // The analytics data lives in its own store, loaded by the Analytics panel.
    // Refresh it here so overlays appear even if that tab was never opened.
    if (history) loadAnalytics();
  }
</script>

<div class="layout">
  <header>
    <strong>{$t('app.title')}</strong>
    <span class="live">
      <span class="dot" class:on={$connected}></span>
      {$connected ? $t('app.connected') : $t('app.offline')}
    </span>

    <nav class="tabs">
      <button class:active={tab === 'map'} on:click={() => (tab = 'map')}>{$t('tab.map')}</button>
      <button class:active={tab === 'session'} on:click={() => (tab = 'session')}>
        {$t('tab.session')}
        {#if $players.length}<span class="badge">{$players.length}</span>{/if}
      </button>
      <button class:active={tab === 'debriefs'} on:click={() => (tab = 'debriefs')}>{$t('tab.debriefs')}</button>
      <button class:active={tab === 'stats'} on:click={() => (tab = 'stats')}>{$t('tab.stats')}</button>
      <button class:active={tab === 'analytics'} on:click={() => (tab = 'analytics')}>{$t('tab.analytics')}</button>
      <button class:active={tab === 'aerodromes'} on:click={() => (tab = 'aerodromes')}>{$t('tab.aerodromes')}</button>
    </nav>

    {#if $mission}
      <span class="mission" title={$mission.name}>
        <span class="mission-dot"></span>{$mission.name}
      </span>
    {/if}

    {#if tab === 'map'}
      <span class="meta">
        {$units.length}
        {$units.length === 1 ? ($lang === 'fr' ? 'unité suivie' : 'unit tracked') : ($lang === 'fr' ? 'unités suivies' : 'units tracked')}
      </span>
      <span class="meta">
        {$visibleUnits.length}
        {$visibleUnits.length === 1 ? ($lang === 'fr' ? 'affichée' : 'shown') : ($lang === 'fr' ? 'affichées' : 'shown')}
      </span>
      {#if $lastUpdate}
        <span class="meta">{$lang === 'fr' ? 'maj' : 'updated'} {$lastUpdate.toLocaleTimeString()}</span>
      {/if}
      {#if $basemaps.length}
        <label class="basemap">
          {$t('app.basemap')}
          <select bind:value={$basemapId}>
            {#each $basemaps as b (b.id)}
              <option value={b.id}>{b.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      <button class="action" on:click={() => mapView?.recenter()} title={$t('app.recenterTitle')}>
        {$t('app.recenter')}
      </button>
      <button
        class="action"
        class:on={history}
        on:click={toggleHistory}
        title={$t('app.historyTitle')}
      >
        {$t('app.history')}
      </button>
    {:else if tab === 'session'}
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

  {#if $visibility && $visibility.mode !== 'all' && !$visibility.override}
    <div
      class="fog-banner"
      title={$t(`visibility.note.${$visibility.mode}`)}
      class:fog={$visibility.mode === 'unknown'}
    >
      <span class="fog-icon">◐</span>
      {$t('visibility.prefix')}: {$t(`visibility.mode.${$visibility.mode}`)}
      — <span class="fog-note">{$t(`visibility.note.${$visibility.mode}`)}</span>
    </div>
  {/if}

  <main>
    {#if tab === 'map'}
      <Sidebar />
      <div class="map-wrap">
        <MapView bind:this={mapView} bind:history />
        <UnitDetails />
      </div>
    {:else if tab === 'session'}
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
    {:else if tab === 'analytics'}
      <div class="session wide">
        <AnalyticsPanel />
      </div>
    {:else}
      <div class="session wide">
        <AerodromePanel />
      </div>
    {/if}
  </main>
</div>

<style>
  .layout {
    display: grid;
    grid-template-rows: auto auto 1fr;
    height: 100%;
  }

  header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
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

  .basemap {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    margin-left: auto;
    color: var(--muted);
    font-size: 0.8rem;
  }

  .basemap select {
    padding: 0.3rem 0.45rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .basemap select:focus {
    outline: none;
    border-color: var(--blue);
  }

  .lang {
    margin-left: 0.5rem;
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

  .basemap + .action {
    margin-left: 0;
  }

  .action {
    margin-left: auto;
    padding: 0.35rem 0.7rem;
    font-size: 0.8rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .action:hover {
    border-color: var(--blue);
  }

  .action.on {
    color: var(--bg);
    background: var(--blue);
    border-color: var(--blue);
  }

  main {
    display: flex;
    min-height: 0;
  }

  .fog-banner {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.9rem;
    font-size: 0.78rem;
    color: #f0b429;
    background: color-mix(in srgb, #f0b429 12%, var(--panel));
    border-bottom: 1px solid var(--border);
  }

  .fog-banner.fog {
    color: #9aa4b2;
    background: var(--panel);
  }

  .fog-icon {
    font-size: 0.9rem;
  }

  .fog-note {
    color: var(--muted);
  }

  .map-wrap {
    position: relative;
    flex: 1;
    min-width: 0;
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
