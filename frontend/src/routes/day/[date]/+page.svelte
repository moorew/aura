<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import { COLUMNS, type Task, type TaskStatus } from '$lib/types';
  import { appendPosition, formatDate, isToday, offsetDate, today, weekStart } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import KanbanColumn from '$lib/components/KanbanColumn.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import EmailPanel from '$lib/components/EmailPanel.svelte';
  import MiniCalendar from '$lib/components/MiniCalendar.svelte';

  let date = $derived($page.params.date ?? today());
  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let draggingId     = $state<string | null>(null);
  let dragOverStatus = $state<TaskStatus | null>(null);
  let emailPanel     = $state<EmailPanel | undefined>(undefined);
  let rightTab       = $state<'inbox' | 'upcoming'>('inbox');

  let panelOpen   = $state(false);
  let panelTask   = $state<Task | null>(null);
  let panelStatus = $state<TaskStatus>('planned');

  async function loadTasks() {
    loading = true; error = null;
    try { tasks = await api.tasks.listByDate(date); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load tasks'; }
    finally { loading = false; }
  }

  onMount(loadTasks);
  $effect(() => { date; loadTasks(); });

  function columnTasks(status: TaskStatus): Task[] {
    return tasks.filter(t => t.status === status).sort((a, b) => a.position - b.position);
  }

  // ── Drag & drop ──────────────────────────────────────────────────────────
  function handleDragStart(id: string) { draggingId = id; }

  async function handleDrop(targetStatus: TaskStatus) {
    if (!draggingId || !dragOverStatus) return;
    const id = draggingId;
    draggingId = null; dragOverStatus = null;
    const task = tasks.find(t => t.id === id);
    if (!task || task.status === targetStatus) return;
    const newPos = appendPosition(tasks.filter(t => t.status === targetStatus).map(t => t.position));
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
  function handleFocus(id: string, title: string) { pomodoro.start(id, title); }

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

  // ── Navigation ───────────────────────────────────────────────────────────
  function navigate(delta: number) { goto(`/day/${offsetDate(date, delta)}`); }
</script>

<svelte:head><title>{isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<!-- ── Header ─────────────────────────────────────────────────────────────── -->
<header class="sticky top-0 z-10 border-b border-gray-100 bg-white/95 backdrop-blur-sm
               dark:border-gray-800/60 dark:bg-gray-900/95">
  <div class="flex items-center justify-between px-6 py-3">
    <!-- Date navigation -->
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

    <!-- Actions -->
    <div class="flex items-center gap-2">
      {#if !isToday(date)}
        <button onclick={() => goto(`/day/${today()}`)}
                class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          Today
        </button>
      {/if}
      <button onclick={() => openCreate('planned')}
              class="flex items-center gap-1.5 rounded-lg bg-blue-500 px-3 py-1.5 text-xs font-semibold
                     text-white hover:bg-blue-600 transition-colors shadow-sm shadow-blue-200 dark:shadow-none">
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
    {#if loading}
      <div class="flex h-64 items-center justify-center text-sm text-gray-300 dark:text-gray-700">
        Loading…
      </div>
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

  <!-- ── Right panel: calendar + inbox ─────────────────────────────────── -->
  <aside class="w-72 shrink-0 flex flex-col border-l border-gray-100 bg-white overflow-hidden
                dark:border-gray-800/60 dark:bg-gray-900">

    <!-- Mini calendar -->
    <div class="shrink-0 border-b border-gray-100 dark:border-gray-800/60">
      <MiniCalendar {date} />
    </div>

    <!-- Tab switcher -->
    <div class="flex shrink-0 border-b border-gray-100 dark:border-gray-800/60">
      <button onclick={() => rightTab = 'inbox'}
              class="flex-1 py-2.5 text-xs font-medium transition-colors
                     {rightTab === 'inbox'
                       ? 'border-b-2 border-blue-500 text-blue-600 dark:text-blue-400'
                       : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}">
        Inbox
      </button>
      <button onclick={() => rightTab = 'upcoming'}
              class="flex-1 py-2.5 text-xs font-medium transition-colors
                     {rightTab === 'upcoming'
                       ? 'border-b-2 border-blue-500 text-blue-600 dark:text-blue-400'
                       : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'}">
        Upcoming
      </button>
    </div>

    <!-- Tab content -->
    <div class="flex-1 overflow-hidden">
      {#if rightTab === 'inbox'}
        <EmailPanel
          bind:this={emailPanel}
          onTaskCreated={(task) => { tasks = [...tasks, task]; }}
        />
      {:else}
        <div class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-gray-100 dark:bg-gray-800">
            <svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
              <rect x="3" y="4" width="18" height="18" rx="2"/><path stroke-linecap="round" d="M16 2v4M8 2v4M3 10h18"/>
            </svg>
          </div>
          <div>
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">No calendar connected</p>
            <p class="mt-1 text-[10px] text-gray-400 dark:text-gray-600">Calendar integration coming soon</p>
          </div>
          <a href="/settings/integrations"
             class="text-[10px] text-blue-500 hover:underline dark:text-blue-400">
            Set up integrations →
          </a>
        </div>
      {/if}
    </div>
  </aside>
</div>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={date}
           onSave={handlePanelSave} onClose={() => (panelOpen = false)} />
