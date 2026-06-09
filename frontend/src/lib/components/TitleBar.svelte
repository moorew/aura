<script lang="ts">
  import { isTauri } from '$lib/tauri/bridge';
  import { onMount } from 'svelte';

  let win: any = null;
  let isMaximized = $state(false);

  onMount(async () => {
    if (!isTauri()) return;
    try {
      const { getCurrentWindow } = await import('@tauri-apps/api/window');
      win = getCurrentWindow();
      isMaximized = await win.isMaximized();
      win.onResized(async () => {
        isMaximized = await win.isMaximized();
      });
    } catch { /* Tauri API unavailable */ }
  });

  async function minimize() { await win?.minimize(); }
  async function toggleMax() { await win?.toggleMaximize(); }
  async function close() { await win?.close(); }
</script>

{#if isTauri()}
<div
  data-tauri-drag-region
  class="titlebar"
  style="
    height: 38px;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    background: var(--sempa-bg-panel);
    border-bottom: 1px solid var(--sempa-border);
    user-select: none;
    flex-shrink: 0;
  ">
  <!-- Quiet drag strip: the brand lives in the sidebar, so the titlebar is just
       a drag region + window controls (no duplicate logo/wordmark). -->

  <!-- Windows-style window controls on right -->
  <div style="display: flex; align-items: stretch; height: 38px;">
    <button
      onclick={minimize}
      title="Minimize"
      style="
        width: 46px; height: 38px; border: none; background: transparent; cursor: pointer;
        display: flex; align-items: center; justify-content: center;
        color: var(--sempa-text-soft); font-size: 12px; line-height: 1;
        transition: background 100ms;
      "
      onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.08)'}
      onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'transparent'}>
      <svg width="10" height="1" viewBox="0 0 10 1" fill="currentColor">
        <rect width="10" height="1"/>
      </svg>
    </button>
    <button
      onclick={toggleMax}
      title={isMaximized ? 'Restore' : 'Maximize'}
      style="
        width: 46px; height: 38px; border: none; background: transparent; cursor: pointer;
        display: flex; align-items: center; justify-content: center;
        color: var(--sempa-text-soft);
        transition: background 100ms;
      "
      onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'rgba(0,0,0,0.08)'}
      onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'transparent'}>
      {#if isMaximized}
        <!-- Restore icon (two overlapping squares) -->
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">
          <rect x="0" y="2" width="8" height="8"/>
          <path d="M2 2V0h8v8H8"/>
        </svg>
      {:else}
        <!-- Maximize icon (single square) -->
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">
          <rect x="0" y="0" width="10" height="10"/>
        </svg>
      {/if}
    </button>
    <button
      onclick={close}
      title="Close"
      style="
        width: 46px; height: 38px; border: none; background: transparent; cursor: pointer;
        display: flex; align-items: center; justify-content: center;
        color: var(--sempa-text-soft);
        transition: background 100ms, color 100ms;
      "
      onmouseenter={(e) => { (e.currentTarget as HTMLElement).style.background = '#c42b1c'; (e.currentTarget as HTMLElement).style.color = 'white'; }}
      onmouseleave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent'; (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-soft)'; }}>
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.2">
        <line x1="0" y1="0" x2="10" y2="10"/>
        <line x1="10" y1="0" x2="0" y2="10"/>
      </svg>
    </button>
  </div>
</div>
{/if}

<style>
  button:active {
    transform: none !important; /* override global scale(0.97) on buttons */
  }
</style>
