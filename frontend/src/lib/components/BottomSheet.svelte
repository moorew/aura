<script lang="ts">
  import type { Snippet } from 'svelte';
  import { dismissibleSheet } from '$lib/actions/sheet';
  import { viewport } from '$lib/stores/viewport.svelte';
  import { hapticTick } from '$lib/haptics';

  let {
    open,
    onClose,
    children,
  }: {
    open: boolean;
    onClose: () => void;
    children: Snippet;
  } = $props();

</script>

{#if open}
  <!-- Overlay -->
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[89] bg-black/40"
       style="animation: sempa-fade-in 200ms ease both;"
       onclick={onClose}></div>

  <!-- Sheet — lifted above the soft keyboard so any inputs/footer stay reachable -->
  <!-- max-height off the LIVE layout viewport (100%) so it can't get stuck at a
       stale visualViewport height when the Android keyboard dismisses. -->
  <div class="fixed left-0 right-0 z-[90] flex flex-col overflow-hidden"
       style="bottom: {viewport.keyboardHeight}px;
              max-height: calc(100% - max(32px, env(safe-area-inset-top, 0px)) - {viewport.keyboardHeight}px);
              border-radius: 20px 20px 0 0;
              background: var(--sempa-bg-panel);
              padding-bottom: {viewport.keyboardHeight > 0 ? '0px' : 'env(safe-area-inset-bottom)'};
              transition: bottom 180ms ease-out;
              animation: sempa-sheet-up 320ms cubic-bezier(0.32, 0.72, 0, 1) both;"
       role="dialog" aria-modal="true"
       use:dismissibleSheet={{ onClose, scrollSelector: '[data-sheet-scroll]', onDismissHaptic: hapticTick }}>

    <!-- Drag handle -->
    <div class="flex justify-center pt-3 pb-2 cursor-grab shrink-0" data-sheet-handle>
      <div class="h-1 w-9 rounded-full" style="background: var(--sempa-border);"></div>
    </div>

    <!-- Content -->
    <!-- flex-[1_1_auto]+min-h-0: basis-0 (flex-1) collapses inside a max-height
         flex column in Chromium, leaving the sheet stuck at handle height. -->
    <div class="flex-[1_1_auto] min-h-0 overflow-y-auto overscroll-contain" data-sheet-scroll
         style="-webkit-overflow-scrolling: touch;">
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
