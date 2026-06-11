<script lang="ts">
  import { api } from '$lib/api';
  import type { ICalEvent, Task } from '$lib/types';
  import { formatMinutes, today as getToday } from '$lib/utils';
  import { calendars, calFg, calBg } from '$lib/stores/calendars.svelte';

  let {
    date,
    tasks,
    onSchedule,  // (taskId, start ISO, end ISO) => void
    onUnschedule, // (taskId) => void
  }: {
    date: string;
    tasks: Task[];
    onSchedule?: (taskId: string, start: string, end: string) => void;
    onUnschedule?: (taskId: string) => void;
  } = $props();

  const START_HOUR = 6;
  const END_HOUR   = 22;
  const HOUR_PX    = 56;
  const TOTAL      = END_HOUR - START_HOUR;
  const hours      = Array.from({ length: TOTAL }, (_, i) => START_HOUR + i);

  let containerEl = $state<HTMLElement | undefined>();
  let dragOver    = $state(false);
  let ghostHour   = $state<number | null>(null);
  let icalEvents  = $state<ICalEvent[]>([]);
  let nowPx       = $state<number | null>(null);

  // ── Calendar show/hide ──────────────────────────────────────────────────────
  // Visibility + brand colour live in the shared `calendars` store (also driven
  // by the Calendars settings tab), so toggling there updates the schedule live.
  let showFilter = $state(false);

  // Distinct calendars present in the current day's events (key + label + colour).
  const eventCalendars = $derived.by(() => {
    const map = new Map<string, { key: string; name: string; color: string }>();
    for (const ev of icalEvents) {
      if (!map.has(ev.subscription_id)) {
        map.set(ev.subscription_id, {
          key: ev.subscription_id,
          name: ev.calendar || 'Calendar',
          color: ev.color || '#6b7280',
        });
      }
    }
    return [...map.values()].sort((a, b) => a.name.localeCompare(b.name));
  });

  const visibleEvents = $derived(icalEvents.filter(ev => !ev.all_day && !calendars.isHidden(ev.subscription_id)));

  // Past events fade out (today only) when the "Dim past events" pref is on.
  function isPast(iso: string): boolean {
    if (!calendars.display.dimPastEvents || date !== getToday()) return false;
    return new Date(iso).getTime() < Date.now();
  }

  function updateNow() {
    if (date !== getToday()) { nowPx = null; return; }
    const now = new Date();
    const h = now.getHours() + now.getMinutes() / 60;
    nowPx = (h >= START_HOUR && h < END_HOUR) ? (h - START_HOUR) * HOUR_PX : null;
  }

  $effect(() => {
    date; updateNow();
    const id = setInterval(updateNow, 60_000);
    return () => clearInterval(id);
  });

  $effect(() => {
    date; // re-load when date changes
    api.ical.listEvents(date).then(evs => { icalEvents = evs; }).catch(() => {});
  });

  const scheduled = $derived(
    tasks.filter(t => t.scheduled_start && t.scheduled_start.startsWith(date))
  );

  // ── Overlap layout ──────────────────────────────────────────────────────────
  // Pack concurrent items into side-by-side columns (like Google Calendar) so
  // two events at the same time sit next to each other instead of stacking on
  // top. Task blocks and calendar events share one layout so they never cover
  // each other either. Returns a map: item key → { col, cols }.
  function minutesOf(iso: string): number {
    const d = new Date(iso);
    return d.getHours() * 60 + d.getMinutes();
  }

  type LayoutItem = { key: string; start: number; end: number };
  const layout = $derived.by(() => {
    const items: LayoutItem[] = [];
    for (const ev of visibleEvents) {
      items.push({ key: 'e:' + ev.id, start: minutesOf(ev.start_time), end: Math.max(minutesOf(ev.end_time), minutesOf(ev.start_time) + 15) });
    }
    for (const t of scheduled) {
      const s = minutesOf(t.scheduled_start!);
      const e = t.scheduled_end ? minutesOf(t.scheduled_end) : s + 30;
      items.push({ key: 't:' + t.id, start: s, end: Math.max(e, s + 15) });
    }
    items.sort((a, b) => a.start - b.start || a.end - b.end);

    const result = new Map<string, { col: number; cols: number }>();
    let cluster: (LayoutItem & { col: number })[] = [];
    let clusterEnd = -Infinity;

    const flush = () => {
      const colEnds: number[] = []; // last end time placed in each column
      for (const it of cluster) {
        let placed = false;
        for (let c = 0; c < colEnds.length; c++) {
          if (colEnds[c] <= it.start) { colEnds[c] = it.end; it.col = c; placed = true; break; }
        }
        if (!placed) { it.col = colEnds.length; colEnds.push(it.end); }
      }
      const cols = colEnds.length;
      for (const it of cluster) result.set(it.key, { col: it.col, cols });
      cluster = [];
      clusterEnd = -Infinity;
    };

    for (const it of items) {
      if (cluster.length && it.start >= clusterEnd) flush();
      cluster.push({ ...it, col: 0 });
      clusterEnd = Math.max(clusterEnd, it.end);
    }
    if (cluster.length) flush();
    return result;
  });

  // Left/width CSS for an item, leaving a small gutter between columns.
  function colStyle(key: string): string {
    const pos = layout.get(key);
    if (!pos || pos.cols <= 1) return 'left: 2px; right: 2px;';
    const w = 100 / pos.cols;
    return `left: calc(${pos.col * w}% + 2px); width: calc(${w}% - 4px);`;
  }

  function blockStyle(task: Task): { top: string; height: string } | null {
    if (!task.scheduled_start) return null;
    const s = new Date(task.scheduled_start);
    const e = task.scheduled_end
      ? new Date(task.scheduled_end)
      : new Date(s.getTime() + 30 * 60000);

    const startH = s.getHours() + s.getMinutes() / 60;
    const endH   = e.getHours() + e.getMinutes() / 60;
    const top    = Math.max(0, (startH - START_HOUR) * HOUR_PX);
    const height = Math.max(20, (endH - startH) * HOUR_PX);
    return { top: `${top}px`, height: `${height}px` };
  }

  function formatHour(h: number): string {
    if (h === 0 || h === 12) return h === 0 ? '12 AM' : '12 PM';
    return h < 12 ? `${h} AM` : `${h - 12} PM`;
  }

  function snapToHalfHour(clientY: number): { hour: number; min: number } {
    if (!containerEl) return { hour: START_HOUR, min: 0 };
    const rect  = containerEl.getBoundingClientRect();
    const y     = Math.max(0, clientY - rect.top);
    const frac  = y / HOUR_PX;
    const hour  = Math.floor(frac) + START_HOUR;
    const min   = Math.round((frac % 1) * 2) * 30; // snap to :00 or :30
    return { hour: Math.min(hour, END_HOUR - 1), min: Math.min(min, 30) };
  }

  function isoAt(h: number, m: number): string {
    return `${date}T${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:00`;
  }

  function handleDragover(e: DragEvent) {
    const hasTask = e.dataTransfer?.types.includes('application/x-sempa-task');
    if (!hasTask) return;
    e.preventDefault();
    dragOver = true;
    const { hour } = snapToHalfHour(e.clientY);
    ghostHour = hour;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    const taskId = e.dataTransfer?.getData('application/x-sempa-task');
    if (!taskId) return;
    const { hour, min } = snapToHalfHour(e.clientY);
    const start = isoAt(hour, min);
    const end   = isoAt(hour, min + 30 <= 60 ? min + 30 : 30);
    onSchedule?.(taskId, start, end);
    ghostHour = null;
  }

  // Scheduled task blocks take the on-brand terracotta; calendar-sourced ones use
  // sage so they read as "from a calendar" without falling back to cold blue.
  function taskCal(task: Task): { fg: string; bg: string } {
    const key = task.source === 'google_calendar' ? 'sage' : 'terracotta';
    return { fg: calFg(key), bg: calBg(key) };
  }

  function blockLabel(task: Task): string {
    if (!task.scheduled_start) return task.title;
    const s = new Date(task.scheduled_start);
    const hh = String(s.getHours()).padStart(2,'0');
    const mm = String(s.getMinutes()).padStart(2,'0');
    return `${hh}:${mm} · ${task.title}`;
  }
</script>

<div class="flex h-full flex-col overflow-hidden">
  <div class="shrink-0 px-4 py-2 border-b border-gray-100 dark:border-gray-800/60">
    <div class="flex items-center justify-between gap-2">
      <p class="text-[10.5px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-600"
         title="Drag tasks here to place them on the timeline">
        Schedule
      </p>
      <div class="flex shrink-0 items-center gap-1">
        {#if eventCalendars.length > 0}
          <button onclick={() => showFilter = !showFilter}
                  class="flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10.5px] font-medium transition-colors"
                  style="color: var(--sempa-text-dim); {showFilter ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);' : ''}"
                  title="Show or hide calendars">
            <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="3" y="4" width="18" height="18" rx="2"/><path stroke-linecap="round" d="M16 2v4M8 2v4M3 10h18"/>
            </svg>
            Calendars
          </button>
        {/if}
        <a href="/settings/calendars"
           class="flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10.5px] font-medium transition-colors"
           style="color: var(--sempa-text-dim);"
           title="Calendar settings">
          <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="3"/><path stroke-linecap="round" d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          Settings
        </a>
      </div>
    </div>

    {#if showFilter && eventCalendars.length > 0}
      <div class="mt-2 flex flex-col gap-1.5">
        {#each eventCalendars as cal (cal.key)}
          {@const isHidden = calendars.isHidden(cal.key)}
          {@const fg = calFg(calendars.colorKey(cal.key))}
          <button onclick={() => calendars.toggleHidden(cal.key)}
                  class="flex items-center gap-2 text-left transition-opacity"
                  style="opacity: {isHidden ? 0.4 : 1};"
                  title={isHidden ? 'Show this calendar' : 'Hide this calendar'}>
            <span class="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-[3px]"
                  style="background: {isHidden ? 'transparent' : fg}; border: 1.5px solid {fg};">
              {#if !isHidden}
                <svg class="h-2 w-2 text-white" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
              {/if}
            </span>
            <span class="truncate text-[11px] {isHidden ? 'line-through' : ''}" style="color: var(--sempa-text-soft);">{cal.name}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto"
       ondragover={handleDragover}
       ondragleave={() => { dragOver = false; ghostHour = null; }}
       ondrop={handleDrop}>

    <div bind:this={containerEl}
         class="relative ml-10 mr-2"
         style="height: {TOTAL * HOUR_PX}px;">

      <!-- Hour grid lines + labels -->
      {#each hours as h}
        <div class="absolute left-0 right-0 border-t border-gray-100 dark:border-gray-800/50"
             style="top: {(h - START_HOUR) * HOUR_PX}px;">
          <span class="absolute -left-10 -top-2 w-9 text-right text-[10.5px] text-gray-400 dark:text-gray-600 leading-none select-none">
            {formatHour(h)}
          </span>
        </div>
      {/each}

      <!-- Ghost drop line -->
      {#if dragOver && ghostHour !== null}
        <div class="absolute left-0 right-0 border-t-2 border-dashed z-10 pointer-events-none"
             style="top: {(ghostHour - START_HOUR) * HOUR_PX}px; border-color: var(--sempa-accent);">
        </div>
      {/if}

      <!-- Current time indicator — brand accent, not red (7px dot + 1.5px rule) -->
      {#if nowPx !== null}
        <div class="absolute left-0 right-0 z-20 pointer-events-none flex items-center"
             style="top: {nowPx}px;">
          <div class="shrink-0 rounded-full" style="width:7px; height:7px; margin-left:-3.5px; background: var(--sempa-accent);"></div>
          <div class="flex-1" style="height:1.5px; background: var(--sempa-accent);"></div>
        </div>
      {/if}

      <!-- ICS / external calendar events (read-only). On-brand left-border accent
           + tinted background; concurrent events lay out side-by-side, hidden
           calendars are filtered, past events dim when that pref is on. -->
      {#each visibleEvents as ev (ev.id)}
        {@const s = new Date(ev.start_time)}
        {@const e = new Date(ev.end_time)}
        {@const startH = s.getHours() + s.getMinutes() / 60}
        {@const endH   = e.getHours() + e.getMinutes() / 60}
        {@const top    = Math.max(0, (startH - START_HOUR) * HOUR_PX)}
        {@const height = Math.max(20, (endH - startH) * HOUR_PX)}
        {@const key    = calendars.colorKey(ev.subscription_id)}
        <div class="cal-event absolute pointer-events-none overflow-hidden"
             style="top:{top}px; height:{height}px; {colStyle('e:' + ev.id)} --cal-fg:{calFg(key)}; --cal-bg:{calBg(key)}; opacity:{isPast(ev.end_time) ? 0.5 : 1};"
             title={ev.calendar ? ev.summary + ' · ' + ev.calendar : ev.summary}>
          <p class="title leading-tight truncate">{ev.summary}</p>
        </div>
      {/each}

      <!-- Scheduled task blocks — on-brand terracotta / sage -->
      {#each scheduled as task (task.id)}
        {@const style = blockStyle(task)}
        {@const cal   = taskCal(task)}
        {#if style}
          <button
            class="cal-event absolute text-left overflow-hidden cursor-pointer hover:brightness-95 transition-all"
            style="top: {style.top}; height: {style.height}; {colStyle('t:' + task.id)} --cal-fg:{cal.fg}; --cal-bg:{cal.bg}; opacity:{isPast(task.scheduled_end ?? task.scheduled_start!) ? 0.55 : 1};"
            onclick={() => onUnschedule?.(task.id)}
            title="Click to unschedule">
            <p class="title leading-tight truncate">{blockLabel(task)}</p>
            {#if task.time_estimate_minutes}
              <p class="time">{formatMinutes(task.time_estimate_minutes)}</p>
            {/if}
          </button>
        {/if}
      {/each}
    </div>
  </div>

  {#if scheduled.length === 0}
    <div class="shrink-0 px-4 pb-3 pt-1">
      <p class="text-[10.5px] text-gray-300 dark:text-gray-700">
        No tasks scheduled · drag from kanban ↗
      </p>
    </div>
  {/if}
</div>
