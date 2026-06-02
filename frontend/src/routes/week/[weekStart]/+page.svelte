<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { appendPosition, formatWeekRange, offsetDate, weekStart as calcWeekStart } from '$lib/utils';

  let weekStartDate = $derived($page.params.weekStart ?? calcWeekStart(new Date().toISOString().split('T')[0]));

  let objectives = $state<Objective[]>([]);
  let tasks      = $state<Task[]>([]);
  let loading    = $state(true);
  let error      = $state<string | null>(null);

  let expandedId = $state<string | null>(null);
  let addingTitle = $state('');
  let showAddForm = $state(false);
  let addingInput: HTMLInputElement | undefined = $state();

  async function load() {
    loading = true;
    error   = null;
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(weekStartDate),
        api.tasks.listByWeek(weekStartDate),
      ]);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }

  onMount(load);
  $effect(() => { weekStartDate; void load(); });

  // Task counts per objective
  function objectiveTasks(id: string): Task[] {
    return tasks.filter(t => t.weekly_objective_id === id);
  }
  function doneTasks(id: string): Task[] {
    return objectiveTasks(id).filter(t => t.status === 'done');
  }
  function progressPct(id: string): number {
    const total = objectiveTasks(id).length;
    return total === 0 ? 0 : Math.round((doneTasks(id).length / total) * 100);
  }

  // Totals across all objectives
  let totalObjectives = $derived(objectives.length);
  let completedObjectives = $derived(objectives.filter(o => o.status === 'completed').length);

  async function toggleStatus(obj: Objective) {
    const next = obj.status === 'completed' ? 'active' : 'completed';
    objectives = objectives.map(o => o.id === obj.id ? { ...o, status: next } : o);
    try {
      await api.objectives.update(obj.id, { status: next });
    } catch {
      objectives = objectives.map(o => o.id === obj.id ? obj : o); // rollback
    }
  }

  async function addObjective() {
    const title = addingTitle.trim();
    if (!title) return;
    addingTitle = '';
    showAddForm = false;

    const pos = appendPosition(objectives.map(o => o.position));
    try {
      const obj = await api.objectives.create({
        week_start: weekStartDate,
        title,
        position: pos,
      });
      objectives = [...objectives, obj];
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to add objective';
    }
  }

  async function deleteObjective(id: string) {
    objectives = objectives.filter(o => o.id !== id);
    try {
      await api.objectives.delete(id);
    } catch {}
  }

  function navigate(delta: number) {
    goto(`/week/${offsetDate(weekStartDate, delta * 7)}`);
  }

  const statusColors: Record<string, string> = {
    active:    'bg-blue-500',
    completed: 'bg-green-500',
    cancelled: 'bg-gray-300',
  };
</script>

<svelte:head><title>Week of {formatWeekRange(weekStartDate)} — Sempa</title></svelte:head>

<!-- Header -->
<header class="sticky top-0 z-10 border-b border-gray-200 bg-white/90 backdrop-blur-sm">
  <div class="flex items-center justify-between px-6 py-3">
    <div class="flex items-center gap-3">
      <button onclick={() => navigate(-1)} class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 transition-colors" aria-label="Previous week">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/></svg>
      </button>
      <div>
        <p class="text-sm font-semibold text-gray-800">{formatWeekRange(weekStartDate)}</p>
        <p class="text-xs text-gray-400">{completedObjectives}/{totalObjectives} objectives complete</p>
      </div>
      <button onclick={() => navigate(1)} class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 transition-colors" aria-label="Next week">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/></svg>
      </button>
    </div>
    <div class="flex items-center gap-2">
      <a href="/week/{weekStartDate}/review"
         class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium
                text-gray-600 hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
        </svg>
        Review week
      </a>
      <button
        onclick={() => { showAddForm = true; setTimeout(() => addingInput?.focus(), 0); }}
        class="flex items-center gap-1.5 rounded-lg bg-blue-500 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-600 transition-colors"
      >
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>
        Add objective
      </button>
    </div>
  </div>
</header>

<!-- Body -->
<main class="mx-auto max-w-2xl px-6 py-6">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm text-gray-400">Loading…</div>
  {:else if error}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">
      {error} <button onclick={load} class="ml-2 underline">Retry</button>
    </div>
  {:else}
    <!-- Overall progress bar -->
    {#if totalObjectives > 0}
      <div class="mb-6">
        <div class="mb-1 flex justify-between text-xs text-gray-400">
          <span>Week progress</span>
          <span>{completedObjectives}/{totalObjectives}</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-gray-100">
          <div
            class="h-full rounded-full bg-blue-500 transition-all duration-500"
            style="width: {totalObjectives ? Math.round((completedObjectives/totalObjectives)*100) : 0}%"
          ></div>
        </div>
      </div>
    {/if}

    <!-- Objectives list -->
    <div class="flex flex-col gap-3">
      {#each objectives as obj (obj.id)}
        {@const linked = objectiveTasks(obj.id)}
        {@const done   = doneTasks(obj.id)}
        {@const pct    = progressPct(obj.id)}
        {@const isExpanded = expandedId === obj.id}

        <div class="rounded-xl border border-gray-200 bg-white shadow-xs transition-shadow hover:shadow-sm">
          <!-- Objective row -->
          <div class="flex items-start gap-3 p-4">
            <!-- Status dot (click to toggle) -->
            <button
              onclick={() => toggleStatus(obj)}
              class="mt-0.5 h-3.5 w-3.5 shrink-0 rounded-full border-2 transition-colors
                     {obj.status === 'completed' ? 'bg-green-500 border-green-500' : 'border-gray-300 hover:border-blue-400'}"
              title="{obj.status === 'completed' ? 'Mark active' : 'Mark complete'}"
            ></button>

            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-gray-800 {obj.status === 'completed' ? 'line-through text-gray-400' : ''}">
                {obj.title}
              </p>

              <!-- Task progress -->
              {#if linked.length > 0}
                <div class="mt-2 flex items-center gap-2">
                  <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-100">
                    <div class="h-full rounded-full bg-blue-400 transition-all" style="width: {pct}%"></div>
                  </div>
                  <span class="shrink-0 text-xs text-gray-400">{done.length}/{linked.length}</span>
                </div>
              {:else}
                <p class="mt-1 text-xs text-gray-400">No tasks linked yet</p>
              {/if}
            </div>

            <div class="flex shrink-0 items-center gap-1">
              <!-- Expand toggle -->
              {#if linked.length > 0}
                <button
                  onclick={() => expandedId = isExpanded ? null : obj.id}
                  class="rounded p-1 text-gray-300 hover:text-gray-500 transition-colors"
                  aria-label="{isExpanded ? 'Collapse' : 'Expand'} tasks"
                >
                  <svg class="h-4 w-4 transition-transform {isExpanded ? 'rotate-180' : ''}" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/>
                  </svg>
                </button>
              {/if}
              <!-- Delete -->
              <button
                onclick={() => deleteObjective(obj.id)}
                class="rounded p-1 text-gray-200 hover:text-red-400 transition-colors"
                aria-label="Delete objective"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </div>

          <!-- Expanded task list -->
          {#if isExpanded && linked.length > 0}
            <div class="border-t border-gray-100 px-4 py-2">
              {#each linked as t (t.id)}
                <div class="flex items-center gap-2 py-1.5">
                  <div class="h-1.5 w-1.5 shrink-0 rounded-full {t.status === 'done' ? 'bg-green-400' : 'bg-gray-300'}"></div>
                  <span class="text-xs text-gray-600 {t.status === 'done' ? 'line-through text-gray-400' : ''}">{t.title}</span>
                  {#if t.planned_date}
                    <span class="ml-auto text-xs text-gray-300">{t.planned_date}</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}

      <!-- Empty state -->
      {#if objectives.length === 0 && !showAddForm}
        <div class="rounded-xl border-2 border-dashed border-gray-200 p-8 text-center">
          <p class="text-sm text-gray-400">No objectives for this week yet.</p>
          <button
            onclick={() => { showAddForm = true; setTimeout(() => addingInput?.focus(), 0); }}
            class="mt-2 text-sm text-blue-500 hover:underline"
          >Add your first objective</button>
        </div>
      {/if}

      <!-- Inline add form -->
      {#if showAddForm}
        <div class="rounded-xl border border-blue-200 bg-blue-50 p-4">
          <input
            bind:this={addingInput}
            bind:value={addingTitle}
            onkeydown={(e) => { if (e.key === 'Enter') addObjective(); if (e.key === 'Escape') showAddForm = false; }}
            type="text"
            placeholder="What do you want to accomplish this week?"
            class="w-full rounded-lg border border-blue-200 bg-white px-3 py-2.5 text-sm
                   text-gray-800 placeholder-gray-400 outline-none focus:ring-2 focus:ring-blue-300"
          />
          <div class="mt-2 flex gap-2">
            <button onclick={addObjective} disabled={!addingTitle.trim()}
              class="rounded-lg bg-blue-500 px-4 py-1.5 text-xs font-medium text-white
                     hover:bg-blue-600 disabled:opacity-40 transition-colors">
              Add
            </button>
            <button onclick={() => showAddForm = false}
              class="rounded-lg px-4 py-1.5 text-xs text-gray-500 hover:bg-blue-100 transition-colors">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</main>
