<script lang="ts">
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { today as getToday, offsetDate, weekStart, formatMinutes } from '$lib/utils';
  import { hapticTick } from '$lib/haptics';
  import TimeslotCalendar from '$lib/components/TimeslotCalendar.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import { ChevronLeft, ChevronRight, CalendarClock, Clock } from 'lucide-svelte';

  const todayDate = getToday();

  let date    = $state(todayDate);
  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state('');

  // Task editor (TaskPanel manages its own mobile bottom sheet)
  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  // "Pick a task" sheet, opened by tapping an empty slot
  let slotSheet  = $state<{ start: string; end: string } | null>(null);

  $effect(() => { date; loadTasks(); });

  async function loadTasks() {
    loading = true; error = '';
    try {
      tasks = await api.tasks.listByDate(date);
    } catch (e: any) {
      error = e.message ?? 'Failed to load tasks';
    } finally { loading = false; }
  }

  function navigateDay(delta: number) {
    hapticTick();
    date = offsetDate(date, delta);
  }

  // ── Date strip (centred, swipeable) ─────────────────────────────────────────
  const STRIP_RANGE = 21;
  const stripDays = $derived(
    Array.from({ length: STRIP_RANGE * 2 + 1 }, (_, i) => {
      const d = offsetDate(date, i - STRIP_RANGE);
      const dt = new Date(d + 'T12:00:00');
      return {
        date: d,
        dayName: dt.toLocaleDateString('en-US', { weekday: 'short' }),
        dayNum: dt.toLocaleDateString('en-US', { day: 'numeric' }),
        isToday: d === todayDate,
        isWeekend: dt.getDay() === 0 || dt.getDay() === 6,
      };
    })
  );

  const headerLabel = $derived.by(() => {
    const dt = new Date(date + 'T12:00:00');
    if (date === todayDate) return 'Today';
    return dt.toLocaleDateString('en-US', { weekday: 'long', day: 'numeric', month: 'short' });
  });

  // Keep the selected day centred whenever the date changes.
  let stripEl = $state<HTMLElement | undefined>();
  $effect(() => {
    date;
    if (!stripEl) return;
    queueMicrotask(() => {
      const el = stripEl?.querySelector<HTMLElement>(`[data-strip-date="${date}"]`);
      if (el && stripEl) stripEl.scrollLeft = el.offsetLeft - stripEl.clientWidth / 2 + el.clientWidth / 2;
    });
  });

  // Unscheduled, plannable tasks for this day → candidates for an empty slot.
  const unscheduledForDay = $derived(
    tasks.filter(t => !t.scheduled_start && t.status !== 'done' && t.status !== 'cancelled' && !t.parent_task_id)
  );

  // ── Schedule / unschedule (same mutations as the desktop panel) ─────────────
  async function handleSchedule(taskId: string, start: string, end: string) {
    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === taskId ? { ...t, scheduled_start: start, scheduled_end: end } : t);
    try {
      const updated = await api.tasks.update(taskId, { scheduled_start: start, scheduled_end: end });
      tasks = tasks.map(t => t.id === updated.id ? updated : t);
    } catch { tasks = prev; }
  }

  async function handleUnschedule(taskId: string) {
    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === taskId ? { ...t, scheduled_start: null, scheduled_end: null } : t);
    try {
      await api.tasks.update(taskId, { scheduled_start: null, scheduled_end: null });
    } catch { tasks = prev; }
  }

  function openTask(id: string) {
    const t = tasks.find(t => t.id === id);
    if (t) { panelTask = t; panelOpen = true; }
  }

  function onPanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    const idx = tasks.findIndex(t => t.id === saved.id);
    tasks = idx >= 0 ? tasks.map(t => t.id === saved.id ? saved : t) : [...tasks, saved];
  }

  // ── Tap empty slot → pick a task to schedule there ──────────────────────────
  function onSlotTap(start: string, end: string) {
    hapticTick();
    slotSheet = { start, end };
  }

  async function pickTaskForSlot(taskId: string) {
    if (!slotSheet) return;
    const { start, end } = slotSheet;
    slotSheet = null;
    // Ensure the task is anchored to this day, then schedule it at the tapped time.
    const t = tasks.find(t => t.id === taskId);
    if (t && (t.planned_date !== date || t.status === 'backlog')) {
      await api.tasks.update(taskId, {
        planned_date: date, week_start: weekStart(date),
        status: t.status === 'backlog' ? 'planned' : t.status,
      }).catch(() => {});
    }
    await handleSchedule(taskId, start, end);
  }

  function slotTimeLabel(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  const scheduledCount = $derived(tasks.filter(t => t.scheduled_start?.startsWith(date)).length);
</script>

<div class="flex h-full flex-col" style="background: var(--sempa-bg-main);">

  <!-- Header -->
  <header class="sticky top-0 z-[40] px-4 pt-4 pb-2"
          style="background: var(--sempa-bg-main); padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px)); border-bottom: 1px solid var(--sempa-border);">
    <div class="flex items-center justify-between">
      <button onclick={() => navigateDay(-1)} aria-label="Previous day"
              class="flex h-10 w-10 items-center justify-center rounded-xl transition-colors active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronLeft size={20} />
      </button>
      <div class="text-center">
        <h1 class="flex items-center gap-2" style="font-size: 22px; font-weight: 600; letter-spacing: -0.02em; color: var(--sempa-text);">
          <CalendarClock size={20} /> {headerLabel}
        </h1>
        {#if scheduledCount > 0}
          <p class="text-[11px]" style="color: var(--sempa-text-dim);">{scheduledCount} scheduled</p>
        {/if}
      </div>
      <button onclick={() => navigateDay(1)} aria-label="Next day"
              class="flex h-10 w-10 items-center justify-center rounded-xl transition-colors active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronRight size={20} />
      </button>
    </div>

    <!-- Date strip -->
    <div bind:this={stripEl}
         class="no-scrollbar -mx-4 mt-2 flex snap-x snap-proximity gap-1 overflow-x-auto scroll-px-4 px-4"
         style="-webkit-overflow-scrolling: touch; overscroll-behavior-x: contain;">
      {#each stripDays as day (day.date)}
        {@const isSel = day.date === date}
        <button onclick={() => { hapticTick(); date = day.date; }}
                data-strip-date={day.date}
                aria-current={isSel ? 'date' : undefined}
                class="flex shrink-0 snap-center flex-col items-center gap-0.5 rounded-xl px-2.5 py-1.5 transition-colors"
                style={isSel
                  ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                  : day.isToday
                    ? 'color: var(--sempa-accent);'
                    : day.isWeekend
                      ? 'color: var(--sempa-text-dim); opacity: 0.7;'
                      : 'color: var(--sempa-text-dim);'}>
          <span class="text-[10.5px] font-semibold uppercase">{day.dayName}</span>
          <span class="flex h-7 w-7 items-center justify-center rounded-full text-[13px] font-semibold"
                style={day.isToday && !isSel ? 'background: var(--sempa-today-bg); color: var(--sempa-today-fg);' : ''}>
            {day.dayNum}
          </span>
        </button>
      {/each}
    </div>
  </header>

  <!-- Timeline -->
  <div class="flex-1 overflow-hidden">
    {#if error}
      <p class="px-4 py-6 text-sm text-red-500">{error}</p>
    {:else}
      <TimeslotCalendar
        {date}
        {tasks}
        onSchedule={handleSchedule}
        onUnschedule={handleUnschedule}
        onOpenTask={openTask}
        onSlotTap={onSlotTap}
        onEventConverted={(t) => { tasks = [...tasks, t]; }}
      />
    {/if}
  </div>
</div>

<!-- Pick-a-task sheet (tap empty slot) -->
<BottomSheet open={slotSheet !== null} onClose={() => slotSheet = null}>
  {#if slotSheet}
    <div class="px-5 pb-6 pt-1" data-sheet-scroll>
      <div class="flex items-center gap-2">
        <Clock size={16} style="color: var(--sempa-accent);" />
        <h2 class="text-base font-semibold" style="color: var(--sempa-text);">
          Schedule at {slotTimeLabel(slotSheet.start)}
        </h2>
      </div>

      {#if unscheduledForDay.length === 0}
        <p class="mt-4 text-sm" style="color: var(--sempa-text-dim);">
          No unscheduled tasks for this day. Plan a task first, then place it here.
        </p>
      {:else}
        <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">Pick a task to place on the timeline</p>
        <ul class="mt-3 space-y-1">
          {#each unscheduledForDay as t (t.id)}
            <li>
              <button onclick={() => pickTaskForSlot(t.id)}
                      class="flex w-full items-center gap-3 rounded-xl px-3 py-3 text-left transition-colors active:bg-gray-50 dark:active:bg-gray-800/40"
                      style="border: 1px solid var(--sempa-border);">
                <span class="min-w-0 flex-1 truncate text-sm" style="color: var(--sempa-text);">{t.title}</span>
                {#if t.time_estimate_minutes}
                  <span class="shrink-0 text-xs" style="color: var(--sempa-text-dim);">{formatMinutes(t.time_estimate_minutes)}</span>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</BottomSheet>

<TaskPanel
  open={panelOpen}
  task={panelTask}
  defaultDate={date}
  onSave={onPanelSave}
  onClose={() => panelOpen = false}
/>
