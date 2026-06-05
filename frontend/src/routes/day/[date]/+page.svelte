<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task, TaskStatus } from '$lib/types';
  import { appendPosition, formatMinutes, insertPosition, isToday, offsetDate, today, weekStart } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import WeekDayColumn from '$lib/components/WeekDayColumn.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import EmailPanel from '$lib/components/EmailPanel.svelte';
  import MiniCalendar from '$lib/components/MiniCalendar.svelte';
  import TimeslotCalendar from '$lib/components/TimeslotCalendar.svelte';
  import WeeklyObjectivesWidget from '$lib/components/WeeklyObjectivesWidget.svelte';
  import { ChevronLeft, ChevronRight, Plus, Clock, Mail } from 'lucide-svelte';
  import JiraPanel from '$lib/components/JiraPanel.svelte';
  import MobileTaskCard from '$lib/components/MobileTaskCard.svelte';
  import MobileTaskView from '$lib/components/MobileTaskView.svelte';
  import { syncWidgetData } from '$lib/widget-bridge';

  // "date" is used to anchor the week and mark today
  let date      = $derived($page.params.date ?? today());
  let ws        = $derived(weekStart(date));
  let todayDate = $derived(today());

  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  // Rollover
  let rolloverTasks    = $state<Task[]>([]);
  let rolloverDismissed = $state(false);

  let kanbanScroll  = $state<HTMLElement | undefined>();
  let draggingId    = $state<string | null>(null);
  let dragOverDate  = $state<string | null>(null);
  let emailPanel    = $state<EmailPanel | undefined>(undefined);
  let rightPanel    = $state<'schedule' | 'mail' | 'jira'>('schedule');

  let panelOpen   = $state(false);
  let panelTask   = $state<Task | null>(null);
  let panelStatus = $state<TaskStatus>('planned');
  let panelDate   = $state(date);

  // Mobile task detail view (read-first, tap Edit to open full panel)
  let mobileViewOpen = $state(false);
  let mobileViewTaskId = $state<string | null>(null);
  // Derived so complete/edit actions update the view in real-time
  const mobileViewTask = $derived(mobileViewTaskId ? (tasks.find(t => t.id === mobileViewTaskId) ?? null) : null);

  function openMobileView(task: Task) {
    mobileViewTaskId = task.id;
    mobileViewOpen = true;
  }

  // Week days: Mon–Sun
  const weekDays = $derived(
    Array.from({ length: 7 }, (_, i) => {
      const d = offsetDate(ws, i);
      const dt = new Date(d + 'T12:00:00');
      return {
        date: d,
        dayName: dt.toLocaleDateString('en-US', { weekday: 'short' }),
        dayNum: dt.toLocaleDateString('en-US', { day: 'numeric' }),
        monthName: dt.toLocaleDateString('en-US', { month: 'short' }),
        fullDayName: dt.toLocaleDateString('en-US', { weekday: 'long' }),
        isToday: d === todayDate,
        isWeekend: dt.getDay() === 0 || dt.getDay() === 6,
      };
    })
  );

  // Mobile: current selected day info
  const selectedDay = $derived(weekDays.find(d => d.date === date) ?? weekDays[0]);

  // ── Pomodoro update ───────────────────────────────────────────────────────
  $effect(() => {
    const upd = pomodoro.lastTimeUpdate;
    if (upd) tasks = tasks.map(t => t.id === upd.taskId ? { ...t, time_actual_minutes: upd.newActual } : t);
  });

  // ── Load ──────────────────────────────────────────────────────────────────
  async function loadTasks() {
    loading = true; error = null;
    try { tasks = await api.tasks.listByWeek(ws); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed'; }
    finally { loading = false; }
  }

  async function loadRollover() {
    try {
      const prev = await api.tasks.listByDate(offsetDate(todayDate, -1));
      rolloverTasks = prev.filter(t => (t.status === 'planned' || t.status === 'in_progress') && !t.recurrence_origin_id);
    } catch { /* ignore */ }
  }

  onMount(() => { loadTasks(); loadRollover(); });
  $effect(() => { ws; loadTasks(); });

  // Handle FAB deep link — runs on mount AND whenever search params change
  // (same-page goto('/day/today?new=1') doesn't re-trigger onMount)
  $effect(() => {
    const newParam = $page.url.searchParams.get('new');
    if (!newParam) return;
    openCreate(date);
    history.replaceState({}, '', $page.url.pathname);
  });

  async function rolloverAll() {
    await Promise.all(rolloverTasks.map(t =>
      api.tasks.update(t.id, { planned_date: todayDate, week_start: weekStart(todayDate), status: 'planned' })
    ));
    rolloverTasks = []; await loadTasks();
  }

  // ── Tasks per day ──────────────────────────────────────────────────────────
  function dayTasks(d: string): Task[] {
    return tasks
      .filter(t => t.planned_date === d && t.status !== 'cancelled')
      .sort((a, b) => a.position - b.position);
  }

  // Day stats for header
  const totalTasks   = $derived(tasks.filter(t => t.status !== 'cancelled'));
  const doneTasks    = $derived(totalTasks.filter(t => t.status === 'done').length);
  const estimateMins = $derived(totalTasks.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0));
  const actualMins   = $derived(totalTasks.reduce((s, t) => s + (t.time_actual_minutes ?? 0), 0));

  // Sync task data to Android widgets whenever tasks change
  $effect(() => {
    if (tasks.length === 0 && loading) return;
    const todayList = tasks.filter(t => t.planned_date === todayDate && t.status !== 'cancelled');
    const weekCounts = new Map<string, number>();
    for (const t of tasks) {
      if (t.status === 'cancelled') continue;
      if (t.planned_date) weekCounts.set(t.planned_date, (weekCounts.get(t.planned_date) ?? 0) + 1);
    }
    syncWidgetData(todayList, weekCounts);
  });

  // Mobile: stats for selected day
  const mobileDayTasks  = $derived(dayTasks(date));
  const mobileActive    = $derived(mobileDayTasks.filter(t => t.status !== 'done'));
  const mobileDone      = $derived(mobileDayTasks.filter(t => t.status === 'done'));
  const mobileDayEstimate = $derived(mobileDayTasks.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0));

  // ── Week navigation ────────────────────────────────────────────────────────
  function navigateWeek(delta: number) {
    const newWs = offsetDate(ws, delta * 7);
    goto(`/day/${newWs}`);
  }
  function goToday() { goto(`/day/${todayDate}`); }

  function handleCalendarDateClick(d: string) {
    goto(`/day/${d}`);
  }

  // ── Mobile day navigation ────────────────────────────────────────────────
  function navigateDay(delta: number) {
    goto(`/day/${offsetDate(date, delta)}`);
  }

  $effect(() => {
    if (mobile.value) return;
    const d = date;
    requestAnimationFrame(() => {
      if (!kanbanScroll) return;
      const el = document.getElementById(`day-col-${d}`) as HTMLElement | null;
      if (!el) return;
      const targetLeft = el.offsetLeft - (kanbanScroll.clientWidth / 2 - el.offsetWidth / 2);
      kanbanScroll.scrollTo({ left: Math.max(0, targetLeft), behavior: 'smooth' });
    });
  });

  // ── Drag & drop between days ───────────────────────────────────────────────
  function handleDragStart(id: string) { draggingId = id; }

  async function handleDrop(targetDate: string, insertIdx?: number) {
    if (!draggingId) return;
    const id = draggingId;
    draggingId = null; dragOverDate = null;
    const task = tasks.find(t => t.id === id);
    if (!task) return;

    const colTasks = tasks
      .filter(t => t.planned_date === targetDate && t.status !== 'cancelled' && t.id !== id)
      .sort((a, b) => a.position - b.position);
    const positions = colTasks.map(t => t.position);
    const newPos = insertIdx !== undefined ? insertPosition(positions, insertIdx) : appendPosition(positions);

    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === id ? { ...t, planned_date: targetDate, position: newPos } : t);
    try {
      const updated = await api.tasks.update(id, {
        planned_date: targetDate,
        week_start: ws,
        position: newPos,
        status: task.status === 'backlog' ? 'planned' : task.status,
      });
      tasks = tasks.map(t => t.id === updated.id ? updated : t);
    } catch { tasks = prev; }
  }

  // ── Complete ──────────────────────────────────────────────────────────────
  async function handleComplete(id: string) {
    const task = tasks.find(t => t.id === id);
    if (!task) return;
    const newStatus = task.status === 'done' ? 'planned' : 'done';
    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === id ? { ...t, status: newStatus } : t);
    try {
      const updated = await api.tasks.update(id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      tasks = tasks.map(t => t.id === updated.id ? updated : t);
    } catch { tasks = prev; }
  }

  // ── Focus (Pomodoro) ──────────────────────────────────────────────────────
  function handleFocus(id: string, title: string) {
    const t = tasks.find(t => t.id === id);
    pomodoro.start(id, title, t?.time_actual_minutes ?? 0);
  }

  // ── Focus mode (full-screen) ───────────────────────────────────────────────
  function handleFocusMode(id: string) {
    goto(`/focus/${id}`);
  }

  // ── Hover tracking for keyboard shortcut ─────────────────────────────────
  let hoveredTaskId = $state<string | null>(null);
  function handleTaskHover(id: string | null) { hoveredTaskId = id; }

  // ── Keyboard shortcut: n = new task, e = edit hovered ────────────────────
  function handleKeydown(e: KeyboardEvent) {
    const tgt = e.target as HTMLElement;
    if (tgt.tagName === 'INPUT' || tgt.tagName === 'TEXTAREA' || tgt.isContentEditable) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === 'n' && !panelOpen) { e.preventDefault(); openCreate(todayDate); }
    if (e.key === 'e' && !panelOpen && hoveredTaskId) {
      e.preventDefault();
      const t = tasks.find(t => t.id === hoveredTaskId);
      if (t) openEdit(t);
    }
  }

  // ── Trash (with confirm modal) ──────────────────────────────────────────
  let trashConfirmOpen  = $state(false);
  let trashTaskId       = $state<string | null>(null);
  let trashTaskTitle    = $state('');

  function handleTrashRequest(id: string, title: string) {
    trashTaskId = id;
    trashTaskTitle = title;
    trashConfirmOpen = true;
  }

  async function confirmTrash() {
    if (!trashTaskId) return;
    const id = trashTaskId;
    const prev = tasks.slice();
    tasks = tasks.filter(t => t.id !== id);
    trashConfirmOpen = false;
    trashTaskId = null;
    try { await api.tasks.delete(id); }
    catch { tasks = prev; }
  }

  function cancelTrash() {
    trashConfirmOpen = false;
    trashTaskId = null;
  }

  // ── Panel ─────────────────────────────────────────────────────────────────
  function openCreate(d: string) {
    panelTask = null; panelStatus = 'planned'; panelDate = d; panelOpen = true;
  }
  function openEdit(task: Task) { panelTask = task; panelOpen = true; }

  async function handlePanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    if (!panelTask && saved.recurrence_rule) { await loadTasks(); return; }
    const idx = tasks.findIndex(t => t.id === saved.id);
    if (idx >= 0) tasks = tasks.map(t => t.id === saved.id ? saved : t);
    else tasks = [...tasks, saved];
  }

  // ── Email drop ────────────────────────────────────────────────────────────
  async function handleEmailDrop(emailData: { id: string; subject: string }, targetDate: string) {
    try {
      const task = await api.integrations.fastmail.toTask(emailData.id, emailData.subject);
      const updated = await api.tasks.update(task.id, {
        planned_date: targetDate, week_start: ws, status: 'planned',
      });
      tasks = [...tasks, updated];
      emailPanel?.removeEmail(emailData.id);
    } catch (e: any) { error = e.message; }
  }

  // ── Calendar schedule / unschedule ────────────────────────────────────────
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

  // Heading
  function weekLabel(): string {
    const start = new Date(ws + 'T00:00:00');
    const end   = offsetDate(ws, 6);
    const endDt = new Date(end + 'T00:00:00');
    const mo = (d: Date) => d.toLocaleDateString('en-US', { month: 'short' });
    const dy = (d: Date) => d.getDate();
    if (start.getMonth() === endDt.getMonth()) return `${mo(start)} ${dy(start)}–${dy(endDt)}`;
    return `${mo(start)} ${dy(start)} – ${mo(endDt)} ${dy(endDt)}`;
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
  <title>{isToday(date) ? 'Today' : weekLabel()} — Sempa</title>
</svelte:head>

<!-- ═══════════════════════════════════════════════════════════════════════ -->
<!-- MOBILE LAYOUT                                                          -->
<!-- ═══════════════════════════════════════════════════════════════════════ -->
{#if mobile.value}

  <!-- Mobile header -->
  <header class="sticky top-0 z-10 px-5 pt-4 pb-3"
          style="background: var(--sempa-bg-main); padding-top: 16px;">
    <div class="flex items-center justify-between mb-1">
      <button onclick={() => navigateDay(-1)} aria-label="Previous day"
              class="flex h-10 w-10 items-center justify-center rounded-xl transition-colors
                     active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronLeft size={20} />
      </button>
      <div class="text-center">
        <h1 style="font-size: 28px; font-weight: 600; letter-spacing: -0.025em; color: var(--sempa-text);">
          {selectedDay.isToday ? 'Today' : selectedDay.fullDayName + ', ' + selectedDay.dayNum}
        </h1>
        {#if !selectedDay.isToday}
          <p class="text-xs" style="color: var(--sempa-text-dim);">{selectedDay.monthName}</p>
        {/if}
      </div>
      <button onclick={() => navigateDay(1)} aria-label="Next day"
              class="flex h-10 w-10 items-center justify-center rounded-xl transition-colors
                     active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronRight size={20} />
      </button>
    </div>

    <!-- Quick date strip -->
    <div class="flex justify-between mt-2">
      {#each weekDays as day (day.date)}
        <button onclick={() => goto(`/day/${day.date}`)}
                class="flex flex-col items-center gap-0.5 rounded-xl px-2 py-1.5 transition-colors"
                style={day.date === date
                  ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                  : day.isToday
                    ? 'color: var(--sempa-accent);'
                    : 'color: var(--sempa-text-dim);'}>
          <span class="text-[10px] font-semibold uppercase">{day.dayName}</span>
          <span class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
                style={day.isToday && day.date !== date
                  ? 'background: var(--sempa-today-bg); color: var(--sempa-today-fg);'
                  : ''}>
            {day.dayNum}
          </span>
        </button>
      {/each}
    </div>

    <!-- Day stats -->
    {#if mobileDayTasks.length > 0}
      <div class="flex items-center gap-3 mt-2 text-[11px]" style="color: var(--sempa-text-dim);">
        <span>{mobileDone.length}/{mobileDayTasks.length} done</span>
        {#if mobileDayEstimate > 0}<span>{formatMinutes(mobileDayEstimate)} planned</span>{/if}
      </div>
    {/if}
  </header>

  <!-- Rollover banner (mobile) -->
  {#if rolloverTasks.length > 0 && !rolloverDismissed && isToday(date)}
    <div class="mx-4 mb-3 flex items-center gap-2 rounded-xl px-3 py-2.5 animate-slide-down"
         style="border: 1px solid var(--sempa-amber); background: color-mix(in srgb, var(--sempa-amber) 8%, var(--sempa-bg-main));">
      <p class="flex-1 text-xs" style="color: var(--sempa-amber);">
        <strong>{rolloverTasks.length}</strong> from yesterday
      </p>
      <button onclick={rolloverAll}
              class="rounded-lg px-2.5 py-1 text-[11px] font-medium"
              style="background: var(--sempa-amber); color: var(--sempa-btn-fg);">
        Roll over
      </button>
      <button onclick={() => rolloverDismissed = true} aria-label="Dismiss"
              style="color: var(--sempa-amber); opacity: 0.7;">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  {/if}

  <!-- Mobile task list -->
  <main class="px-4 pb-24 animate-fade-in">
    {#if loading}
      <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading...</div>
    {:else if error}
      <div class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-600
                  dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
        {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
      </div>
    {:else if mobileDayTasks.length === 0}
      <div class="flex flex-col items-center justify-center py-16 gap-3">
        <div class="h-12 w-12 rounded-full flex items-center justify-center"
             style="background: var(--sempa-accent-bg);">
          <Plus size={20} style="color: var(--sempa-accent);" />
        </div>
        <p class="text-sm" style="color: var(--sempa-text-dim);">No tasks for this day</p>
        <button onclick={() => openCreate(date)}
                class="rounded-[9px] px-4 py-2 text-[13px] font-medium"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
          Add task
        </button>
      </div>
    {:else}
      <!-- Active tasks -->
      <div class="flex flex-col gap-2">
        {#each mobileActive as task (task.id)}
          <MobileTaskCard
            {task}
            onComplete={handleComplete}
            onTrash={handleTrashRequest}
            onClick={openMobileView}
            onFocusClick={handleFocus}
          />
        {/each}
      </div>

      <!-- Completed tasks -->
      {#if mobileDone.length > 0}
        <div class="mt-4 pt-3" style="border-top: 1px solid var(--sempa-border);">
          <p class="mb-2 text-[11px] font-medium uppercase tracking-wider" style="color: var(--sempa-text-dim);">
            {mobileDone.length} completed
          </p>
          <div class="flex flex-col gap-1.5">
            {#each mobileDone as task (task.id)}
              <MobileTaskCard
                {task}
                onComplete={handleComplete}
                onTrash={handleTrashRequest}
                onClick={openMobileView}
              />
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  </main>

  <!-- Mobile task detail view: read-first, Edit button opens full TaskPanel -->
  <MobileTaskView
    open={mobileViewOpen}
    task={mobileViewTask}
    onClose={() => mobileViewOpen = false}
    onEdit={() => { const t = mobileViewTask; mobileViewOpen = false; if (t) openEdit(t); }}
    onComplete={handleComplete}
    onDelete={handleTrashRequest}
    onFocusStart={handleFocus}
  />

  <!-- TaskPanel handles its own mobile bottom sheet -->
  <TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={panelDate}
             onSave={handlePanelSave} onClose={() => panelOpen = false} />

<!-- ═══════════════════════════════════════════════════════════════════════ -->
<!-- DESKTOP LAYOUT (unchanged)                                             -->
<!-- ═══════════════════════════════════════════════════════════════════════ -->
{:else}

<!-- ── Header ─────────────────────────────────────────────────────────────── -->
<header class="sticky top-0 z-10 backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               border-bottom: 1px solid var(--sempa-border);">
  <div class="flex items-center justify-between px-6 py-3">
    <!-- Week nav -->
    <div class="flex items-center gap-2">
      <button onclick={() => navigateWeek(-1)} aria-label="Previous week"
              class="rounded-lg p-1.5 transition-colors"
              style="color: var(--sempa-text-dim);">
        <ChevronLeft size={16} />
      </button>
      <div>
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">{weekLabel()}</p>
        {#if isToday(date)}
          <p class="text-[10px] font-medium uppercase tracking-wider" style="color:var(--a500)">This week</p>
        {/if}
      </div>
      <button onclick={() => navigateWeek(1)} aria-label="Next week"
              class="rounded-lg p-1.5 transition-colors"
              style="color: var(--sempa-text-dim);">
        <ChevronRight size={16} />
      </button>
    </div>

    <!-- Stats -->
    {#if !loading && totalTasks.length > 0}
      <div class="hidden md:flex items-center gap-4 text-xs" style="color: var(--sempa-text-dim);">
        <span>{doneTasks}/{totalTasks.length} done this week</span>
        {#if estimateMins > 0}<span>~{formatMinutes(estimateMins)} planned</span>{/if}
        {#if actualMins > 0}<span style="color: var(--sempa-accent);">{formatMinutes(actualMins)} logged</span>{/if}
      </div>
    {/if}

    <!-- Actions -->
    <div class="flex items-center gap-2">
      {#if !isToday(date)}
        <button onclick={goToday}
                class="font-medium"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: transparent; border-radius: 9px; padding: 6px 12px;
                       font-size: 12px; cursor: pointer; transition: all 150ms ease;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'transparent'}>
          Today
        </button>
      {/if}
      <button onclick={() => openCreate(todayDate)}
              class="flex items-center gap-1.5 rounded-[9px] px-3 py-1.5 text-[13px] font-[500]
                     tracking-[-0.01em] transition-colors shadow-sm"
              style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);"
              onmouseenter={(e)=>(e.currentTarget as HTMLElement).style.opacity='0.88'}
              onmouseleave={(e)=>(e.currentTarget as HTMLElement).style.opacity='1'}>
        <Plus size={13} strokeWidth={2.5} />
        New task
      </button>
    </div>
  </div>
</header>

<!-- ── Body ───────────────────────────────────────────────────────────────── -->
<div class="flex h-[calc(100vh-57px)] overflow-hidden">

  <!-- Kanban area -->
  <main bind:this={kanbanScroll} class="flex-1 overflow-auto px-4 py-5 animate-fade-in">

    <!-- Rollover banner -->
    {#if rolloverTasks.length > 0 && !rolloverDismissed}
      <div class="mb-4 flex items-center gap-3 rounded-xl px-4 py-3 animate-slide-down"
           style="border: 1px solid var(--sempa-amber); background: color-mix(in srgb, var(--sempa-amber) 8%, var(--sempa-bg-main));">
        <svg class="h-4 w-4 shrink-0" style="color: var(--sempa-amber);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="flex-1 text-xs" style="color: var(--sempa-amber);">
          <strong>{rolloverTasks.length}</strong> unfinished from yesterday —
          {rolloverTasks.slice(0,2).map(t=>t.title).join(', ')}{rolloverTasks.length > 2 ? '...' : ''}
        </p>
        <button onclick={rolloverAll}
                class="rounded-lg px-3 py-1 text-xs font-medium transition-colors"
                style="background: var(--sempa-amber); color: var(--sempa-btn-fg);">
          Roll over
        </button>
        <button onclick={() => rolloverDismissed = true} aria-label="Dismiss"
                style="color: var(--sempa-amber); opacity: 0.7;">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    {/if}

    {#if loading}
      <div class="flex h-64 items-center justify-center text-sm text-gray-300 dark:text-gray-700">Loading...</div>
    {:else if error}
      <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-600
                  dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
        {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
      </div>
    {:else}
      <div class="flex items-start gap-3 pb-6">
        <!-- Mon–Fri -->
        {#each weekDays.slice(0, 5) as day (day.date)}
          <div id="day-col-{day.date}" class="w-56 shrink-0">
            <WeekDayColumn
              date={day.date} dayName={day.dayName} dayNum={day.dayNum}
              isToday={day.isToday} isWeekend={false}
              tasks={dayTasks(day.date)}
              isDragOver={dragOverDate === day.date}
              onTaskDragStart={handleDragStart}
              onTaskFocusClick={handleFocus}
              onTaskFocusMode={handleFocusMode}
              onTaskComplete={handleComplete}
              onTaskTrash={handleTrashRequest}
              onTaskClick={openEdit}
              onTaskHover={handleTaskHover}
              onDrop={handleDrop}
              onEmailDrop={handleEmailDrop}
              onDragOver={(d) => (dragOverDate = d)}
              onDragLeave={() => (dragOverDate = null)}
              onAddClick={openCreate}
            />
          </div>
        {/each}

        <!-- Thin divider before weekend -->
        <div class="w-px self-stretch bg-gray-200 dark:bg-gray-700/50 mt-7 mb-2"></div>

        <!-- Sat–Sun (narrower, visually softer) -->
        {#each weekDays.slice(5) as day (day.date)}
          <div id="day-col-{day.date}" class="w-44 shrink-0">
            <WeekDayColumn
              date={day.date} dayName={day.dayName} dayNum={day.dayNum}
              isToday={day.isToday} isWeekend={true}
              tasks={dayTasks(day.date)}
              isDragOver={dragOverDate === day.date}
              onTaskDragStart={handleDragStart}
              onTaskFocusClick={handleFocus}
              onTaskFocusMode={handleFocusMode}
              onTaskComplete={handleComplete}
              onTaskTrash={handleTrashRequest}
              onTaskClick={openEdit}
              onTaskHover={handleTaskHover}
              onDrop={handleDrop}
              onEmailDrop={handleEmailDrop}
              onDragOver={(d) => (dragOverDate = d)}
              onDragLeave={() => (dragOverDate = null)}
              onAddClick={openCreate}
            />
          </div>
        {/each}
      </div>
    {/if}
  </main>

  <!-- ── Right panel ─────────────────────────────────────────────────────── -->
  <aside class="w-72 shrink-0 flex flex-col overflow-hidden"
         style="background: var(--sempa-bg-panel); border-left: 1px solid var(--sempa-border);">

    <!-- Always-visible: mini calendar + objectives -->
    <div class="shrink-0" style="border-bottom: 1px solid var(--sempa-border);">
      <MiniCalendar {date} onDateClick={handleCalendarDateClick} />
    </div>
    <WeeklyObjectivesWidget {date} />

    <!-- Switchable panel + icon strip -->
    <div class="flex flex-1 overflow-hidden" style="border-top: 1px solid var(--sempa-border);">

      <!-- Panel content -->
      <div class="flex-1 overflow-hidden">
        {#if rightPanel === 'schedule'}
          <TimeslotCalendar
            date={date}
            tasks={tasks}
            onSchedule={handleSchedule}
            onUnschedule={handleUnschedule}
          />
        {:else if rightPanel === 'mail'}
          <EmailPanel bind:this={emailPanel} onTaskCreated={(t) => { tasks = [...tasks, t]; }} />
        {:else if rightPanel === 'jira'}
          <JiraPanel
            onTaskDragStart={(id) => { draggingId = id; }}
            onTasksReloaded={loadTasks}
          />
        {/if}
      </div>

      <!-- Icon strip -->
      <div class="flex shrink-0 flex-col items-center gap-1 px-1 py-2"
           style="border-left: 1px solid var(--sempa-border); width: 40px;">
        {#each [
          { id: 'schedule', label: 'Schedule' },
          { id: 'mail',     label: 'Mail' },
          { id: 'jira',     label: 'Jira' },
        ] as panel}
          <button onclick={() => rightPanel = panel.id as typeof rightPanel}
                  title={panel.label}
                  class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors"
                  style={rightPanel === panel.id
                    ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                    : 'color: var(--sempa-text-dim);'}>
            {#if panel.id === 'schedule'}
              <Clock size={15} />
            {:else if panel.id === 'mail'}
              <Mail size={15} />
            {:else}
              <!-- Jira logo -->
              <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor">
                <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
              </svg>
            {/if}
          </button>
        {/each}
      </div>
    </div>
  </aside>
</div>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={panelDate}
           onSave={handlePanelSave} onClose={() => (panelOpen = false)} />
{/if}

<!-- ── Trash confirm modal ──────────────────────────────────────────────── -->
{#if trashConfirmOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/30 backdrop-blur-sm animate-fade-in"
       onclick={cancelTrash}>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="w-full max-w-sm mx-4 rounded-2xl p-6 shadow-2xl animate-scale-in"
         style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
         onclick={(e) => e.stopPropagation()}>
      <!-- Icon -->
      <div class="mx-auto mb-4 flex h-11 w-11 items-center justify-center rounded-full"
           style="background: color-mix(in srgb, #ef4444 12%, var(--sempa-bg-panel));">
        <svg class="h-5 w-5 text-red-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
        </svg>
      </div>
      <!-- Text -->
      <h3 class="mb-1 text-center text-sm font-semibold" style="color: var(--sempa-text);">Delete task?</h3>
      <p class="mb-5 text-center text-xs leading-relaxed" style="color: var(--sempa-text-soft);">
        <span class="font-medium" style="color: var(--sempa-text);">"{trashTaskTitle}"</span> will be permanently removed.
      </p>
      <!-- Actions -->
      <div class="flex items-center gap-2">
        <button onclick={cancelTrash}
                class="flex-1 rounded-[9px] px-3 py-2 text-[13px] font-medium transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft); background: transparent;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'transparent'}>
          Cancel
        </button>
        <button onclick={confirmTrash}
                class="flex-1 rounded-[9px] px-3 py-2 text-[13px] font-medium transition-colors shadow-sm"
                style="background: #ef4444; color: white;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          Delete
        </button>
      </div>
    </div>
  </div>
{/if}
