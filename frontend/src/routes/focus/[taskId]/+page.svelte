<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { formatMinutes, today } from '$lib/utils';
  import SubTaskList from '$lib/components/SubTaskList.svelte';
  import RichText from '$lib/components/RichText.svelte';

  let taskId  = $derived($page.params.taskId);
  let task    = $state<Task | null>(null);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  onMount(async () => {
    try { task = await api.tasks.get(taskId!); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load task'; }
    finally { loading = false; }
  });

  async function toggleDone() {
    if (!task) return;
    const newStatus = task.status === 'done' ? 'planned' : 'done';
    try {
      task = await api.tasks.update(task.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
    } catch { /* ignore */ }
  }

  function startPomodoro() {
    if (!task) return;
    pomodoro.start(task.id, task.title, task.time_actual_minutes ?? 0);
  }

  const isDone       = $derived(task?.status === 'done' || false);
  const isMyPomodoro = $derived(!!task && pomodoro.taskId === task.id);
</script>

<svelte:head><title>{task?.title ?? 'Focus'} — Sempa</title></svelte:head>

<!-- Back button -->
<div class="fixed left-4 top-4 z-10">
  <button onclick={() => history.back()}
          class="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-colors"
          style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);
                 color: var(--sempa-text-soft);">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" d="M19 12H5m7-7-7 7 7 7"/>
    </svg>
    Back
  </button>
</div>

<div class="flex min-h-full flex-col items-center px-4 py-20 animate-fade-in">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>

  {:else if error || !task}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error ?? 'Task not found'}</div>

  {:else}
    <div class="w-full max-w-xl">

      <!-- Task header -->
      <div class="mb-8 flex items-start gap-4">
        <button onclick={toggleDone}
                class="mt-1.5 h-6 w-6 shrink-0 rounded-full border-2 flex items-center justify-center transition-all"
                class:border-green-500={isDone} class:bg-green-500={isDone}
                style={isDone ? '' : 'border-color: var(--sempa-border);'}
                onmouseenter={(e) => { if (!isDone) (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-success)'; }}
                onmouseleave={(e) => { if (!isDone) (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; }}>
          {#if isDone}
            <svg class="h-3.5 w-3.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {/if}
        </button>
        <div class="flex-1 min-w-0">
          <h1 class="text-3xl font-bold leading-tight tracking-tight
                     {isDone ? 'line-through opacity-40' : ''}"
              style="color: var(--sempa-text);">
            {task.title}
          </h1>
          {#if task.planned_date}
            <p class="mt-1 text-sm" style="color: var(--sempa-text-dim);">{task.planned_date}</p>
          {/if}
        </div>
      </div>

      <!-- Description -->
      {#if task.description}
        <div class="mb-8 rounded-xl px-4 py-3" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <div class="text-sm leading-relaxed" style="color: var(--sempa-text-soft);">
            <RichText text={task.description} />
          </div>
        </div>
      {/if}

      <!-- Sub-tasks -->
      <div class="mb-8 rounded-xl p-4" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <SubTaskList parentId={task.id} parentDate={task.planned_date ?? undefined} />
      </div>

      <!-- Pomodoro panel -->
      <div class="rounded-2xl p-6 text-center" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        {#if isMyPomodoro}
          <p class="text-xs font-semibold uppercase tracking-wider mb-3" style="color: var(--sempa-accent);">
            {pomodoro.phaseLabel}
          </p>
          <div class="font-mono text-6xl font-bold mb-4 tabular-nums" style="color: var(--sempa-text);">
            {pomodoro.display}
          </div>
          <!-- Progress arc (simple bar) -->
          <div class="mx-auto mb-5 h-1.5 w-48 overflow-hidden rounded-full" style="background: var(--sempa-border);">
            <div class="h-full rounded-full transition-all duration-1000"
                 style="width: {pomodoro.progressPct}%; background: var(--sempa-accent);"></div>
          </div>
          <div class="flex items-center justify-center gap-3">
            <button onclick={() => pomodoro.togglePause()}
                    class="flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                    style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);"
                    onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                    onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
              {#if pomodoro.isRunning}
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M10 9v6m4-6v6"/>
                </svg>
                Pause
              {:else}
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 3l14 9-14 9V3z"/>
                </svg>
                Resume
              {/if}
            </button>
            <button onclick={() => pomodoro.stop()}
                    class="rounded-xl px-4 py-2.5 text-sm transition-colors"
                    style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
              Stop
            </button>
          </div>
        {:else}
          <div class="mb-4">
            {#if task.time_actual_minutes}
              <p class="text-sm mb-0.5" style="color: var(--sempa-text-dim);">
                {formatMinutes(task.time_actual_minutes)} logged
              </p>
            {:else}
              <p class="text-sm mb-0.5" style="color: var(--sempa-text-dim);">No time logged yet</p>
            {/if}
            {#if task.time_estimate_minutes}
              <p class="text-xs" style="color: var(--sempa-text-dim);">
                Estimated: {formatMinutes(task.time_estimate_minutes)}
              </p>
            {/if}
          </div>
          <button onclick={startPomodoro} disabled={isDone}
                  class="flex items-center gap-2 mx-auto rounded-xl px-6 py-3 text-sm font-medium
                         transition-colors disabled:opacity-40"
                  style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);"
                  onmouseenter={(e) => { if (!isDone) (e.currentTarget as HTMLElement).style.opacity = '0.88'; }}
                  onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
            </svg>
            Start Pomodoro
          </button>
        {/if}
      </div>
    </div>
  {/if}
</div>
