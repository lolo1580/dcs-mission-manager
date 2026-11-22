<script>
  export let title = '';
  export let message = '';
  export let icon = 'M12 3l9 18H3zM12 10v5M12 18h.01';
</script>

<div class="empty">
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true">
    <path d={icon} />
  </svg>
  {#if title}<h3>{title}</h3>{/if}
  {#if message}<p>{message}</p>{/if}
  <slot />
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.4rem;
    padding: 2rem 1rem;
    text-align: center;
    color: var(--muted);
  }

  svg {
    width: 34px;
    height: 34px;
    opacity: 0.5;
    margin-bottom: 0.25rem;
  }

  h3 {
    margin: 0;
    color: var(--text);
    font-size: 0.88rem;
    font-weight: 600;
  }

  p {
    margin: 0;
    max-width: 42ch;
    font-size: 0.78rem;
    line-height: 1.45;
  }
</style>
