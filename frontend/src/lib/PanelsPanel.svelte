<script>
  /**
   * Panels: the Logitech/Saitek flight panels the manager drives directly, and
   * the DCS-BIOS link it listens to. This is the cockpit-hardware side, absorbed
   * from the DCS Panel Manager.
   */
  import { onMount, tick } from 'svelte';
  import PanelBoard from './PanelBoard.svelte';
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
    outputs,
    displays,
    sendingEnabled,
    outputsEnabled,
    testMode,
    mappingEvents,
    displayPreview,
    mappingsError,
    mappingsLoading,
    mappingAircraft,
    aircraftList,
    controls,
    controlsAvailable,
    loadMappings,
    loadAircraft,
    setSending,
    setOutputs,
    setTestMode,
    addBinding,
    removeBinding,
    addOutput,
    removeOutput,
    addDisplay,
    removeDisplay,
    previewDisplay,
    testDisplay,
    saveProfile,
  } from './mappings.js';
  import { t } from './i18n.js';

  let pluginState = {connected: false, aircraft: '', accepted: 0};
  let pluginBusy = false;
  let pluginError = '';
  async function setPluginTrim(enabled) {
    pluginBusy = true; pluginError = '';
    try {
      const r = await fetch('/api/panels/plugin', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({trimEnabled:enabled})});
      const body = await r.json();
      if (!r.ok) { if (body.state) pluginState=body.state; throw new Error(body.error || r.status); }
      pluginState=body;
    } catch (e) { pluginError=e.message; }
    finally { pluginBusy=false; }
  }
  async function loadPlugin() {
    try {
      const r = await fetch('/api/panels/plugin');
      if (!r.ok) throw new Error('plugin status unavailable');
      pluginState = await r.json();
    } catch (_) { pluginState = {connected: false, aircraft: '', accepted: 0}; }
  }
  onMount(() => { loadPlugin(); const timer = setInterval(loadPlugin, 2000); return () => clearInterval(timer); });

  onMount(async () => {
    await loadPanels();
    await loadAircraft();
    // The bindings are per aircraft. Prefer the one DCS-BIOS reports as active;
    // when DCS is not running (no link), fall back to the first profile so the
    // mappings can still be edited and tested without a mission loaded.
    const want = $biosState.aircraft || $aircraftList[0] || '';
    if (want) loadMappings(want);
  });

  // The Refresh button reloads everything on this tab: the panels, the link, and
  // the bindings of the current aircraft. Reloading only the panels left a binding
  // saved through the API invisible until the aircraft changed.
  async function refreshAll() {
    await loadPanels();
    await loadAircraft();
    const aircraft = $mappingAircraft || $biosState.aircraft;
    if (aircraft) await loadMappings(aircraft);
  }

  // When the active aircraft changes, reload its bindings — but only if the user
  // has not picked another one manually below.
  let lastAircraft = '';
  let manualAircraft = false;
  $: if ($biosState.aircraft && $biosState.aircraft !== lastAircraft) {
    lastAircraft = $biosState.aircraft;
    if (!manualAircraft) loadMappings($biosState.aircraft);
  }

  // The operator can pick the aircraft by hand: needed to edit or test a profile
  // when DCS (and so DCS-BIOS) is not running, which is exactly when a mapping is
  // set up.
  function pickAircraft(e) {
    const name = e.currentTarget.value;
    manualAircraft = true;
    if (name) loadMappings(name);
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
  let newInvert = false;
  let newMode = '';
  let customStates = false;
  let stateOn = 1;
  let stateOff = 0;
  let pulseEnabled = false;
  let pulseReset = 1;
  let editingBinding = null;
  let editingOutput = null;
  let editingDisplay = null;
  let ruleRows = [];
  let importPreview = null;
  let profileNotice = '';
  let editorAircraft = '';
  let boardModel = 'pz70';
  let boardMode = 'ALT';
  let editorTab = 'input';
  let selected = '';
  let commandSearch = '';
  let sourceSearch = '';
  let inputEditor;
  let outputEditor;
  let lcdEditor;
  $: commandChoices = $controls.filter(c => c.inputs?.length && matchesSearch(c, commandSearch));
  $: sourceChoices = $controls.filter(c => c.outputs?.length && matchesSearch(c, sourceSearch));
  function matchesSearch(c, query) {
    return `${c.identifier} ${c.description || ''} ${c.category || ''}`.toLowerCase().includes(query.trim().toLowerCase());
  }
  function commandLabel(c) { return c.description ? `${c.description} — ${c.identifier}` : c.identifier; }
  async function selectControl({ detail: choice }) {
    boardModel = choice.model; editorTab = choice.kind;
    commandSearch = ''; sourceSearch = '';
    if (choice.kind === 'input') {
      selected = `input/${choice.id}/${choice.mode}`;
      const b = $bindings.find(b => b.model === choice.model && b.control === choice.id && (b.mode || '').toUpperCase() === choice.mode);
      if (b) editBinding(b);
      else {
        editingBinding = null; newPanel = choice.model; newControl = choice.id; newMode = choice.mode;
        newCommand = ''; newInterface = ''; newInvert = false;
        customStates = false; stateOn = 1; stateOff = 0;
        pulseEnabled = false; pulseReset = 1;
      }
    } else if (choice.kind === 'output') {
      selected = `output/${choice.id}`;
      const o = $outputs.find(o => o.model === choice.model && o.target === choice.id);
      if (o) editOutput(o);
      else { editingOutput = null; outPanel = choice.model; outTarget = choice.id; outCommand = ''; outColor = 'green'; ruleRows = []; }
    } else {
      selected = `display/${choice.mode}/${choice.id}`;
      const d = $displays.find(d => d.mode.toUpperCase() === choice.mode && d.line === choice.id);
      if (d) editDisplay(d);
      else { editingDisplay = null; dispMode = choice.mode; dispLine = choice.id; dispCommand = ''; dispExport = 0; dispScale = 1; dispOffset = 0; dispUnit = ''; }
    }
    await tick();
    const editor = choice.kind === 'input' ? inputEditor : choice.kind === 'output' ? outputEditor : lcdEditor;
    editor?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    editor?.querySelector('input[type="search"]')?.focus({ preventScroll: true });
  }
  $: if ($mappingAircraft !== editorAircraft) {
    editorAircraft = $mappingAircraft;
    editingBinding = null; editingOutput = null; editingDisplay = null;
    selected = ''; commandSearch = ''; sourceSearch = '';
    newControl = ''; newCommand = ''; newMode = ''; newInvert = false;
    customStates = false;
    outTarget = ''; outCommand = ''; ruleRows = []; dispCommand = '';
  }
  function editBinding(b) {
    commandSearch = '';
    editingBinding = b; newPanel = b.model; newControl = b.control;
    newCommand = b.command; newInterface = b.interface; newInvert = !!b.invert; newMode = b.mode || '';
    customStates = b.state_on != null || b.state_off != null;
    stateOn = b.state_on ?? ($controls.find(c => c.identifier === b.command)?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 1);
    stateOff = b.state_off ?? 0;
    pulseEnabled = b.pulse_reset != null;
    pulseReset = b.pulse_reset ?? 1;
  }
  function editOutput(o) {
    sourceSearch = '';
    editingOutput = o; outPanel = o.model; outTarget = o.target; outCommand = o.command; outColor = o.color || 'green';
    ruleRows = (o.rules || []).map(r => ({ ...r }));
  }
  function editDisplay(d) {
    sourceSearch = '';
    editingDisplay = d; dispMode = d.mode; dispLine = d.line; dispCommand = d.command;
    dispExport = d.export || 0; dispScale = d.scale || 1; dispOffset = d.offset || 0; dispUnit = d.unit || '';
  }
  function exportProfile() {
    const profile = { aircraft: $mappingAircraft, bindings: $bindings, outputs: $outputs, displays: $displays };
    const url = URL.createObjectURL(new Blob([JSON.stringify(profile, null, 2)], { type: 'application/json' }));
    const a = document.createElement('a'); a.href = url; a.download = `${$mappingAircraft}-panels.json`; a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function readProfile(event) {
    importPreview = null; profileNotice = '';
    try {
      const file = event.currentTarget.files[0];
      if (!file) return;
      if (file.size > 1024 * 1024) throw new Error($t('panels.invalidProfile'));
      const p = JSON.parse(await file.text());
      if (!p || !Array.isArray(p.bindings) || !Array.isArray(p.outputs) || (p.displays != null && !Array.isArray(p.displays))) throw new Error($t('panels.invalidProfile'));
      importPreview = { aircraft: p.aircraft || '', bindings: p.bindings, outputs: p.outputs, displays: p.displays || [] };
    } catch (e) { profileNotice = e.message; }
  }
  async function importProfile() {
    if (importPreview && await saveProfile(importPreview)) { importPreview = null; await loadAircraft(); }
  }

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
    const usePulse = pulseEnabled && newInterface === 'set_state' && ['LCD_WHEEL', 'PITCH_TRIM'].includes(newControl);
    if (usePulse && (!Number.isInteger(pulseReset) || pulseReset < 0 || pulseReset > (chosenControl?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 65535))) {
      mappingsError.set($t('panels.invalidStates', {maximum: chosenControl?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 65535}));
      return;
    }
    if (newInterface === 'set_state' && customStates) {
      const maximum = chosenControl?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 65535;
      if (![stateOn, stateOff].every(value => Number.isInteger(value) && value >= 0 && value <= maximum)) {
        mappingsError.set($t('panels.invalidStates', {maximum}));
        return;
      }
    }
    const ok = await addBinding({
      model: newPanel,
      control: newControl,
      command: newCommand,
      interface: newInterface,
      invert: newInvert,
      mode: newPanel === 'pz70' && newControl === 'LCD_WHEEL' ? newMode : '',
      ...(newInterface === 'set_state' && customStates ? {state_on: Number(stateOn), state_off: Number(stateOff)} : {}),
      ...(usePulse ? {pulse_reset: Number(pulseReset)} : {}),
    }, editingBinding);
    if (ok) {
      newControl = '';
      newCommand = '';
      newInterface = '';
      newInvert = false;
      newMode = ''; editingBinding = null;
      pulseEnabled = false;
      customStates = false;
    }
  }

  // LED output targets per panel, and the colours the PZ55's bicolour indicators
  // accept (the PZ70's are single-colour). Kept in step with internal/panel.
  const PANEL_OUTPUTS = {
    pz55: [
      'LIGHT_GEAR_UPPER', 'LIGHT_GEAR_LEFT', 'LIGHT_GEAR_RIGHT',
    ],
    pz70: [
      'LIGHT_AP', 'LIGHT_HDG', 'LIGHT_NAV', 'LIGHT_IAS', 'LIGHT_ALT', 'LIGHT_VS',
      'LIGHT_APR', 'LIGHT_REV',
    ],
  };
  const COLORS = ['green', 'red', 'yellow'];

  let outPanel = 'pz55';
  let outTarget = '';
  let outCommand = '';
  let outColor = 'green';

  $: outputTargetList = PANEL_OUTPUTS[outPanel] ?? [];

  // The PZ55's gear indicators are bicolour; the PZ70's are single-colour, so its
  // colour picker is not offered.
  $: colorChoices = outPanel === 'pz55' ? COLORS : ['green'];

  // Keep the target valid when the panel changes.
  $: if (outTarget && !outputTargetList.includes(outTarget)) {
    outTarget = '';
  }

  async function submitOutput() {
    if (!outTarget || !outCommand) return;
    const ok = await addOutput({
      model: outPanel,
      target: outTarget,
      command: outCommand,
      color: outColor,
      rules: ruleRows.map(r => ({ ...r, export: Number(r.export), value: Number(r.value) })),
    }, editingOutput);
    if (ok) {
      outTarget = '';
      outCommand = '';
      editingOutput = null; ruleRows = [];
    }
  }

  // --- PZ70 LCD display editor ---------------------------------------------
  // A display binding answers one selector mode (ALT/VS/IAS/HDG/CRS) on one LCD line.
  const DISPLAY_MODES = ['ALT', 'VS', 'IAS', 'HDG', 'CRS'];
  const DISPLAY_LINES = ['upper', 'lower'];

  let dispMode = 'ALT';
  let dispLine = 'upper';
  let dispCommand = '';
  let dispExport = 0;
  let dispScale = 1;
  let dispOffset = 0;
  let dispUnit = '';

  // The controls the source picker offers: any control with an integer export
  // (read-only displays, gauges, selected values), even if it also has inputs.
  $: exportControls = $controls.filter((c) => (c.outputs ?? []).some((o) => (o.type ?? 'integer') === 'integer'));

  // Reset the export index when the source changes, so it never points past the
  // new control's outputs.
  $: chosenSource = exportControls.find((c) => c.identifier === dispCommand) ?? null;
  $: exportCount = chosenSource ? (chosenSource.outputs ?? []).length : 0;
  $: if (dispExport >= exportCount) dispExport = 0;

  async function submitDisplay() {
    if (!dispCommand) return;
    const ok = await addDisplay({
      model: 'pz70',
      mode: dispMode,
      line: dispLine,
      command: dispCommand,
      export: Number(dispExport) || 0,
      scale: Number(dispScale) || 0,
      offset: Number(dispOffset) || 0,
      unit: dispUnit,
    }, editingDisplay);
    if (ok) {
      dispCommand = '';
      dispExport = 0;
      dispUnit = '';
      editingDisplay = null;
    }
  }

  function previewCurrent() {
    previewDisplay({
      aircraft: $mappingAircraft,
      command: dispCommand,
      export: Number(dispExport) || 0,
      scale: Number(dispScale) || 0,
      offset: Number(dispOffset) || 0,
      line: dispLine,
    });
  }

  // Hardware test: write known numbers to the two lines. 12345 / -1234 exercises
  // every cell and the minus sign.
  let testUpper = 12345;
  let testLower = -1234;
  async function runHardwareTest() {
    await testDisplay({ upper: Number(testUpper), lower: Number(testLower) });
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
  <div class="block">
    <button on:click={exportProfile} disabled={!$mappingAircraft || $mappingsLoading}>{$t('panels.exportProfile')}</button>
    <label>{$t('panels.importProfile')} <input type="file" accept=".json,application/json" on:change={readProfile} /></label>
    {#if profileNotice}<p class="error">{profileNotice}</p>{/if}
    {#if importPreview}
      <p>{importPreview.aircraft} → {$mappingAircraft}: {importPreview.bindings.length} {$t('panels.mappings')}, {importPreview.outputs.length} LED, {importPreview.displays.length} LCD</p>
      <p>{$t('panels.replaceWarning')}</p>
      <button disabled={!$mappingAircraft || $mappingsLoading} on:click={importProfile}>{$t('panels.applyImport')}</button>
      <button on:click={() => importPreview = null}>{$t('panels.cancel')}</button>
    {/if}
  </div>

  {#if $panelsError}
    <p class="error">{$panelsError}</p>
  {/if}

  {#if !$panelsSupported}
    <p class="empty">{$t('panels.unsupported')}</p>
  {:else}
    <div class="block">
      <h3>{$t('panels.plugin.title')} <span class="badge" class:on={pluginState.connected}>{$t(pluginState.connected ? 'panels.connected' : 'panels.disconnected')}</span></h3>
      <p class="hint">{$t('panels.plugin.hint')}</p>
      <label><input type="checkbox" checked={!!pluginState.trimEnabled} disabled={pluginBusy || (!pluginState.trimEnabled && !pluginState.connected)} on:change={e => setPluginTrim(e.currentTarget.checked)} /> {$t('panels.plugin.trimEnable')}</label>
      <p class="hint">{$t('panels.plugin.trimWarning')}</p>
      {#if pluginError}<p class="error">{pluginError}</p>{/if}
      <p>{pluginState.aircraft || $t('panels.noAircraft')} · {pluginState.accepted} {$t('panels.plugin.accepted')}</p>
      {#if pluginState.error}<p class="error">{pluginState.error}</p>{/if}
      {#if !pluginState.connected}<p class="hint">{$t('panels.plugin.install')}</p>{/if}
    </div>
    {#if !$biosState.aircraft}<p class="hint">{$t('panels.noAircraftInputWarning')}</p>{/if}
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

    <div class="workspace">
      {#if $aircraftList.length > 0}
        <label class="pick">{$t('panels.aircraftPick')}
          <select value={$mappingAircraft} on:change={pickAircraft}>{#each $aircraftList as a}<option value={a}>{a}</option>{/each}</select>
        </label>
      {:else}<p class="empty">{$t('panels.noProfiles')}</p>{/if}
      <div class="toolbar">
        <button class:chosen={boardModel === 'pz55'} on:click={() => {boardModel = 'pz55'; selected = '';}}>PZ55 Switch Panel</button>
        <button class:chosen={boardModel === 'pz70'} on:click={() => {boardModel = 'pz70'; selected = '';}}>PZ70 Multi Panel</button>
      </div>
      <PanelBoard model={boardModel} mode={boardMode} bindings={$bindings} outputs={$outputs} displays={$displays} {selected}
        disabled={!$mappingAircraft || $mappingsLoading || !$controlsAvailable} on:select={selectControl} on:mode={e => boardMode = e.detail} />
      <nav class="toolbar" aria-label={$t('panels.configuration')}>
        {#each ['input','output','display','diagnostics'] as tab}
          <button class:chosen={editorTab === tab} on:click={() => editorTab = tab}>{$t('panels.tab.' + tab)}</button>
        {/each}
      </nav>
      {#if $mappingsLoading}<p class="hint">{$t('panels.loadingProfile')}</p>{/if}
    </div>

    <div class="block" class:hidden={editorTab !== 'input'}>
      <h3>
        {$t('panels.mappings')}
        <span class="badge" class:on={$sendingEnabled}>
          {$sendingEnabled ? $t('panels.sendingOn') : $t('panels.sendingOff')}
        </span>
      </h3>

      <p class="hint">{$t('panels.mappingsHint')}</p>

      {#if !$mappingAircraft}
        <p class="empty">{$t('panels.pickAircraft')}</p>
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

        <details class="profile-list">
          <summary>{$t('panels.showBindings')} ({$bindings.length})</summary>
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
              {#each $bindings as b (b.model + '/' + b.control + '/' + (b.mode || ''))}
                <tr>
                  <td>{modelLabel(b.model)}</td>
                  <td class="mono">{b.control} {b.mode || ''}</td>
                  <td class="mono">{b.command}{#if b.invert}<span class="tag">{$t('panels.inverted')}</span>{/if}</td>
                  <td class="mono">{b.interface}</td>
                  <td class="actions">
                    <button on:click={() => editBinding(b)}>{$t('panels.edit')}</button>
                    <button class="remove" on:click={() => removeBinding(b.model, b.control, b.mode || '')} title={$t('panels.remove')}>×</button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
        </details>

        <div class="editor-heading">{$t(editingBinding ? 'panels.editExisting' : 'panels.newBinding')} {newControl} {newMode}</div>
        <div class="add" bind:this={inputEditor}>
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
          <input type="search" bind:value={commandSearch} placeholder={$t('panels.searchCommand')} aria-label={$t('panels.searchCommand')} />
          <select bind:value={newCommand} aria-label={$t('panels.col.command')}>
            <option value="">{$t('panels.pickCommand')}</option>
            {#if newCommand && !commandChoices.some(c => c.identifier === newCommand)}<option value={newCommand}>{newCommand}</option>{/if}
            {#each commandChoices as c (c.identifier)}
              <option value={c.identifier}>{commandLabel(c)}</option>
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
            {$t(editingBinding ? 'panels.save' : 'panels.add')}
          </button>
          <label><input type="checkbox" bind:checked={newInvert} /> {$t('panels.inverted')}</label>
          {#if newInterface === 'set_state'}
            {#if ['LCD_WHEEL', 'PITCH_TRIM'].includes(newControl)}
              <label><input type="checkbox" bind:checked={pulseEnabled} /> {$t('panels.pulseReset')}</label>
              {#if pulseEnabled}<label>{$t('panels.pulseResetValue')} <input type="number" min="0" max={chosenControl?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 65535} bind:value={pulseReset} /></label>{/if}
            {/if}
            <label><input type="checkbox" bind:checked={customStates} /> {$t('panels.customStates')}</label>
            {#if customStates}
              <label>{$t('panels.stateOn')} <input type="number" min="0" max={chosenControl?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 65535} bind:value={stateOn} /></label>
              <label>{$t('panels.stateOff')} <input type="number" min="0" max={chosenControl?.inputs?.find(i => i.interface === 'set_state')?.max_value ?? 65535} bind:value={stateOff} /></label>
              <p class="hint">{$t('panels.customStatesHint')}</p>
            {/if}
          {/if}
          {#if newPanel === 'pz70' && newControl === 'LCD_WHEEL'}
            <label>{$t('panels.mode')} <select bind:value={newMode}><option value="">{$t('panels.allModes')}</option>{#each DISPLAY_MODES as mode}<option value={mode}>{mode}</option>{/each}</select></label>
          {/if}
          {#if editingBinding}<span>{$t('panels.edit')}: {editingBinding.control}</span><button on:click={() => { editingBinding = null; newControl = ''; newMode = ''; }}>{$t('panels.cancel')}</button>{/if}
        </div>

        <div class="test">
          <label class="toggle test-toggle">
            <input type="checkbox" checked={$testMode} on:change={(e) => setTestMode(e.currentTarget.checked)} />
            {$t('panels.testMode')}
          </label>
          <p class="hint">{$t('panels.testHint')}</p>

          {#if $mappingEvents.length === 0}
            <p class="empty">{$t('panels.noTestEvents')}</p>
          {:else}
            <ul class="test-log" aria-label={$t('panels.testLog')}>
              {#each $mappingEvents as e}
                <li>
                  <span class="dev">{e.model === 'pz55' ? 'PZ55' : 'PZ70'}</span>
                  <span class="ctl mono">{e.control}</span>
                  <span class="arrow">→</span>
                  {#if e.commands && e.commands.length}
                    <span class="mono">{e.commands.map((c) => c.trim().replace(/\s+/, ' = ')).join(', ')}</span>
                  {:else}
                    <span class="msg">{$t('panels.testNoBinding')}</span>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}
    </div>

    <div class="block" class:hidden={editorTab !== 'output'}>
      <h3>
        {$t('panels.outputs')}
        <span class="badge" class:on={$outputsEnabled}>
          {$outputsEnabled ? $t('panels.sendingOn') : $t('panels.sendingOff')}
        </span>
      </h3>
      <p class="hint">{$t('panels.outputsHint')}</p>

      {#if !$mappingAircraft}
        <p class="empty">{$t('panels.pickAircraft')}</p>
      {:else if !$controlsAvailable}
        <p class="empty">{$t('panels.noMetadata', { aircraft: $mappingAircraft })}</p>
      {:else}
        <label class="toggle safety">
          <input type="checkbox" checked={$outputsEnabled} on:change={(e) => setOutputs(e.currentTarget.checked)} />
          {$t('panels.outputsEnable')}
        </label>
        <p class="hint">{$t('panels.outputsSwitchHint')}</p>

        <details class="profile-list">
          <summary>{$t('panels.showOutputs')} ({$outputs.length})</summary>
        {#if $outputs.length === 0}
          <p class="empty">{$t('panels.noOutputs')}</p>
        {:else}
          <table class="bindings">
            <thead>
              <tr>
                <th>{$t('panels.col.panel')}</th>
                <th>{$t('panels.col.target')}</th>
                <th>{$t('panels.col.source')}</th>
                <th>{$t('panels.col.color')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each $outputs as o (o.model + '/' + o.target)}
                <tr>
                  <td>{modelLabel(o.model)}</td>
                  <td class="mono">{o.target}</td>
                  <td class="mono">{o.command}</td>
                  <td>{o.color ? $t(`panels.color.${o.color}`) : '—'} {#if o.rules?.length}({o.rules.length}){/if}</td>
                  <td class="actions">
                    <button on:click={() => editOutput(o)}>{$t('panels.edit')}</button>
                    <button class="remove" on:click={() => removeOutput(o.model, o.target)} title={$t('panels.remove')}>×</button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
        </details>

        <div class="editor-heading">{$t(editingOutput ? 'panels.editExisting' : 'panels.newBinding')} {outTarget}</div>
        <div class="add" bind:this={outputEditor}>
          <select bind:value={outPanel} aria-label={$t('panels.col.panel')}>
            <option value="pz55">PZ55</option>
            <option value="pz70">PZ70</option>
          </select>
          <select bind:value={outTarget} aria-label={$t('panels.col.target')}>
            <option value="">{$t('panels.pickTarget')}</option>
            {#each outputTargetList as tgt (tgt)}
              <option value={tgt}>{tgt}</option>
            {/each}
          </select>
          <input type="search" bind:value={sourceSearch} placeholder={$t('panels.searchSource')} aria-label={$t('panels.searchSource')} />
          <select bind:value={outCommand} aria-label={$t('panels.col.source')}>
            <option value="">{$t('panels.pickSource')}</option>
            {#if outCommand && !sourceChoices.some(c => c.identifier === outCommand)}<option value={outCommand}>{outCommand}</option>{/if}
            {#each sourceChoices as c (c.identifier)}
              <option value={c.identifier}>{commandLabel(c)}</option>
            {/each}
          </select>
          <select bind:value={outColor} aria-label={$t('panels.col.color')} disabled={colorChoices.length === 1}>
            {#each colorChoices as c (c)}
              <option value={c}>{$t(`panels.color.${c}`)}</option>
            {/each}
          </select>
          <button class="add-btn" on:click={submitOutput} disabled={!outTarget || !outCommand}>
            {$t(editingOutput ? 'panels.save' : 'panels.addOutput')}
          </button>
        </div>
        <p>{$t('panels.ruleHint')}</p>
        {#each ruleRows as rule, i}
          <div class="add">
            <select bind:value={rule.command} aria-label={$t('panels.col.source')}>{#each exportControls as c}<option value={c.identifier}>{c.identifier}</option>{/each}</select>
            <label>{$t('panels.exportIndex')} <input type="number" min="0" bind:value={rule.export} /></label>
            <select bind:value={rule.operator} aria-label={$t('panels.comparison')}>{#each ['eq','ne','gt','lt','ge','le'] as op}<option value={op}>{op}</option>{/each}</select>
            <input type="number" bind:value={rule.value} aria-label={$t('panels.ruleValue')} />
            <select bind:value={rule.color} aria-label={$t('panels.col.color')}>{#each (outPanel === 'pz55' ? ['green','red','yellow','off'] : ['green','off']) as color}<option value={color}>{color}</option>{/each}</select>
            <button on:click={() => ruleRows = ruleRows.filter((_, index) => index !== i)}>×</button>
          </div>
        {/each}
        <button disabled={ruleRows.length >= 16} on:click={() => ruleRows = [...ruleRows, {command: outCommand || exportControls[0]?.identifier || '', export: 0, operator: 'eq', value: 1, color: outPanel === 'pz55' ? 'green' : 'green'}]}>{$t('panels.addRule')}</button>
        {#if editingOutput}<span>{$t('panels.edit')}: {editingOutput.target}</span><button on:click={() => {editingOutput = null; ruleRows = []; outTarget = '';}}>{$t('panels.cancel')}</button>{/if}
      {/if}
    </div>

    <div class="block" class:hidden={editorTab !== 'display'}>
      <h3>{$t('panels.displays')}</h3>
      <p class="hint">{$t('panels.displaysHint')}</p>

      {#if !$mappingAircraft}
        <p class="empty">{$t('panels.pickAircraft')}</p>
      {:else if !$controlsAvailable}
        <p class="empty">{$t('panels.noMetadata', { aircraft: $mappingAircraft })}</p>
      {:else}
        <details class="profile-list">
          <summary>{$t('panels.showDisplays')} ({$displays.length})</summary>
        {#if $displays.length === 0}
          <p class="empty">{$t('panels.noDisplays')}</p>
        {:else}
          <table class="bindings">
            <thead>
              <tr>
                <th>{$t('panels.col.mode')}</th>
                <th>{$t('panels.col.line')}</th>
                <th>{$t('panels.col.source')}</th>
                <th>{$t('panels.col.conversion')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each $displays as d (d.mode + '/' + d.line)}
                <tr>
                  <td class="mono">{d.mode}</td>
                  <td>{$t('panels.line.' + d.line)}</td>
                  <td class="mono">{d.command}</td>
                  <td class="mono">×{(d.scale ?? 0) || 1}{(d.offset ? (d.offset > 0 ? ' +' : ' ') + d.offset : '')}{#if d.unit}<span class="tag">{d.unit}</span>{/if}</td>
                  <td class="actions">
                    <button on:click={() => editDisplay(d)}>{$t('panels.edit')}</button>
                    <button class="remove" on:click={() => removeDisplay(d.mode, d.line)} title={$t('panels.remove')}>×</button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
        </details>

        <div class="editor-heading">{$t(editingDisplay ? 'panels.editExisting' : 'panels.newBinding')} {dispMode} · {$t('panels.line.' + dispLine)}</div>
        <div class="add" bind:this={lcdEditor}>
          <select bind:value={dispMode} aria-label={$t('panels.col.mode')}>
            {#each DISPLAY_MODES as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
          <select bind:value={dispLine} aria-label={$t('panels.col.line')}>
            {#each DISPLAY_LINES as l (l)}
              <option value={l}>{$t('panels.line.' + l)}</option>
            {/each}
          </select>
          <input type="search" bind:value={sourceSearch} placeholder={$t('panels.searchSource')} aria-label={$t('panels.searchSource')} />
          <select bind:value={dispCommand} aria-label={$t('panels.col.source')}>
            <option value="">{$t('panels.pickSource')}</option>
            {#if dispCommand && !exportControls.some(c => c.identifier === dispCommand && matchesSearch(c, sourceSearch))}<option value={dispCommand}>{dispCommand}</option>{/if}
            {#each exportControls.filter(c => matchesSearch(c, sourceSearch)) as c (c.identifier)}
              <option value={c.identifier}>{commandLabel(c)}</option>
            {/each}
          </select>
          <input class="num" type="number" min="0" bind:value={dispExport} title={$t('panels.exportIndex')} placeholder="0" />
          <input class="num" type="number" step="any" bind:value={dispScale} title={$t('panels.scale')} placeholder="1" />
          <input class="num" type="number" step="any" bind:value={dispOffset} title={$t('panels.offset')} placeholder="0" />
          <input class="num wide" type="text" bind:value={dispUnit} title={$t('panels.unit')} placeholder={$t('panels.unit')} />
          <button class="add-btn" on:click={submitDisplay} disabled={!dispCommand}>{$t(editingDisplay ? 'panels.save' : 'panels.addDisplay')}</button>
          {#if editingDisplay}<span>{$t('panels.edit')}: {editingDisplay.mode}/{editingDisplay.line}</span><button on:click={() => {editingDisplay = null; dispCommand = '';}}>{$t('panels.cancel')}</button>{/if}
          <button class="add-btn" on:click={previewCurrent} disabled={!dispCommand}>{$t('panels.preview')}</button>
        </div>

        {#if $displayPreview}
          <p class="line preview">
            {#if $displayPreview.available}
              <span class="seg">{dispMode}</span>
              <span class="lcd">{$displayPreview.text}</span>
              <span class="muted">{$t('panels.previewRaw')} {$displayPreview.raw} → {$displayPreview.value}</span>
            {:else}
              <span class="muted">{$t('panels.previewUnavailable', { reason: $displayPreview.reason })}</span>
            {/if}
          </p>
        {/if}

        <div class="hwtest">
          <span class="muted">{$t('panels.hwTest')}</span>
          <input class="num" type="number" bind:value={testUpper} title={$t('panels.line.upper')} />
          <input class="num" type="number" bind:value={testLower} title={$t('panels.line.lower')} />
          <button class="add-btn" on:click={runHardwareTest}>{$t('panels.hwTestRun')}</button>
          <span class="muted">{$t('panels.hwTestHint')}</span>
        </div>
      {/if}
    </div>

    <div class="block" class:hidden={editorTab !== 'diagnostics'}>
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

    <div class="block" class:hidden={editorTab !== 'diagnostics'}>
      <h3>
        {$t('panels.monitor')}
        <span class="count">{$panelEvents.length}</span>
      </h3>
      {#if $panelEvents.length === 0}
        <p class="empty">{$t('panels.noEvents')}</p>
      {:else}
        <ul class="events">
          <!-- Unkeyed: the key was at+device+control, which can repeat for two
               events on the same control within one millisecond and makes Svelte
               throw on duplicate keys. The rows hold no local state. -->
          {#each $panelEvents as e}
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
  .block > button, .add > button:not(.add-btn), .actions > button:not(.remove) { padding:.4rem .7rem; border:1px solid var(--border); border-radius:6px; color:var(--text); background:var(--bg); cursor:pointer; }
  .hidden { display:none; }
  .toolbar { display:flex; flex-wrap:wrap; gap:.4rem; margin:.7rem 0; }
  .toolbar button { padding:.5rem .8rem; border:1px solid var(--border); border-radius:6px; background:var(--bg); color:var(--text); cursor:pointer; }
  .toolbar .chosen { border-color:var(--blue); background:var(--bg-hover, var(--bg)); box-shadow:inset 0 -3px var(--blue); }
  .profile-list { margin:.6rem 0; }
  summary { cursor:pointer; color:var(--muted); font-size:.8rem; padding:.4rem 0; }
  .editor-heading { margin:.7rem 0; font-size:.85rem; font-weight:600; }
  input[type="search"] { min-width:180px; padding:.5rem; color:var(--text); background:var(--bg); border:1px solid var(--border); border-radius:5px; }
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
    max-width: 80ch;
    line-height: 1.5;
  }

  .pick {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.78rem;
    color: var(--muted);
    margin-bottom: 0.5rem;
  }

  .pick select {
    padding: 0.24rem 0.4rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.78rem;
    cursor: pointer;
  }

  .pick select:focus {
    outline: none;
    border-color: var(--blue);
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

  .add input.num,
  .hwtest input.num {
    width: 5.5rem;
    padding: 0.26rem 0.4rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.74rem;
  }

  .add input.num.wide {
    width: 9rem;
  }

  .add input.num:focus,
  .hwtest input.num:focus {
    outline: none;
    border-color: var(--blue);
  }

  .preview {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin: 0.4rem 0 0;
  }

  .seg {
    font-size: 0.7rem;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0 0.4rem;
  }

  .lcd {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.9rem;
    letter-spacing: 0.12em;
    color: var(--green);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.1rem 0.5rem;
    white-space: pre;
  }

  .hwtest {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.5rem;
    padding-top: 0.5rem;
    border-top: 1px dashed var(--border);
    font-size: 0.74rem;
    max-width: 820px;
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

  /* Live mapping test: a switch under the bindings, and the resolved commands. */
  .test {
    margin-top: 0.8rem;
    padding-top: 0.6rem;
    border-top: 1px dashed var(--border);
    max-width: 820px;
  }

  .toggle.test-toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.78rem;
    color: var(--text);
    cursor: pointer;
  }

  .test-log {
    list-style: none;
    margin: 0.4rem 0 0;
    padding: 0;
    font-size: 0.76rem;
    max-height: 260px;
    overflow-y: auto;
  }

  .test-log li {
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
    padding: 0.15rem 0.3rem;
    border-top: 1px solid var(--border);
  }

  .test-log li:first-child {
    border-top: none;
  }

  .arrow {
    color: var(--muted);
    flex: none;
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
