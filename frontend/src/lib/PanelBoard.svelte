<script>
  import { createEventDispatcher } from 'svelte';
  import { t, lang } from './i18n.js';
  export let model = 'pz70';
  export let bindings = [];
  export let outputs = [];
  export let displays = [];
  export let mode = 'ALT';
  export let selected = '';
  export let disabled = false;
  const dispatch = createEventDispatcher();
  const modes = ['ALT', 'VS', 'IAS', 'HDG', 'CRS'];
  const switches = ['MASTER_BAT','MASTER_ALT','AVIONICS_MASTER','FUEL_PUMP','DE_ICE','PITOT_HEAT','COWL'];
  const lights = ['LIGHTS_PANEL','LIGHTS_BEACON','LIGHTS_NAV','LIGHTS_STROBE','LIGHTS_TAXI','LIGHTS_LANDING'];
  const engines = ['ENGINE_OFF','ENGINE_RIGHT','ENGINE_LEFT','ENGINE_BOTH','ENGINE_START'];
  const buttons = ['AP','HDG','NAV','IAS','ALT','VS','APR','REV'];
  const labels = {
    MASTER_BAT: ['Batterie', 'Battery'], MASTER_ALT: ['Alternateur', 'Alternator'],
    AVIONICS_MASTER: ['Avionique', 'Avionics'], FUEL_PUMP: ['Pompe carburant', 'Fuel pump'],
    DE_ICE: ['Dégivrage', 'De-ice'], PITOT_HEAT: ['Chauffage Pitot', 'Pitot heat'], COWL: ['Volets de capot', 'Cowl flaps'],
    LIGHTS_PANEL: ['Éclairage panneau', 'Panel lights'], LIGHTS_BEACON: ['Balise', 'Beacon'],
    LIGHTS_NAV: ['Navigation', 'Navigation'], LIGHTS_STROBE: ['Stroboscopes', 'Strobes'],
    LIGHTS_TAXI: ['Roulage', 'Taxi'], LIGHTS_LANDING: ['Atterrissage', 'Landing'],
    ENGINE_OFF: ['OFF', 'OFF'], ENGINE_RIGHT: ['R', 'R'], ENGINE_LEFT: ['L', 'L'], ENGINE_BOTH: ['BOTH', 'BOTH'], ENGINE_START: ['START', 'START'],
    GEAR_UP: ['Train ↑', 'Gear ↑'], GEAR_DOWN: ['Train ↓', 'Gear ↓'],
    PITCH_TRIM: ['Trim de profondeur ↔', 'Pitch trim ↔'], AUTO_THROTTLE: ['Auto-throttle ON/OFF', 'Auto-throttle ON/OFF'],
    FLAPS_UP: ['Volets ↑', 'Flaps ↑'], FLAPS_DOWN: ['Volets ↓', 'Flaps ↓'], LCD_WHEEL: ['Molette ↔', 'Wheel ↔'],
  };
  function label(id) { return labels[id]?.[$lang === 'fr' ? 0 : 1] || id; }
  function binding(id, context = '') {
    return bindings.find(b => b.model === model && b.control === id && (b.mode || '').toUpperCase() === context)
      || (context ? bindings.find(b => b.model === model && b.control === id && !b.mode) : null);
  }
  function source(id, context = '') { return binding(id, context)?.command || $t('panels.unassigned'); }
  function choose(kind, id, context = '') { dispatch('select', { kind, id, mode: context, model }); }
</script>

{#key bindings}
{#key model + '/' + $lang}
<div class="board" aria-label={model === 'pz70' ? 'PZ70 Multi Panel' : 'PZ55 Switch Panel'}>
  <div class="title">LOGITECH / SAITEK <strong>{model.toUpperCase()}</strong><span>{$t('panels.boardHint')}</span></div>
  {#if model === 'pz70'}
    <div class="multi-top">
      <div class="modes">
        {#each modes as m}
          <div class="mode-row">
            <button class:active={mode === m} on:click={() => dispatch('mode', m)}>{m}</button>
                    <button class="selector-binding" disabled={disabled} title={$t('panels.selectorBinding') + ' ' + m} aria-label={$t('panels.selectorBinding') + ' ' + m} on:click={() => choose('input', 'KNOB_' + m)} class:assigned={!!binding('KNOB_' + m)}>⚙</button>
          </div>
        {/each}
      </div>
      <div class="lcd">
        {#each ['upper', 'lower'] as line}
          {@const display = displays.find(d => d.mode.toUpperCase() === mode && d.line === line)}
          <button disabled={disabled} class:selected={selected === 'display/' + mode + '/' + line} on:click={() => choose('display', line, mode)}>
            <small>{mode} · {$t('panels.line.' + line)}</small>
            <span>{display?.command || $t('panels.unassigned')}</span>
          </button>
        {/each}
      </div>
      <button class="wheel" disabled={disabled} class:selected={selected === 'input/LCD_WHEEL/' + mode} on:click={() => choose('input', 'LCD_WHEEL', mode)}>
        <strong>◉</strong>{label('LCD_WHEEL')} · {mode}<small>{source('LCD_WHEEL', mode)}</small>
      </button>
    </div>
    <div class="ap-buttons">
      {#each buttons as b}
        {@const output = outputs.find(o => o.model === model && o.target === 'LIGHT_' + b)}
        <div>
          <button class="led" class:assigned={!!output} disabled={disabled} class:selected={selected === 'output/LIGHT_' + b} title={$t('panels.configureLED') + ' ' + b} aria-label={$t('panels.configureLED') + ' ' + b} on:click={() => choose('output', 'LIGHT_' + b)}>● <small>LED</small></button>
          <button disabled={disabled} class:assigned={!!binding(b + '_BUTTON')} class:selected={selected === 'input/' + b + '_BUTTON/'} on:click={() => choose('input', b + '_BUTTON')}><strong>{b}</strong><small>{source(b + '_BUTTON')}</small></button>
        </div>
      {/each}
    </div>
    <div class="aux">
      {#each ['AUTO_THROTTLE', 'FLAPS_UP', 'FLAPS_DOWN', 'PITCH_TRIM'] as id}
        <button disabled={disabled} class:assigned={!!binding(id)} class:selected={selected === 'input/' + id + '/'} on:click={() => choose('input', id)}>{label(id)}<small>{source(id)}</small></button>
      {/each}
    </div>
  {:else}
    <div class="switches">
      {#each switches as id}
        <button disabled={disabled} class:assigned={!!binding(id)} class:selected={selected === 'input/' + id + '/'} on:click={() => choose('input', id)}><span>▯</span>{label(id)}<small>{source(id)}</small></button>
      {/each}
    </div>
    <div class="switches">
      {#each lights as id}
        <button disabled={disabled} class:assigned={!!binding(id)} class:selected={selected === 'input/' + id + '/'} on:click={() => choose('input', id)}><span>▯</span>{label(id)}<small>{source(id)}</small></button>
      {/each}
    </div>
    <div class="aux">
      <div class="engine">
        <span>{$t('panels.engineSelector')}</span>
        {#each engines as id}<button disabled={disabled} title={source(id)} class:assigned={!!binding(id)} class:selected={selected === 'input/' + id + '/'} on:click={() => choose('input', id)}>{label(id)}</button>{/each}
      </div>
      <div class="gear">
        {#each ['UPPER', 'LEFT', 'RIGHT'] as position}
          {@const target = 'LIGHT_GEAR_' + position}
          <button class="led" disabled={disabled} class:assigned={outputs.some(o => o.model === model && o.target === target)} class:selected={selected === 'output/' + target} on:click={() => choose('output', target)}>{$t('panels.gear.' + position)} ●</button>
        {/each}
        {#each ['GEAR_UP', 'GEAR_DOWN'] as id}<button disabled={disabled} class:assigned={!!binding(id)} class:selected={selected === 'input/' + id + '/'} on:click={() => choose('input', id)}>{label(id)}<small>{source(id)}</small></button>{/each}
      </div>
    </div>
  {/if}
  <p class="legend">{$t('panels.boardLegend')}</p>
</div>
{/key}
{/key}

<style>
  .board { background: #171d27; color: #eef1f6; border: 1px solid #485061; border-radius: 14px; padding: 1rem; margin: .7rem 0; }
  .title { display:flex; gap:.6rem; align-items:center; flex-wrap:wrap; font-size:.7rem; letter-spacing:.06em; margin-bottom:.9rem; }
  .title span { margin-left:auto; letter-spacing:0; color:#acb6c7; }
  button { min-width:0; background:#252d3a; color:#edf1f7; border:1px solid #566173; border-radius:7px; padding:.6rem .4rem; cursor:pointer; font:inherit; font-size:.76rem; }
  button:hover:not(:disabled), button.selected { border-color:#67b3ff; outline:2px solid #67b3ff; outline-offset:1px; }
  button:disabled { opacity:.45; cursor:default; }
  button.assigned { border-bottom:3px solid #6dc692; }
  small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:.61rem; margin-top:.3rem; color:#afbbcd; }
  .multi-top { display:grid; grid-template-columns:90px minmax(120px,1fr) minmax(90px,.5fr); gap:.7rem; }
  .mode-row { display:flex; gap:.3rem; margin-bottom:.2rem; }
  .mode-row button:first-child { flex:1; }
  .mode-row button { padding:.3rem; }
  .mode-row .active { background:#316796; }
  .selector-binding { width:24px; }
  .lcd { display:flex; flex-direction:column; gap:.6rem; }
  .lcd button { flex:1; background:#080e08; color:#b8e888; text-align:left; padding:.8rem; }
  .lcd button span { display:block; overflow-wrap:anywhere; font-family:monospace; }
  .wheel strong { display:block; font-size:2.5rem; }
  .ap-buttons { display:grid; grid-template-columns:repeat(8,minmax(0,1fr)); gap:.5rem; margin-top:.8rem; }
  .ap-buttons button { width:100%; }
  .led { color:#a9b2c0; padding:.35rem; }
  .led.assigned { color:#a6d5af; }
  .legend { margin:.8rem 0 0; font-size:.65rem; color:#a9b2c0; }
  .led small { display:inline; }
  .aux { display:flex; gap:.6rem; flex-wrap:wrap; margin-top:.8rem; }
  .aux > button { flex:1; }
  .switches { display:grid; grid-template-columns:repeat(7,minmax(0,1fr)); gap:.5rem; margin-bottom:.6rem; }
  .switches span { display:block; font-size:1.8rem; }
  .engine,.gear { display:flex; flex-wrap:wrap; align-items:center; gap:.4rem; }
  .engine { flex:1; }
  .gear { flex:1; }
  .engine span { width:100%; font-size:.7rem; }
  @media(max-width:1000px) { .ap-buttons { grid-template-columns:repeat(4,minmax(0,1fr)); } .switches { grid-template-columns:repeat(3,minmax(0,1fr)); } .multi-top { grid-template-columns:75px minmax(0,1fr); } .wheel { grid-column:1 / -1; } .wheel strong { display:inline; font-size:1rem; margin-right:.5rem; } }
</style>
