<script lang="ts">
  /**
   * Reminder popup — the contents of the chromeless, always-on-top Tauri window
   * spawned top-right when a task reminder fires (see src-tauri/src/windows.rs
   * create_reminder_popup). It's a Granola-style stack of cards that floats over
   * the desktop, OUTSIDE the main app window, and stays until dismissed.
   *
   * Data flow (the main window owns the truth — $lib/desktopReminderPopup):
   *   • on mount we emit `reminder:ready` and listen for `reminder:list`
   *   • each user action emits `reminder:action` { action, taskId } back to main
   * The window resizes itself to fit the cards (top-right stays anchored since
   * only the height changes).
   */
  import { onMount } from 'svelte';
  import { Bell, Check, X } from 'lucide-svelte';

  interface Card {
    taskId: string;
    title: string;
    subtitle: string;
  }

  let cards = $state<Card[]>([]);
  let stackEl: HTMLElement | undefined = $state();

  // — Tauri bridges (loaded lazily; this route only ever runs in the desktop shell) —
  let emitFn: ((event: string, payload?: unknown) => Promise<void>) | null = null;

  async function act(action: 'open' | 'done' | 'snooze' | 'dismiss', taskId: string) {
    // Optimistically drop the card so the UI feels instant; the main window is
    // the source of truth and will re-emit the authoritative list anyway.
    if (action !== 'open') cards = cards.filter((c) => c.taskId !== taskId);
    await emitFn?.('reminder:action', { action, taskId });
  }

  // Resize the window to hug the card stack. Width is fixed (set in Rust); only
  // the height changes, so the top-right corner stays put.
  async function fitWindow() {
    if (typeof window === 'undefined') return;
    try {
      const { getCurrentWebviewWindow } = await import('@tauri-apps/api/webviewWindow');
      const { LogicalSize } = await import('@tauri-apps/api/dpi');
      const h = Math.max(80, Math.ceil((stackEl?.scrollHeight ?? 120) + 24));
      await getCurrentWebviewWindow().setSize(new LogicalSize(384, h));
    } catch {
      /* not in Tauri / API unavailable */
    }
  }

  // Re-fit whenever the cards change (after the DOM updates).
  $effect(() => {
    cards.length; // track
    requestAnimationFrame(fitWindow);
  });

  onMount(() => {
    let unlisten: (() => void) | null = null;
    (async () => {
      try {
        const { listen, emit } = await import('@tauri-apps/api/event');
        emitFn = emit;
        unlisten = await listen<Card[]>('reminder:list', (e) => {
          cards = Array.isArray(e.payload) ? e.payload : [];
        });
        // Tell the main window we're ready so it (re)sends the current list —
        // avoids the race where the first emit fires before this window mounts.
        await emit('reminder:ready');
      } catch {
        /* not in Tauri */
      }
    })();
    return () => unlisten?.();
  });
</script>

<!-- Transparent window: only the cards are visible, floating over the desktop. -->
<div class="popup-root">
  <div class="stack" bind:this={stackEl}>
    {#each cards as c (c.taskId)}
      <div class="card">
        <!-- Body = click to open the task -->
        <button class="open" onclick={() => act('open', c.taskId)} title="Open task">
          <span class="bar"></span>
          <span class="icon"><Bell size={15} strokeWidth={2} /></span>
          <span class="text">
            <span class="label">Reminder</span>
            <span class="title">{c.title}</span>
            {#if c.subtitle}<span class="sub">{c.subtitle}</span>{/if}
          </span>
        </button>

        <div class="actions">
          <button class="act-btn" onclick={() => act('done', c.taskId)} title="Mark done" aria-label="Mark done">
            <Check size={15} strokeWidth={2.25} />
          </button>
          <button class="act-btn" onclick={() => act('dismiss', c.taskId)} title="Dismiss" aria-label="Dismiss">
            <X size={15} strokeWidth={2.25} />
          </button>
        </div>
      </div>
    {/each}
  </div>
</div>

<style>
  :global(html), :global(body) {
    background: transparent !important;
    margin: 0;
    overflow: hidden;
  }

  .popup-root {
    width: 100vw;
    min-height: 100vh;
    padding: 8px;
    box-sizing: border-box;
    font-family: 'Plus Jakarta Sans', system-ui, sans-serif;
  }

  .stack {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  /* Dark card regardless of OS/app theme — reads cleanly over any wallpaper,
     matching the Granola reference. */
  .card {
    display: flex;
    align-items: stretch;
    background: rgba(28, 22, 18, 0.97);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 12px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.45), 0 2px 8px rgba(0, 0, 0, 0.3);
    overflow: hidden;
    backdrop-filter: blur(8px);
  }

  .open {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1 1 auto;
    min-width: 0;
    padding: 11px 4px 11px 0;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    transition: background 120ms ease;
  }
  .open:hover { background: rgba(255, 255, 255, 0.04); }

  .bar {
    width: 4px;
    align-self: stretch;
    margin-right: 8px;
    border-radius: 0 3px 3px 0;
    background: #cc6e3a; /* terracotta accent */
    flex: 0 0 auto;
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border-radius: 8px;
    background: rgba(204, 110, 58, 0.16);
    color: #e08a54;
    flex: 0 0 auto;
  }

  .text { display: flex; flex-direction: column; min-width: 0; line-height: 1.25; }
  .label {
    font-size: 9.5px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #e08a54;
  }
  .title {
    font-size: 13.5px;
    font-weight: 600;
    color: #f4efe9;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .sub { font-size: 11px; color: rgba(244, 239, 233, 0.55); }

  .actions {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 8px 0 4px;
    flex: 0 0 auto;
  }
  .act-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 8px;
    border: none;
    background: none;
    color: rgba(244, 239, 233, 0.55);
    cursor: pointer;
    transition: background 120ms ease, color 120ms ease;
  }
  .act-btn:hover { background: rgba(255, 255, 255, 0.08); color: #f4efe9; }
</style>
