<script lang="ts">
  /**
   * SempaSelect — a fully custom, Sempa-styled dropdown.
   *
   * Native <select> elements render as off-brand Android system pickers, so we
   * replace them with this component. On mobile it opens as a bottom sheet with
   * large touch targets; on desktop it opens as an inline absolute dropdown that
   * sits *below* the top nav (z-dropdown < z-top-nav) so it can never cover it.
   *
   * No native <select> is rendered anywhere in the DOM.
   */
  import { mobile } from '$lib/stores/mobile.svelte';
  import { Check, ChevronDown } from 'lucide-svelte';

  type OptionValue = string | number | null;
  interface Option {
    value: OptionValue;
    label: string;
    icon?: string;
  }

  let {
    value = $bindable(),
    options,
    placeholder = 'Select…',
    id,
    onchange,
  }: {
    value: OptionValue;
    options: Option[];
    placeholder?: string;
    id?: string;
    onchange?: (value: OptionValue) => void;
  } = $props();

  let open = $state(false);
  let rootEl = $state<HTMLElement | undefined>();

  const selected = $derived(options.find((o) => o.value === value));
  const hasValue = $derived(selected !== undefined && selected.value !== null && selected.value !== '');

  function choose(opt: Option) {
    value = opt.value;
    onchange?.(opt.value);
    open = false;
  }

  // Close on Escape and on outside click/tap (desktop). The mobile scrim handles
  // outside taps for the bottom sheet.
  $effect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') { open = false; }
    }
    function onDown(e: Event) {
      if (rootEl && !rootEl.contains(e.target as Node)) open = false;
    }
    window.addEventListener('keydown', onKey);
    // capture phase so we see the tap before it's swallowed by other handlers
    window.addEventListener('pointerdown', onDown, true);
    return () => {
      window.removeEventListener('keydown', onKey);
      window.removeEventListener('pointerdown', onDown, true);
    };
  });
</script>

<div bind:this={rootEl} style="position: relative;">
  <!-- Trigger -->
  <button
    {id}
    type="button"
    onclick={() => (open = !open)}
    aria-haspopup="listbox"
    aria-expanded={open}
    class="flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2.5 text-sm outline-none transition-colors"
    style="background: var(--sempa-bg-nav); border: 1px solid var(--sempa-border);
           color: {hasValue ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'};">
    <span class="flex min-w-0 items-center gap-2 truncate">
      {#if selected?.icon}<span class="shrink-0">{selected.icon}</span>{/if}
      <span class="truncate">{selected ? selected.label : placeholder}</span>
    </span>
    <span class="shrink-0 transition-transform" style="transform: rotate({open ? 180 : 0}deg); color: var(--sempa-text-dim);">
      <ChevronDown size={16} strokeWidth={2} />
    </span>
  </button>

  {#if open}
    {#if mobile.value}
      <!-- Mobile: bottom sheet -->
      <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
      <div class="sempa-select-scrim" role="presentation" onclick={() => (open = false)}></div>
      <div class="sempa-select-sheet" role="listbox" aria-label={placeholder}>
        <!-- Drag handle pill -->
        <div class="flex justify-center pt-3 pb-2">
          <div class="h-1 w-9 rounded-full" style="background: var(--sempa-border);"></div>
        </div>
        <div class="overflow-y-auto" style="max-height: 60vh;">
          {#each options as opt, i}
            {@const isSel = opt.value === value}
            <button
              type="button"
              role="option"
              aria-selected={isSel}
              onclick={() => choose(opt)}
              class="flex w-full items-center gap-3 px-5 text-left text-[15px]"
              style="min-height: 52px; color: {isSel ? 'var(--sempa-accent)' : 'var(--sempa-text)'};
                     {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}">
              {#if isSel}
                <span class="shrink-0" style="color: var(--sempa-accent);"><Check size={20} strokeWidth={2.5} /></span>
              {:else}
                <span class="shrink-0 rounded-full" style="width: 20px; height: 20px; border: 2px solid var(--sempa-border);"></span>
              {/if}
              {#if opt.icon}<span class="shrink-0">{opt.icon}</span>{/if}
              <span class="truncate">{opt.label}</span>
            </button>
          {/each}
        </div>
      </div>
    {:else}
      <!-- Desktop: inline absolute dropdown (below top nav) -->
      <div
        class="sempa-select-menu"
        role="listbox"
        aria-label={placeholder}>
        {#each options as opt}
          {@const isSel = opt.value === value}
          <button
            type="button"
            role="option"
            aria-selected={isSel}
            onclick={() => choose(opt)}
            class="sempa-select-row flex w-full items-center gap-2 px-3 py-2 text-left text-sm"
            style="color: {isSel ? 'var(--sempa-accent)' : 'var(--sempa-text)'};">
            <span class="flex h-4 w-4 shrink-0 items-center justify-center" style="color: var(--sempa-accent);">
              {#if isSel}<Check size={15} strokeWidth={2.5} />{/if}
            </span>
            {#if opt.icon}<span class="shrink-0">{opt.icon}</span>{/if}
            <span class="truncate">{opt.label}</span>
          </button>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  /* Desktop inline dropdown — absolute, BELOW the top nav (z-dropdown < z-top-nav). */
  .sempa-select-menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    z-index: var(--z-dropdown, 30);
    max-height: 16rem;
    overflow-y: auto;
    border-radius: 0.5rem;
    border: 1px solid var(--sempa-border);
    background: var(--sempa-bg-nav);
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
    animation: sempa-fade-in 140ms ease both;
  }
  .sempa-select-row {
    transition: background-color 120ms ease;
  }
  .sempa-select-row:hover {
    background: color-mix(in srgb, var(--sempa-accent) 12%, transparent);
  }

  /* Mobile bottom sheet — intentionally above everything (z-sheet). */
  .sempa-select-scrim {
    position: fixed;
    inset: 0;
    z-index: var(--z-scrim, 89);
    background: rgba(0, 0, 0, 0.5);
    animation: sempa-fade-in 160ms ease both;
  }
  .sempa-select-sheet {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: var(--z-sheet, 90);
    border-radius: 20px 20px 0 0;
    background: var(--sempa-bg-panel);
    box-shadow: 0 -8px 30px rgba(0, 0, 0, 0.4);
    padding-bottom: max(16px, env(safe-area-inset-bottom, 16px));
    animation: sempa-sheet-up 280ms ease-out both;
  }

  @keyframes sempa-sheet-up {
    from { transform: translateY(100%); }
    to   { transform: translateY(0); }
  }
  @media (prefers-reduced-motion: reduce) {
    .sempa-select-menu, .sempa-select-scrim, .sempa-select-sheet { animation: none; }
  }
</style>
