<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task, TaskStatus } from '$lib/types';
  import { appendPosition, formatMinutes, insertPosition, isToday, offsetDate, today, weekStart } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import WeekDayColumn from '$lib/components/WeekDayColumn.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import EmailPanel from '$lib/components/EmailPanel.svelte';
  import MiniCalendar from '$lib/components/MiniCalendar.svelte';
  import TimeslotCalendar from '$lib/components/TimeslotCalendar.svelte';
  import WeeklyObjectivesWidget from '$lib/components/WeeklyObjectivesWidget.svelte';
  import { ChevronLeft, ChevronRight, Plus } from 'lucide-svelte';

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

  let draggingId    = $state<string | null>(null);
  let dragOverDate  = $state<string | null>(null);
  let emailPanel    = $state<EmailPanel | undefined>(undefined);
  let rightTab      = $state<'mail' | 'schedule'>('mail');

  let panelOpen   = $state(false);
  let panelTask   = $state<Task | null>(null);
  let panelStatus = $state<TaskStatus>('planned');
  let panelDate   = $state(date);

  // Week days: Mon–Sun
  const weekDays = $derived(
    Array.from({ length: 7 }, (_, i) => {
      const d = offsetDate(ws, i);
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
      rolloverTasks = prev.filter(t => t.status === 'planned' || t.status === 'in_progress');
    } catch { /* ignore */ }
  }

  onMount(() => { loadTasks(); loadRollover(); });
  $effect(() => { ws; loadTasks(); });

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

  // ── Week navigation ────────────────────────────────────────────────────────
  function navigateWeek(delta: number) {
    const newWs = offsetDate(ws, delta * 7);
    goto(`/day/${newWs}`);
  }
  function goToday() { goto(`/day/${todayDate}`); }

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

<svelte:head>
  <title>{isToday(date) ? 'Today' : weekLabel()} — Sempa</title>
</svelte:head>

<!-- ── Header ─────────────────────────────────────────────────────────────── -->
<header class="sticky top-0 z-10 border-b border-gray-100 bg-white/95 backdrop-blur-sm
               dark:border-gray-800/60 dark:bg-gray-900/95">
  <div class="flex items-center justify-between px-6 py-3">
    <!-- Week nav -->
    <div class="flex items-center gap-2">
      <button onclick={() => navigateWeek(-1)} aria-label="Previous week"
              class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-400">
        <ChevronLeft size={16} />
      </button>
      <div>
        <p class="text-sm font-semibold text-gray-900 dark:text-gray-50">{weekLabel()}</p>
        {#if isToday(date)}
          <p class="text-[10px] font-medium uppercase tracking-wider" style="color:var(--a500)">This week</p>
        {/if}
      </div>
      <button onclick={() => navigateWeek(1)} aria-label="Next week"
              class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-400">
        <ChevronRight size={16} />
      </button>
    </div>

    <!-- Stats -->
    {#if !loading && totalTasks.length > 0}
      <div class="hidden md:flex items-center gap-4 text-xs text-gray-400 dark:text-gray-600">
        <span>{doneTasks}/{totalTasks.length} done this week</span>
        {#if estimateMins > 0}<span>~{formatMinutes(estimateMins)} planned</span>{/if}
        {#if actualMins > 0}<span class="text-green-600 dark:text-green-500">{formatMinutes(actualMins)} logged</span>{/if}
      </div>
    {/if}

    <!-- Actions -->
    <div class="flex items-center gap-2">
      {#if !isToday(date)}
        <button onclick={goToday}
                class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          Today
        </button>
      {/if}
      <button onclick={() => openCreate(todayDate)}
              class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold
                     text-white transition-colors shadow-sm"
              style="background:var(--a500);"
              onmouseenter={(e)=>(e.currentTarget as HTMLElement).style.background='var(--a600)'}
              onmouseleave={(e)=>(e.currentTarget as HTMLElement).style.background='var(--a500)'}>
        <Plus size={13} strokeWidth={2.5} />
        New task
      </button>
    </div>
  </div>
</header>

<!-- ── Body ───────────────────────────────────────────────────────────────── -->
<div class="flex h-[calc(100vh-57px)] overflow-hidden">

  <!-- Kanban area -->
  <main class="flex-1 overflow-auto px-4 py-5">

    <!-- Rollover banner -->
    {#if rolloverTasks.length > 0 && !rolloverDismissed}
      <div class="mb-4 flex items-center gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3
                  dark:border-amber-800/50 dark:bg-amber-950/40">
        <svg class="h-4 w-4 shrink-0 text-amber-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="flex-1 text-xs text-amber-700 dark:text-amber-400">
          <strong>{rolloverTasks.length}</strong> unfinished from yesterday —
          {rolloverTasks.slice(0,2).map(t=>t.title).join(', ')}{rolloverTasks.length > 2 ? '…' : ''}
        </p>
        <button onclick={rolloverAll}
                class="rounded-lg bg-amber-500 px-3 py-1 text-xs font-medium text-white hover:bg-amber-600 transition-colors">
          Roll over
        </button>
        <button onclick={() => rolloverDismissed = true} aria-label="Dismiss"
                class="text-amber-400 hover:text-amber-600">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    {/if}

    {#if loading}
      <div class="flex h-64 items-center justify-center text-sm text-gray-300 dark:text-gray-700">Loading…</div>
    {:else if error}
      <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-600
                  dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
        {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
      </div>
    {:else}
      <div class="flex items-start gap-3 pb-6">
        <!-- Mon–Fri main columns -->
        {#each weekDays.slice(0, 5) as day (day.date)}
          <WeekDayColumn
            date={day.date} dayName={day.dayName} dayNum={day.dayNum}
            isToday={day.isToday}
            tasks={dayTasks(day.date)}
            isDragOver={dragOverDate === day.date}
            onTaskDragStart={handleDragStart}
            onTaskFocusClick={handleFocus}
            onTaskComplete={handleComplete}
            onTaskClick={openEdit}
            onDrop={handleDrop}
            onEmailDrop={handleEmailDrop}
            onDragOver={(d) => (dragOverDate = d)}
            onDragLeave={() => (dragOverDate = null)}
            onAddClick={openCreate}
          />
        {/each}

        <!-- Sat–Sun (narrower) -->
        <div class="flex shrink-0 gap-2">
          {#each weekDays.slice(5) as day (day.date)}
            <div class="w-40">
              <WeekDayColumn
                date={day.date} dayName={day.dayName} dayNum={day.dayNum}
                isToday={day.isToday}
                tasks={dayTasks(day.date)}
                isDragOver={dragOverDate === day.date}
                onTaskDragStart={handleDragStart}
                onTaskFocusClick={handleFocus}
                onTaskComplete={handleComplete}
                onTaskClick={openEdit}
                onDrop={handleDrop}
                onEmailDrop={handleEmailDrop}
                onDragOver={(d) => (dragOverDate = d)}
                onDragLeave={() => (dragOverDate = null)}
                onAddClick={openCreate}
              />
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </main>

  <!-- ── Right panel ─────────────────────────────────────────────────────── -->
  <aside class="w-72 shrink-0 flex flex-col border-l border-gray-100 bg-white overflow-hidden
                dark:border-gray-800/60 dark:bg-gray-900">
    <div class="shrink-0 border-b border-gray-100 dark:border-gray-800/60">
      <MiniCalendar {date} />
    </div>
    <WeeklyObjectivesWidget {date} />

    <div class="flex shrink-0 border-b border-gray-100 dark:border-gray-800/60">
      <button onclick={() => rightTab = 'mail'}
              class="flex-1 py-2.5 text-xs font-medium transition-colors
                     {rightTab === 'mail'
                       ? 'border-b-2 text-[var(--a600)] dark:text-[var(--a400)]'
                       : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}"
              style={rightTab === 'mail' ? 'border-color:var(--a500)' : ''}>
        Mail
      </button>
      <button onclick={() => rightTab = 'schedule'}
              class="flex-1 py-2.5 text-xs font-medium transition-colors
                     {rightTab === 'schedule'
                       ? 'border-b-2 text-[var(--a600)] dark:text-[var(--a400)]'
                       : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}"
              style={rightTab === 'schedule' ? 'border-color:var(--a500)' : ''}>
        Schedule
      </button>
    </div>

    <div class="flex-1 overflow-hidden">
      {#if rightTab === 'mail'}
        <EmailPanel bind:this={emailPanel} onTaskCreated={(t) => { tasks = [...tasks, t]; }} />
      {:else}
        <TimeslotCalendar
          date={date}
          tasks={tasks}
          onSchedule={handleSchedule}
          onUnschedule={handleUnschedule}
        />
      {/if}
    </div>
  </aside>
</div>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={panelDate}
           onSave={handlePanelSave} onClose={() => (panelOpen = false)} />
