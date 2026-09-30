<script>
  /**
   * Analytics plot: a projection-free, top-down view of the recorded data.
   *
   * The live map is gone, so this is how the analysis stays visual. It draws the
   * heatmap (traffic or loss clusters) as coloured grid cells and the recorded
   * flight trails as polylines, both in raw lat/lng scaled to equal aspect.
   * Pure SVG: no map library, no tiles, no network.
   */
  import { bounds, projector, fmtLat, fmtLng, trackColor } from './plots.js';
  import { t } from './i18n.js';

  /** Heat points ({lat,lng,weight}); grid is their size in degrees. */
  export let heat = [];
  export let grid = 0.05;
  /** Recorded trails, keyed by unit id. */
  export let trails = {};
  export let showHeat = true;
  export let showTrails = true;

  // A fixed drawing box, scaled by CSS through the viewBox, so the plot does not
  // need a resize observer.
  const W = 1000;
  const H = 620;
  const PAD = 44;

  // Svelte only tracks the stores a reactive statement names directly. Referencing
  // heat and trails here (rather than inside a helper) is what makes the box, and
  // therefore the projection, recompute once the data arrives.
  $: plotPoints = [...heat, ...Object.values(trails).flat()];
  $: box = bounds(plotPoints);
  $: proj = box ? projector(box, W, H, PAD) : null;

  /** Trails as drawable strings, one entry per unit, longest first. */
  $: drawnTrails = !proj
    ? []
    : Object.entries(trails)
        .filter(([, pts]) => (pts?.length ?? 0) > 1)
        .sort((a, b) => b[1].length - a[1].length)
        .map(([id, pts], i) => ({
          id,
          color: trackColor(i),
          // A path string, rounded to keep the DOM small on long tracks.
          d: pts.map((p, j) => `${j ? 'L' : 'M'}${proj.x(p.lng).toFixed(1)} ${proj.y(p.lat).toFixed(1)}`).join(' '),
          start: pts[0],
          end: pts[pts.length - 1],
        }));

  /** Highest cell weight, so the colour scale uses the full range. */
  $: maxWeight = heat.reduce((m, p) => Math.max(m, p.weight ?? 0), 0) || 1;

  /** Cells sized to the aggregation grid, so the picture is not sparse dots. */
  $: cells =
    !proj || !heat.length
      ? []
      : heat.map((p) => {
          const x0 = proj.x(p.lng - grid / 2);
          const x1 = proj.x(p.lng + grid / 2);
          const y0 = proj.y(p.lat + grid / 2);
          const y1 = proj.y(p.lat - grid / 2);
          return {
            x: x0,
            y: y0,
            w: Math.max(1, x1 - x0) + 0.5,
            h: Math.max(1, y1 - y0) + 0.5,
            // Square-root scaling: a few cells are far heavier than the rest, and
            // a linear scale would leave everything else invisible.
            t: Math.sqrt((p.weight ?? 0) / maxWeight),
          };
        });

  /** Colour of a heat cell: blue (sparse) to red (dense), translucent. */
  function cellColor(t) {
    const hue = 220 - 220 * t;
    return `hsl(${hue}, 90%, ${52 + 8 * (1 - t)}%)`;
  }

  function cellOpacity(t) {
    return (0.18 + 0.62 * t).toFixed(2);
  }

  $: hasHeat = showHeat && cells.length > 0;
  $: hasTrails = showTrails && drawnTrails.length > 0;
  $: empty = !box || (!hasHeat && !hasTrails);
</script>

{#if empty}
  <p class="empty">
    {#if !showHeat && !showTrails}
      {$t('analytics.noLayers')}
    {:else}
      {$t('analytics.noTrack')}
    {/if}
  </p>
{:else}
  <figure class="plot">
    <svg viewBox="0 0 {W} {H}" role="img" aria-label={$t('analytics.plotLabel')}>
      <rect class="frame" x="1" y="1" width={W - 2} height={H - 2} rx="10" />

      <!-- Corner coordinates, so the plot can be read geographically. -->
      <text class="axis" x={PAD} y={H - 12}>{fmtLng(box.minLng)}</text>
      <text class="axis" x={W - PAD} y={H - 12} text-anchor="end">{fmtLng(box.maxLng)}</text>
      <text class="axis" x={PAD} y={PAD - 14}>{fmtLat(box.maxLat)}</text>
      <text class="axis" x={PAD} y={H - PAD + 26}>{fmtLat(box.minLat)}</text>

      {#if hasHeat}
        <g class="heat">
          {#each cells as c, i (i)}
            <rect
              x={c.x}
              y={c.y}
              width={c.w}
              height={c.h}
              fill={cellColor(c.t)}
              opacity={cellOpacity(c.t)}
            />
          {/each}
        </g>
      {/if}

      {#if hasTrails}
        <g class="trails">
          {#each drawnTrails as tr (tr.id)}
            <path d={tr.d} fill="none" stroke={tr.color} stroke-width="2.2" stroke-linejoin="round" stroke-linecap="round" />
            <circle cx={proj.x(tr.start.lng)} cy={proj.y(tr.start.lat)} r="4" fill="none" stroke={tr.color} stroke-width="2" />
            <circle cx={proj.x(tr.end.lng)} cy={proj.y(tr.end.lat)} r="4.5" fill={tr.color} />
          {/each}
        </g>
      {/if}
    </svg>

    <figcaption>
      {#if hasHeat}
        <span class="legend">
          <span class="legend-label">{$t('analytics.density')}</span>
          <span class="ramp" aria-hidden="true"></span>
          <span class="legend-note">{$t('analytics.low')} → {$t('analytics.high')} ({maxWeight})</span>
        </span>
      {/if}
      {#if hasTrails}
        <span class="legend">
          <span class="track-sample" aria-hidden="true"></span>
          <span class="legend-note">
            {drawnTrails.length}
            {drawnTrails.length === 1 ? $t('analytics.track') : $t('analytics.tracks')}
          </span>
        </span>
      {/if}
    </figcaption>
  </figure>
{/if}

<style>
  .plot {
    margin: 0;
  }

  svg {
    display: block;
    width: 100%;
    height: auto;
    max-height: 62vh;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 10px;
  }

  .frame {
    fill: none;
    stroke: var(--border);
  }

  .axis {
    fill: var(--muted);
    font-size: 15px;
    font-variant-numeric: tabular-nums;
  }

  .trails path {
    /* A trace of the dark background keeps light tracks legible over dark cells. */
    filter: drop-shadow(0 0 1.5px rgba(0, 0, 0, 0.85));
  }

  figcaption {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 1.1rem;
    margin-top: 0.4rem;
    font-size: 0.72rem;
    color: var(--muted);
  }

  .legend {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }

  .legend-label {
    color: var(--text);
  }

  .ramp {
    width: 90px;
    height: 9px;
    border-radius: 5px;
    background: linear-gradient(90deg, hsl(220, 90%, 58%), hsl(120, 90%, 57%), hsl(0, 90%, 56%));
  }

  .track-sample {
    width: 20px;
    height: 0;
    border-top: 2.5px solid var(--blue);
    border-radius: 2px;
  }

  .legend-note {
    font-variant-numeric: tabular-nums;
  }

  .empty {
    color: var(--muted);
    font-size: 0.8rem;
    line-height: 1.5;
  }
</style>
