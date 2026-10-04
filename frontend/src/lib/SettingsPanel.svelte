<script>
  /**
   * Settings: everything that configures the manager or the DCS side, gathered
   * behind one tab. Its inner sub-tabs are the panels that used to sit in the
   * main bar on their own — installed modules, the DCS-side scripts and mods, and
   * the cockpit panels (DCS-BIOS and the Logitech hardware).
   */
  import ModulesPanel from './ModulesPanel.svelte';
  import InstallPanel from './InstallPanel.svelte';
  import PanelsPanel from './PanelsPanel.svelte';
  import { t } from './i18n.js';

  // "install" first: it is the place to check what is set up on the DCS side.
  let section = 'install';

  const SECTIONS = [
    { id: 'install', label: 'tab.install' },
    { id: 'modules', label: 'tab.modules' },
    { id: 'panels', label: 'tab.panels' },
  ];
</script>

<section class="settings">
  <nav class="subtabs">
    {#each SECTIONS as s (s.id)}
      <button class:active={section === s.id} on:click={() => (section = s.id)}>{$t(s.label)}</button>
    {/each}
  </nav>

  <div class="body">
    {#if section === 'install'}
      <InstallPanel />
    {:else if section === 'modules'}
      <ModulesPanel />
    {:else}
      <PanelsPanel />
    {/if}
  </div>
</section>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }

  .subtabs {
    display: flex;
    gap: 0.2rem;
    flex: none;
    padding: 0.4rem 0.9rem 0.5rem;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
  }

  .subtabs button {
    padding: 0.28rem 0.7rem;
    font-size: 0.8rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
  }

  .subtabs button:hover {
    color: var(--text);
    border-color: var(--border);
  }

  .subtabs button.active {
    color: var(--text);
    background: var(--panel);
    border-color: var(--border);
  }

  .body {
    flex: 1 1 auto;
    min-height: 0;
    min-width: 0;
  }
</style>
