<script>
  export let label = '';
  export let value = '—';
  export let hint = '';
  export let tone = 'blue';
  export let icon = '';

  const COLORS = {
    blue: 'var(--blue)',
    green: 'var(--green)',
    red: 'var(--red)',
    amber: 'var(--amber)',
  };
</script>

<div class="stat" style="--stat-color:{COLORS[tone] ?? COLORS.blue}">
  <div class="top">
    <span class="label">{label}</span>
    {#if icon}
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
        <path d={icon} />
      </svg>
    {/if}
  </div>
  <strong>{value}</strong>
  {#if hint}<span class="hint">{hint}</span>{/if}
</div>

<style>
  .stat {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
    padding: 0.8rem 0.9rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-soft);
  }

  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
  }

  .label {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--muted);
    font-weight: 600;
  }

  svg {
    width: 16px;
    height: 16px;
    color: var(--muted);
    flex: none;
  }

  strong {
    font-size: 1.55rem;
    line-height: 1.15;
    font-variant-numeric: tabular-nums;
    letter-spacing: -0.02em;
    color: var(--stat-color);
  }

  .hint {
    font-size: 0.72rem;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
