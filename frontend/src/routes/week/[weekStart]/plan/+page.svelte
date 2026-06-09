<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { appendPosition, formatWeekRange, offsetDate, weekStart as calcWeekStart } from '$lib/utils';

  let ws = $derived($page.params.weekStart ?? calcWeekStart(new Date().toISOString().split('T')[0]));

  let step = $state(1);
  let loading = $state(true);
  let saving  = $state(false);
  let copied  = $state(false);
  let error   = $state('');

  let objectives = $state<Objective[]>([]);
  let tasks      = $state<Task[]>([]);

  // Step 1: draft objective titles (mirrors objectives array; separate to allow optimistic edits)
  let draftTitles = $state<string[]>([]);
  let addingTitle = $state('');

  // Step 2: per-objective quick-add inputs
  let taskDrafts  = $state<Record<string, string>>({});
  let taskDays    = $state<Record<string, string>>({}); // per-obj planned_date
  let addingTask  = $state<Record<string, boolean>>({});

  // Day options for the current week (Mon–Sun)
  const weekDays = $derived.by(() => {
    const days: { label: string; value: string }[] = [];
    for (let i = 0; i < 7; i++) {
      const d = offsetDate(ws, i);
      days.push({
        value: d,
        label: new Date(d + 'T12:00:00').toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
      });
    }
    return days;
  });

  async function load() {
    loading = true;
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(ws),
        api.tasks.listByWeek(ws),
      ]);
      draftTitles = objectives.map(o => o.title);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  // ── Objective helpers ─────────────────────────────────────────────────────

  function objectiveTasks(id: string): Task[] {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled');
  }
  function doneTasks(id: string) {
    return objectiveTasks(id).filter(t => t.status === 'done');
  }
  function pct(id: string): number {
    const total = objectiveTasks(id).length;
    return total === 0 ? 0 : Math.round((doneTasks(id).length / total) * 100);
  }

  // ── Step 1: objectives ────────────────────────────────────────────────────

  async function addObjective() {
    const title = addingTitle.trim();
    if (!title) return;
    addingTitle = '';
    const pos = appendPosition(objectives.map(o => o.position));
    try {
      const obj = await api.objectives.create({ week_start: ws, title, position: pos });
      objectives = [...objectives, obj];
      draftTitles = [...draftTitles, obj.title];
    } catch {}
  }

  async function saveTitle(i: number) {
    const obj = objectives[i];
    const t = draftTitles[i]?.trim();
    if (!t || t === obj.title) return;
    try {
      const updated = await api.objectives.update(obj.id, { title: t });
      objectives = objectives.map(o => o.id === updated.id ? updated : o);
    } catch {}
  }

  async function deleteObjective(id: string) {
    objectives = objectives.filter(o => o.id !== id);
    draftTitles = draftTitles.filter((_, i) => objectives[i]?.id !== id);
    await api.objectives.delete(id).catch(() => {});
  }

  // ── Step 2: tasks per objective ────────────────────────────────────────────

  async function addTask(objId: string) {
    const title = taskDrafts[objId]?.trim();
    if (!title) return;
    addingTask = { ...addingTask, [objId]: true };
    taskDrafts = { ...taskDrafts, [objId]: '' };
    const pos = appendPosition(objectiveTasks(objId).map(t => t.position));
    const plannedDate = taskDays[objId] ?? ws; // default to Monday
    try {
      const t = await api.tasks.create({
        title,
        weekly_objective_id: objId,
        week_start: ws,
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

  // ── Markdown ───────────────────────────────────────────────────────────────

  function generateMarkdown(): string {
    const lines = [
      `# Week of ${formatWeekRange(ws)}`,
      '',
      '## Objectives',
      '',
    ];
    for (const obj of objectives) {
      const linked = objectiveTasks(obj.id);
      const p = pct(obj.id);
      const icon = obj.status === 'completed' ? '✅' : '🎯';
      lines.push(`### ${icon} ${obj.title}${linked.length ? ` — ${p}% complete` : ''}`);
      if (linked.length === 0) {
        lines.push('*No tasks yet*');
      } else {
        for (const t of linked) {
          lines.push(`- [${t.status === 'done' ? 'x' : ' '}] ${t.title}`);
        }
      }
      lines.push('');
    }
    if (objectives.length === 0) lines.push('*No objectives set*\n');
    lines.push('---');
    lines.push(`*Sempa weekly plan · ${new Date().toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}*`);
    return lines.join('\n');
  }

  async function copyMarkdown() {
    try {
      await navigator.clipboard.writeText(generateMarkdown());
      copied = true;
      setTimeout(() => copied = false, 2000);
    } catch {}
  }
</script>

<svelte:head><title>Plan week · {formatWeekRange(ws)} — Sempa</title></svelte:head>

<div class="mx-auto max-w-2xl px-6 py-10 animate-fade-in"
     style="padding-top: calc(env(safe-area-inset-top, 0px) + 40px);">

  <!-- Header -->
  <div class="mb-8 flex items-start justify-between">
    <div>
      <a href="/week/{ws}" class="text-xs" style="color: var(--sempa-text-dim);">
        ← Back to week
      </a>
      <h1 class="mt-2 text-xl font-bold" style="color: var(--sempa-text);">Plan your week</h1>
      <p class="text-sm" style="color: var(--sempa-text-soft);">{formatWeekRange(ws)}</p>
    </div>
    <!-- Step indicator pills -->
    <div class="flex items-center gap-1.5 pt-1">
      {#each [{ n: 1, label: 'Objectives' }, { n: 2, label: 'Tasks' }, { n: 3, label: 'Export' }] as s}
        <button onclick={() => step = s.n}
                class="transition-colors"
                style={step === s.n
                  ? 'background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:9999px; padding:4px 14px; font-size:12px; font-weight:500;'
                  : step > s.n
                    ? 'background: var(--sempa-success-soft); color: var(--sempa-success); border-radius:9999px; padding:4px 14px; font-size:12px; font-weight:500;'
                    : 'background: var(--sempa-accent-bg); color: var(--sempa-text-dim); border-radius:9999px; padding:4px 14px; font-size:12px;'}>
          {s.n < step ? '✓ ' : ''}{s.label}
        </button>
      {/each}
    </div>
  </div>

  {#if loading}
    <div class="flex h-40 items-center justify-center text-sm text-gray-400">Loading…</div>
  {:else if error}
    <p class="rounded-xl bg-red-50 p-4 text-sm text-red-600">{error}</p>

  <!-- ── Step 1: Objectives ─────────────────────────────────────────────── -->
  {:else if step === 1}
    <div class="space-y-4">
      <p class="text-sm" style="color: var(--sempa-text-soft);">
        What do you want to accomplish this week?
        Be specific enough to know when you've achieved each one.
      </p>

      <div class="space-y-2">
        {#each objectives as obj, i (obj.id)}
          <div class="flex items-center gap-2 rounded-xl px-3 py-2"
               style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
            <span class="select-none" style="color: var(--sempa-text-dim);">🎯</span>
            <input
              bind:value={draftTitles[i]}
              onblur={() => saveTitle(i)}
              onkeydown={(e) => { if (e.key === 'Enter') { saveTitle(i); addingTitle = ''; (document.getElementById('new-obj') as HTMLInputElement)?.focus(); } }}
              type="text"
              class="flex-1 bg-transparent text-sm font-medium outline-none"
              style="color: var(--sempa-text);"
            />
            <button onclick={() => deleteObjective(obj.id)} aria-label="Delete objective"
                    class="transition-colors"
                    style="color: var(--sempa-text-dim);"
                    onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = '#f87171'}
                    onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
        {/each}

        <!-- Add new objective input -->
        <div class="flex items-center gap-2 rounded-xl border border-dashed px-3 py-2
                    focus-within:border-[var(--a500)]"
             style="border-color: var(--sempa-border); background: var(--sempa-accent-bg);">
          <span class="select-none" style="color: var(--sempa-text-dim);">🎯</span>
          <input
            id="new-obj"
            bind:value={addingTitle}
            onkeydown={(e) => { if (e.key === 'Enter') addObjective(); }}
            type="text"
            placeholder="Add an objective… (press Enter)"
            class="flex-1 bg-transparent text-sm outline-none"
            style="color: var(--sempa-text);"
          />
          {#if addingTitle.trim()}
            <button onclick={addObjective}
                    class="text-xs" style="color: var(--sempa-accent);">Add</button>
          {/if}
        </div>
      </div>

      <div class="pt-2 flex justify-end">
        <button onclick={() => step = 2} disabled={objectives.length === 0}
                class="disabled:opacity-40 transition-colors"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                       padding:10px 24px; font-size:14px; font-weight:500; border:none; cursor:pointer;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          Next: add tasks →
        </button>
      </div>
    </div>

  <!-- ── Step 2: Tasks per objective ────────────────────────────────────── -->
  {:else if step === 2}
    <div class="space-y-4">
      <p class="text-sm" style="color: var(--sempa-text-soft);">
        Break each objective into <strong style="color: var(--sempa-text);">concrete tasks</strong>.
        Each task counts equally toward the objective's progress.
      </p>

      {#each objectives as obj (obj.id)}
        {@const linked = objectiveTasks(obj.id)}
        {@const done   = doneTasks(obj.id)}
        {@const p = pct(obj.id)}
        <div style="border-radius:12px; border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <!-- Objective header -->
          <div class="flex items-center gap-3 px-4 py-3" style="border-bottom: 1px solid var(--sempa-border);">
            <div class="flex-1">
              <p class="text-sm font-semibold" style="color: var(--sempa-text);">🎯 {obj.title}</p>
              {#if linked.length > 0}
                <div class="mt-1.5 flex items-center gap-2">
                  <div class="h-1.5 w-24 overflow-hidden rounded-full" style="background: var(--sempa-border);">
                    <div style="width:{p}%; height:100%; border-radius:3px;
                                background: var(--sempa-accent); transition: width 500ms ease-out;"></div>
                  </div>
                  <span class="text-xs" style="color: var(--sempa-text-dim);">{done.length}/{linked.length} · {p}%</span>
                </div>
              {/if}
            </div>
          </div>

          <!-- Task list -->
          <div class="px-4 py-2 space-y-1">
            {#each linked as t (t.id)}
              <div class="flex items-center gap-2.5 py-1">
                <button onclick={() => toggleTask(t)}
                        class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                               {t.status === 'done' ? 'border-green-500 bg-green-500' : 'border-gray-300'}"
                        onmouseenter={(e) => { if (t.status !== 'done') (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-success)'; }}
                        onmouseleave={(e) => { if (t.status !== 'done') (e.currentTarget as HTMLElement).style.borderColor = ''; }}>
                  {#if t.status === 'done'}
                    <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <span class="text-sm {t.status === 'done' ? 'line-through' : ''}"
                      style="color: {t.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                  {t.title}
                </span>
                {#if linked.length > 0}
                  <span class="ml-auto text-[10.5px] shrink-0" style="color: var(--sempa-text-dim);">
                    {Math.round(100 / linked.length)}%
                  </span>
                {/if}
              </div>
            {/each}

            <!-- Quick-add -->
            <div class="space-y-1.5">
              <div class="flex items-center gap-2 rounded-lg border border-dashed px-2 py-1.5
                          focus-within:border-[var(--a500)]"
                   style="border-color: var(--sempa-border);">
                <svg class="h-3.5 w-3.5 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
                </svg>
                <input
                  bind:value={taskDrafts[obj.id]}
                  onkeydown={(e) => { if (e.key === 'Enter') addTask(obj.id); }}
                  type="text"
                  placeholder="Add a task… (Enter to save)"
                  class="flex-1 bg-transparent text-xs outline-none"
                  style="color: var(--sempa-text);"
                />
              </div>
              {#if taskDrafts[obj.id]?.trim()}
                <div class="flex items-center gap-2 pl-1">
                  <select bind:value={taskDays[obj.id]}
                          class="rounded-md px-2 py-1 text-xs outline-none"
                          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);
                                 color: var(--sempa-text-soft);">
                    {#each weekDays as d}
                      <option value={d.value}>{d.label}</option>
                    {/each}
                  </select>
                  <button onclick={() => addTask(obj.id)} disabled={addingTask[obj.id]}
                          class="rounded-md bg-[var(--a500)] px-3 py-1 text-xs font-medium text-white
                                 hover:bg-[var(--a600)] disabled:opacity-40 transition-colors">
                    Add task
                  </button>
                </div>
              {/if}
            </div>
          </div>
        </div>
      {/each}

      <div class="pt-2 flex gap-2 justify-end">
        <button onclick={() => step = 1}
                class="transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: transparent; border-radius: 9px; padding: 7px 14px; font-size: 14px;">
          ← Back
        </button>
        <button onclick={() => step = 3}
                class="transition-colors"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                       padding:10px 24px; font-size:14px; font-weight:500; border:none; cursor:pointer;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          Preview & export →
        </button>
      </div>
    </div>

  <!-- ── Step 3: Export ──────────────────────────────────────────────────── -->
  {:else if step === 3}
    <div class="space-y-4">
      <p class="text-sm" style="color: var(--sempa-text-soft);">
        Your week is set. Copy this as markdown to paste into any doc or report.
      </p>

      <!-- Markdown preview -->
      <div class="relative">
        <pre class="text-xs leading-relaxed"
             style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);
                    border-radius:12px; padding:16px; overflow-x:auto; color: var(--sempa-text);">{generateMarkdown()}</pre>
        <button onclick={copyMarkdown}
                class="absolute right-3 top-3 flex items-center gap-1.5 font-medium transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: var(--sempa-bg-panel); border-radius: 9px; padding: 7px 14px; font-size: 12px;">
          {#if copied}
            <svg class="h-3.5 w-3.5 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
            Copied!
          {:else}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
            Copy markdown
          {/if}
        </button>
      </div>

      <div class="flex gap-2 justify-end">
        <button onclick={() => step = 2}
                class="transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: transparent; border-radius: 9px; padding: 7px 14px; font-size: 14px;">
          ← Back
        </button>
        <button onclick={() => goto(`/week/${ws}`)}
                class="transition-colors"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                       padding:10px 24px; font-size:14px; font-weight:500; border:none; cursor:pointer;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          Start the week →
        </button>
      </div>
    </div>
  {/if}
</div>
