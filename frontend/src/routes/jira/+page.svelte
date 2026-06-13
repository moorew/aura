<script lang="ts">
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { today, weekStart, offsetDate } from '$lib/utils';
  import {
    parseJiraMeta,
    applyJiraFilters,
    JIRA_TOGGLE_DEFS,
    JIRA_SELECT_DEFS,
    optionsFor,
    activeSelectCount,
    defaultJiraFilterState,
  } from '$lib/jira/filters';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import {
    ChevronLeft, Search, X, RefreshCw, SlidersHorizontal,
    CalendarPlus, CirclePlus, ExternalLink, SquareKanban,
  } from 'lucide-svelte';

  type SempaScope = 'unplanned' | 'planned' | 'all';

  let allTasks    = $state<Task[]>([]);
  let loading     = $state(true);
  let syncing     = $state(false);
  let connected   = $state(true);
  let error       = $state('');
  let scope       = $state<SempaScope>('unplanned');
  let showFilters = $state(false);

  let filterState = $state(defaultJiraFilterState());

  // Detail / plan sheet
  let sheetTask  = $state<Task | null>(null);
  let pickDate   = $state(today());
  let planning   = $state(false);

  const todayDate = today();

  $effect(() => { loadAll(); });

  async function loadAll() {
    loading = true; error = '';
    try {
      const cfg = await api.integrations.jira.get() as any;
      connected = cfg.connected ?? false;
      if (!connected) { loading = false; return; }
      allTasks = await api.tasks.listBySource('jira');
    } catch (e: any) {
      error = e.message ?? 'Failed to load Jira issues';
    } finally { loading = false; }
  }

  async function sync() {
    syncing = true; error = '';
    try {
      await api.integrations.jira.sync();
      await loadAll();
    } catch (e: any) { error = e.message ?? 'Sync failed'; }
    finally { syncing = false; }
  }

  const filteredTasks = $derived.by(() => {
    const scoped = allTasks.filter(t => {
      if (scope === 'unplanned' && t.status !== 'backlog') return false;
      if (scope === 'planned' && (t.status === 'backlog' || t.status === 'done')) return false;
      return true;
    });
    return applyJiraFilters(scoped, filterState);
  });

  const activeFilters = $derived(activeSelectCount(filterState));

  function openSheet(task: Task) {
    sheetTask = task;
    pickDate = task.planned_date && task.planned_date >= todayDate ? task.planned_date : todayDate;
  }

  // Plan an issue onto a date — identical mutation to the desktop drag-to-plan,
  // so the Jira-sourced task lands on the board exactly the same way.
  async function planFor(date: string) {
    if (!sheetTask) return;
    planning = true;
    const id = sheetTask.id;
    const prevStatus = sheetTask.status;
    try {
      const updated = await api.tasks.update(id, {
        planned_date: date,
        week_start: weekStart(date),
        status: prevStatus === 'backlog' ? 'planned' : prevStatus,
      });
      allTasks = allTasks.map(t => t.id === id ? updated : t);
      sheetTask = null;
    } catch (e: any) {
      error = e.message ?? 'Could not plan issue';
    } finally { planning = false; }
  }

  function priorityDot(priority?: string): string {
    if (priority === 'Highest' || priority === 'High') return 'bg-red-500';
    if (priority === 'Medium') return 'bg-yellow-400';
    if (priority === 'Low' || priority === 'Lowest') return 'bg-blue-400';
    return 'bg-gray-300 dark:bg-gray-600';
  }

  function plannedLabel(t: Task): string {
    if (!t.planned_date) return '';
    if (t.planned_date === todayDate) return 'Today';
    if (t.planned_date === offsetDate(todayDate, 1)) return 'Tomorrow';
    return new Date(t.planned_date + 'T00:00:00').toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
  }
</script>

<div class="flex h-full flex-col" style="background: var(--sempa-bg-main);">

  <!-- Header -->
  <header class="sticky top-0 z-[40] px-4 pt-4 pb-3"
          style="background: var(--sempa-bg-main); padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px)); border-bottom: 1px solid var(--sempa-border);">
    <div class="flex items-center gap-2">
      <button onclick={() => history.back()} aria-label="Back"
              class="flex h-9 w-9 items-center justify-center rounded-xl transition-colors active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronLeft size={20} />
      </button>
      <h1 class="flex items-center gap-2" style="font-size: 22px; font-weight: 600; letter-spacing: -0.02em; color: var(--sempa-text);">
        <SquareKanban size={20} /> Jira
      </h1>
      <button onclick={sync} disabled={syncing || loading} aria-label="Sync Jira"
              class="ml-auto flex h-9 w-9 items-center justify-center rounded-xl transition-colors active:bg-gray-100 dark:active:bg-gray-800 disabled:opacity-40"
              style="color: var(--sempa-text-dim);">
        <RefreshCw size={17} class={syncing ? 'animate-spin' : ''} />
      </button>
    </div>

    {#if connected}
      <!-- Search -->
      <div class="relative mt-3">
        <Search size={15} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2" style="color: var(--sempa-text-dim);" />
        <input bind:value={filterState.query} type="text" placeholder="Search key or title…"
               class="w-full rounded-xl border py-2 pl-9 pr-9 text-sm"
               style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
        {#if filterState.query}
          <button onclick={() => filterState.query = ''} aria-label="Clear search"
                  class="absolute right-2.5 top-1/2 -translate-y-1/2" style="color: var(--sempa-text-dim);">
            <X size={15} />
          </button>
        {/if}
      </div>

      <!-- Scope -->
      <div class="mt-2 flex gap-1">
        {#each [['unplanned', 'Unplanned'], ['planned', 'Planned'], ['all', 'All']] as [val, label]}
          <button onclick={() => scope = val as SempaScope}
                  class="flex-1 rounded-lg py-1.5 text-xs font-medium transition-colors"
                  style={scope === val
                    ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                    : 'color: var(--sempa-text-dim);'}>
            {label}
          </button>
        {/each}
      </div>

      <!-- Toggles + filters disclosure -->
      <div class="mt-2 flex flex-wrap items-center gap-1.5">
        {#each JIRA_TOGGLE_DEFS as def (def.id)}
          {@const on = filterState.toggles[def.id]}
          <button onclick={() => filterState.toggles[def.id] = !on} aria-pressed={on}
                  class="rounded-full border px-3 py-1 text-xs font-medium transition-colors"
                  style={on
                    ? 'border-color: transparent; background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                    : 'border-color: var(--sempa-border); color: var(--sempa-text-dim);'}>
            {def.label}
          </button>
        {/each}
        <button onclick={() => showFilters = !showFilters}
                class="ml-auto flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-medium transition-colors"
                style={activeFilters > 0
                  ? 'border-color: transparent; background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                  : 'border-color: var(--sempa-border); color: var(--sempa-text-dim);'}>
          <SlidersHorizontal size={13} />
          {activeFilters > 0 ? `Filters (${activeFilters})` : 'Filters'}
        </button>
      </div>

      {#if showFilters}
        <div class="mt-2 space-y-1.5">
          {#each JIRA_SELECT_DEFS as def (def.id)}
            {@const opts = optionsFor(def, allTasks)}
            {#if opts.length}
              <select bind:value={filterState.selects[def.id]}
                      class="w-full rounded-lg border px-3 py-2 text-sm"
                      style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);">
                <option value="">{def.label}: any</option>
                {#each opts as o}<option value={o}>{o}</option>{/each}
              </select>
            {/if}
          {/each}
        </div>
      {/if}
    {/if}
  </header>

  <!-- List -->
  <div class="flex-1 overflow-y-auto" data-sheet-scroll>
    {#if !connected && !loading}
      <div class="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
        <SquareKanban size={28} style="color: var(--sempa-text-dim);" />
        <p style="color: var(--sempa-text-dim);">Jira isn't connected</p>
        <a href="/settings/integrations/jira" class="text-sm text-blue-500">Set up Jira →</a>
      </div>

    {:else if loading}
      <div class="space-y-px">
        {#each Array(6) as _}
          <div class="flex items-start gap-3 px-4 py-3.5">
            <div class="h-4 w-14 shrink-0 rounded mt-0.5 animate-pulse" style="background: var(--sempa-border);"></div>
            <div class="flex-1 space-y-2">
              <div class="h-3 w-full rounded animate-pulse" style="background: var(--sempa-border);"></div>
              <div class="h-2.5 w-1/2 rounded animate-pulse" style="background: var(--sempa-border);"></div>
            </div>
          </div>
        {/each}
      </div>

    {:else if error}
      <p class="px-4 py-6 text-sm text-red-500">{error}</p>

    {:else if filteredTasks.length === 0}
      <div class="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
        <p style="color: var(--sempa-text-dim);">No issues match</p>
        {#if allTasks.length === 0}
          <button onclick={sync} class="text-sm text-blue-500">Sync now</button>
        {/if}
      </div>

    {:else}
      <ul>
        {#each filteredTasks as task (task.id)}
          {@const meta = parseJiraMeta(task.source_metadata)}
          <li>
            <button onclick={() => openSheet(task)}
                    class="flex w-full items-start gap-3 px-4 py-3.5 text-left transition-colors active:bg-gray-50 dark:active:bg-gray-800/40"
                    style="border-bottom: 1px solid var(--sempa-border);">
              <div class="min-w-0 flex-1">
                <div class="flex items-baseline gap-2">
                  {#if meta?.key}
                    <span class="shrink-0 font-mono text-xs font-semibold text-blue-500 dark:text-blue-400">{meta.key}</span>
                  {/if}
                  <span class="truncate text-sm" style="color: var(--sempa-text);">{task.title}</span>
                </div>
                <div class="mt-1 flex items-center gap-1.5 text-xs" style="color: var(--sempa-text-dim);">
                  {#if meta?.priority}
                    <span class="h-1.5 w-1.5 shrink-0 rounded-full {priorityDot(meta.priority)}"></span>
                  {/if}
                  {#if meta?.status}<span class="truncate">{meta.status}</span>{/if}
                  {#if meta?.issueType}<span style="color: var(--sempa-border);">·</span><span class="truncate">{meta.issueType}</span>{/if}
                </div>
              </div>
              {#if task.planned_date && task.status !== 'backlog'}
                <span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium"
                      style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                  {plannedLabel(task)}
                </span>
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>

<!-- Detail + plan sheet -->
<BottomSheet open={sheetTask !== null} onClose={() => { if (!planning) sheetTask = null; }}>
  {#if sheetTask}
    {@const meta = parseJiraMeta(sheetTask.source_metadata)}
    <div class="px-5 pb-6 pt-1" data-sheet-scroll>

      <!-- Plan actions FIRST (anchored at top for thumb reach) -->
      <div class="space-y-2">
        {#if sheetTask.planned_date && sheetTask.status !== 'backlog'}
          <p class="text-xs" style="color: var(--sempa-text-dim);">
            Planned for <span style="color: var(--sempa-accent);">{plannedLabel(sheetTask)}</span> · choose a new day below
          </p>
        {/if}
        <div class="flex gap-2">
          <button onclick={() => planFor(todayDate)} disabled={planning}
                  class="flex flex-1 items-center justify-center gap-1.5 rounded-xl py-2.5 text-sm font-semibold transition-colors disabled:opacity-50"
                  style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
            <CirclePlus size={16} /> {planning ? 'Adding…' : 'Add to today'}
          </button>
        </div>
        <div class="flex items-center gap-2">
          <span class="flex items-center gap-1.5 text-xs" style="color: var(--sempa-text-dim);">
            <CalendarPlus size={15} /> Pick a day
          </span>
          <input type="date" bind:value={pickDate} min={todayDate}
                 class="flex-1 rounded-xl border px-3 py-2 text-sm"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
          <button onclick={() => planFor(pickDate)} disabled={planning || !pickDate}
                  class="rounded-xl px-4 py-2 text-sm font-semibold transition-colors disabled:opacity-50"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
            Plan
          </button>
        </div>
      </div>

      <!-- Issue summary -->
      <div class="mt-5 border-t pt-4" style="border-color: var(--sempa-border);">
        <div class="flex items-center gap-2">
          {#if meta?.key}<span class="font-mono text-sm font-semibold text-blue-500">{meta.key}</span>{/if}
          {#if sheetTask.source_url}
            <a href={sheetTask.source_url} target="_blank" rel="noopener noreferrer"
               class="ml-auto flex items-center gap-1 text-xs text-blue-500">Open in Jira <ExternalLink size={12} /></a>
          {/if}
        </div>
        <h2 class="mt-1.5 text-base font-medium leading-snug" style="color: var(--sempa-text);">{sheetTask.title}</h2>
        <div class="mt-2.5 flex flex-wrap items-center gap-2 text-xs">
          {#if meta?.status}
            <span class="rounded-md px-2 py-0.5 font-medium" style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">{meta.status}</span>
          {/if}
          {#if meta?.priority}
            <span class="flex items-center gap-1" style="color: var(--sempa-text-dim);">
              <span class="h-2 w-2 rounded-full {priorityDot(meta.priority)}"></span>{meta.priority}
            </span>
          {/if}
          {#if meta?.issueType}<span style="color: var(--sempa-text-dim);">{meta.issueType}</span>{/if}
          {#if meta?.assignee}<span style="color: var(--sempa-text-dim);">· {meta.assignee}</span>{/if}
        </div>
        {#if meta?.epicName || meta?.epicKey}
          <p class="mt-2 text-xs" style="color: var(--sempa-text-dim);">Epic: {meta.epicName || meta.epicKey}</p>
        {/if}
      </div>
    </div>
  {/if}
</BottomSheet>
