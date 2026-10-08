<script>
  import ChartViewer from './lib/ChartViewer.svelte';
  import logo from './assets/logo.png';
  import DebriefPanel from './lib/DebriefPanel.svelte';
  import CareerPanel from './lib/CareerPanel.svelte';
  import AerodromePanel from './lib/AerodromePanel.svelte';
  import SettingsPanel from './lib/SettingsPanel.svelte';
  import { connected, feedStatus, mission, fetchTheatres, connect } from './lib/units.js';
  import { t, lang, LANGUAGES, setLang } from './lib/i18n.js';
  import { onMount } from 'svelte';
  import { updateInfo, checkForUpdates } from './lib/updates.js';

  // The Debriefs view is built from the redesign mockup: the shell is a sidebar +
  // topbar, and each view keeps its existing panel. Debriefs stays hidden for
  // now (the panel is still in place) — restore its entry below to re-enable it.
  let tab = 'stats';

  // Views grouped in the sidebar, like the mockup: analysis, reference, config.
  const groups = [
    {
      label: 'nav.analysis',
      items: [
        { id: 'stats', label: 'tab.stats', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
        { id: 'debriefs', label: 'tab.debriefs', hidden: true, icon: 'M6 2h9l5 5v15H6zM15 2v5h5M9 13h6M9 17h6' },
      ],
    },
    {
      label: 'nav.reference',
      items: [
        { id: 'aerodromes', label: 'tab.aerodromes', icon: 'M12 3v6M12 15v6M3 12h6M15 12h6' },
      ],
    },
    {
      label: 'nav.config',
      items: [
        { id: 'settings', label: 'tab.settings', icon: 'M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2' },
      ],
    },
  ];

  // Per-view subtitle, shown under the title in the topbar.
  const subtitles = {
    stats: 'sub.stats',
    debriefs: 'sub.debriefs',
    aerodromes: 'sub.aerodromes',
    settings: 'sub.settings',
  };

  // Title for the current view. A reactive declaration (not a function called in
  // the template): Svelte tracks `tab` here, whereas `{$t(title())}` referenced
  // only the function and never re-rendered.
  $: titleKey = (() => {
    for (const g of groups) for (const it of g.items) if (it.id === tab) return it.label;
    return 'app.title';
  })();

  function goTo(id) {
    tab = id;
  }

  // Open the live stream once, for the whole application: the mission name in the
  // topbar, the cockpit-hardware updates and the pause indicator all depend on it.
  onMount(() => {
    const stopStream = connect();
    checkForUpdates();
    const updateTimer = setInterval(() => checkForUpdates(), 12 * 60 * 60 * 1000);
    return () => { stopStream(); clearInterval(updateTimer); };
  });

  // Theatres drive the airfields view: load them once at startup.
  fetchTheatres().catch(() => {});
</script>

<div class="shell">
  <aside>
    <div class="brand">
      <img class="logo" src={logo} alt="" aria-hidden="true" />
      <span class="brand-text">
        <strong>{$t('app.title')}</strong>
        <small class="muted">DCS World companion</small>
      </span>
    </div>

    <nav>
      {#each groups as g (g.label)}
        <div class="group">{$t(g.label)}</div>
        {#each g.items as item (item.id)}
          {#if !item.hidden}
            <button class="nav-item" class:active={tab === item.id} on:click={() => goTo(item.id)}>
              <svg class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d={item.icon} /></svg>
              {$t(item.label)}
            </button>
          {/if}
        {/each}
      {/each}
    </nav>

    <div class="status">
      {#if $updateInfo?.status === 'available'}
        <a class="update-link" href={$updateInfo.downloadUrl} target="_blank" rel="noopener noreferrer">
          {$t('update.available')}: {$updateInfo.latestVersion}
        </a>
      {/if}
      <span class="live">
        <span class="dot" class:on={$connected}></span>
        {$connected ? $t('app.connected') : $t('app.offline')}
      </span>
    </div>
  </aside>

  <div class="content">
    <header class="topbar">
      <div class="titles">
        <h1>{$t(titleKey)}</h1>
        <span class="sub muted">{$t(subtitles[tab] || 'app.title')}</span>
      </div>
      <span class="spacer"></span>

      {#if $mission}
        <span class="mission" title={$mission.name}>
          <span class="mission-dot"></span>{$mission.name}
        </span>
      {/if}

      <label class="lang">
        <select value={$lang} on:change={(e) => setLang(e.currentTarget.value)} aria-label="Language">
          {#each LANGUAGES as l (l.id)}
            <option value={l.id}>{l.label}</option>
          {/each}
        </select>
      </label>
    </header>

    {#if $feedStatus === 'waiting'}
      <div class="pause-banner" title={$t('app.feedWaitingNote')}>
        <span class="pause-icon">◌</span>
        {$t('app.feedWaiting')} — <span class="pause-note">{$t('app.feedWaitingNote')}</span>
      </div>
    {:else if $feedStatus === 'interrupted'}
      <div class="pause-banner" title={$t('app.feedStoppedNote')}>
        <span class="pause-icon">⏸</span>
        {$t('app.feedStopped')} — <span class="pause-note">{$t('app.feedStoppedNote')}</span>
      </div>
    {/if}

    <main>
      {#if tab === 'debriefs'}
        <div class="view">
          <DebriefPanel />
        </div>
      {:else if tab === 'stats'}
        <div class="view">
          <CareerPanel />
        </div>
      {:else if tab === 'settings'}
        <div class="view">
          <SettingsPanel />
        </div>
      {:else}
        <div class="view">
          <AerodromePanel />
        </div>
      {/if}
    </main>
  </div>
</div>

<!-- Modal chart viewer: a scan is displayed whole, on top of everything. -->
<ChartViewer />

<style>
  /* Sidebar + content, per the redesign mockup. The panels themselves are
     unchanged; this only moves navigation to a rail and adds a titled topbar. */
  .shell {
    display: grid;
    grid-template-columns: 224px 1fr;
    height: 100%;
    min-width: 0;
  }

  aside {
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--panel);
    border-right: 1px solid var(--border);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.9rem 1rem 0.8rem;
  }

  .logo {
    width: 32px;
    height: 32px;
    flex: none;
    border-radius: 50%;
  }

  .brand-text {
    display: flex;
    flex-direction: column;
    line-height: 1.15;
    min-width: 0;
  }

  .brand-text strong {
    white-space: nowrap;
    font-size: 0.92rem;
  }

  .brand-text small {
    font-size: 0.7rem;
  }

  nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0.3rem 0.55rem;
    overflow-y: auto;
  }

  .group {
    padding: 0.7rem 0.6rem 0.3rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.09em;
    color: var(--muted);
    font-weight: 600;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.6rem;
    font-size: 0.84rem;
    color: var(--muted);
    background: none;
    border: 1px solid transparent;
    border-radius: 9px;
    cursor: pointer;
    text-align: left;
  }

  .nav-item:hover {
    color: var(--text);
    background: rgba(255, 255, 255, 0.03);
  }

  .nav-item.active {
    color: var(--text);
    background: var(--bg);
    border-color: var(--border);
    box-shadow: inset 2px 0 0 var(--blue);
  }

  .nav-item.active .ic {
    color: var(--blue);
  }

  .ic {
    width: 17px;
    height: 17px;
    flex: none;
    opacity: 0.9;
  }

  .status {
    margin-top: auto;
    padding: 0.8rem 1rem;
    border-top: 1px solid var(--border);
  }

  .update-link {
    display: block;
    margin-bottom: 0.6rem;
    color: var(--blue);
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  .live {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--muted);
    font-size: 0.8rem;
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

  .content {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  .topbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
    row-gap: 0.35rem;
    padding: 0.6rem 1.1rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
  }

  .titles {
    display: flex;
    flex-direction: column;
    line-height: 1.2;
    min-width: 0;
  }

  .titles h1 {
    margin: 0;
    font-size: 1.02rem;
    font-weight: 650;
  }

  .titles .sub {
    font-size: 0.74rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .spacer {
    flex: 1;
  }

  .mission {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    max-width: 260px;
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
    /* Take all the height the topbar and banners leave, and never shrink below
       the viewport: min-height: 0 is what lets the inner panel scroll instead of
       stretching the page. */
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

  .view {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
  }
</style>
