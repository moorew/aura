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

  let date = $derived($page.params.date ?? today());
  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let draggingId = $state<string | null>(null);
  let dragOverStatus = $state<TaskStatus | null>(null);
  let showEmailPanel = $state(false);
  let emailPanel = $state<EmailPanel | undefined>(undefined);

  // Panel state — null = closed, undefined task = create, Task = edit
  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);
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
  function openCreate(status: TaskStatus) {
    panelTask = null; panelStatus = status; panelOpen = true;
  }

  function openEdit(task: Task) {
    panelTask = task; panelOpen = true;
  }

  async function handlePanelSave(saved: Task) {
    panelOpen = false;
    // Deletion signal: status 'cancelled' and not in original list
    if (saved.status === 'cancelled' && !tasks.find(t => t.id === saved.id)) {
      tasks = tasks.filter(t => t.id !== saved.id);
      return;
    }
    // If it was a create with recurrence, reload (generate was triggered server-side)
    if (!panelTask && saved.recurrence_rule) {
      await loadTasks();
      return;
    }
    // Remove deleted tasks
    if (saved.status === 'cancelled') {
      tasks = tasks.filter(t => t.id !== saved.id);
      return;
    }
    // Update or insert
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
    } catch (e: any) {
      error = e.message;
    }
  }

  // ── Navigation ───────────────────────────────────────────────────────────
  function navigate(delta: number) { goto(`/day/${offsetDate(date, delta)}`); }
</script>

<svelte:head><title>{isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<header class="sticky top-0 z-10 border-b border-gray-200 bg-white/90 backdrop-blur-sm
               dark:border-gray-800 dark:bg-gray-900/90">
  <div class="mx-auto flex max-w-[1400px] items-center justify-between px-6 py-3">
    <div class="flex items-center gap-3">
      <button onclick={() => navigate(-1)}
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div class="text-center">
        <p class="text-sm font-semibold text-gray-800 dark:text-gray-100">{formatDate(date)}</p>
        {#if isToday(date)}<p class="text-xs text-blue-500 font-medium dark:text-blue-400">Today</p>{/if}
      </div>
      <button onclick={() => navigate(1)}
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
    <div class="flex items-center gap-2">
      {#if !isToday(date)}
        <button onclick={() => goto(`/day/${today()}`)}
                class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          Today
        </button>
      {/if}
      <button onclick={() => openCreate('planned')}
              class="flex items-center gap-1.5 rounded-lg bg-blue-500 px-3 py-1.5 text-xs font-medium
                     text-white hover:bg-blue-600 transition-colors">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
        </svg>
        New task
      </button>
      <button onclick={() => showEmailPanel = !showEmailPanel}
              title="Toggle email inbox"
              class="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors
                     {showEmailPanel
                       ? 'border-blue-400 bg-blue-50 text-blue-600 dark:border-blue-600 dark:bg-blue-950 dark:text-blue-400'
                       : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800'}">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
        </svg>
        Inbox
      </button>
    </div>
  </div>
</header>

<div class="flex h-[calc(100vh-57px)] overflow-hidden">
  <main class="flex-1 overflow-auto px-6 py-6">
    {#if loading}
      <div class="flex h-64 items-center justify-center text-sm text-gray-400 dark:text-gray-600">Loading…</div>
    {:else if error}
      <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700
                  dark:border-red-900 dark:bg-red-950 dark:text-red-400">
        {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
      </div>
    {:else}
      <div class="flex gap-4 overflow-x-auto pb-4">
        {#each COLUMNS as col (col.status)}
          <KanbanColumn
            label={col.label} status={col.status} tasks={columnTasks(col.status)}
            accent={col.accent} bg={col.bg} border={col.border}
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

  {#if showEmailPanel}
    <div class="w-72 shrink-0 overflow-hidden">
      <EmailPanel
        bind:this={emailPanel}
        onTaskCreated={(task) => { tasks = [...tasks, task]; }}
      />
    </div>
  {/if}
</div>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={date}
           onSave={handlePanelSave} onClose={() => (panelOpen = false)} />
