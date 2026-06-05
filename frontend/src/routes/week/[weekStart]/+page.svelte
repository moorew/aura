<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { appendPosition, formatWeekRange, offsetDate, today, weekStart as calcWeekStart } from '$lib/utils';
  import { mobile } from '$lib/stores/mobile.svelte';

  let weekStartDate = $derived($page.params.weekStart ?? calcWeekStart(new Date().toISOString().split('T')[0]));

  let objectives       = $state<Objective[]>([]);
  let tasks            = $state<Task[]>([]);
  let loading          = $state(true);
  let error            = $state<string | null>(null);
  let copied           = $state(false);
  let showUnscheduled  = $state(false);

  const unscheduled = $derived(
    tasks.filter(t => !t.planned_date && t.status !== 'done' && t.status !== 'cancelled')
         .sort((a, b) => a.position - b.position)
  );

  let expandedId   = $state<string | null>(null);
  let addingTitle  = $state('');
  let showAddForm  = $state(false);
  let addingInput: HTMLInputElement | undefined = $state();
  let taskDrafts   = $state<Record<string, string>>({});
  let addingTask   = $state<Record<string, boolean>>({});
  let taskDays     = $state<Record<string, string>>({});

  async function load() {
    loading = true; error = null;
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(weekStartDate),
        api.tasks.listByWeek(weekStartDate),
      ]);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally { loading = false; }
  }

  onMount(load);
  $effect(() => { weekStartDate; void load(); });

  // ── Task helpers ───────────────────────────────────────────────────────────
  function objectiveTasks(id: string): Task[] {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled')
                .sort((a, b) => a.position - b.position);
  }
  function doneTasks(id: string) { return objectiveTasks(id).filter(t => t.status === 'done'); }
  function progressPct(id: string): number {
    const total = objectiveTasks(id).length;
    return total === 0 ? 0 : Math.round((doneTasks(id).length / total) * 100);
  }

  // ── Objective actions ──────────────────────────────────────────────────────
  let totalObjectives     = $derived(objectives.length);
  let completedObjectives = $derived(objectives.filter(o => o.status === 'completed').length);

  async function toggleStatus(obj: Objective) {
    const next = obj.status === 'completed' ? 'active' : 'completed';
    objectives = objectives.map(o => o.id === obj.id ? { ...o, status: next } : o);
    try { await api.objectives.update(obj.id, { status: next }); }
    catch { objectives = objectives.map(o => o.id === obj.id ? obj : o); }
  }

  async function addObjective() {
    const title = addingTitle.trim();
    if (!title) return;
    addingTitle = ''; showAddForm = false;
    const pos = appendPosition(objectives.map(o => o.position));
    try {
      const obj = await api.objectives.create({ week_start: weekStartDate, title, position: pos });
      objectives = [...objectives, obj];
    } catch (e) { error = e instanceof Error ? e.message : 'Failed'; }
  }

  async function scheduleToday(t: Task) {
    const d = today();
    tasks = tasks.map(x => x.id === t.id ? { ...x, planned_date: d } : x);
    try {
      const updated = await api.tasks.update(t.id, { planned_date: d, week_start: weekStartDate, status: 'planned' });
      tasks = tasks.map(x => x.id === updated.id ? updated : x);
    } catch { await load(); }
  }

  async function deleteObjective(id: string) {
    objectives = objectives.filter(o => o.id !== id);
    await api.objectives.delete(id).catch(() => {});
  }

  // ── Task actions ───────────────────────────────────────────────────────────
  async function addTask(objId: string) {
    const title = taskDrafts[objId]?.trim();
    if (!title) return;
    addingTask = { ...addingTask, [objId]: true };
    taskDrafts = { ...taskDrafts, [objId]: '' };
    const pos = appendPosition(objectiveTasks(objId).map(t => t.position));
    const plannedDate = taskDays[objId] ?? today();
    try {
      const t = await api.tasks.create({
        title,
        weekly_objective_id: objId,
        week_start: weekStartDate,
        planned_date: plannedDate,
        status: 'planned',
        position: pos,
      });
      tasks = [...tasks, t];
    } catch {}
    addingTask = { ...addingTask, [objId]: false };
  }

  async function toggleTask(t: Task) {
    const newStatus = t.status === 'done' ? 'planned' : 'done';
    tasks = tasks.map(x => x.id === t.id ? { ...x, status: newStatus } : x);
    try {
      const updated = await api.tasks.update(t.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      tasks = tasks.map(x => x.id === updated.id ? updated : x);
    } catch { await load(); }
  }

  // ── Markdown export ────────────────────────────────────────────────────────
  function generateMarkdown(): string {
    const lines = [
      `# Week of ${formatWeekRange(weekStartDate)}`,
      '',
      '## Objectives',
      '',
    ];
    for (const obj of objectives) {
      const linked = objectiveTasks(obj.id);
      const p = progressPct(obj.id);
      const icon = obj.status === 'completed' ? '✅' : '🎯';
      lines.push(`### ${icon} ${obj.title}${linked.length ? ` — ${p}% complete` : ''}`);
      if (linked.length === 0) {
        lines.push('*No tasks linked*');
      } else {
        for (const t of linked) {
          lines.push(`- [${t.status === 'done' ? 'x' : ' '}] ${t.title}`);
        }
      }
      lines.push('');
    }
    if (objectives.length === 0) lines.push('*No objectives set*\n');
    lines.push('---');
    lines.push(`*Sempa · ${new Date().toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}*`);
    return lines.join('\n');
  }

  async function copyMarkdown() {
    try {
      await navigator.clipboard.writeText(generateMarkdown());
      copied = true;
      setTimeout(() => copied = false, 2000);
    } catch {}
  }

  function navigate(delta: number) { goto(`/week/${offsetDate(weekStartDate, delta * 7)}`); }
</script>

<svelte:head><title>Week of {formatWeekRange(weekStartDate)} — Sempa</title></svelte:head>

<!-- Header -->
<header class="sticky top-0 z-[40] backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               border-bottom: 1px solid var(--sempa-border);
               padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
  <div class="flex items-center justify-between px-6 py-3">
    <div class="flex items-center gap-2">
      <button onclick={() => navigate(-1)} aria-label="Previous week"
              class="rounded-lg p-1.5 transition-colors"
              style="color: var(--sempa-text-dim);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div>
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">{formatWeekRange(weekStartDate)}</p>
        <p class="text-xs" style="color: var(--sempa-text-dim);">{completedObjectives}/{totalObjectives} objectives complete</p>
      </div>
      <button onclick={() => navigate(1)} aria-label="Next week"
              class="rounded-lg p-1.5 transition-colors"
              style="color: var(--sempa-text-dim);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>

    <div class="flex items-center gap-2">
      {#if mobile.value}
        <!-- Mobile: icon-only actions -->
        <button onclick={copyMarkdown}
                title="Copy as markdown"
                class="flex items-center justify-center rounded-lg transition-colors"
                style="width:34px; height:34px; border: 1px solid var(--sempa-border);
                       color: {copied ? '#22c55e' : 'var(--sempa-text-soft)'}; background: transparent;">
          {#if copied}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {:else}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="9" y="9" width="13" height="13" rx="2"/>
              <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
          {/if}
        </button>
        <a href="/week/{weekStartDate}/review"
           title="Review week"
           class="flex items-center justify-center rounded-lg transition-colors"
           style="width:34px; height:34px; border: 1px solid var(--sempa-border);
                  color: var(--sempa-text-soft); background: transparent;">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
          </svg>
        </a>
        <a href="/week/{weekStartDate}/plan"
           class="flex items-center gap-1 shadow-sm"
           style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius: 9px;
                  padding: 7px 12px; font-size: 13px; font-weight: 500; border: none; cursor: pointer;"
           onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
           onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
          </svg>
          Plan
        </a>
      {:else}
        <!-- Desktop: text buttons -->
        <button onclick={copyMarkdown}
                class="flex items-center gap-1.5 font-medium transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: transparent; border-radius: 9px; padding: 7px 14px; font-size: 12px;">
          {#if copied}
            <svg class="h-3.5 w-3.5 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
            Copied!
          {:else}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="9" y="9" width="13" height="13" rx="2"/>
              <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
            Copy as markdown
          {/if}
        </button>

        <a href="/week/{weekStartDate}/review"
           class="flex items-center gap-1.5 font-medium transition-colors"
           style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                  background: transparent; border-radius: 9px; padding: 7px 14px; font-size: 12px;">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
        </svg>
        Review week
      </a>

      <a href="/week/{weekStartDate}/plan"
         class="flex items-center gap-1.5 shadow-sm"
         style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius: 9px;
                padding: 8px 20px; font-size: 13px; font-weight: 500; border: none; cursor: pointer;"
         onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
         onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
        </svg>
        Plan week
      </a>
      {/if}
    </div>
  </div>
</header>

<!-- Body -->
<main class="mx-auto max-w-2xl px-6 py-6 animate-fade-in">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm text-gray-400">Loading…</div>

  {:else if error}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
      {error} <button onclick={load} class="ml-2 underline">Retry</button>
    </div>

  {:else}
    <!-- Overall progress -->
    {#if totalObjectives > 0}
      <div class="mb-6">
        <div class="mb-1.5 flex justify-between text-xs" style="color: var(--sempa-text-dim);">
          <span>Week progress</span>
          <span>{completedObjectives}/{totalObjectives} objectives · {totalObjectives ? Math.round((completedObjectives/totalObjectives)*100) : 0}%</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full" style="background: var(--sempa-border);">
          <div style="width:{totalObjectives ? Math.round((completedObjectives/totalObjectives)*100) : 0}%; height:100%; border-radius:9999px;
                      background: var(--sempa-accent); transition: width 500ms ease-out;"></div>
        </div>
      </div>
    {/if}

    <!-- Objectives list -->
    <div class="flex flex-col gap-3">
      {#each objectives as obj (obj.id)}
        {@const linked  = objectiveTasks(obj.id)}
        {@const done    = doneTasks(obj.id)}
        {@const p       = progressPct(obj.id)}
        {@const isExp   = expandedId === obj.id}
        {@const isDone  = obj.status === 'completed'}

        <div class="transition-shadow hover:shadow-md"
             style="border-radius:12px; border: 1px solid var(--sempa-border);
                    background: var(--sempa-bg-panel);">

          <!-- Objective header row -->
          <div class="flex items-start gap-3 p-4">
            <!-- Completion circle -->
            <button onclick={() => toggleStatus(obj)} title="{isDone ? 'Mark active' : 'Mark complete'}"
                    class="mt-0.5 h-5 w-5 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                           {isDone ? 'border-green-500 bg-green-500' : 'border-gray-300'}"
                    onmouseenter={(e) => { if (!isDone) (e.currentTarget as HTMLElement).style.borderColor = '#22c55e'; }}
                    onmouseleave={(e) => { if (!isDone) (e.currentTarget as HTMLElement).style.borderColor = ''; }}>
              {#if isDone}
                <svg class="h-3 w-3 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
              {/if}
            </button>

            <div class="flex-1 min-w-0">
              <p class="text-sm font-semibold {isDone ? 'line-through' : ''}"
                 style="color: {isDone ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                {obj.title}
              </p>

              <!-- Task progress bar + label -->
              {#if linked.length > 0}
                <div class="mt-2 flex items-center gap-2">
                  <div class="h-1.5 flex-1 max-w-[160px] overflow-hidden rounded-full" style="background: var(--sempa-border);">
                    <div style="width:{p}%; height:100%; border-radius:3px;
                                background: {isDone ? '#22c55e' : 'var(--sempa-accent)'}; transition: width 500ms ease-out;"></div>
                  </div>
                  <span class="text-xs font-medium"
                        style="color: {isDone || p === 100 ? '#22c55e' : 'var(--sempa-text-dim)'}">
                    {p}% · {done.length}/{linked.length} tasks
                  </span>
                </div>
              {:else}
                <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">No tasks linked yet</p>
              {/if}
            </div>

            <!-- Actions -->
            <div class="flex shrink-0 items-center gap-0.5">
              <button onclick={() => expandedId = isExp ? null : obj.id}
                      class="rounded-lg p-1.5 transition-colors"
                      style="color: var(--sempa-text-dim);"
                      aria-label="{isExp ? 'Collapse' : 'Expand'}">
                <svg class="h-4 w-4 transition-transform {isExp ? 'rotate-180' : ''}" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/>
                </svg>
              </button>
              <button onclick={() => deleteObjective(obj.id)} aria-label="Delete objective"
                      class="rounded-lg p-1.5 transition-colors"
                      style="color: var(--sempa-text-dim);"
                      onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = '#f87171'}
                      onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </div>

          <!-- Expanded: task list + inline add -->
          {#if isExp}
            <div class="px-4 py-3 space-y-1" style="border-top: 1px solid var(--sempa-border);">
              {#each linked as t (t.id)}
                <div class="group flex items-center gap-2.5 rounded-lg px-1 py-1.5">
                  <button onclick={() => toggleTask(t)}
                          class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                                 {t.status === 'done' ? 'border-green-500 bg-green-500' : 'border-gray-300'}"
                          onmouseenter={(e) => { if (t.status !== 'done') (e.currentTarget as HTMLElement).style.borderColor = '#22c55e'; }}
                          onmouseleave={(e) => { if (t.status !== 'done') (e.currentTarget as HTMLElement).style.borderColor = ''; }}>
                    {#if t.status === 'done'}
                      <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                      </svg>
                    {/if}
                  </button>
                  <span class="flex-1 text-sm {t.status === 'done' ? 'line-through' : ''}"
                        style="color: {t.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                    {t.title}
                  </span>
                  {#if linked.length > 0}
                    <span class="text-[10px] shrink-0" style="color: var(--sempa-text-dim);">
                      {Math.round(100 / linked.length)}%
                    </span>
                  {/if}
                  {#if t.planned_date}
                    <span class="text-[10px] shrink-0" style="color: var(--sempa-text-dim);">{t.planned_date.slice(5)}</span>
                  {/if}
                </div>
              {/each}

              <!-- Quick add task -->
              <div class="mt-1 space-y-1.5">
                <div class="flex items-center gap-2 rounded-lg border border-dashed px-2 py-1.5
                            focus-within:border-[var(--a500)]"
                     style="border-color: var(--sempa-border);">
                  <svg class="h-3.5 w-3.5 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
                  </svg>
                  <input bind:value={taskDrafts[obj.id]}
                         onkeydown={(e) => { if (e.key === 'Enter') addTask(obj.id); }}
                         type="text"
                         placeholder="Add a task… (Enter to save)"
                         class="flex-1 bg-transparent text-xs outline-none"
                         style="color: var(--sempa-text);" />
                </div>
                {#if taskDrafts[obj.id]?.trim()}
                  <div class="flex items-center gap-2 pl-1">
                    <select bind:value={taskDays[obj.id]}
                            class="rounded-md px-2 py-1 text-xs outline-none"
                            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);
                                   color: var(--sempa-text-soft);">
                      {#each Array.from({length:7},(_,i)=>offsetDate(weekStartDate,i)) as d}
                        <option value={d}>{new Date(d+'T12:00:00').toLocaleDateString('en-US',{weekday:'short',month:'short',day:'numeric'})}</option>
                      {/each}
                    </select>
                    <button onclick={() => addTask(obj.id)} disabled={addingTask[obj.id]}
                            class="rounded-md bg-[var(--a500)] px-3 py-1 text-xs font-medium text-white
                                   hover:bg-[var(--a600)] disabled:opacity-40 transition-colors">
                      Add
                    </button>
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        </div>
      {/each}

      <!-- Empty state -->
      {#if objectives.length === 0 && !showAddForm}
        <div style="border: 2px dashed var(--sempa-border); border-radius:12px; padding:40px; text-align:center;">
          <p class="text-sm" style="color: var(--sempa-text-dim);">No objectives yet.</p>
          <a href="/week/{weekStartDate}/plan"
             class="mt-2 inline-block hover:underline"
             style="color: var(--sempa-accent); font-size:14px;">
            Start the weekly planning ritual →
          </a>
        </div>
      {/if}

      <!-- Inline add form -->
      {#if showAddForm}
        <div style="border-radius:12px; border: 1px solid var(--sempa-accent-bg);
                    background: var(--sempa-accent-bg); padding:16px;">
          <input bind:this={addingInput}
                 bind:value={addingTitle}
                 onkeydown={(e) => { if (e.key === 'Enter') addObjective(); if (e.key === 'Escape') showAddForm = false; }}
                 onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
                 onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}
                 type="text"
                 placeholder="What do you want to accomplish this week?"
                 class="w-full rounded-lg px-3 py-2.5 text-sm outline-none"
                 style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);
                        color: var(--sempa-text);" />
          <div class="mt-2 flex gap-2">
            <button onclick={addObjective} disabled={!addingTitle.trim()}
                    class="rounded-lg px-4 py-1.5 text-xs font-medium disabled:opacity-40 transition-colors"
                    style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
              Add
            </button>
            <button onclick={() => showAddForm = false}
                    class="rounded-lg px-4 py-1.5 text-xs transition-colors"
                    style="color: var(--sempa-text-soft);">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </div>

    <!-- Unscheduled tasks for this week -->
    {#if unscheduled.length > 0}
      <div class="mt-4" style="border-radius:12px; border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <button class="flex w-full items-center justify-between px-4 py-3"
                onclick={() => showUnscheduled = !showUnscheduled}>
          <span class="text-sm font-semibold" style="color: var(--sempa-text);">Unscheduled this week</span>
          <div class="flex items-center gap-2">
            <span class="rounded-full px-2 py-0.5 text-xs font-medium"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
              {unscheduled.length}
            </span>
            <svg class="h-4 w-4 transition-transform {showUnscheduled ? 'rotate-180' : ''}"
                 fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"
                 style="color: var(--sempa-text-dim);">
              <path stroke-linecap="round" d="M19 9l-7 7-7-7"/>
            </svg>
          </div>
        </button>
        {#if showUnscheduled}
          <div class="space-y-1 px-4 pb-4 pt-1" style="border-top: 1px solid var(--sempa-border);">
            {#each unscheduled as t (t.id)}
              <div class="group flex items-center gap-3 rounded-lg px-1 py-2">
                <button onclick={() => toggleTask(t)}
                        class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                               {t.status === 'done' ? 'border-green-500 bg-green-500' : 'border-gray-300'}">
                  {#if t.status === 'done'}
                    <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <span class="flex-1 text-sm {t.status === 'done' ? 'line-through' : ''}"
                      style="color: {t.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                  {t.title}
                </span>
                <button onclick={() => scheduleToday(t)}
                        class="rounded-lg px-2 py-1 text-[11px] font-medium transition-all
                               {mobile.value ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}"
                        style="background: var(--sempa-accent-bg); color: var(--sempa-accent);"
                        title="Schedule to today">
                  Plan today
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</main>
