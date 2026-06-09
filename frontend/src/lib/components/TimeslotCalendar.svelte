<script lang="ts">
  import { api } from '$lib/api';
  import type { ICalEvent, Task } from '$lib/types';
  import { formatMinutes, today as getToday } from '$lib/utils';

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

  function taskColor(task: Task): string {
    if (task.source === 'google_calendar') return 'bg-purple-100 border-purple-300 text-purple-700 dark:bg-purple-950/60 dark:border-purple-700 dark:text-purple-300';
    return 'bg-blue-100 border-blue-300 text-blue-700 dark:bg-blue-950/60 dark:border-blue-700 dark:text-blue-300';
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
    <p class="text-[10.5px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-600">
      Schedule — drag tasks to place them
    </p>
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
        <div class="absolute left-0 right-0 border-t-2 border-dashed border-blue-400 z-10 pointer-events-none"
             style="top: {(ghostHour - START_HOUR) * HOUR_PX}px;">
        </div>
      {/if}

      <!-- Current time indicator -->
      {#if nowPx !== null}
        <div class="absolute left-0 right-0 z-20 pointer-events-none flex items-center"
             style="top: {nowPx}px;">
          <div class="h-2.5 w-2.5 shrink-0 rounded-full bg-red-500" style="margin-left: -5px;"></div>
          <div class="h-px flex-1 bg-red-500/70"></div>
        </div>
      {/if}

      <!-- ICS / external calendar events (read-only) -->
      {#each icalEvents as ev (ev.id)}
        {@const s = new Date(ev.start_time)}
        {@const e = new Date(ev.end_time)}
        {@const startH = s.getHours() + s.getMinutes() / 60}
        {@const endH   = e.getHours() + e.getMinutes() / 60}
        {@const top    = Math.max(0, (startH - START_HOUR) * HOUR_PX)}
        {@const height = Math.max(20, (endH - startH) * HOUR_PX)}
        {#if !ev.all_day}
          <div class="absolute left-0.5 right-0.5 rounded-lg border px-2 py-1 pointer-events-none opacity-80"
               style="top:{top}px; height:{height}px; background:{ev.color}22; border-color:{ev.color}55; color:{ev.color};">
            <p class="text-[10.5px] font-medium leading-tight truncate">{ev.summary}</p>
          </div>
        {/if}
      {/each}

      <!-- Scheduled task blocks -->
      {#each scheduled as task (task.id)}
        {@const style = blockStyle(task)}
        {#if style}
          <button
            class="absolute left-0.5 right-0.5 rounded-lg border px-2 py-1 text-left
                   overflow-hidden cursor-pointer hover:brightness-95 transition-all
                   {taskColor(task)}"
            style="top: {style.top}; height: {style.height};"
            onclick={() => onUnschedule?.(task.id)}
            title="Click to unschedule">
            <p class="text-[10.5px] font-medium leading-tight truncate">{blockLabel(task)}</p>
            {#if task.time_estimate_minutes}
              <p class="text-[10.5px] opacity-70">{formatMinutes(task.time_estimate_minutes)}</p>
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
