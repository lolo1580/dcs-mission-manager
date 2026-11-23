<script>
  import ChartViewer from './lib/ChartViewer.svelte';
  import logo from './assets/logo.png';
  import OverviewPanel from './lib/OverviewPanel.svelte';
  import ServerOverview from './lib/ServerOverview.svelte';
  import FlightsPanel from './lib/FlightsPanel.svelte';
  import CareerPanel from './lib/CareerPanel.svelte';
  import StatsPanel from './lib/StatsPanel.svelte';
  import ChecklistsPanel from './lib/ChecklistsPanel.svelte';
  import AerodromePanel from './lib/AerodromePanel.svelte';
  import ModsPanel from './lib/ModsPanel.svelte';
  import SettingsPanel from './lib/SettingsPanel.svelte';
  import ProfilePanel from './lib/ProfilePanel.svelte';
  import AdminPanel from './lib/AdminPanel.svelte';
  import SchoolPanel from './lib/SchoolPanel.svelte';
  import CulturePanel from './lib/CulturePanel.svelte';
  import LoginPage from './lib/LoginPage.svelte';
  import ResetPasswordPage from './lib/ResetPasswordPage.svelte';
  import Toasts from './lib/ui/Toasts.svelte';
  import { connected, feedStatus, mission, fetchTheatres, connect } from './lib/units.js';
  import { t, lang, LANGUAGES, setLang } from './lib/i18n.js';
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { updateInfo, checkForUpdates } from './lib/updates.js';
  import { NAV_GROUPS, SERVER_NAV_GROUPS, routeID, routeByID, initRouter, navigate } from './lib/router.js';
  import {
    appMode,
    sessionUser,
    sessionLoading,
    isServer,
    isAdmin,
    loadSession,
    logout,
  } from './lib/session.js';

  const SERVER_ROUTES = new Set(SERVER_NAV_GROUPS.flatMap((group) => group.items.map((item) => item.id)));
  const DESKTOP_ROUTES = new Set(NAV_GROUPS.flatMap((group) => group.items.map((item) => item.id)));
  const resetPath = typeof window !== 'undefined' && (window.location.pathname.replace(/\/+$/, '') || '/') === '/reset';

  $: navGroups = ($isServer ? SERVER_NAV_GROUPS : NAV_GROUPS)
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => item.id !== 'admin' || $isAdmin),
    }))
    .filter((group) => group.items.length > 0);

  // A server link opened on a desktop-only path (or the reverse) lands on the
  // profile's home page instead of rendering a page with missing data. These
  // guards wait until the session probe has finished, so an administrator link
  // is not redirected away during the first render.
  $: if (!resetPath && !$sessionLoading && $isServer && !SERVER_ROUTES.has($routeID)) navigate('squadron');
  $: if (!resetPath && !$sessionLoading && !$isServer && !DESKTOP_ROUTES.has($routeID)) navigate('overview');
  $: if (!resetPath && !$sessionLoading && $sessionUser && $isServer && $routeID === 'admin' && !$isAdmin) navigate('profile');

  // Keep the window title in step with the client-side route. The route is
  // resolved from the store here (and in the template) rather than through an
  // intermediate variable, so the first shell render after login is already
  // correct.
  $: if (typeof document !== 'undefined') {
    const current = routeByID($routeID);
    document.title = resetPath
      ? `${$t('reset.title')} · ${$t('app.title')}`
      : $isServer && !$sessionUser
        ? `${$t('login.title')} · ${$t('app.title')}`
        : `${$t(current.label)} · ${$t('app.title')}`;
  }

  // Open the live stream once, for the whole desktop application. The server
  // profile has no telemetry, so it never opens it.
  onMount(() => {
    const stopRouter = resetPath ? () => {} : initRouter();
    let stopStream = null;
    let updateTimer = null;
    let cancelled = false;

    loadSession().then(() => {
      if (cancelled || get(isServer)) return;
      stopStream = connect();
      checkForUpdates();
      updateTimer = setInterval(() => checkForUpdates(), 12 * 60 * 60 * 1000);
    });
    return () => {
      cancelled = true;
      stopRouter();
      if (stopStream) stopStream();
      if (updateTimer) clearInterval(updateTimer);
    };
  });

  // Theatres drive the airfields view on both profiles. On the server they are
  // requested only once a session exists; the endpoint is protected.
  $: if (!$isServer || $sessionUser) fetchTheatres().catch(() => {});

  async function signOut() {
    await logout();
    navigate('squadron');
  }
</script>

{#if resetPath}
  <ResetPasswordPage />
{:else if $sessionLoading}
  <div class="boot">
    <img class="boot-logo" src={logo} alt="" aria-hidden="true" />
    <p>{$t('app.title')}</p>
  </div>
{:else if $isServer && !$sessionUser}
  <LoginPage />
{:else}
<div class="shell">
  <aside>
    <a class="brand" href="/overview" on:click|preventDefault={() => navigate('overview')}>
      <img class="logo" src={logo} alt="" aria-hidden="true" />
      <span class="brand-text">
        <strong>{$t('app.title')}</strong>
        <small class="muted">DCS World companion</small>
      </span>
    </a>

    <nav>
      {#each navGroups as group (group.label)}
        <div class="group">{$t(group.label)}</div>
        {#each group.items as item (item.id)}
          <a
            class="nav-item"
            class:active={$routeID === item.id}
            href={item.path}
            title={$t(item.label)}
            aria-current={$routeID === item.id ? 'page' : undefined}
            on:click|preventDefault={() => navigate(item.id)}
          >
            <svg class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d={item.icon} /></svg>
            <span class="label">{$t(item.label)}</span>
          </a>
        {/each}
      {/each}
    </nav>

    <div class="status">
      {#if $isServer}
        <div class="user">
          <span class="avatar">{$sessionUser?.username?.slice(0, 2).toUpperCase() ?? '?'}</span>
          <div class="user-text">
            <strong class="label">{$sessionUser?.displayName || $sessionUser?.username}</strong>
            <small class="label">{$isAdmin ? $t('admin.role.admin') : $t('admin.role.member')}</small>
          </div>
          <button class="logout" type="button" on:click={signOut} title={$t('login.logout')} aria-label={$t('login.logout')}>⎋</button>
        </div>
      {:else}
        {#if $updateInfo?.status === 'available'}
          <a class="update-link" href={$updateInfo.downloadUrl} target="_blank" rel="noopener noreferrer">
            {$t('update.available')}: {$updateInfo.latestVersion}
          </a>
        {/if}
        <div class="status-row">
          <span class="live" title={$connected ? $t('app.connected') : $t('app.offline')}>
            <span class="dot" class:on={$connected}></span>
            <span class="label">{$connected ? $t('app.connected') : $t('app.offline')}</span>
          </span>
          {#if $updateInfo?.currentVersion}
            <span class="installed-version label" title="DCS Manager {$updateInfo.currentVersion}">v{$updateInfo.currentVersion}</span>
          {/if}
        </div>
      {/if}
    </div>
  </aside>

  <div class="content">
    <header class="topbar">
      <div class="topbar-inner">
        <div class="titles">
          <h1>{$t(routeByID($routeID).label)}</h1>
          <span class="sub muted">{$t(routeByID($routeID).subtitle)}</span>
        </div>
        <span class="spacer"></span>

        {#if $mission && !$isServer}
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
      </div>
    </header>

    {#if !$isServer}
      {#if $feedStatus === 'waiting'}
        <div class="pause-banner" title={$t('app.feedWaitingNote')}>
          <div class="pause-inner">
            <span class="pause-icon">◌</span>
            {$t('app.feedWaiting')} — <span class="pause-note">{$t('app.feedWaitingNote')}</span>
          </div>
        </div>
      {:else if $feedStatus === 'interrupted'}
        <div class="pause-banner" title={$t('app.feedStoppedNote')}>
          <div class="pause-inner">
            <span class="pause-icon">⏸</span>
            {$t('app.feedStopped')} — <span class="pause-note">{$t('app.feedStoppedNote')}</span>
          </div>
        </div>
      {/if}
    {/if}

    <main>
      {#if $isServer}
        {#if $routeID === 'squadron'}
          <div class="view"><ServerOverview /></div>
        {:else if $routeID === 'flights'}
          <div class="view"><FlightsPanel /></div>
        {:else if $routeID === 'stats'}
          <div class="view"><StatsPanel /></div>
        {:else if $routeID === 'checklists'}
          <div class="view"><ChecklistsPanel /></div>
        {:else if $routeID === 'airfields'}
          <div class="view"><AerodromePanel /></div>
        {:else if $routeID === 'school'}
          <div class="view"><SchoolPanel /></div>
        {:else if $routeID === 'culture'}
          <div class="view"><CulturePanel /></div>
        {:else if $routeID === 'admin' && $isAdmin}
          <div class="view"><AdminPanel /></div>
        {:else if $routeID === 'profile'}
          <div class="view"><ProfilePanel /></div>
        {:else}
          <div class="view"><ServerOverview /></div>
        {/if}
      {:else if $routeID === 'overview'}
        <div class="view"><OverviewPanel /></div>
      {:else if $routeID === 'flights'}
        <div class="view"><FlightsPanel /></div>
      {:else if $routeID === 'career'}
        <div class="view"><CareerPanel /></div>
      {:else if $routeID === 'stats'}
        <div class="view"><StatsPanel /></div>
      {:else if $routeID === 'checklists'}
        <div class="view"><ChecklistsPanel /></div>
      {:else if $routeID === 'mods'}
        <div class="view"><ModsPanel /></div>
      {:else if $routeID === 'settings'}
        <div class="view"><SettingsPanel /></div>
      {:else}
        <div class="view"><AerodromePanel /></div>
      {/if}
    </main>
  </div>
</div>
{/if}

<!-- Modal chart viewer: a scan is displayed whole, on top of everything. -->
<ChartViewer />
<Toasts />

<style>
  .shell {
    display: grid;
    grid-template-columns: 224px minmax(0, 1fr);
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
    color: inherit;
    text-decoration: none;
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
    font-size: 0.66rem;
    text-transform: uppercase;
    letter-spacing: 0.09em;
    color: var(--faint);
    font-weight: 650;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.6rem;
    font-size: 0.84rem;
    color: var(--muted);
    text-decoration: none;
    border: 1px solid transparent;
    border-radius: var(--radius);
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
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  .status-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
  }

  .installed-version {
    color: var(--muted);
    font-size: 0.7rem;
    white-space: nowrap;
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
    box-shadow: 0 0 0 3px var(--green-soft);
  }

  .user {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }

  .user .avatar {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    flex: none;
    border-radius: 50%;
    background: var(--blue-soft);
    color: var(--blue);
    font-size: 0.68rem;
    font-weight: 700;
  }

  .user-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: 1.2;
  }

  .user-text strong {
    font-size: 0.78rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .user-text small {
    color: var(--muted);
    font-size: 0.66rem;
  }

  .logout {
    margin-left: auto;
    padding: 0.15rem 0.3rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: 0.9rem;
  }

  .logout:hover {
    color: var(--text);
    border-color: var(--border);
  }

  .boot {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.6rem;
    height: 100%;
    color: var(--muted);
    font-size: 0.85rem;
  }

  .boot-logo {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    animation: pulse 1.4s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.55; }
    50% { opacity: 1; }
  }

  .content {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  .topbar {
    padding: 0.6rem 1.1rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
  }

  .topbar-inner {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
    row-gap: 0.35rem;
    width: 100%;
    max-width: var(--content-max);
    margin: 0 auto;
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
    border-radius: var(--radius-sm);
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
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
    cursor: pointer;
  }

  .lang select:focus {
    outline: none;
    border-color: var(--blue);
  }

  main {
    display: flex;
    flex: 1 1 auto;
    min-height: 0;
    min-width: 0;
  }

  .pause-banner {
    padding: 0.35rem 1.1rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
  }

  .pause-inner {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
    width: 100%;
    max-width: var(--content-max);
    margin: 0 auto;
    font-size: 0.78rem;
    color: #9aa4b2;
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

  /* Narrow windows (tablet, small laptop): keep the navigation usable as an
     icon rail rather than letting it squeeze the content. */
  @media (max-width: 900px) {
    .shell {
      grid-template-columns: 62px minmax(0, 1fr);
    }

    .brand {
      justify-content: center;
      padding: 0.75rem 0.4rem;
    }

    .brand-text,
    .group,
    .nav-item .label,
    .status .label {
      display: none;
    }

    .nav-item {
      justify-content: center;
      padding: 0.55rem 0.4rem;
    }

    .status {
      padding: 0.7rem 0.4rem;
    }

    .status-row,
    .live {
      justify-content: center;
    }

    .installed-version {
      display: none;
    }
  }

  @media (max-width: 560px) {
    .shell {
      grid-template-columns: 52px minmax(0, 1fr);
    }

    .topbar {
      padding: 0.5rem 0.7rem;
    }

    .titles .sub {
      display: none;
    }

    .mission {
      display: none;
    }
  }
</style>
