<script lang="ts">
  /**
   * SempaDatePicker — a fully custom, Sempa-styled calendar picker.
   *
   * Native <input type="date"> renders an off-brand Android/OS picker, so we
   * replace it with this component. The trigger shows a formatted date (or a
   * placeholder); clicking it opens a calendar dropdown. No native date input
   * is rendered anywhere in the DOM.
   *
   * Value is a `YYYY-MM-DD` string (bindable). Emits `onchange(value)` too.
   */
  import { Calendar, ChevronLeft, ChevronRight, X } from 'lucide-svelte';

  let {
    value = $bindable(''),
    disabled = false,
    id,
    placeholder = 'No date',
    onchange,
  }: {
    value?: string;
    disabled?: boolean;
    id?: string;
    placeholder?: string;
    onchange?: (value: string) => void;
  } = $props();

  let open = $state(false);
  let rootEl = $state<HTMLElement | undefined>();

  const DOW = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];

  function pad(n: number): string {
    return n < 10 ? `0${n}` : String(n);
  }
  function toYmd(y: number, m: number, d: number): string {
    return `${y}-${pad(m + 1)}-${pad(d)}`;
  }
  function todayYmd(): string {
    const n = new Date();
    return toYmd(n.getFullYear(), n.getMonth(), n.getDate());
  }

  // Date currently shown in the picker (defaults to value, else today)
  let viewYear = $state(0);
  let viewMonth = $state(0);

  function syncView() {
    const base = value || todayYmd();
    const [y, m] = base.split('-').map(Number);
    viewYear = y;
    viewMonth = m - 1;
  }

  function formatTrigger(v: string): string {
    if (!v) return placeholder;
    const [y, m, d] = v.split('-').map(Number);
    const date = new Date(y, m - 1, d);
    return date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
  }

  const hasValue = $derived(!!value);
  const today = todayYmd();

  const monthLabel = $derived(
    new Date(viewYear, viewMonth, 1).toLocaleDateString('en-US', { month: 'long', year: 'numeric' })
  );

  // Calendar grid cells: leading blanks + day numbers
  const cells = $derived.by(() => {
    const firstDow = new Date(viewYear, viewMonth, 1).getDay();
    const daysInMonth = new Date(viewYear, viewMonth + 1, 0).getDate();
    const out: (number | null)[] = [];
    for (let i = 0; i < firstDow; i++) out.push(null);
    for (let d = 1; d <= daysInMonth; d++) out.push(d);
    return out;
  });

  function toggle() {
    if (disabled) return;
    if (!open) syncView();
    open = !open;
  }

  function selectDay(d: number) {
    value = toYmd(viewYear, viewMonth, d);
    onchange?.(value);
    open = false;
  }

  function selectToday() {
    value = today;
    onchange?.(value);
    open = false;
  }

  function clear(e: MouseEvent) {
    e.stopPropagation();
    value = '';
    onchange?.('');
  }

  function prevMonth() {
    if (viewMonth === 0) { viewMonth = 11; viewYear--; }
    else viewMonth--;
  }
  function nextMonth() {
    if (viewMonth === 11) { viewMonth = 0; viewYear++; }
    else viewMonth++;
  }

  $effect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') open = false; }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });
</script>

<div bind:this={rootEl} style="position: relative;">
  <!-- Trigger -->
  <button
    {id}
    type="button"
    {disabled}
    onclick={toggle}
    aria-haspopup="dialog"
    aria-expanded={open}
    class="flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2.5 text-sm outline-none transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
    style="background: var(--sempa-bg-main); border: 1px solid {open ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
           color: {hasValue ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'};">
    <span class="flex min-w-0 items-center gap-2 truncate">
      <span class="shrink-0" style="color: {hasValue ? 'var(--sempa-accent)' : 'var(--sempa-text-dim)'};">
        <Calendar size={15} strokeWidth={2} />
      </span>
      <span class="truncate">{formatTrigger(value)}</span>
    </span>
    {#if hasValue && !disabled}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <span class="shrink-0 rounded p-0.5" style="color: var(--sempa-text-dim);"
            role="button" tabindex="-1" aria-label="Clear date"
            onclick={clear} onkeydown={() => {}}>
        <X size={14} strokeWidth={2} />
      </span>
    {/if}
  </button>

  {#if open}
    <!-- Backdrop -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div style="position: fixed; inset: 0; z-index: 59;" onclick={() => (open = false)}></div>

    <!-- Calendar dropdown -->
    <div class="sempa-date-menu" role="dialog" aria-label="Choose date">
      <!-- Month nav -->
      <div class="flex items-center justify-between px-3 py-2">
        <button type="button" onclick={prevMonth} aria-label="Previous month"
                class="rounded-lg p-1.5 transition-colors" style="color: var(--sempa-text-soft);">
          <ChevronLeft size={16} strokeWidth={2} />
        </button>
        <span class="text-sm font-semibold" style="color: var(--sempa-text);">{monthLabel}</span>
        <button type="button" onclick={nextMonth} aria-label="Next month"
                class="rounded-lg p-1.5 transition-colors" style="color: var(--sempa-text-soft);">
          <ChevronRight size={16} strokeWidth={2} />
        </button>
      </div>

      <!-- Day-of-week header -->
      <div class="grid grid-cols-7 px-2 pb-1">
        {#each DOW as d}
          <div class="text-center text-[11px] font-medium" style="color: var(--sempa-text-dim);">{d}</div>
        {/each}
      </div>

      <!-- Day grid -->
      <div class="grid grid-cols-7 gap-0.5 px-2 pb-2">
        {#each cells as cell}
          {#if cell === null}
            <div class="h-8"></div>
          {:else}
            {@const ymd = toYmd(viewYear, viewMonth, cell)}
            {@const isSel = ymd === value}
            {@const isToday = ymd === today}
            <button type="button" onclick={() => selectDay(cell)}
                    class="sempa-date-day flex h-8 items-center justify-center rounded-lg text-sm transition-colors"
                    style={isSel
                      ? 'background: var(--sempa-accent); color: var(--sempa-btn-fg); font-weight: 600;'
                      : isToday
                        ? 'border: 1px solid var(--sempa-accent); color: var(--sempa-accent);'
                        : 'color: var(--sempa-text);'}>
              {cell}
            </button>
          {/if}
        {/each}
      </div>

      <!-- Footer: Today shortcut -->
      <div class="px-2 py-2" style="border-top: 1px solid var(--sempa-border);">
        <button type="button" onclick={selectToday}
                class="w-full rounded-lg py-1.5 text-xs font-medium transition-colors"
                style="color: var(--sempa-accent);">
          Today
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .sempa-date-menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 60;
    width: 17rem;
    max-width: calc(100vw - 2rem);
    border-radius: 0.75rem;
    border: 1px solid var(--sempa-border);
    background: var(--sempa-bg-nav);
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
    animation: sempa-fade-in 140ms ease both;
  }
  .sempa-date-day:hover {
    background: var(--sempa-border);
  }
  @media (prefers-reduced-motion: reduce) {
    .sempa-date-menu { animation: none; }
  }
</style>
