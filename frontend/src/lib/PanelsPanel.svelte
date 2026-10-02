<script>
  /**
   * Panels: the Logitech/Saitek flight panels the manager drives directly, and
   * the DCS-BIOS link it listens to. This is the cockpit-hardware side, absorbed
   * from the DCS Panel Manager.
   */
  import { onMount } from 'svelte';
  import {
    panels,
    panelsSupported,
    panelsError,
    panelsLoading,
    biosState,
    panelEvents,
    loadPanels,
    modelLabel,
    fmtIDs,
  } from './panels.js';
  import {
    bindings,
    sendingEnabled,
    mappingsError,
    mappingAircraft,
    controls,
    controlsAvailable,
    loadMappings,
    setSending,
    addBinding,
    removeBinding,
  } from './mappings.js';
  import { t } from './i18n.js';

  onMount(async () => {
    await loadPanels();
    // The bindings are per aircraft, and the aircraft comes from DCS-BIOS. Load
    // them once the link has told us which one is current.
    if ($biosState.aircraft) loadMappings($biosState.aircraft);
  });

  // The Refresh button reloads everything on this tab: the panels, the link, and
  // the bindings of the current aircraft. Reloading only the panels left a binding
  // saved through the API invisible until the aircraft changed.
  async function refreshAll() {
    await loadPanels();
    if ($biosState.aircraft) await loadMappings($biosState.aircraft);
  }

  // When the active aircraft changes, reload its bindings.
  let lastAircraft = '';
  $: if ($biosState.aircraft !== lastAircraft) {
    lastAircraft = $biosState.aircraft;
    if (lastAircraft) loadMappings(lastAircraft);
  }

  const KIND_KEYS = {
    connected: 'panels.kind.connected',
    disconnected: 'panels.kind.disconnected',
    input: 'panels.kind.input',
    error: 'panels.kind.error',
  };

  function kindLabel(kind) {
    return $t(KIND_KEYS[kind] ?? kind);
  }

  /** The panel a device path belongs to, for the event list. */
  function deviceLabel(path) {
    const p = $panels.find((d) => d.path === path);
    return p ? modelLabel(p.model) : path;
  }

  function timeOf(ms) {
    return new Date(ms).toLocaleTimeString();
  }

  // New-binding form state.
  let newPanel = 'pz70';
  let newControl = '';
  let newCommand = '';
  let newInterface = '';

  /** The panel controls of the chosen panel model, for the picker. */
  const PANEL_CONTROLS = {
    pz55: [
      'MASTER_BAT', 'MASTER_ALT', 'AVIONICS_MASTER', 'FUEL_PUMP', 'DE_ICE', 'PITOT_HEAT',
      'COWL', 'LIGHTS_PANEL', 'LIGHTS_BEACON', 'LIGHTS_NAV', 'LIGHTS_STROBE', 'LIGHTS_TAXI',
      'LIGHTS_LANDING', 'ENGINE_OFF', 'ENGINE_RIGHT', 'ENGINE_LEFT', 'ENGINE_BOTH',
      'ENGINE_START', 'GEAR_UP', 'GEAR_DOWN',
    ],
    pz70: [
      'KNOB_ALT', 'KNOB_VS', 'KNOB_IAS', 'KNOB_HDG', 'KNOB_CRS', 'LCD_WHEEL', 'AP_BUTTON',
      'HDG_BUTTON', 'NAV_BUTTON', 'IAS_BUTTON', 'ALT_BUTTON', 'VS_BUTTON', 'APR_BUTTON',
      'REV_BUTTON', 'AUTO_THROTTLE', 'FLAPS_UP', 'FLAPS_DOWN', 'PITCH_TRIM',
    ],
  };

  $: panelControlList = PANEL_CONTROLS[newPanel] ?? [];

  /** The interfaces the chosen command accepts, so only valid ones are offered. */
  $: chosenControl = $controls.find((c) => c.identifier === newCommand) ?? null;
  $: availableInterfaces = (chosenControl?.inputs ?? []).map((i) => i.interface);

  // Keep the chosen interface valid when the command changes.
  $: if (newCommand && availableInterfaces.length && !availableInterfaces.includes(newInterface)) {
    newInterface = availableInterfaces[0];
  }

  async function submitBinding() {
    if (!newControl || !newCommand || !newInterface) return;
    const ok = await addBinding({
      model: newPanel,
      control: newControl,
      command: newCommand,
      interface: newInterface,
    });
    if (ok) {
      newControl = '';
      newCommand = '';
      newInterface = '';
    }
  }
</script>

<section class="panels">
  <header>
    <h2>
      {$t('panels.title')}
      <span class="count">{$panels.length}</span>
    </h2>
    <button class="refresh" on:click={refreshAll} disabled={$panelsLoading}>
      {$t('stats.refresh')}
    </button>
  </header>

  {#if $panelsError}
    <p class="error">{$panelsError}</p>
  {/if}

  {#if !$panelsSupported}
    <p class="empty">{$t('panels.unsupported')}</p>
  {:else}
    <div class="block">
      <h3>
        {$t(($biosState.available ? 'panels.bios' : 'panels.biosUnavailable'))}
        <span class="badge" class:on={$biosState.connected}>
          {$biosState.connected ? $t('panels.connected') : $t('panels.disconnected')}
        </span>
      </h3>
      {#if $biosState.available}
        <p class="line">
          {#if $biosState.aircraft}
            {$t('panels.aircraft')}: <b>{$biosState.aircraft}</b>
          {:else}
            <span class="muted">{$t('panels.noAircraft')}</span>
          {/if}
        </p>
        <p class="line muted">{$biosState.frames} {$t('panels.frames')}</p>
      {:else}
        <p class="line muted">{$t('panels.biosUnavailableNote')}</p>
      {/if}
    </div>

    <div class="block">
      <h3>
        {$t('panels.mappings')}
        <span class="badge" class:on={$sendingEnabled}>
          {$sendingEnabled ? $t('panels.sendingOn') : $t('panels.sendingOff')}
        </span>
      </h3>

      {#if !$biosState.aircraft}
        <p class="empty">{$t('panels.mappingsNeedAircraft')}</p>
      {:else if !$controlsAvailable}
        <p class="empty">{$t('panels.noMetadata', { aircraft: $mappingAircraft })}</p>
      {:else}
        <label class="toggle safety">
          <input
            type="checkbox"
            checked={$sendingEnabled}
            on:change={(e) => setSending(e.currentTarget.checked)}
          />
          {$t('panels.enableSending')}
        </label>
        <p class="hint">{$t('panels.sendingHint')}</p>

        {#if $mappingsError}
          <p class="error">{$mappingsError}</p>
        {/if}

        {#if $bindings.length === 0}
          <p class="empty">{$t('panels.noMappings')}</p>
        {:else}
          <table class="bindings">
            <thead>
              <tr>
                <th>{$t('panels.col.panel')}</th>
                <th>{$t('panels.col.control')}</th>
                <th>{$t('panels.col.command')}</th>
                <th>{$t('panels.col.interface')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each $bindings as b (b.model + '/' + b.control)}
                <tr>
                  <td>{modelLabel(b.model)}</td>
                  <td class="mono">{b.control}</td>
                  <td class="mono">{b.command}{#if b.invert}<span class="tag">{$t('panels.inverted')}</span>{/if}</td>
                  <td class="mono">{b.interface}</td>
                  <td class="actions">
                    <button class="remove" on:click={() => removeBinding(b.model, b.control)} title={$t('panels.remove')}>×</button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}

        <div class="add">
          <select bind:value={newPanel} aria-label={$t('panels.col.panel')}>
            <option value="pz55">PZ55</option>
            <option value="pz70">PZ70</option>
          </select>
          <select bind:value={newControl} aria-label={$t('panels.col.control')}>
            <option value="">{$t('panels.pickControl')}</option>
            {#each panelControlList as c (c)}
              <option value={c}>{c}</option>
            {/each}
          </select>
          <select bind:value={newCommand} aria-label={$t('panels.col.command')}>
            <option value="">{$t('panels.pickCommand')}</option>
            {#each $controls as c (c.identifier)}
              <option value={c.identifier}>{c.identifier}</option>
            {/each}
          </select>
          <select bind:value={newInterface} aria-label={$t('panels.col.interface')} disabled={!availableInterfaces.length}>
            {#if !availableInterfaces.length}
              <option value="">{$t('panels.pickCommandFirst')}</option>
            {/if}
            {#each availableInterfaces as iface (iface)}
              <option value={iface}>{iface}</option>
            {/each}
          </select>
          <button
            class="add-btn"
            on:click={submitBinding}
            disabled={!newControl || !newCommand || !newInterface}
          >
            {$t('panels.add')}
          </button>
        </div>
      {/if}
    </div>

    <div class="block">
      <h3>{$t('panels.devices')}</h3>
      {#if $panels.length === 0}
        <p class="empty">{$t('panels.none')}</p>
      {:else}
        <div class="cards">
          {#each $panels as d (d.Path ?? d.path)}
            <div class="card">
              <div class="row">
                <strong>{modelLabel(d.Model ?? d.model)}</strong>
                <span class="badge on">{$t('panels.connected')}</span>
              </div>
              <dl>
                <div><dt>{$t('panels.ids')}</dt><dd>{fmtIDs(d.VendorID ?? d.vendorId, d.ProductID ?? d.productId)}</dd></div>
                {#if d.Product ?? d.product}
                  <div><dt>{$t('panels.product')}</dt><dd>{d.Product ?? d.product}</dd></div>
                {/if}
                {#if d.Serial ?? d.serial}
                  <div><dt>{$t('panels.serial')}</dt><dd>{d.Serial ?? d.serial}</dd></div>
                {/if}
              </dl>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="block">
      <h3>
        {$t('panels.monitor')}
        <span class="count">{$panelEvents.length}</span>
      </h3>
      {#if $panelEvents.length === 0}
        <p class="empty">{$t('panels.noEvents')}</p>
      {:else}
        <ul class="events">
          {#each $panelEvents as e (`${e.at}-${e.device}-${e.input?.control ?? e.kind}`)}
            <li class:error={e.kind === 'error'}>
              <span class="t">{timeOf(e.at)}</span>
              <span class="dev">{deviceLabel(e.device)}</span>
              {#if e.kind === 'input'}
                <span class="ctl">{e.input.control}</span>
                <span class="state" class:on={e.input.active}>
                  {e.input.active ? $t('panels.on') : $t('panels.off')}
                </span>
              {:else if e.kind === 'error'}
                <span class="msg">{e.error}</span>
              {:else}
                <span class="msg">{kindLabel(e.kind)}</span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</section>

<style>
  .panels {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    padding: 0.9rem 1rem;
    overflow-y: auto;
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
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
  }

  .count {
    color: var(--text);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
    text-transform: none;
    letter-spacing: 0;
    font-size: 0.76rem;
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

  .block {
    margin-bottom: 1.1rem;
  }

  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .line {
    margin: 0.2rem 0;
    font-size: 0.78rem;
    color: var(--text);
  }

  .line.muted {
    color: var(--muted);
  }

  .badge {
    padding: 0.05rem 0.4rem;
    border-radius: 999px;
    font-size: 0.66rem;
    color: var(--muted);
    border: 1px solid var(--border);
  }

  .badge.on {
    color: var(--green);
    border-color: color-mix(in srgb, var(--green) 45%, var(--border));
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 0.5rem;
    max-width: 820px;
  }

  /* Mapping editor: the safety switch, the table of bindings, the add row. */
  .toggle.safety {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    cursor: pointer;
  }

  .hint {
    margin: 0.25rem 0 0.5rem;
    font-size: 0.72rem;
    color: var(--muted);
    max-width: 70ch;
    line-height: 1.5;
  }

  table.bindings {
    width: 100%;
    max-width: 820px;
    border-collapse: collapse;
    font-size: 0.76rem;
  }

  table.bindings th {
    text-align: left;
    font-weight: 500;
    color: var(--muted);
    padding: 0.25rem 0.4rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.7rem;
  }

  table.bindings td {
    padding: 0.25rem 0.4rem;
    border-top: 1px solid var(--border);
  }

  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.72rem;
  }

  td.actions {
    text-align: right;
    width: 2rem;
  }

  .remove {
    padding: 0 0.35rem;
    color: var(--muted);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    cursor: pointer;
    line-height: 1.4;
  }

  .remove:hover {
    color: var(--red);
    border-color: var(--red);
  }

  .tag {
    margin-left: 0.35rem;
    padding: 0 0.3rem;
    font-size: 0.62rem;
    text-transform: uppercase;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 999px;
  }

  .add {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    margin-top: 0.5rem;
    max-width: 820px;
  }

  .add select {
    padding: 0.28rem 0.4rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.74rem;
    cursor: pointer;
    max-width: 14rem;
  }

  .add select:focus {
    outline: none;
    border-color: var(--blue);
  }

  .add-btn {
    padding: 0.28rem 0.6rem;
    font-size: 0.74rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }

  .add-btn:hover:not(:disabled) {
    border-color: var(--blue);
  }

  .add-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .card {
    padding: 0.5rem 0.6rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.35rem;
  }

  .row strong {
    font-size: 0.82rem;
  }

  dl {
    margin: 0;
    display: grid;
    gap: 0.15rem;
    font-size: 0.74rem;
  }

  dl div {
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
  }

  dt {
    color: var(--muted);
  }

  dd {
    margin: 0;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.72rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .events {
    list-style: none;
    margin: 0;
    padding: 0;
    font-size: 0.76rem;
    max-width: 820px;
    max-height: 320px;
    overflow-y: auto;
  }

  .events li {
    display: flex;
    gap: 0.5rem;
    padding: 0.15rem 0.3rem;
    border-top: 1px solid var(--border);
  }

  .events li:first-child {
    border-top: none;
  }

  .events li.error {
    color: #f0b429;
  }

  .t {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    flex: none;
  }

  .dev {
    color: var(--blue);
    flex: none;
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ctl {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.72rem;
  }

  .state {
    color: var(--muted);
    flex: none;
  }

  .state.on {
    color: var(--green);
  }

  .msg {
    color: var(--muted);
    overflow-wrap: anywhere;
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
