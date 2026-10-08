<script>
  import { t } from './i18n.js';
  export let points = [];

  const metrics = [
    { id: 'score', label: 'players.score' },
    { id: 'kills', label: 'stats.killsCol' },
    { id: 'landings', label: 'stats.landingsFull' },
  ];

  function date(point) {
    return new Date(point.startedAt).toLocaleDateString();
  }

  function height(point, id, max) {
    return Math.max(0, Math.min(100, ((point[id] ?? 0) / max) * 100));
  }
</script>

{#if points.length === 0}
  <p class="empty">{$t('stats.trendEmpty')}</p>
{:else}
  <div class="charts">
    {#each metrics as metric (metric.id)}
      {@const max = Math.max(1, ...points.map((point) => point[metric.id] ?? 0))}
      <div class="chart">
        <div class="chart-head"><strong>{$t(metric.label)}</strong><span>0–{max.toLocaleString()}</span></div>
        <div class="plot" role="img" aria-label={`${$t(metric.label)} · ${points.length} ${$t('stats.missions')}`}>
          {#each points as point (point.missionId)}
            <div class="column" title={`${point.name} · ${date(point)} · ${$t(metric.label)}: ${point[metric.id]}`}>
              <div class="bar {metric.id}" style={`height: ${height(point, metric.id, max)}%`}></div>
            </div>
          {/each}
        </div>
        <div class="dates"><span>{date(points[0])}</span><span>{date(points[points.length - 1])}</span></div>
      </div>
    {/each}
  </div>
  <p class="hint">{$t('stats.trendHint')}</p>
{/if}

<style>
  .charts { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 0.7rem; }
  .chart { min-width: 0; padding: 0.6rem; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; }
  .chart-head, .dates { display: flex; justify-content: space-between; gap: 0.5rem; }
  .chart-head { font-size: 0.76rem; }
  .chart-head span, .dates, .hint { color: var(--muted); }
  .plot { height: 100px; display: flex; align-items: flex-end; gap: 3px; overflow-x: auto; margin-top: 0.6rem; border-bottom: 1px solid var(--border); }
  .column { flex: 1 0 10px; height: 100%; display: flex; align-items: flex-end; }
  .bar { width: 100%; min-height: 2px; border-radius: 3px 3px 0 0; background: var(--blue); }
  .bar.kills { background: var(--green); }
  .bar.landings { background: #f0b429; }
  .dates { margin-top: 0.25rem; font-size: 0.68rem; }
  .hint { font-size: 0.7rem; margin: 0.4rem 0 0; }
  .empty { color: var(--muted); font-size: 0.8rem; }
  @media (max-width: 780px) { .charts { grid-template-columns: 1fr; } }
</style>
