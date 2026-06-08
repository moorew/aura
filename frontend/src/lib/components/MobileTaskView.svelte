<script lang="ts">
  import type { Task } from '$lib/types';
  import { formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { hapticClick, hapticTick } from '$lib/haptics';
  import { dismissibleSheet } from '$lib/actions/sheet';
  import { viewport } from '$lib/stores/viewport.svelte';
  import SubTaskList from './SubTaskList.svelte';

  let {
    open,
    task,
    onClose,
    onEdit,
    onComplete,
    onDelete,
    onFocusStart,
  }: {
    open: boolean;
    task: Task | null;
    onClose: () => void;
    onEdit: () => void;
    onComplete: (id: string) => void;
    onDelete: (id: string, title: string) => void;
    onFocusStart?: (id: string, title: string) => void;
  } = $props();

  const isDone     = $derived(task?.status === 'done');
  const isRunning  = $derived(!!task && pomodoro.taskId === task.id);

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Calendar',
  };

  // Sheet shrinks above the soft keyboard via the visual viewport.
  const maxHeight = $derived(Math.round(viewport.height * 0.9));

  // Keep a focused field (e.g. the add-subtask input) visible above keyboard.
  function keepInView(e: FocusEvent) {
    const el = e.target as HTMLElement | null;
    if (!el || !el.matches('input, textarea, select')) return;
    setTimeout(() => el.scrollIntoView({ block: 'center', behavior: 'smooth' }), 250);
  }

  function formatDate(d: string): string {
    const dt = new Date(d + 'T12:00:00');
    return dt.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
  }

  function formatScheduled(iso: string): string {
    const dt = new Date(iso);
    return dt.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  }
</script>

{#if open && task}
  <!-- Overlay -->
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[89] bg-black/30 backdrop-blur-sm"
       style="animation: sempa-fade-in 200ms ease both;"
       onclick={onClose}></div>

  <!-- Sheet — lifted above the soft keyboard so its content/inputs stay reachable -->
  <div role="dialog" aria-modal="true" aria-label="Task details" tabindex="-1"
       class="fixed left-0 right-0 z-[90] flex flex-col shadow-2xl"
       style="border-radius: 20px 20px 0 0; background: var(--sempa-bg-panel);
              bottom: {viewport.keyboardHeight}px;
              max-height: {maxHeight}px;
              transition: bottom 180ms ease-out;
              animation: task-view-up 320ms cubic-bezier(0.32, 0.72, 0, 1) both;"
       use:dismissibleSheet={{ onClose, scrollSelector: '[data-sheet-scroll]', threshold: 90, onDismissHaptic: hapticTick }}>

    <!-- Drag handle -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="flex justify-center pt-3 pb-1 cursor-grab shrink-0" data-sheet-handle onclick={onClose}>
      <div class="h-1 w-8 rounded-full" style="background: var(--sempa-border);"></div>
    </div>

    <!-- Scrollable content -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="flex-1 overflow-y-auto overscroll-contain px-5 pb-4" data-sheet-scroll
         style="-webkit-overflow-scrolling: touch; scroll-padding-bottom: 96px;"
         onfocusin={keepInView}>

      <!-- Title + status -->
      <div class="flex items-start gap-3 pt-2 pb-4" style="border-bottom: 1px solid var(--sempa-border);">
        <!-- Complete circle (large tap target) -->
        <button
          type="button"
          onclick={() => { hapticClick(); onComplete(task.id); }}
          class="mt-1 h-6 w-6 shrink-0 rounded-full border-2 flex items-center justify-center transition-all"
          class:border-green-500={isDone}
          class:bg-green-500={isDone}
          style={isDone ? '' : 'border-color: var(--sempa-border);'}
          aria-label={isDone ? 'Mark incomplete' : 'Mark complete'}>
          {#if isDone}
            <svg class="h-3.5 w-3.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {/if}
        </button>
        <h2 class="flex-1 text-xl font-semibold leading-snug {isDone ? 'line-through opacity-40' : ''}"
            style="color: var(--sempa-text);">
          {task.title}
        </h2>
      </div>

      <!-- Meta chips row -->
      <div class="flex flex-wrap gap-2 py-4" style="border-bottom: 1px solid var(--sempa-border);">
        <!-- Status -->
        {#if task.status === 'in_progress'}
          <span class="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
            ● In progress
          </span>
        {:else if isDone}
          <span class="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold
                       bg-green-50 text-green-600">
            ✓ Done
          </span>
        {/if}

        <!-- Date -->
        {#if task.planned_date}
          <span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs"
                style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
            <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>
            </svg>
            {formatDate(task.planned_date)}
          </span>
        {/if}

        <!-- Scheduled time -->
        {#if task.scheduled_start}
          <span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs"
                style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
            <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
            </svg>
            {formatScheduled(task.scheduled_start)}{task.scheduled_end ? ` – ${formatScheduled(task.scheduled_end)}` : ''}
          </span>
        {/if}

        <!-- Time estimate -->
        {#if task.time_estimate_minutes}
          <span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono"
                style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
            ~{formatMinutes(task.time_estimate_minutes)}
          </span>
        {/if}

        <!-- Actual time logged -->
        {#if task.time_actual_minutes}
          <span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
            {formatMinutes(task.time_actual_minutes)} logged
          </span>
        {/if}

        <!-- Recurrence -->
        {#if task.recurrence_rule || task.recurrence_origin_id}
          <span class="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs"
                style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-text-dim);">
            ↺ Recurring
          </span>
        {/if}

        <!-- Source -->
        {#if task.source && task.source !== 'manual'}
          <span class="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
            From {sourceLabel[task.source] ?? task.source}
          </span>
        {/if}

        <!-- Tags -->
        {#each (task.tags ?? []) as tag}
          <span class="inline-flex items-center rounded-full px-3 py-1 text-xs font-medium text-white"
                style="background-color: {tagStore.colorFor(tag)}">
            {tag}
          </span>
        {/each}
      </div>

      <!-- Description -->
      {#if task.description}
        <div class="py-4" style="border-bottom: 1px solid var(--sempa-border);">
          <p class="text-[11px] font-semibold uppercase tracking-wider mb-2"
             style="color: var(--sempa-text-dim);">Notes</p>
          <p class="text-sm leading-relaxed whitespace-pre-wrap" style="color: var(--sempa-text-soft);">
            {task.description}
          </p>
        </div>
      {/if}

      <!-- Sub-tasks -->
      <div class="py-4" style="border-bottom: 1px solid var(--sempa-border);">
        <SubTaskList parentId={task.id} parentDate={task.planned_date ?? undefined} />
      </div>

      <!-- Pomodoro status -->
      {#if isRunning}
        <div class="mt-4 rounded-xl px-4 py-3 flex items-center gap-3"
             style="background: var(--sempa-accent-bg); border: 1px solid var(--sempa-accent-bg);">
          <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2" style="color: var(--sempa-accent);" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
          </svg>
          <div>
            <p class="text-xs font-semibold" style="color: var(--sempa-accent);">{pomodoro.phaseLabel}</p>
            <p class="font-mono text-lg font-bold" style="color: var(--sempa-accent);">{pomodoro.display}</p>
          </div>
        </div>
      {/if}
    </div>

    <!-- Action bar -->
    <div class="shrink-0 px-4 py-3 flex items-center gap-2"
         style="border-top: 1px solid var(--sempa-border);
                padding-bottom: max(12px, env(safe-area-inset-bottom, 12px));
                background: var(--sempa-bg-panel);">

      <!-- Complete / Undo -->
      <button onclick={() => { hapticClick(); onComplete(task.id); onClose(); }}
              class="flex flex-1 items-center justify-center gap-2 rounded-xl py-3 text-sm font-medium transition-colors"
              style="{isDone
                ? 'background: var(--sempa-border); color: var(--sempa-text-soft);'
                : 'background: var(--sempa-success); color: white;'}">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          {#if isDone}
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 14l-4-4m0 0l4-4m-4 4h15"/>
          {:else}
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
          {/if}
        </svg>
        {isDone ? 'Undo' : 'Done'}
      </button>

      <!-- Focus / Pomodoro -->
      {#if onFocusStart && !isDone}
        <button onclick={() => { hapticClick(); onFocusStart!(task.id, task.title); onClose(); }}
                aria-label="Start focus timer"
                class="flex h-12 w-12 items-center justify-center rounded-xl transition-colors"
                style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-accent);">
          <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
          </svg>
        </button>
      {/if}

      <!-- Edit -->
      <button onclick={onEdit}
              aria-label="Edit task"
              class="flex h-12 w-12 items-center justify-center rounded-xl transition-colors"
              style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M11 5H6a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2v-5m-1.414-9.414a2 2 0 1 1 2.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
        </svg>
      </button>

      <!-- Delete -->
      <button onclick={() => { hapticClick(); onDelete(task.id, task.title); onClose(); }}
              aria-label="Delete task"
              class="flex h-12 w-12 items-center justify-center rounded-xl transition-colors"
              style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: #f87171;">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
        </svg>
      </button>
    </div>
  </div>
{/if}

<style>
  @keyframes task-view-up {
    from { transform: translateY(100%); }
    to   { transform: translateY(0); }
  }
</style>
