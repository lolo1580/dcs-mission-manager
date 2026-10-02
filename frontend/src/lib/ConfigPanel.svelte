<script>
  /**
   * DCS configuration: the game's own settings, read from
   * Saved Games\DCS\Config — the options.lua groups (graphics, difficulty, VR…),
   * the pluginsEnabled.lua toggles and the manager's own config.
   */
  import { onMount } from 'svelte';
  import {
    dcsConfig,
    configError,
    configLoading,
    configSection,
    configSections,
    currentSection,
    disabledPlugins,
    loadDCSConfig,
  } from './dcsconfig.js';
  import { t } from './i18n.js';

  onMount(loadDCSConfig);

  const SECTION_KEYS = {
    graphics: 'config.section.graphics',
    difficulty: 'config.section.difficulty',
    VR: 'config.section.vr',
    sound: 'config.section.sound',
    views: 'config.section.views',
    cockpit: 'config.section.cockpit',
    miscellaneous: 'config.section.miscellaneous',
    plugins: 'config.section.plugins',
  };

  function sectionLabel(name) {
    const key = SECTION_KEYS[name];
    return key ? $t(key) : name;
  }
</script>

<section class="config">
  <header>
    <h2>{$t('config.title')}</h2>
    <button class="refresh" on:click={loadDCSConfig} disabled={$configLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  {#if $configError}
    <p class="error">{$configError}</p>
  {/if}

  {#if !$dcsConfig || ($dcsConfig.sections ?? []).length === 0}
    <p class="empty">{$t('config.none')}</p>
  {:else}
    <div class="meta">
      {#if $dcsConfig.language}
        <span class="chip">{$t('config.language')}: <b>{$dcsConfig.language}</b></span>
      {/if}
      <span class="chip">{$t('config.path')}: <code>{$dcsConfig.path}</code></span>
    </div>

    {#if $disabledPlugins.length}
      <p class="off">
        {$t('config.pluginsOff')}
        {#each $disabledPlugins as p (p.name)}<span class="tag">{p.name}</span>{/each}
      </p>
    {/if}

    <div class="tabs">
      {#each $configSections as name (name)}
        <button
          class:active={$currentSection?.name === name}
          on:click={() => configSection.set(name)}
        >
          {sectionLabel(name)}
          <span class="n">{$dcsConfig.sections.find((s) => s.name === name)?.settings.length ?? 0}</span>
        </button>
      {/each}
    </div>

    {#if $currentSection}
      <table>
        <thead>
          <tr><th>{$t('config.col.setting')}</th><th>{$t('config.col.value')}</th></tr>
        </thead>
        <tbody>
          {#each $currentSection.settings as s (s.key)}
            <tr>
              <td class="key">{s.key}</td>
              <td class="val" class:off={s.value === 'off'} class:on={s.value === 'on'}>{s.value}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}

    {#if ($dcsConfig.manager ?? []).length}
      <div class="block">
        <h3>{$t('config.manager')}</h3>
        <table>
          <tbody>
            {#each $dcsConfig.manager as s (s.key)}
              <tr><td class="key">{s.key}</td><td class="val">{s.value}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}
</section>

<style>
  .config {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    overflow: hidden;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
  }

  h2 {
    margin: 0;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  .refresh {
    padding: 0.25rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .refresh:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--blue);
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    margin-bottom: 0.5rem;
    font-size: 0.74rem;
    color: var(--muted);
  }

  .chip code {
    color: var(--text);
    font-size: 0.72rem;
  }

  .off {
    margin: 0 0 0.6rem;
    font-size: 0.76rem;
    color: #f0b429;
  }

  .tag {
    margin-left: 0.4rem;
    padding: 0.05rem 0.4rem;
    border: 1px solid color-mix(in srgb, #f0b429 45%, var(--border));
    border-radius: 999px;
    font-size: 0.7rem;
  }

  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin-bottom: 0.6rem;
  }

  .tabs button {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.22rem 0.55rem;
    font-size: 0.74rem;
    color: var(--muted);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    cursor: pointer;
  }

  .tabs button.active {
    color: var(--text);
    border-color: var(--blue);
    background: color-mix(in srgb, var(--blue) 18%, var(--bg));
  }

  .tabs .n {
    opacity: 0.6;
    font-variant-numeric: tabular-nums;
    font-size: 0.68rem;
  }

  table {
    width: 100%;
    max-width: 720px;
    border-collapse: collapse;
    font-size: 0.78rem;
    overflow-y: auto;
  }

  th {
    position: sticky;
    top: 0;
    text-align: left;
    font-weight: 500;
    color: var(--muted);
    padding: 0.3rem 0.4rem;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    font-size: 0.7rem;
  }

  th:nth-child(2),
  td.val {
    text-align: right;
  }

  td {
    padding: 0.26rem 0.4rem;
    border-top: 1px solid var(--border);
    vertical-align: baseline;
  }

  td.key {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.74rem;
    color: var(--text);
  }

  td.val {
    font-variant-numeric: tabular-nums;
    color: var(--muted);
    white-space: nowrap;
  }

  td.val.on {
    color: var(--green);
  }

  td.val.off {
    color: #f0b429;
  }

  .block {
    margin-top: 1rem;
  }

  .block h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
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
