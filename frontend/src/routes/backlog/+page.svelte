<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { today, weekStart, formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import { Plus } from 'lucide-svelte';

  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  onMount(load);

  async function load() {
    loading = true; error = null;
    try { tasks = await api.tasks.listBacklog(); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load'; }
    finally { loading = false; }
  }

  async function scheduleToday(id: string) {
    const d  = today();
    const ws = weekStart(d);
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.update(id, { planned_date: d, week_start: ws, status: 'planned' }); }
    catch { await load(); }
  }

  async function complete(id: string) {
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.update(id, { status: 'done', completed_at: new Date().toISOString() }); }
    catch { await load(); }
  }

  async function remove(id: string) {
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.delete(id); }
    catch { await load(); }
  }

  function openCreate() { panelTask = null; panelOpen = true; }
  function openEdit(t: Task) { panelTask = t; panelOpen = true; }

  async function handlePanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    if (saved.planned_date) { tasks = tasks.filter(t => t.id !== saved.id); return; }
    if (saved.status === 'done') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    const idx = tasks.findIndex(t => t.id === saved.id);
    if (idx >= 0) tasks = tasks.map(t => t.id === saved.id ? saved : t);
    else tasks = [...tasks, saved];
  }

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Cal',
  };
</script>

<svelte:head><title>Backlog — Sempa</title></svelte:head>

<header class="sticky top-0 z-10 backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               border-bottom: 1px solid var(--sempa-border);">
  <div class="flex items-center justify-between px-6 py-3">
    <div>
      <p class="text-sm font-semibold" style="color: var(--sempa-text);">Backlog</p>
      <p class="text-[10px]" style="color: var(--sempa-text-dim);">
        {tasks.length} item{tasks.length !== 1 ? 's' : ''} waiting to be scheduled
      </p>
    </div>
    <button onclick={openCreate}
            class="flex items-center gap-1.5 rounded-[9px] px-3 py-1.5 text-[13px] font-[500]
                   tracking-[-0.01em] transition-colors shadow-sm"
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
      <Plus size={13} strokeWidth={2.5} />
      Add to backlog
    </button>
  </div>
</header>

<main class="mx-auto max-w-2xl px-6 py-6 animate-fade-in">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>

  {:else if error}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700
                dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
      {error} <button onclick={load} class="ml-2 underline">Retry</button>
    </div>

  {:else if tasks.length === 0}
    <div class="flex flex-col items-center justify-center py-20 gap-4"
         style="border: 2px dashed var(--sempa-border); border-radius: 16px;">
      <div class="h-12 w-12 rounded-full flex items-center justify-center"
           style="background: var(--sempa-accent-bg);">
        <svg class="h-6 w-6" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
        </svg>
      </div>
      <p class="text-sm" style="color: var(--sempa-text-dim);">Your backlog is clear.</p>
      <button onclick={openCreate}
              class="rounded-[9px] px-4 py-2 text-[13px] font-medium"
              style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
        Add a task
      </button>
    </div>

  {:else}
    <div class="flex flex-col gap-2">
      {#each tasks as task (task.id)}
        <div class="group flex items-center gap-3 rounded-xl px-4 py-3 transition-all hover:shadow-sm"
             style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">

          <!-- Complete circle -->
          <button onclick={() => complete(task.id)}
                  class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all cursor-pointer"
                  style="border-color: var(--sempa-border);"
                  onmouseenter={(e) => (e.currentTarget as HTMLElement).style.borderColor = '#22c55e'}
                  onmouseleave={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}
                  title="Mark done">
          </button>

          <!-- Title + tags -->
          <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
          <div class="flex-1 min-w-0 cursor-pointer" onclick={() => openEdit(task)}>
            <p class="text-sm font-medium" style="color: var(--sempa-text);">{task.title}</p>
            {#if task.tags?.length || task.time_estimate_minutes || (task.source && task.source !== 'manual')}
              <div class="mt-1 flex flex-wrap gap-1">
                {#each (task.tags ?? []) as tag}
                  <span class="rounded-full px-2 py-0.5 text-[10px] font-medium text-white"
                        style="background-color: {tagStore.colorFor(tag)}">{tag}</span>
                {/each}
                {#if task.time_estimate_minutes}
                  <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-mono"
                        style="color: var(--sempa-text-dim);">
                    {formatMinutes(task.time_estimate_minutes)}
                  </span>
                {/if}
                {#if task.source && task.source !== 'manual'}
                  <span class="rounded px-1.5 py-0.5 text-[10px]"
                        style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                    {sourceLabel[task.source] ?? task.source}
                  </span>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Actions (always visible on hover, "Plan today" is primary) -->
          <div class="flex shrink-0 items-center gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
            <button onclick={() => scheduleToday(task.id)}
                    class="rounded-lg px-2.5 py-1 text-[11px] font-medium transition-colors"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);"
                    title="Schedule to today">
              Plan today
            </button>
            <button onclick={() => remove(task.id)}
                    class="rounded p-1 transition-colors"
                    style="color: var(--sempa-text-dim);"
                    onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = '#f87171'}
                    onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}
                    title="Delete">
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</main>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus="backlog" defaultDate={today()}
           onSave={handlePanelSave} onClose={() => panelOpen = false} />
