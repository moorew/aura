<script lang="ts">
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { mobile } from '$lib/stores/mobile.svelte';

  let {
    onTaskDragStart,
    onTasksReloaded,
  }: {
    onTaskDragStart?: (id: string) => void;
    onTasksReloaded?: () => void;
  } = $props();

  type SempaFilter = 'unplanned' | 'planned' | 'all';
  type View = 'list' | 'card';

  let allTasks      = $state<Task[]>([]);
  let allStatuses   = $state<{ id: string; name: string; statusCategory: { key: string } }[]>([]);
  let loading       = $state(true);
  let syncing       = $state(false);
  let connected     = $state(true);
  let error         = $state('');
  let sempaFilter   = $state<SempaFilter>('unplanned');
  let statusFilter  = $state('');
  let view          = $state<View>('list');

  // Card view state
  let cardTask      = $state<Task | null>(null);
  let cardDetail    = $state<any>(null);
  let cardLoading   = $state(false);
  let cardError     = $state('');
  let cardTransitions = $state<{ id: string; name: string }[]>([]);
  let transitioning   = $state(false);

  $effect(() => { loadAll(); });

  async function loadAll() {
    loading = true; error = '';
    try {
      const cfg = await api.integrations.jira.get() as any;
      connected = cfg.connected ?? false;
      if (!connected) { loading = false; return; }
      const [tasks, statuses] = await Promise.all([
        api.tasks.listBySource('jira'),
        api.integrations.jira.getStatuses().catch(() => [] as any[]),
      ]);
      allTasks = tasks;
      allStatuses = statuses;
    } catch (e: any) {
      error = e.message ?? 'Failed';
    } finally { loading = false; }
  }

  async function sync() {
    syncing = true; error = '';
    try {
      await api.integrations.jira.sync();
      await loadAll();
      onTasksReloaded?.();
    } catch (e: any) { error = e.message ?? 'Sync failed'; }
    finally { syncing = false; }
  }

  const filteredTasks = $derived.by(() => {
    return allTasks.filter(t => {
      if (sempaFilter === 'unplanned' && t.status !== 'backlog') return false;
      if (sempaFilter === 'planned' && (t.status === 'backlog' || t.status === 'done')) return false;
      if (statusFilter) {
        const m = parseMeta(t.source_metadata);
        if (m?.status !== statusFilter) return false;
      }
      return true;
    });
  });

  async function openCard(task: Task) {
    const meta = parseMeta(task.source_metadata);
    if (!meta?.key) return;
    cardTask = task;
    cardDetail = null;
    cardError = '';
    cardTransitions = [];
    view = 'card';
    cardLoading = true;
    try {
      const [detail, transitions] = await Promise.all([
        api.integrations.jira.getIssue(meta.key),
        api.integrations.jira.getTransitions(meta.key),
      ]);
      cardDetail = detail;
      cardTransitions = transitions;
    } catch (e: any) {
      cardError = e.message ?? 'Failed to load issue';
    } finally { cardLoading = false; }
  }

  async function doTransition(transitionId: string) {
    const meta = parseMeta(cardTask?.source_metadata ?? null);
    if (!meta?.key || !cardTask) return;
    transitioning = true;
    try {
      await api.integrations.jira.transition(meta.key, transitionId);
      const t = cardTransitions.find(t => t.id === transitionId);
      if (t && cardDetail) {
        cardDetail = { ...cardDetail, fields: { ...cardDetail.fields, status: { ...cardDetail.fields.status, name: t.name } } };
        allTasks = allTasks.map(tk => {
          if (tk.id !== cardTask!.id) return tk;
          const m = parseMeta(tk.source_metadata) ?? {};
          m.status = t.name;
          return { ...tk, source_metadata: JSON.stringify(m) };
        });
      }
    } catch (e: any) { cardError = e.message ?? 'Transition failed'; }
    finally { transitioning = false; }
  }

  function parseMeta(raw: string | null) {
    if (!raw) return null;
    try { return JSON.parse(raw); } catch { return null; }
  }

  function priorityDot(priority: string): string {
    if (priority === 'Highest' || priority === 'High') return 'bg-red-500';
    if (priority === 'Medium') return 'bg-yellow-400';
    if (priority === 'Low' || priority === 'Lowest') return 'bg-blue-400';
    return 'bg-gray-300 dark:bg-gray-600';
  }

  function sempaLabel(status: string) {
    return { backlog: 'Unplanned', planned: 'Planned', in_progress: 'In progress', done: 'Done' }[status] ?? status;
  }

  function fmtDate(iso: string) {
    if (!iso) return '';
    return new Date(iso).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
  }

  function onDragStart(e: DragEvent, task: Task) {
    onTaskDragStart?.(task.id);
    e.dataTransfer!.effectAllowed = 'move';
  }
</script>

<div class="flex h-full flex-col text-xs">

  {#if view === 'card' && cardTask}
    <!-- ── Card view ──────────────────────────────────────────────────────── -->
    {@const meta = parseMeta(cardTask.source_metadata)}

    <!-- Card header -->
    <div class="flex shrink-0 items-center gap-2 px-3 py-2"
         style="border-bottom: 1px solid var(--sempa-border);">
      <button onclick={() => { view = 'list'; cardTask = null; cardDetail = null; }}
              class="transition-colors" style="color: var(--sempa-text-dim);"
              aria-label="Back">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <span class="font-mono font-semibold text-blue-500">{meta?.key ?? ''}</span>
      {#if cardTask.source_url}
        <a href={cardTask.source_url} target="_blank" rel="noopener noreferrer"
           class="ml-auto shrink-0 text-blue-500 hover:underline">Open ↗</a>
      {/if}
    </div>

    <div class="flex-1 overflow-y-auto">
      {#if cardLoading}
        <div class="space-y-3 p-4 animate-pulse">
          {#each Array(5) as _}
            <div class="h-3 rounded" style="background: var(--sempa-border);"></div>
          {/each}
        </div>
      {:else if cardError}
        <p class="p-4 text-red-500">{cardError}</p>
      {:else if cardDetail}
        {@const f = cardDetail.fields}
        <div class="divide-y" style="--d: var(--sempa-border);">

          <!-- Title + status -->
          <div class="p-4 space-y-2">
            <h3 class="text-sm font-medium leading-snug" style="color: var(--sempa-text);">{f.summary}</h3>
            <div class="flex flex-wrap items-center gap-2">
              <span class="rounded px-2 py-0.5 font-medium"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                {f.status?.name}
              </span>
              {#if f.priority?.name}
                <span class="flex items-center gap-1" style="color: var(--sempa-text-dim);">
                  <span class="h-2 w-2 rounded-full {priorityDot(f.priority.name)}"></span>
                  {f.priority.name}
                </span>
              {/if}
              {#if f.issuetype?.name}
                <span style="color: var(--sempa-text-dim);">{f.issuetype.name}</span>
              {/if}
            </div>
          </div>

          <!-- People -->
          <div class="grid grid-cols-2 gap-x-4 gap-y-2 p-4">
            {#if f.assignee}
              <div>
                <p style="color: var(--sempa-text-dim);">Assignee</p>
                <p style="color: var(--sempa-text);">{f.assignee.displayName}</p>
              </div>
            {/if}
            {#if f.reporter}
              <div>
                <p style="color: var(--sempa-text-dim);">Reporter</p>
                <p style="color: var(--sempa-text);">{f.reporter.displayName}</p>
              </div>
            {/if}
            <div>
              <p style="color: var(--sempa-text-dim);">Created</p>
              <p style="color: var(--sempa-text);">{fmtDate(f.created)}</p>
            </div>
            <div>
              <p style="color: var(--sempa-text-dim);">Updated</p>
              <p style="color: var(--sempa-text);">{fmtDate(f.updated)}</p>
            </div>
          </div>

          <!-- Labels -->
          {#if f.labels?.length}
            <div class="p-4">
              <p class="mb-1.5" style="color: var(--sempa-text-dim);">Labels</p>
              <div class="flex flex-wrap gap-1">
                {#each f.labels as label}
                  <span class="rounded px-2 py-0.5"
                        style="background: var(--sempa-border); color: var(--sempa-text);">{label}</span>
                {/each}
              </div>
            </div>
          {/if}

          <!-- Description -->
          {#if f.description}
            <div class="p-4">
              <p class="mb-1.5" style="color: var(--sempa-text-dim);">Description</p>
              <p class="whitespace-pre-wrap leading-relaxed" style="color: var(--sempa-text);">
                {f.description}
              </p>
            </div>
          {/if}

          <!-- Sempa status -->
          <div class="p-4">
            <p class="mb-1.5" style="color: var(--sempa-text-dim);">In Sempa</p>
            <p style="color: var(--sempa-text);">{sempaLabel(cardTask.status)}</p>
          </div>

          <!-- Transitions -->
          {#if cardTransitions.length}
            <div class="p-4">
              <p class="mb-2" style="color: var(--sempa-text-dim);">Move to</p>
              {#if cardError && transitioning === false}
                <p class="mb-2 text-red-500">{cardError}</p>
              {/if}
              <div class="flex flex-wrap gap-1.5">
                {#each cardTransitions as t}
                  <button onclick={() => doTransition(t.id)}
                          disabled={transitioning}
                          class="rounded-md px-2.5 py-1 font-medium transition-colors disabled:opacity-50"
                          style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                    {transitioning ? '…' : t.name}
                  </button>
                {/each}
              </div>
            </div>
          {/if}

          <!-- Comments (latest 3) -->
          {#if f.comment?.comments?.length}
            <div class="p-4">
              <p class="mb-2" style="color: var(--sempa-text-dim);">
                Comments ({f.comment.total})
              </p>
              <div class="space-y-3">
                {#each f.comment.comments.slice(-3) as c}
                  <div>
                    <div class="flex items-baseline gap-2 mb-0.5">
                      <span class="font-medium" style="color: var(--sempa-text);">{c.author.displayName}</span>
                      <span style="color: var(--sempa-text-dim);">{fmtDate(c.created)}</span>
                    </div>
                    <p class="leading-relaxed whitespace-pre-wrap" style="color: var(--sempa-text-soft, var(--sempa-text));">
                      {c.body}
                    </p>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      {/if}
    </div>

  {:else}
    <!-- ── List view ──────────────────────────────────────────────────────── -->

    <!-- Filters -->
    <div class="shrink-0 space-y-1.5 px-3 py-2" style="border-bottom: 1px solid var(--sempa-border);">
      <div class="flex gap-1">
        {#each [['unplanned', 'Unplanned'], ['planned', 'Planned'], ['all', 'All']] as [val, label]}
          <button onclick={() => sempaFilter = val as SempaFilter}
                  class="flex-1 rounded py-1 font-medium transition-colors"
                  style={sempaFilter === val
                    ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                    : 'color: var(--sempa-text-dim);&:hover{color:var(--sempa-text)}'}>
            {label}
          </button>
        {/each}
      </div>

      <select bind:value={statusFilter}
              class="w-full rounded border px-2 py-1"
              style="border-color: var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);">
        <option value="">All stages</option>
        {#each allStatuses as s}
          <option value={s.name}>{s.name}</option>
        {/each}
      </select>
    </div>

    <!-- Count + sync -->
    <div class="flex shrink-0 items-center justify-between px-3 py-1.5"
         style="border-bottom: 1px solid var(--sempa-border);">
      <span style="color: var(--sempa-text-dim);">
        {filteredTasks.length} issue{filteredTasks.length === 1 ? '' : 's'}
      </span>
      <button onclick={sync} disabled={syncing || loading}
              aria-label="Sync Jira"
              class="transition-colors disabled:opacity-40" style="color: var(--sempa-text-dim);">
        <svg class="h-3.5 w-3.5 {syncing ? 'animate-spin' : ''}" fill="none" stroke="currentColor"
             stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
        </svg>
      </button>
    </div>

    <div class="flex-1 overflow-y-auto">
      {#if !connected}
        <div class="flex flex-col items-center justify-center gap-2 p-6 text-center">
          <p style="color: var(--sempa-text-dim);">Jira not connected</p>
          <a href="/settings/integrations/jira" class="text-blue-500 hover:underline">Set up →</a>
        </div>

      {:else if loading}
        <div class="space-y-px">
          {#each Array(4) as _}
            <div class="flex items-start gap-2 px-3 py-2.5 animate-pulse">
              <div class="h-4 w-12 shrink-0 rounded mt-0.5" style="background: var(--sempa-border);"></div>
              <div class="flex-1 space-y-1.5">
                <div class="h-2.5 w-full rounded" style="background: var(--sempa-border);"></div>
                <div class="h-2 w-2/3 rounded" style="background: var(--sempa-border);"></div>
              </div>
            </div>
          {/each}
        </div>

      {:else if error}
        <p class="p-4 text-red-500">{error}</p>

      {:else if filteredTasks.length === 0}
        <div class="flex h-24 flex-col items-center justify-center gap-1.5">
          <p style="color: var(--sempa-text-dim);">No issues</p>
          {#if allTasks.length === 0}
            <button onclick={sync} class="text-blue-500 hover:underline">Sync now</button>
          {/if}
        </div>

      {:else}
        <ul>
          {#each filteredTasks as task (task.id)}
            {@const meta = parseMeta(task.source_metadata)}
            <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
            <li class="group flex cursor-grab items-start gap-2 px-3 py-2.5 transition-colors
                       active:cursor-grabbing hover:bg-gray-50 dark:hover:bg-gray-800/40"
                style="border-bottom: 1px solid var(--sempa-border);"
                draggable="true"
                ondragstart={(e) => onDragStart(e, task)}>

              <div class="min-w-0 flex-1" role="button" tabindex="0"
                   onclick={() => openCard(task)}
                   onkeydown={(e) => e.key === 'Enter' && openCard(task)}>
                <div class="flex items-baseline gap-1.5">
                  {#if meta?.key}
                    <span class="shrink-0 font-mono font-semibold text-blue-500 dark:text-blue-400">
                      {meta.key}
                    </span>
                  {/if}
                  <span class="truncate" style="color: var(--sempa-text);">{task.title}</span>
                </div>
                {#if meta}
                  <div class="mt-0.5 flex items-center gap-1.5" style="color: var(--sempa-text-dim);">
                    {#if meta.priority && priorityDot(meta.priority)}
                      <span class="h-1.5 w-1.5 rounded-full shrink-0 {priorityDot(meta.priority)}"></span>
                    {/if}
                    <span class="truncate">{meta.status}</span>
                    {#if meta.issueType}
                      <span style="color: var(--sempa-border);">·</span>
                      <span class="truncate">{meta.issueType}</span>
                    {/if}
                  </div>
                {/if}
              </div>

              <!-- Open card -->
              <button onclick={() => openCard(task)}
                      class="mt-0.5 shrink-0 transition-opacity
                             {mobile.value ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}"
                      style="color: var(--sempa-text-dim);" title="View details">
                <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M9 5l7 7-7 7"/>
                </svg>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    {#if connected && !loading && filteredTasks.length > 0}
      <div class="shrink-0 px-3 py-1.5" style="border-top: 1px solid var(--sempa-border); color: var(--sempa-text-dim);">
        Drag issues onto a day to plan them
      </div>
    {/if}
  {/if}
</div>
