<script lang="ts">
  import { onMount } from 'svelte';
  import { isTauri } from '$lib/tauri/bridge';
  import { api } from '$lib/api';
  import type { Task, Objective, ICalEvent } from '$lib/types';
  import { today, weekStart } from '$lib/utils';
  import { Check, X, Plus } from 'lucide-svelte';

  type Tab = 'today' | 'objectives';
  let tab = $state<Tab>('today');

  let tasks      = $state<Task[]>([]);
  let objectives = $state<Objective[]>([]);
  let upNext     = $state<{ title: string; when: string } | null>(null);
  let newTitle   = $state('');
  let adding     = $state(false);

  const todayDate = today();
  const ws        = weekStart(todayDate);

  // Top-level, non-cancelled tasks for today, scheduled blocks first.
  const todayTasks = $derived(
    tasks
      .filter((t) => !t.parent_task_id && t.status !== 'cancelled')
      .sort((a, b) => {
        if (a.scheduled_start && b.scheduled_start) return a.scheduled_start < b.scheduled_start ? -1 : 1;
        if (a.scheduled_start) return -1;
        if (b.scheduled_start) return 1;
        return a.position - b.position;
      })
  );
  const doneCount  = $derived(todayTasks.filter((t) => t.status === 'done').length);
  const totalCount = $derived(todayTasks.length);
  const progress   = $derived(totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0);

  const openObjectives = $derived(objectives.filter((o) => o.status !== 'cancelled'));

  onMount(() => {
    if (!isTauri()) return;
    void loadAll();
    const interval = setInterval(loadAll, 30000);
    return () => clearInterval(interval);
  });

  async function loadAll() {
    await Promise.all([loadTasks(), loadObjectives(), loadUpNext()]);
  }

  async function loadTasks() {
    try { tasks = await api.tasks.listByDate(todayDate); } catch { /* offline */ }
  }
  async function loadObjectives() {
    try { objectives = await api.objectives.listByWeek(ws); } catch { /* offline */ }
  }

  // "Up Next" — the soonest upcoming item today, drawn from scheduled task blocks
  // and (best-effort) calendar events, whichever starts next.
  async function loadUpNext() {
    const now = Date.now();
    const candidates: { title: string; start: number }[] = [];

    for (const t of tasks) {
      if (t.scheduled_start && t.status !== 'done' && t.status !== 'cancelled') {
        const ms = new Date(t.scheduled_start).getTime();
        if (ms >= now) candidates.push({ title: t.title, start: ms });
      }
    }
    try {
      const events = await api.ical.listEvents(todayDate);
      for (const ev of (events as ICalEvent[])) {
        if (ev.all_day) continue;
        const ms = new Date(ev.start_time).getTime();
        if (ms >= now) candidates.push({ title: ev.summary, start: ms });
      }
    } catch { /* ical not reachable — scheduled tasks still cover Up Next */ }

    candidates.sort((a, b) => a.start - b.start);
    const next = candidates[0];
    upNext = next
      ? { title: next.title, when: new Date(next.start).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' }) }
      : null;
  }

  // ── Mutations (optimistic; sync engine replays to the server) ───────────────
  async function toggleTask(t: Task) {
    const next = t.status === 'done' ? 'planned' : 'done';
    tasks = tasks.map((x) => x.id === t.id ? { ...x, status: next } : x);
    try {
      await api.tasks.update(t.id, {
        status: next,
        completed_at: next === 'done' ? new Date().toISOString() : null,
      });
    } catch { void loadTasks(); }
    void loadUpNext();
  }

  async function toggleObjective(o: Objective) {
    const next = o.status === 'completed' ? 'active' : 'completed';
    objectives = objectives.map((x) => x.id === o.id ? { ...x, status: next } : x);
    try { await api.objectives.update(o.id, { status: next }); }
    catch { void loadObjectives(); }
  }

  async function addTask() {
    const title = newTitle.trim();
    if (!title || adding) return;
    adding = true;
    try {
      await api.tasks.create({ title, planned_date: todayDate, week_start: ws, status: 'planned' });
      newTitle = '';
      await loadTasks();
    } catch { /* will retry on next add */ }
    finally { adding = false; }
  }

  function schedLabel(t: Task): string | null {
    if (!t.scheduled_start) return null;
    return new Date(t.scheduled_start).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  async function closeWidget() {
    try {
      const { getCurrentWindow } = await import('@tauri-apps/api/window');
      await getCurrentWindow().close();
    } catch { /* not in Tauri */ }
  }
</script>

<svelte:head>
  <style>
    /* Transparent, gap-free shell so only the card's single rounded shape shows
       against the desktop — no window backdrop or shadow halo peeking out with a
       mismatched corner radius. */
    html, body { background: transparent !important; margin: 0; height: 100%; overflow: hidden; }
  </style>
</svelte:head>

<div class="widget">
  <!-- Title bar — the primary drag handle. Tall + full-width with a visible grip
       so it's easy to grab and reposition the window. -->
  <div class="widget-header" data-tauri-drag-region>
    <span class="widget-logo" data-tauri-drag-region>sempa</span>
    <span class="drag-grip" data-tauri-drag-region aria-hidden="true">
      <span></span><span></span><span></span>
    </span>
    <div class="widget-header-right">
      <span class="widget-progress" data-tauri-drag-region>{progress}%</span>
      <button class="widget-close" onclick={closeWidget}
              title="Hide widget (re-open from the tray)" aria-label="Hide widget">
        <X size={12} strokeWidth={2.5} />
      </button>
    </div>
  </div>

  <!-- Progress bar (also draggable — non-interactive surface) -->
  <div class="widget-bar" data-tauri-drag-region>
    <div class="widget-bar-fill" style="width: {progress}%"></div>
  </div>

  <!-- Up Next -->
  {#if upNext}
    <div class="up-next" data-tauri-drag-region>
      <span class="up-next-label" data-tauri-drag-region>Up Next · {upNext.when}</span>
      <span class="up-next-title" data-tauri-drag-region>{upNext.title}</span>
    </div>
  {/if}

  <!-- Toggle -->
  <div class="toggle">
    <button class="toggle-btn" class:active={tab === 'today'} onclick={() => tab = 'today'}>Today’s Tasks</button>
    <button class="toggle-btn" class:active={tab === 'objectives'} onclick={() => tab = 'objectives'}>Objectives</button>
  </div>

  <!-- List -->
  <div class="list">
    {#if tab === 'today'}
      {#if todayTasks.length === 0}
        <p class="empty">No tasks today — add one below.</p>
      {:else}
        {#each todayTasks as task (task.id)}
          <div class="row" class:done={task.status === 'done'} class:active={task.status === 'in_progress'}>
            <button class="checkbox" class:checked={task.status === 'done'}
                    onclick={() => toggleTask(task)}
                    aria-label={task.status === 'done' ? 'Mark incomplete' : 'Mark complete'}>
              {#if task.status === 'done'}<Check size={11} strokeWidth={3} />{/if}
            </button>
            <span class="row-title">
              {#if schedLabel(task)}<span class="row-time">{schedLabel(task)}</span>{/if}
              {task.title}
            </span>
          </div>
        {/each}
      {/if}
    {:else}
      {#if openObjectives.length === 0}
        <p class="empty">No objectives this week.</p>
      {:else}
        {#each openObjectives as obj (obj.id)}
          <div class="row" class:done={obj.status === 'completed'}>
            <button class="checkbox" class:checked={obj.status === 'completed'}
                    onclick={() => toggleObjective(obj)}
                    aria-label={obj.status === 'completed' ? 'Mark incomplete' : 'Mark complete'}>
              {#if obj.status === 'completed'}<Check size={11} strokeWidth={3} />{/if}
            </button>
            <span class="row-title">{obj.title}</span>
          </div>
        {/each}
      {/if}
    {/if}
  </div>

  <!-- Quick add (today only) -->
  {#if tab === 'today'}
    <form class="quick-add" onsubmit={(e) => { e.preventDefault(); addTask(); }}>
      <Plus size={14} strokeWidth={2.25} />
      <input bind:value={newTitle} placeholder="Add a task…" aria-label="Add a task"
             autocomplete="off" spellcheck="false" />
      {#if newTitle.trim()}
        <button type="submit" class="quick-add-go" disabled={adding}>{adding ? '…' : 'Add'}</button>
      {/if}
    </form>
  {:else}
    <div class="widget-footer" data-tauri-drag-region>{doneCount}/{totalCount} tasks done today</div>
  {/if}
</div>

<style>
  .widget {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    box-sizing: border-box;
    padding: 0 12px 12px;
    /* Fill the window edge-to-edge with NO CSS corner radius. The previous 12px
       radius left transparent cutouts at each corner where the window's own grey
       backing showed through as a "box" with a mismatched radius. Painting every
       pixel removes that entirely; the OS rounds the window itself (Win11 DWM),
       so the widget still reads as a soft tile without the grey ring. */
    border-radius: 0;
    background: var(--sempa-bg-panel);
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: var(--sempa-text);
    overflow: hidden;
  }

  /* Tall, full-width title bar = a big, obvious drag handle. */
  .widget-header {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 38px;
    margin: 0 -12px 8px;
    padding: 0 12px;
    cursor: grab;
    border-bottom: 1px solid var(--sempa-border);
  }
  .widget-header:active { cursor: grabbing; }
  .widget-logo { font-size: 13px; font-weight: 600; letter-spacing: -0.02em; color: var(--sempa-accent); }

  /* Centered grip dots signalling the bar is draggable. */
  .drag-grip {
    flex: 1;
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 3px;
    height: 100%;
  }
  .drag-grip span {
    width: 3px; height: 3px; border-radius: 50%;
    background: var(--sempa-text-dim); opacity: 0.5;
  }
  .widget-progress { font-family: 'JetBrains Mono', monospace; font-size: 11px; font-weight: 600; color: var(--sempa-text-soft); }
  .widget-header-right { display: flex; align-items: center; gap: 6px; }

  .widget-close {
    display: flex; align-items: center; justify-content: center;
    width: 18px; height: 18px; border: none; border-radius: 6px;
    background: none; color: var(--sempa-text-dim); cursor: pointer;
    transition: background 120ms ease, color 120ms ease;
  }
  .widget-close:hover { background: rgba(0,0,0,0.06); color: var(--sempa-text); }

  .widget-bar { height: 4px; border-radius: 2px; background: var(--sempa-border); margin-bottom: 10px; overflow: hidden; flex-shrink: 0; }
  .widget-bar-fill { height: 100%; border-radius: 2px; background: var(--sempa-accent); transition: width 500ms ease-out; }

  .up-next {
    display: flex; flex-direction: column; gap: 1px;
    padding: 7px 9px; margin-bottom: 10px;
    border-radius: 8px;
    background: var(--sempa-accent-bg);
    border: 1px solid color-mix(in srgb, var(--sempa-accent) 30%, transparent);
    flex-shrink: 0;
  }
  .up-next-label { font-family: 'JetBrains Mono', monospace; font-size: 9.5px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--sempa-accent); }
  .up-next-title { font-size: 12.5px; font-weight: 600; color: var(--sempa-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .toggle { display: flex; gap: 4px; margin-bottom: 8px; flex-shrink: 0; }
  .toggle-btn {
    flex: 1; padding: 5px 0; font-size: 11px; font-weight: 600;
    border: 1px solid var(--sempa-border); border-radius: 7px;
    background: transparent; color: var(--sempa-text-dim); cursor: pointer;
    transition: all 120ms ease;
  }
  .toggle-btn.active { background: var(--sempa-accent-bg); color: var(--sempa-accent); border-color: color-mix(in srgb, var(--sempa-accent) 35%, transparent); }

  .list { flex: 1; min-height: 0; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; padding-right: 2px; }
  .list::-webkit-scrollbar { width: 5px; }
  .list::-webkit-scrollbar-thumb { background: var(--sempa-border); border-radius: 3px; }

  .empty { font-size: 11.5px; color: var(--sempa-text-dim); text-align: center; padding: 16px 0; }

  .row {
    display: flex; align-items: flex-start; gap: 8px;
    padding: 7px 9px; border-radius: 7px;
    background: var(--sempa-bg-main); border: 1px solid var(--sempa-border);
  }
  .row.active { border-color: var(--sempa-accent); background: var(--sempa-accent-bg); }
  .row.done { opacity: 0.55; }
  .row.done .row-title { text-decoration: line-through; }

  .checkbox {
    flex-shrink: 0; margin-top: 1px;
    display: flex; align-items: center; justify-content: center;
    width: 16px; height: 16px; border-radius: 5px;
    border: 1.5px solid var(--sempa-text-dim);
    background: transparent; color: #fff; cursor: pointer;
    transition: background 120ms ease, border-color 120ms ease;
  }
  .checkbox.checked { background: var(--sempa-accent); border-color: var(--sempa-accent); }

  /* Wrap instead of truncating — full task text is visible. */
  .row-title { font-size: 12px; font-weight: 500; line-height: 1.35; overflow-wrap: anywhere; }
  .row-time { font-weight: 700; color: var(--sempa-accent); margin-right: 4px; }

  .quick-add {
    display: flex; align-items: center; gap: 6px;
    margin-top: 8px; padding: 7px 9px; flex-shrink: 0;
    border-radius: 8px; border: 1px solid var(--sempa-border);
    background: var(--sempa-bg-main); color: var(--sempa-text-dim);
  }
  .quick-add input {
    flex: 1; border: none; background: transparent; outline: none;
    font-family: inherit; font-size: 12px; color: var(--sempa-text);
  }
  .quick-add input::placeholder { color: var(--sempa-text-dim); }
  .quick-add-go {
    flex-shrink: 0; padding: 3px 10px; border: none; border-radius: 6px;
    background: var(--sempa-accent); color: #fff; font-size: 11px; font-weight: 600; cursor: pointer;
  }
  .quick-add-go:disabled { opacity: 0.6; }

  .widget-footer { margin-top: 8px; font-family: 'JetBrains Mono', monospace; font-size: 10.5px; color: var(--sempa-text-dim); text-align: center; flex-shrink: 0; }
</style>
