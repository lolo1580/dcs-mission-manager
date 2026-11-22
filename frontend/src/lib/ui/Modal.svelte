<script>
  import { createEventDispatcher, tick } from 'svelte';

  export let open = false;
  export let title = '';
  export let subtitle = '';
  export let wide = false;

  const dispatch = createEventDispatcher();
  let panel;

  function close() {
    dispatch('close');
  }

  function onKey(event) {
    if (!open) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      close();
    }
  }

  // Move focus into the dialog when it opens so keyboard users do not stay on
  // the page behind it.
  $: if (open && panel) {
    tick().then(() => {
      const target = panel.querySelector('input, select, textarea, button');
      if (target) target.focus();
    });
  }
</script>

<svelte:window on:keydown={onKey} />

{#if open}
  <div class="backdrop" role="presentation" on:click|self={close}>
    <div class="panel" class:wide bind:this={panel} role="dialog" aria-modal="true" aria-label={title}>
      <header>
        <div>
          <h2>{title}</h2>
          {#if subtitle}<p>{subtitle}</p>{/if}
        </div>
        <button class="close" type="button" on:click={close} aria-label="Close">×</button>
      </header>
      <div class="content"><slot /></div>
      {#if $$slots.footer}
        <footer><slot name="footer" /></footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: 4vh 1rem;
    background: rgba(4, 8, 13, 0.68);
    backdrop-filter: blur(2px);
    overflow-y: auto;
  }

  .panel {
    width: min(560px, 100%);
    background: var(--panel);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
    overflow: hidden;
  }

  .panel.wide {
    width: min(860px, 100%);
  }

  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.9rem 1rem 0.7rem;
    border-bottom: 1px solid var(--border);
  }

  h2 {
    margin: 0;
    font-size: 0.98rem;
  }

  p {
    margin: 0.2rem 0 0;
    font-size: 0.76rem;
    color: var(--muted);
  }

  .close {
    flex: none;
    width: 30px;
    height: 30px;
    font-size: 1.25rem;
    line-height: 1;
    color: var(--muted);
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    cursor: pointer;
  }

  .close:hover {
    color: var(--text);
    border-color: var(--border);
  }

  .content {
    padding: 0.9rem 1rem;
  }

  footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    padding: 0.75rem 1rem;
    border-top: 1px solid var(--border);
    background: var(--bg-elevated);
  }
</style>
