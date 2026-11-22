<script>
  export let title = '';
  export let subtitle = '';
  export let compact = false;
</script>

<section class="card" class:compact>
  {#if title || $$slots.actions}
    <header>
      <div class="heading">
        {#if title}<h2>{title}</h2>{/if}
        {#if subtitle}<p>{subtitle}</p>{/if}
      </div>
      {#if $$slots.actions}<div class="actions"><slot name="actions" /></div>{/if}
    </header>
  {/if}
  <div class="body"><slot /></div>
</section>

<style>
  .card {
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-soft);
    overflow: hidden;
  }

  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.85rem 1rem 0.65rem;
  }

  .heading {
    min-width: 0;
  }

  h2 {
    margin: 0;
    font-size: 0.9rem;
    font-weight: 650;
    letter-spacing: 0.01em;
  }

  p {
    margin: 0.15rem 0 0;
    font-size: 0.76rem;
    color: var(--muted);
    line-height: 1.35;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex: none;
  }

  .body {
    flex: 1 1 auto;
    min-width: 0;
    padding: 0 1rem 1rem;
  }

  .compact header {
    padding-bottom: 0.35rem;
  }

  .compact .body {
    padding-bottom: 0.75rem;
  }
</style>
