<script>
  export let tone = 'neutral';

  const COLORS = {
    blue: 'var(--blue)',
    green: 'var(--green)',
    red: 'var(--red)',
    amber: 'var(--amber)',
    neutral: 'var(--muted)',
  };
</script>

<span class="badge" style="--badge-color:{COLORS[tone] ?? COLORS.neutral}"><slot /></span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.08rem 0.42rem;
    border-radius: 999px;
    border: 1px solid color-mix(in srgb, var(--badge-color) 45%, transparent);
    background: color-mix(in srgb, var(--badge-color) 13%, transparent);
    color: var(--badge-color);
    font-size: 0.68rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }
</style>
