<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import { COLUMNS, type Task, type TaskStatus } from '$lib/types';
  import { appendPosition, formatDate, formatMinutes, isToday, insertPosition, offsetDate, today, weekStart } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import KanbanColumn from '$lib/components/KanbanColumn.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import EmailPanel from '$lib/components/EmailPanel.svelte';
  import MiniCalendar from '$lib/components/MiniCalendar.svelte';
  import TimeslotCalendar from '$lib/components/TimeslotCalendar.svelte';
  import WeeklyObjectivesWidget from '$lib/components/WeeklyObjectivesWidget.svelte';

  let date = $derived($page.params.date ?? today());
  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Rollover
  let rolloverTasks = $state<Task[]>([]);
  let rolloverDismissed = $state(false);

  let draggingId     = $state<string | null>(null);
  let dragOverStatus = $state<TaskStatus | null>(null);
  let emailPanel     = $state<EmailPanel | undefined>(undefined);
  let rightTab       = $state<'inbox' | 'upcoming'>('inbox');

  let panelOpen   = $state(false);
  let panelTask   = $state<Task | null>(null);
  let panelStatus = $state<TaskStatus>('planned');

  // React to pomodoro completing a session and updating actual time
  $effect(() => {
    const upd = pomodoro.lastTimeUpdate;
    if (upd) {
      tasks = tasks.map(t => t.id === upd.taskId ? { ...t, time_actual_minutes: upd.newActual } : t);
    }
  });

  async function loadTasks() {
    loading = true; error = null;
    try { tasks = await api.tasks.listByDate(date); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load tasks'; }
    finally { loading = false; }
  }

  async function loadRollover() {
    if (!isToday(date)) return;
    try {
      const yesterday = offsetDate(date, -1);
      const prev = await api.tasks.listByDate(yesterday);
      rolloverTasks = prev.filter(t => t.status === 'planned' || t.status === 'in_progress');
    } catch { /* ignore */ }
  }

  onMount(() => { loadTasks(); loadRollover(); });
  $effect(() => { date; loadTasks(); });

  async function rolloverAll() {
    const ws = weekStart(date);
    await Promise.all(rolloverTasks.map(t =>
      api.tasks.update(t.id, { planned_date: date, week_start: ws, status: 'planned' })
    ));
    rolloverTasks = [];
    await loadTasks();
  }

  function columnTasks(status: TaskStatus): Task[] {
    return tasks.filter(t => t.status === status).sort((a, b) => a.position - b.position);
  }

  // Day stats
  const todayTasks   = $derived(tasks.filter(t => t.status !== 'cancelled'));
  const estimateMins = $derived(todayTasks.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0));
  const actualMins   = $derived(todayTasks.reduce((s, t) => s + (t.time_actual_minutes ?? 0), 0));
  const doneTasks    = $derived(todayTasks.filter(t => t.status === 'done').length);

  // ── Drag & drop ──────────────────────────────────────────────────────────
  function handleDragStart(id: string) { draggingId = id; }

  async function handleDrop(targetStatus: TaskStatus, insertIdx?: number) {
    if (!draggingId) return;
    const id = draggingId;
    draggingId = null; dragOverStatus = null;
    const task = tasks.find(t => t.id === id);
    if (!task) return;

    // Calculate new position
    const colTasks = tasks
      .filter(t => t.status === targetStatus && t.id !== id)
      .sort((a, b) => a.position - b.position);
    const positions = colTasks.map(t => t.position);

    let newPos: number;
    if (insertIdx !== undefined) {
      newPos = insertPosition(positions, insertIdx);
    } else {
      newPos = appendPosition(positions);
    }

    const sameStatus = task.status === targetStatus;
    if (sameStatus && task.position === newPos) return;

    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === id ? { ...t, status: targetStatus, position: newPos } : t);
    try {
      const updated = await api.tasks.update(id, {
        status: targetStatus, position: newPos,
        ...(task.planned_date === null && targetStatus !== 'backlog'
          ? { planned_date: date, week_start: weekStart(date) } : {}),
      });
      tasks = tasks.map(t => t.id === updated.id ? updated : t);
    } catch { tasks = prev; }
  }

  // ── Quick complete ────────────────────────────────────────────────────────
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

  // ── Pomodoro ──────────────────────────────────────────────────────────────
  function handleFocus(id: string, title: string) {
    const t = tasks.find(t => t.id === id);
    pomodoro.start(id, title, t?.time_actual_minutes ?? 0);
  }

  // ── Panel ─────────────────────────────────────────────────────────────────
  function openCreate(status: TaskStatus) { panelTask = null; panelStatus = status; panelOpen = true; }
  function openEdit(task: Task) { panelTask = task; panelOpen = true; }

  async function handlePanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled' && !tasks.find(t => t.id === saved.id)) {
      tasks = tasks.filter(t => t.id !== saved.id); return;
    }
    if (!panelTask && saved.recurrence_rule) { await loadTasks(); return; }
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    const existing = tasks.findIndex(t => t.id === saved.id);
    if (existing >= 0) tasks = tasks.map(t => t.id === saved.id ? saved : t);
    else tasks = [...tasks, saved];
  }

  // ── Email drop ────────────────────────────────────────────────────────────
  async function handleEmailDrop(emailData: { id: string; subject: string }, targetStatus: TaskStatus) {
    try {
      const task = await api.integrations.fastmail.toTask(emailData.id, emailData.subject);
      const updated = targetStatus !== task.status
        ? await api.tasks.update(task.id, { status: targetStatus })
        : task;
      tasks = [...tasks, updated];
      emailPanel?.removeEmail(emailData.id);
    } catch (e: any) { error = e.message; }
  }

  // ── Calendar schedule / unschedule ───────────────────────────────────────
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

  // ── Navigation ───────────────────────────────────────────────────────────
  function navigate(delta: number) { goto(`/day/${offsetDate(date, delta)}`); }
</script>

<svelte:head><title>{isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<!-- ── Header ─────────────────────────────────────────────────────────────── -->
<header class="sticky top-0 z-10 border-b border-gray-100 bg-white/95 backdrop-blur-sm
               dark:border-gray-800/60 dark:bg-gray-900/95">
  <div class="flex items-center justify-between px-6 py-3">
    <div class="flex items-center gap-2">
      <button onclick={() => navigate(-1)} aria-label="Previous day"
              class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-400">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div>
        <p class="text-sm font-semibold text-gray-900 dark:text-gray-50">{formatDate(date)}</p>
        {#if isToday(date)}
          <p class="text-[10px] font-medium text-blue-500 dark:text-blue-400 uppercase tracking-wider">Today</p>
        {/if}
      </div>
      <button onclick={() => navigate(1)} aria-label="Next day"
              class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-400">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>

    <!-- Day stats -->
    {#if !loading && todayTasks.length > 0}
      <div class="hidden sm:flex items-center gap-4 text-xs text-gray-400 dark:text-gray-600">
        <span>{doneTasks}/{todayTasks.length} done</span>
        {#if estimateMins > 0}
          <span>~{formatMinutes(estimateMins)} planned</span>
        {/if}
        {#if actualMins > 0}
          <span class="text-green-600 dark:text-green-500">{formatMinutes(actualMins)} logged</span>
        {/if}
      </div>
    {/if}

    <div class="flex items-center gap-2">
      {#if !isToday(date)}
        <button onclick={() => goto(`/day/${today()}`)}
                class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          Today
        </button>
      {/if}
      <button onclick={() => openCreate('planned')}
              class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold
                     text-white transition-colors shadow-sm"
              style="background:var(--a500);"
              onmouseenter={(e)=>(e.currentTarget as HTMLElement).style.background='var(--a600)'}
              onmouseleave={(e)=>(e.currentTarget as HTMLElement).style.background='var(--a500)'}>
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
        </svg>
        New task
      </button>
    </div>
  </div>
</header>

<!-- ── Body ───────────────────────────────────────────────────────────────── -->
<div class="flex h-[calc(100vh-57px)] overflow-hidden">

  <!-- Kanban area -->
  <main class="flex-1 overflow-auto px-6 py-6">

    <!-- Rollover banner -->
    {#if rolloverTasks.length > 0 && !rolloverDismissed}
      <div class="mb-4 flex items-center gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3
                  dark:border-amber-800/50 dark:bg-amber-950/40">
        <svg class="h-4 w-4 shrink-0 text-amber-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="flex-1 text-xs text-amber-700 dark:text-amber-400">
          <strong>{rolloverTasks.length}</strong> unfinished task{rolloverTasks.length > 1 ? 's' : ''} from yesterday —
          {rolloverTasks.slice(0, 2).map(t => t.title).join(', ')}{rolloverTasks.length > 2 ? `…` : ''}
        </p>
        <button onclick={rolloverAll}
                class="rounded-lg bg-amber-500 px-3 py-1 text-xs font-medium text-white hover:bg-amber-600 transition-colors">
          Roll over
        </button>
        <button onclick={() => rolloverDismissed = true} aria-label="Dismiss"
                class="text-amber-400 hover:text-amber-600 dark:text-amber-600 dark:hover:text-amber-400">
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
      <div class="flex items-start gap-4 pb-6">
        {#each COLUMNS as col (col.status)}
          <KanbanColumn
            label={col.label} status={col.status} tasks={columnTasks(col.status)}
            accent={col.accent}
            isDragOver={dragOverStatus === col.status}
            onTaskDragStart={handleDragStart}
            onTaskFocusClick={handleFocus}
            onTaskComplete={handleComplete}
            onTaskClick={openEdit}
            onDrop={handleDrop}
            onEmailDrop={handleEmailDrop}
            onDragOver={(s) => (dragOverStatus = s)}
            onDragLeave={() => (dragOverStatus = null)}
            onAddClick={openCreate}
          />
        {/each}
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
      <button onclick={() => rightTab = 'inbox'}
              class="flex-1 py-2.5 text-xs font-medium transition-colors
                     {rightTab === 'inbox'
                       ? 'border-b-2 border-[var(--a500)] text-[var(--a600)] dark:text-[var(--a400)]'
                       : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}">
        Mail
      </button>
      <button onclick={() => rightTab = 'upcoming'}
              class="flex-1 py-2.5 text-xs font-medium transition-colors
                     {rightTab === 'upcoming'
                       ? 'border-b-2 border-[var(--a500)] text-[var(--a600)] dark:text-[var(--a400)]'
                       : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}">
        Schedule
      </button>
    </div>

    <div class="flex-1 overflow-hidden">
      {#if rightTab === 'inbox'}
        <EmailPanel
          bind:this={emailPanel}
          onTaskCreated={(task) => { tasks = [...tasks, task]; }}
        />
      {:else}
        <TimeslotCalendar
          {date}
          {tasks}
          onSchedule={handleSchedule}
          onUnschedule={handleUnschedule}
        />
      {/if}
    </div>
  </aside>
</div>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={date}
           onSave={handlePanelSave} onClose={() => (panelOpen = false)} />
