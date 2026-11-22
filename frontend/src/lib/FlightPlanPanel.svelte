<script>
  /**
   * Flight planner: route editor, live navlog and DCS briefing. The backend
   * calculates distances, wind correction and fuel, and produces the printable
   * briefing.
   */
  import { onMount } from 'svelte';
  import {
    plans, plansEnabled, selectedPlanID, draft, navlog, planError,
    planSaving, planDirty, navlogLoading, canSave, loadPlans, selectPlan, newPlan,
    savePlan, deletePlan, updatePlan, setDeparture, setArrival, addAirport, addPoint,
    removeWaypoint, moveWaypoint, briefingURL,
  } from './flightplans.js';
  import { theatre, theatres } from './units.js';
  import { lang, t } from './i18n.js';
  import { sessionUser, isServer } from './session.js';
  import AirportSearch from './AirportSearch.svelte';

  let customOpen = false;
  let custom = { name: '', lat: '', lon: '' };
  let customError = '';
  let briefingFrame;

  $: canEditPlan = !$isServer || !$draft.id || !$draft.ownerId
    || $draft.ownerId === $sessionUser?.id || $sessionUser?.role === 'admin';

  onMount(loadPlans);

  $: routePoints = $draft.route ?? [];
  $: departurePoint = routePoints[0] ?? null;
  $: arrivalPoint = routePoints.length > 1 ? routePoints[routePoints.length - 1] : null;
  $: intermediates = routePoints.length > 2 ? routePoints.slice(1, -1) : [];
  $: readyForWaypoints = routePoints.length >= 2;

  function textField(name, event) {
    updatePlan({ [name]: event.currentTarget.value });
  }

  function numberField(name, event) {
    updatePlan({ [name]: Number(event.currentTarget.value) });
  }

  function deleteCurrent() {
    if (!$draft.id) return;
    if (confirm($t('planner.deleteConfirm'))) deletePlan($draft.id);
  }

  function submitCustom() {
    const lat = Number(custom.lat);
    const lon = Number(custom.lon);
    if (!Number.isFinite(lat) || lat < -90 || lat > 90) {
      customError = $t('planner.invalidLat');
      return;
    }
    if (!Number.isFinite(lon) || lon < -180 || lon > 180) {
      customError = $t('planner.invalidLon');
      return;
    }
    customError = '';
    const label = custom.name.trim() || `WP${routePoints.length - 1}`;
    const ident = label.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 8) || `WP${routePoints.length - 1}`;
    addPoint({ ident, name: label, lat, lon });
    custom = { name: '', lat: '', lon: '' };
    customOpen = false;
  }

  function canDiscard() {
    return !$planDirty || confirm($t('planner.discard'));
  }

  function startNew() {
    if (canDiscard()) newPlan();
  }

  function openPlan(id) {
    if (id === $selectedPlanID) return;
    if (canDiscard()) selectPlan(id);
  }

  function fmt(value, digits = 0) {
    return Number(value || 0).toFixed(digits);
  }

  function duration(minutes) {
    const total = Math.round(minutes || 0);
    return `${Math.floor(total / 60)}h${String(total % 60).padStart(2, '0')}`;
  }

  function planSubtitle(plan) {
    return `${plan.departure || '—'} → ${plan.arrival || '—'}`;
  }

  async function printBriefing() {
    if (!$draft.id) return;
    try {
      const response = await fetch(briefingURL($draft.id, $lang));
      if (!response.ok) throw new Error(`${response.status}`);
      const html = await response.text();
      briefingFrame.onload = () => {
        try {
          briefingFrame.contentWindow.focus();
          briefingFrame.contentWindow.print();
        } catch (error) {
          planError.set($t('error.flightplan', { detail: error.message }));
        }
      };
      briefingFrame.srcdoc = html;
    } catch (error) {
      planError.set($t('error.flightplan', { detail: error.message }));
    }
  }
</script>

<section class="planner">
  <header class="planner-head">
    <div>
      <h2>{$t('tab.planner')}</h2>
      <p class="hint">{$t('planner.trueHeadings')}</p>
    </div>
    <div class="head-actions">
      {#if $planDirty}<span class="dirty">{$t('planner.unsaved')}</span>{/if}
      <button class="ghost" on:click={startNew}>{$t('planner.new')}</button>
      <button class="primary" on:click={savePlan} disabled={!$canSave || $planSaving || !canEditPlan}>
        {$planSaving ? $t('planner.saving') : $t('planner.save')}
      </button>
    </div>
  </header>

  {#if !$plansEnabled}
    <p class="empty">{$t('planner.disabled')}</p>
  {:else}
    <div class="layout">
      <aside class="plans">
        <div class="plans-title">{$t('planner.plans')}</div>
        {#if $plans.length === 0}
          <p class="muted">{$t('planner.noPlans')}</p>
        {:else}
          {#each $plans as plan (plan.id)}
            <button class="plan-item" class:active={$selectedPlanID === plan.id} on:click={() => openPlan(plan.id)}>
              <span class="plan-name">{plan.name}</span>
              <span class="plan-route">{planSubtitle(plan)}</span>
              {#if plan.visibility === 'squadron'}<span class="shared">{$t('checklist.visibilitySquadron')}</span>{/if}
            </button>
          {/each}
        {/if}
      </aside>

      <div class="editor">
        {#if $planError}<p class="error">{$planError}</p>{/if}
        {#if !canEditPlan}<p class="readonly">{$t('planner.readOnly')}</p>{/if}

        <div class="fields">
          <label class="wide">
            <span>{$t('planner.name')}</span>
            <input value={$draft.name} on:input={(e) => textField('name', e)} placeholder={$t('planner.namePlaceholder')} />
          </label>
          <label>
            <span>{$t('planner.aircraft')}</span>
            <input value={$draft.aircraft} on:input={(e) => textField('aircraft', e)} placeholder={$t('planner.aircraftPlaceholder')} />
          </label>
          <label>
            <span>{$t('planner.rules')}</span>
            <select value={$draft.flightRules} on:change={(e) => updatePlan({ flightRules: e.currentTarget.value })}>
              <option value="VFR">VFR</option>
              <option value="IFR">IFR</option>
            </select>
          </label>
          <label>
            <span>{$t('checklist.visibility')}</span>
            <select value={$draft.visibility ?? 'private'} on:change={(e) => updatePlan({ visibility: e.currentTarget.value })}>
              <option value="private">{$t('checklist.visibilityPrivate')}</option>
              <option value="squadron">{$t('checklist.visibilitySquadron')}</option>
            </select>
          </label>
          <label>
            <span>{$t('planner.cruiseAlt')}</span>
            <input type="number" min="0" max="70000" step="1000" value={$draft.cruiseAltFt} on:input={(e) => numberField('cruiseAltFt', e)} />
          </label>
          <label>
            <span>{$t('planner.tas')}</span>
            <input type="number" min="0" max="2000" step="5" value={$draft.cruiseTasKt} on:input={(e) => numberField('cruiseTasKt', e)} />
          </label>
          <label>
            <span>{$t('planner.fuelBurn')}</span>
            <input type="number" min="0" step="10" value={$draft.fuelBurnPph} on:input={(e) => numberField('fuelBurnPph', e)} />
          </label>
          <label>
            <span>{$t('planner.reserve')}</span>
            <input type="number" min="0" max="1440" step="5" value={$draft.reserveMin} on:input={(e) => numberField('reserveMin', e)} />
          </label>
          <label>
            <span>{$t('planner.windFrom')}</span>
            <input type="number" min="0" max="359" step="10" value={$draft.windFromDeg} on:input={(e) => numberField('windFromDeg', e)} />
          </label>
          <label>
            <span>{$t('planner.windSpeed')}</span>
            <input type="number" min="0" max="300" step="5" value={$draft.windKt} on:input={(e) => numberField('windKt', e)} />
          </label>
        </div>

        <div class="route-block">
          <div class="block-head">
            <h3>{$t('planner.route')}</h3>
            <label class="theatre">
              <span>{$t('planner.dcsMap')}</span>
              <select bind:value={$theatre}>
                {#each $theatres as map (map.id)}
                  <option value={map.id}>{map.name}</option>
                {/each}
              </select>
            </label>
          </div>

          <div class="endpoint">
            <span class="endpoint-label">{$t('planner.departure')}</span>
            {#if departurePoint}
              <span class="current"><b>{departurePoint.ident}</b> {departurePoint.name}</span>
            {:else}
              <span class="muted">{$t('planner.notSet')}</span>
            {/if}
            <AirportSearch
              placeholder={$t('planner.searchAirport')}
              theatre={$theatre}
              onSelect={setDeparture}
            />
          </div>

          {#if intermediates.length > 0}
            <div class="waypoints">
              {#each intermediates as point, index (index + ':' + (point.ident ?? '') + ':' + point.lat)}
                <div class="waypoint">
                  <span class="wp-ident">{point.ident || point.name || 'WP'}</span>
                  <span class="wp-name">{point.name}</span>
                  <span class="wp-pos">{fmt(point.lat, 3)}, {fmt(point.lon, 3)}</span>
                  <span class="wp-actions">
                    <button title={$t('planner.up')} disabled={index === 0} on:click={() => moveWaypoint(index + 1, -1)}>↑</button>
                    <button title={$t('planner.down')} disabled={index === intermediates.length - 1} on:click={() => moveWaypoint(index + 1, 1)}>↓</button>
                    <button title={$t('planner.remove')} on:click={() => removeWaypoint(index + 1)}>×</button>
                  </span>
                </div>
              {/each}
            </div>
          {/if}

          <div class="add-waypoint">
            <AirportSearch
              placeholder={$t('planner.addAirport')}
              theatre={$theatre}
              disabled={!readyForWaypoints}
              onSelect={addAirport}
            />
            <button class="ghost" disabled={!readyForWaypoints} on:click={() => { customOpen = !customOpen; customError = ''; }}>
              {$t('planner.addPoint')}
            </button>
          </div>

          {#if customOpen}
            <div class="custom-point">
              <input placeholder={$t('planner.pointName')} value={custom.name} on:input={(e) => (custom = { ...custom, name: e.currentTarget.value })} />
              <input type="number" step="any" placeholder={$t('planner.lat')} value={custom.lat} on:input={(e) => (custom = { ...custom, lat: e.currentTarget.value })} />
              <input type="number" step="any" placeholder={$t('planner.lon')} value={custom.lon} on:input={(e) => (custom = { ...custom, lon: e.currentTarget.value })} />
              <button class="primary" on:click={submitCustom}>{$t('planner.add')}</button>
              <button class="ghost" on:click={() => (customOpen = false)}>{$t('planner.cancel')}</button>
            </div>
            {#if customError}<p class="error">{customError}</p>{/if}
          {/if}

          <div class="endpoint">
            <span class="endpoint-label">{$t('planner.arrival')}</span>
            {#if arrivalPoint}
              <span class="current"><b>{arrivalPoint.ident}</b> {arrivalPoint.name}</span>
            {:else}
              <span class="muted">{$t('planner.notSet')}</span>
            {/if}
            <AirportSearch
              placeholder={$t('planner.searchAirport')}
              theatre={$theatre}
              disabled={!departurePoint}
              onSelect={setArrival}
            />
          </div>
        </div>

        <div class="navlog-block">
          <div class="block-head">
            <h3>{$t('planner.navlog')}</h3>
            {#if $navlogLoading}<span class="muted">{$t('planner.calculating')}</span>{/if}
          </div>
          {#if !$navlog || $navlog.legs.length === 0}
            <p class="empty">{$t('planner.emptyRoute')}</p>
          {:else}
            <table>
              <thead>
                <tr>
                  <th>{$t('planner.from')}</th>
                  <th>{$t('planner.to')}</th>
                  <th>{$t('planner.distance')}</th>
                  <th>{$t('planner.course')}</th>
                  <th>{$t('planner.heading')}</th>
                  <th>{$t('planner.gs')}</th>
                  <th>{$t('planner.time')}</th>
                  <th>{$t('planner.fuel')}</th>
                </tr>
              </thead>
              <tbody>
                {#each $navlog.legs as leg (leg.from + ':' + leg.to + ':' + leg.distanceNm)}
                  <tr>
                    <td>{leg.from}</td>
                    <td>{leg.to}</td>
                    <td class="num">{fmt(leg.distanceNm, 1)} NM</td>
                    <td class="num">{fmt(leg.courseTrue)}°</td>
                    <td class="num">{fmt(leg.headingTrue)}°</td>
                    <td class="num">{fmt(leg.groundSpeedKt)} kt</td>
                    <td class="num">{duration(leg.timeMin)}</td>
                    <td class="num">{fmt(leg.fuel)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
            <div class="totals">
              <div><span>{$t('planner.total')}</span><b>{fmt($navlog.totalDistanceNm, 1)} NM</b></div>
              <div><span>{$t('planner.time')}</span><b>{duration($navlog.totalTimeMin)}</b></div>
              <div><span>{$t('planner.fuel')}</span><b>{fmt($navlog.totalFuel)}</b></div>
              <div><span>{$t('planner.reserveFuel')}</span><b>{fmt($navlog.reserveFuel)}</b></div>
              <div><span>{$t('planner.required')}</span><b>{fmt($navlog.requiredFuel)}</b></div>
              <div><span>{$t('planner.avgGs')}</span><b>{fmt($navlog.averageGs)} kt</b></div>
            </div>
            <p class="hint">{$t('planner.fuelUnitHint')}</p>
          {/if}
        </div>

        <div class="notes-block">
          <label>
            <span>{$t('planner.notes')}</span>
            <textarea rows="3" value={$draft.notes} on:input={(e) => textField('notes', e)}></textarea>
          </label>
        </div>

        <div class="actions">
          {#if $draft.id}
            <button class="primary" on:click={printBriefing}>{$t('planner.exportBriefing')}</button>
            <a class="ghost" href={briefingURL($draft.id, $lang)} download="briefing.html">{$t('planner.downloadBriefing')}</a>
            {#if canEditPlan}<button class="danger" on:click={deleteCurrent}>{$t('planner.delete')}</button>{/if}
          {:else}
            <span class="muted">{$t('planner.saveFirst')}</span>
          {/if}
          <iframe bind:this={briefingFrame} class="briefing-frame" title="DCS briefing"></iframe>
        </div>
      </div>
    </div>
  {/if}
</section>

<style>
  .planner {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    padding: 0.9rem 1rem;
    overflow: hidden;
  }

  .planner-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.8rem;
    margin-bottom: 0.7rem;
  }

  h2 {
    margin: 0;
    font-size: 1.05rem;
  }

  h3 {
    margin: 0;
    font-size: 0.82rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .hint {
    margin: 0.2rem 0 0;
    color: var(--muted);
    font-size: 0.72rem;
  }

  .head-actions,
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.45rem;
  }

  .briefing-frame {
    position: fixed;
    left: -10000px;
    top: 0;
    width: 900px;
    height: 1200px;
    border: 0;
  }

  .dirty {
    color: #f0b429;
    font-size: 0.72rem;
  }

  button,
  .actions a {
    padding: 0.36rem 0.65rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    font-size: 0.76rem;
    cursor: pointer;
    text-decoration: none;
  }

  button:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .primary {
    color: #0b1220;
    background: var(--blue);
    border-color: var(--blue);
    font-weight: 600;
  }

  .ghost {
    color: var(--text);
    background: transparent;
  }

  .danger {
    color: #f85149;
    background: transparent;
    border-color: rgba(248, 81, 73, 0.4);
  }

  .layout {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    gap: 0.9rem;
    min-height: 0;
    flex: 1;
  }

  .plans {
    min-height: 0;
    overflow-y: auto;
    padding-right: 0.25rem;
    border-right: 1px solid var(--border);
  }

  .plans-title {
    margin-bottom: 0.35rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .plan-item {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    width: 100%;
    margin-bottom: 0.25rem;
    padding: 0.45rem 0.5rem;
    text-align: left;
    color: var(--text);
    background: transparent;
    border: 1px solid transparent;
  }

  .plan-item:hover {
    background: var(--bg);
  }

  .plan-item.active {
    background: var(--bg);
    border-color: var(--border);
  }

  .plan-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.78rem;
    font-weight: 600;
  }

  .plan-route {
    color: var(--muted);
    font-size: 0.68rem;
  }

  .shared {
    align-self: flex-start;
    margin-top: 0.15rem;
    padding: 0.02rem 0.35rem;
    color: var(--blue);
    background: var(--blue-soft);
    border-radius: 999px;
    font-size: 0.62rem;
    font-weight: 600;
  }

  .editor {
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
    padding-right: 0.25rem;
  }

  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 0.55rem;
    margin-bottom: 0.9rem;
  }

  .fields label,
  .notes-block label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--muted);
  }

  .fields .wide {
    grid-column: 1 / -1;
  }

  input,
  select,
  textarea {
    width: 100%;
    padding: 0.34rem 0.5rem;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-size: 0.78rem;
  }

  textarea {
    resize: vertical;
  }

  .route-block,
  .navlog-block,
  .notes-block {
    margin-bottom: 1rem;
  }

  .block-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    margin-bottom: 0.45rem;
  }

  .theatre {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--muted);
    font-size: 0.68rem;
  }

  .theatre select {
    width: auto;
  }

  .endpoint {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr) minmax(180px, 240px);
    gap: 0.45rem;
    align-items: center;
    padding: 0.4rem 0;
    border-bottom: 1px solid var(--border);
    font-size: 0.78rem;
  }

  .endpoint-label {
    color: var(--muted);
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .current {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .waypoints {
    margin: 0.25rem 0;
  }

  .waypoint {
    display: grid;
    grid-template-columns: 80px minmax(0, 1fr) 120px auto;
    gap: 0.4rem;
    align-items: center;
    padding: 0.3rem 0.2rem;
    border-bottom: 1px dashed var(--border);
    font-size: 0.76rem;
  }

  .wp-ident {
    font-weight: 600;
  }

  .wp-name,
  .wp-pos {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
  }

  .wp-actions {
    display: flex;
    gap: 0.15rem;
  }

  .wp-actions button {
    padding: 0.1rem 0.35rem;
    font-size: 0.72rem;
  }

  .add-waypoint {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.45rem;
    align-items: center;
    padding: 0.4rem 0;
  }

  .custom-point {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 110px 110px auto auto;
    gap: 0.4rem;
    margin-bottom: 0.4rem;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.74rem;
  }

  th,
  td {
    padding: 0.28rem 0.35rem;
    text-align: left;
    border-bottom: 1px solid var(--border);
  }

  th {
    color: var(--muted);
    font-size: 0.64rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  td.num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .totals {
    display: flex;
    flex-wrap: wrap;
    gap: 0.9rem;
    margin-top: 0.55rem;
    font-size: 0.72rem;
  }

  .totals div {
    display: flex;
    flex-direction: column;
    gap: 0.05rem;
  }

  .totals span {
    color: var(--muted);
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .totals b {
    font-size: 0.9rem;
    font-variant-numeric: tabular-nums;
  }

  .notes-block {
    max-width: 720px;
  }

  .muted {
    color: var(--muted);
    font-size: 0.74rem;
  }

  .empty {
    padding: 0.6rem 0;
    color: var(--muted);
    font-size: 0.78rem;
  }

  .readonly {
    margin: 0 0 0.6rem;
    padding: 0.4rem 0.55rem;
    color: var(--amber);
    background: var(--amber-soft);
    border: 1px solid color-mix(in srgb, var(--amber) 40%, transparent);
    border-radius: var(--radius-sm);
    font-size: 0.76rem;
  }

  .error {
    color: #f85149;
    font-size: 0.76rem;
  }

  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
    .plans {
      border-right: none;
      border-bottom: 1px solid var(--border);
      max-height: 140px;
    }
    .endpoint,
    .waypoint,
    .custom-point {
      grid-template-columns: 1fr;
    }
  }
</style>
