<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    open,
    onClose,
    children,
  }: {
    open: boolean;
    onClose: () => void;
    children: Snippet;
  } = $props();

  let sheetEl = $state<HTMLElement | undefined>();
  let dragStartY = $state(0);
  let dragDeltaY = $state(0);
  let dragging = $state(false);

  function handleTouchStart(e: TouchEvent) {
    dragStartY = e.touches[0].clientY;
    dragDeltaY = 0;
    dragging = true;
  }

  function handleTouchMove(e: TouchEvent) {
    if (!dragging) return;
    const dy = e.touches[0].clientY - dragStartY;
    dragDeltaY = Math.max(0, dy); // only allow dragging down
  }

  function handleTouchEnd() {
    if (!dragging) return;
    dragging = false;
    if (dragDeltaY > 120) {
      onClose();
    }
    dragDeltaY = 0;
  }
</script>

{#if open}
  <!-- Overlay -->
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 bg-black/40 transition-opacity"
       style="animation: sempa-fade-in 200ms ease both;"
       onclick={onClose}></div>

  <!-- Sheet -->
  <div bind:this={sheetEl}
       class="fixed bottom-0 left-0 right-0 z-50 flex flex-col overflow-hidden"
       style="max-height: 92vh; border-radius: 20px 20px 0 0;
              background: var(--sempa-bg-panel);
              padding-bottom: env(safe-area-inset-bottom);
              transform: translateY({dragging ? dragDeltaY : 0}px);
              transition: {dragging ? 'none' : 'transform 300ms ease-out'};
              animation: {dragging ? 'none' : 'sempa-sheet-up 300ms ease-out both'};"
       role="dialog" aria-modal="true">

    <!-- Drag handle -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="flex justify-center pt-3 pb-2 cursor-grab"
         ontouchstart={handleTouchStart}
         ontouchmove={handleTouchMove}
         ontouchend={handleTouchEnd}>
      <div class="h-1 w-9 rounded-full" style="background: var(--sempa-border);"></div>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto">
      {@render children()}
    </div>
  </div>
{/if}

<style>
  @keyframes sempa-sheet-up {
    from { transform: translateY(100%); }
    to   { transform: translateY(0); }
  }
</style>
