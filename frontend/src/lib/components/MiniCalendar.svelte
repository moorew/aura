<script lang="ts">
  import { today } from '$lib/utils';

  let {
    date,
    onDateClick,
    onDateDrop,
    dragActive = false,
  }: {
    date: string;
    onDateClick?: (d: string) => void;
    // Drop a dragged planner task onto a calendar day → move it to ANY date,
    // including ones outside the visible 7-day board. This is the escape hatch
    // for "drag a task to a date that isn't currently in the pane".
    onDateDrop?: (d: string) => void;
    // True while a planner task is being dragged — surfaces a hint so the
    // drop-on-any-day affordance is discoverable rather than hidden.
    dragActive?: boolean;
  } = $props();

  const todayStr = today();
  let viewYear  = $state(new Date().getFullYear());
  let viewMonth = $state(new Date().getMonth()); // 0-indexed

  // ── Drag-and-drop target support ───────────────────────────────────────────
  let dragOver = $state<string | null>(null);

  // Only react to OUR task drags (set by TaskCard / column drag start), never
  // arbitrary file/text drags hovering the calendar.
  function isTaskDrag(e: DragEvent): boolean {
    const types = e.dataTransfer?.types;
    return !!types && Array.prototype.includes.call(types, 'application/x-sempa-task');
  }

  function onDayDragOver(e: DragEvent, ds: string) {
    if (!onDateDrop || !isTaskDrag(e)) return;
    e.preventDefault(); // mark as a valid drop target
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    dragOver = ds;
  }
  function onDayDrop(e: DragEvent, ds: string) {
    if (!onDateDrop || !isTaskDrag(e)) return;
    e.preventDefault();
    dragOver = null;
    onDateDrop(ds);
  }

  // Hovering a dragged task over the month chevrons pages the calendar, so you
  // can reach a date in another month without dropping first. Throttled so a
  // steady hover doesn't blow through several months at once.
  let pageCooldown = 0;
  function dragPage(e: DragEvent, dir: -1 | 1) {
    if (!onDateDrop || !isTaskDrag(e)) return;
    e.preventDefault();
    const now = Date.now();
    if (now - pageCooldown < 500) return;
    pageCooldown = now;
    if (dir < 0) prev(); else next();
  }

  const DAYS   = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'];
  const MONTHS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
  ];

  let cells = $derived.by(() => {
    const first = new Date(viewYear, viewMonth, 1).getDay();
    const days  = new Date(viewYear, viewMonth + 1, 0).getDate();
    const out: (number | null)[] = Array(first).fill(null);
    for (let d = 1; d <= days; d++) out.push(d);
    return out;
  });

  function toStr(y: number, m: number, d: number) {
    return `${y}-${String(m + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
  }

  function prev() {
    if (viewMonth === 0) { viewMonth = 11; viewYear--; } else viewMonth--;
  }
  function next() {
    if (viewMonth === 11) { viewMonth = 0; viewYear++; } else viewMonth++;
  }
</script>

<div class="select-none px-4 py-3"
     style={dragActive && onDateDrop ? 'box-shadow: inset 0 0 0 1.5px var(--sempa-accent); border-radius: 12px;' : ''}>
  {#if dragActive && onDateDrop}
    <div class="mb-2 flex items-center justify-center gap-1.5 rounded-md py-1 text-[10.5px] font-medium"
         style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
      <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M8 7V3m8 4V3M3 11h18M5 7h14a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V9a2 2 0 012-2z"/>
      </svg>
      Drop on any day to reschedule
    </div>
  {/if}
  <!-- Month header -->
  <div class="mb-3 flex items-center justify-between">
    <span class="text-xs font-semibold text-gray-700 dark:text-gray-200">
      {MONTHS[viewMonth]} {viewYear}
    </span>
    <div class="flex gap-0.5">
      <button onclick={prev} aria-label="Previous month"
              ondragover={(e) => dragPage(e, -1)}
              class="rounded p-0.5 transition-colors"
              style="color: var(--sempa-text-dim);"
              onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text)'}
              onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <button onclick={next} aria-label="Next month"
              ondragover={(e) => dragPage(e, 1)}
              class="rounded p-0.5 transition-colors"
              style="color: var(--sempa-text-dim);"
              onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text)'}
              onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- Day-of-week headers -->
  <div class="mb-1 grid grid-cols-7">
    {#each DAYS as day}
      <div class="flex items-center justify-center py-0.5 text-center text-[10.5px] font-medium
                  text-gray-400 dark:text-gray-600">{day}</div>
    {/each}
  </div>

  <!-- Day cells -->
  <div class="grid grid-cols-7 gap-y-0.5">
    {#each cells as cell}
      {#if cell === null}
        <div></div>
      {:else}
        {@const ds = toStr(viewYear, viewMonth, cell)}
        {@const isToday = ds === todayStr}
        {@const isSel = ds === date}
        {@const isDragOver = dragOver === ds}
        <div class="flex items-center justify-center"
             role="gridcell" tabindex="-1"
             ondragover={(e) => onDayDragOver(e, ds)}
             ondragleave={() => { if (dragOver === ds) dragOver = null; }}
             ondrop={(e) => onDayDrop(e, ds)}>
          <button onclick={() => onDateClick?.(ds)}
                  class="flex h-6 w-6 items-center justify-center rounded-full text-[11px] transition-colors
                         {!isToday && !isSel && !isDragOver ? 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800' : ''}"
                  style={isDragOver
                    ? 'background:var(--sempa-accent);color:var(--sempa-btn-fg);font-weight:700;box-shadow:0 0 0 2px var(--sempa-accent-bg);'
                    : isToday && isSel
                    ? 'background:var(--sempa-today-bg);color:var(--sempa-today-fg);font-weight:700;'
                    : isToday
                      ? 'background:var(--sempa-accent-bg);color:var(--sempa-accent);font-weight:600;'
                      : isSel
                        ? 'background:var(--sempa-accent-bg);color:var(--sempa-text);font-weight:500;'
                        : ''}>
            {cell}
          </button>
        </div>
      {/if}
    {/each}
  </div>
</div>
