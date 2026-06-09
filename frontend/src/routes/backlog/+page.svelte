<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { today, weekStart, formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import { Plus, Search } from 'lucide-svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { realtime } from '$lib/stores/realtime.svelte';

  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  // Controls
  let search   = $state('');
  let filter   = $state<'all' | 'jira' | 'personal'>('all');
  let sortNewest = $state(false); // default: oldest first

  onMount(load);

  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'task:change') void load();
  });

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

  // Park the item in this week without pinning a day: assign the current
  // week_start (so it shows under "Unscheduled this week") but leave it
  // undated, distinct from "Today" which pins planned_date to today.
  async function scheduleThisWeek(id: string) {
    const ws = weekStart(today());
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.update(id, { week_start: ws, status: 'planned', planned_date: null }); }
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

  // Age since the task was added, in compact d/w units (e.g. 5d, 3w).
  function ageLabel(createdAt: string | null | undefined): string {
    if (!createdAt) return '';
    const then = new Date(createdAt).getTime();
    if (Number.isNaN(then)) return '';
    const days = Math.max(0, Math.floor((Date.now() - then) / 86400000));
    if (days < 1) return 'today';
    if (days < 7) return `${days}d`;
    return `${Math.floor(days / 7)}w`;
  }

  // The group a task belongs to: its source, or "personal" for manual tasks.
  function groupKey(t: Task): string {
    return t.source && t.source !== 'manual' ? t.source : 'personal';
  }
  function groupTitle(key: string): string {
    return key === 'personal' ? 'Personal' : (sourceLabel[key] ?? key);
  }

  // Oldest item's age, for the subtitle ("oldest 8w").
  const oldestAge = $derived.by(() => {
    const stamps = tasks.map(t => t.created_at ? new Date(t.created_at).getTime() : NaN).filter(n => !Number.isNaN(n));
    if (!stamps.length) return '';
    return ageLabel(new Date(Math.min(...stamps)).toISOString());
  });

  // Filter + search, then group by source, then sort within each group by age.
  const filtered = $derived.by(() => {
    const q = search.trim().toLowerCase();
    return tasks.filter(t => {
      if (q && !t.title.toLowerCase().includes(q)) return false;
      if (filter === 'jira') return t.source === 'jira';
      if (filter === 'personal') return groupKey(t) === 'personal';
      return true;
    });
  });

  const groups = $derived.by(() => {
    const byKey = new Map<string, Task[]>();
    for (const t of filtered) {
      const k = groupKey(t);
      if (!byKey.has(k)) byKey.set(k, []);
      byKey.get(k)!.push(t);
    }
    const dir = sortNewest ? -1 : 1;
    const sortByAge = (a: Task, b: Task) =>
      dir * (a.created_at ?? '').localeCompare(b.created_at ?? '');
    // Stable group order: known sources first (jira), then personal, then others.
    const order = (k: string) => (k === 'jira' ? 0 : k === 'personal' ? 2 : 1);
    return [...byKey.entries()]
      .map(([key, items]) => ({ key, items: [...items].sort(sortByAge) }))
      .sort((a, b) => order(a.key) - order(b.key) || a.key.localeCompare(b.key));
  });
</script>

<svelte:head><title>Backlog — Sempa</title></svelte:head>

<header class="sticky top-0 z-[40] backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               border-bottom: 1px solid var(--sempa-border);
               padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
  <div class="flex flex-wrap items-center justify-between gap-3 px-6 py-3">
    <div>
      <p class="type-page" style="color: var(--sempa-text);">Backlog</p>
      <p style="font-size: 12.5px; color: var(--sempa-text-dim);">
        {tasks.length} item{tasks.length !== 1 ? 's' : ''} waiting to be scheduled{oldestAge ? ` · oldest ${oldestAge}` : ''}
      </p>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <!-- Search -->
      <div class="flex items-center gap-1.5 rounded-lg px-2.5"
           style="border: 1px solid var(--sempa-border); height: 32px;">
        <Search size={13} style="color: var(--sempa-text-dim);" />
        <input bind:value={search} placeholder="Search backlog"
               class="bg-transparent outline-none"
               style="font-size: 12.5px; color: var(--sempa-text); width: 130px;" />
      </div>

      <!-- Filter chips -->
      <div class="flex items-center gap-1">
        {#each [{ k: 'all', l: 'All' }, { k: 'jira', l: 'Jira' }, { k: 'personal', l: 'Personal' }] as chip}
          {@const active = filter === chip.k}
          <button onclick={() => filter = chip.k as typeof filter}
                  class="rounded-lg transition-colors"
                  style="font-size: 12px; padding: 4px 10px;
                         {active
                           ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                           : 'border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);'}">
            {chip.l}
          </button>
        {/each}
      </div>

      <!-- Sort -->
      <button onclick={() => sortNewest = !sortNewest}
              class="rounded-lg transition-colors"
              style="font-size: 12px; padding: 4px 10px; border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
        {sortNewest ? 'Newest first' : 'Oldest first'}
      </button>

      <!-- Add -->
      <button onclick={openCreate}
              class="flex items-center gap-1.5 rounded-[9px] px-3 transition-colors shadow-sm"
              style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); height: 32px; font-size: 13px; font-weight: 500;"
              onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
              onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
        <Plus size={13} strokeWidth={2.5} />
        Add to backlog
      </button>
    </div>
  </div>
</header>

<main class="mx-auto max-w-3xl px-6 py-6 animate-fade-in">
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

  {:else if filtered.length === 0}
    <div class="flex flex-col items-center justify-center py-20 text-center" style="color: var(--sempa-text-dim);">
      <p class="text-sm">No matching items.</p>
    </div>

  {:else}
    <div class="flex flex-col gap-6">
      {#each groups as group (group.key)}
        <div>
          <!-- Group label -->
          <div class="mb-2 flex items-center gap-2 px-1">
            <span class="type-label" style="color: var(--sempa-accent);">{groupTitle(group.key)}</span>
            <span class="type-label" style="color: var(--sempa-text-dim);">{group.items.length}</span>
          </div>

          <!-- One bordered panel; rows divided by a hairline -->
          <div style="border: 1px solid var(--sempa-border); border-radius: 12px; background: var(--sempa-bg-panel); overflow: hidden;">
            {#each group.items as task, i (task.id)}
              <div class="group/row flex items-center gap-3 px-4 transition-colors"
                   style="min-height: {mobile.value ? '44px' : '38px'}; {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                   onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'color-mix(in srgb, var(--sempa-text) 3%, transparent)'}
                   onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = ''}>

                <!-- Complete circle -->
                <button onclick={() => complete(task.id)}
                        class="h-[15px] w-[15px] shrink-0 rounded-full border-2 transition-all cursor-pointer"
                        style="border-color: var(--sempa-text-dim);"
                        onmouseenter={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-success)'}
                        onmouseleave={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-text-dim)'}
                        title="Mark done"></button>

                <!-- Title (single line, truncates) -->
                <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
                <button class="min-w-0 flex-1 truncate text-left type-body cursor-pointer"
                        style="color: var(--sempa-text);" onclick={() => openEdit(task)}>
                  {task.title}
                </button>

                <!-- Right side: estimate + age, swapped for quick actions on hover -->
                <div class="flex shrink-0 items-center gap-2">
                  <!-- meta (hidden on row hover, desktop only) -->
                  <div class="flex items-center gap-2 {mobile.value ? '' : 'group-hover/row:hidden'}">
                    {#if task.time_estimate_minutes}
                      <span class="type-badge rounded" style="padding: 2px 7px; background: color-mix(in srgb, var(--sempa-text) 6%, transparent); color: var(--sempa-text-soft);">
                        {formatMinutes(task.time_estimate_minutes)}
                      </span>
                    {/if}
                    <span style="font-size: 11.5px; color: var(--sempa-text-dim);">{ageLabel(task.created_at)}</span>
                  </div>

                  <!-- quick actions (desktop: hover only; mobile: always) -->
                  <div class="items-center gap-1.5 {mobile.value ? 'flex' : 'hidden group-hover/row:flex'}">
                    <button onclick={() => scheduleToday(task.id)}
                            class="rounded-md transition-colors"
                            style="font-size: 11.5px; font-weight: 600; padding: 2px 8px;
                                   border: 1px solid var(--sempa-accent); color: var(--sempa-accent);">
                      Today
                    </button>
                    <button onclick={() => scheduleThisWeek(task.id)}
                            class="rounded-md transition-colors"
                            style="font-size: 11.5px; font-weight: 600; padding: 2px 8px;
                                   border: 1px solid var(--sempa-accent); color: var(--sempa-accent);">
                      This week
                    </button>
                    <button onclick={() => remove(task.id)}
                            class="rounded p-1 transition-colors"
                            style="color: var(--sempa-text-dim);"
                            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = '#f87171'}
                            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}
                            title="Delete" aria-label="Delete">
                      <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                        <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</main>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus="backlog" defaultDate={today()}
           onSave={handlePanelSave} onClose={() => panelOpen = false} />
