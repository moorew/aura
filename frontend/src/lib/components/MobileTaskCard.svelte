<script lang="ts">
  import type { Task } from '$lib/types';
  import { formatMinutes, bareUrl, prettyUrl } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { hapticClick, hapticTick } from '$lib/haptics';

  let {
    task,
    onComplete,
    onTrash,
    onClick,
    onFocusClick,
  }: {
    task: Task;
    onComplete?: (id: string) => void;
    onTrash?: (id: string, title: string) => void;
    onClick?: (task: Task) => void;
    onFocusClick?: (id: string, title: string) => void;
  } = $props();

  const isDone      = $derived(task.status === 'done');
  const isRecurring = $derived(!!task.recurrence_origin_id);
  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Cal',
  };

  // Swipe state
  let startX = $state(0);
  let startY = $state(0);
  let deltaX = $state(0);
  let swiping = $state(false);
  let locked = $state<null | 'h' | 'v'>(null); // gesture direction, decided once
  let suppressClick = $state(false);
  const SWIPE_THRESHOLD = 60;
  const MAX_SWIPE = 80;
  const TRIGGER = SWIPE_THRESHOLD * 0.4;

  function handleTouchStart(e: TouchEvent) {
    startX = e.touches[0].clientX;
    startY = e.touches[0].clientY;
    deltaX = 0;
    locked = null;
    swiping = true;
  }

  function handleTouchMove(e: TouchEvent) {
    if (!swiping) return;
    const dx = e.touches[0].clientX - startX;
    const dy = e.touches[0].clientY - startY;

    // Decide once whether this is a horizontal swipe or a vertical scroll, so
    // list scrolling never fights the swipe-to-complete gesture.
    if (locked === null) {
      if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return;
      locked = Math.abs(dx) > Math.abs(dy) ? 'h' : 'v';
    }
    if (locked !== 'h' || dx <= 0) return;

    const prev = deltaX;
    deltaX = Math.min(dx * 0.4, MAX_SWIPE);
    if (prev < TRIGGER && deltaX >= TRIGGER) hapticTick();
  }

  function handleTouchEnd() {
    if (!swiping) return;
    swiping = false;
    if (deltaX > TRIGGER) {
      hapticClick();
      onComplete?.(task.id);
    }
    // Suppress the synthetic click that follows any real horizontal swipe so a
    // swipe-to-complete doesn't also open the task detail.
    if (locked === 'h' && deltaX > 4) {
      suppressClick = true;
      setTimeout(() => { suppressClick = false; }, 350);
    }
    deltaX = 0;
    locked = null;
  }

  function handleClick() {
    if (suppressClick) return;
    onClick?.(task);
  }
</script>

<div class="relative overflow-hidden rounded-xl">
  <!-- Swipe reveal (green check) -->
  {#if deltaX > 0}
    <div class="absolute inset-y-0 left-0 flex items-center pl-4"
         style="color: var(--sempa-success); width: {deltaX}px;">
      <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
      </svg>
    </div>
  {/if}

  <!-- Card -->
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div
    class="relative flex items-start gap-3 rounded-xl border p-3.5"
    style="background: var(--card-bg); border-color: var(--card-border);
           transform: translateX({deltaX}px);
           transition: {swiping ? 'none' : 'transform 200ms ease-out'};"
    ontouchstart={handleTouchStart}
    ontouchmove={handleTouchMove}
    ontouchend={handleTouchEnd}
    onclick={handleClick}
  >
    <!-- Complete circle -->
    <button
      type="button"
      onclick={(e) => { e.stopPropagation(); hapticClick(); onComplete?.(task.id); }}
      class="mt-0.5 h-5 w-5 shrink-0 rounded-full border-2 flex items-center justify-center
             {isDone ? 'border-green-500 bg-green-500' : 'border-gray-300 dark:border-gray-600'}"
      aria-label="Complete task"
    >
      {#if isDone}
        <svg class="h-3 w-3 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
        </svg>
      {/if}
    </button>

    <!-- Content -->
    <div class="min-w-0 flex-1">
      <p class="text-[15px] leading-snug
                {isDone ? 'line-through opacity-40' : 'font-medium'}"
         style="color: var(--sempa-text); overflow-wrap: anywhere; word-break: break-word;">
        {#if bareUrl(task.title)}
          <span class="inline-flex max-w-full items-center gap-1 align-middle">
            <svg class="h-3.5 w-3.5 shrink-0 opacity-60" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10 13a5 5 0 007.07 0l3-3a5 5 0 00-7.07-7.07l-1.72 1.71M14 11a5 5 0 00-7.07 0l-3 3a5 5 0 007.07 7.07l1.71-1.71"/>
            </svg>
            <span class="truncate">{prettyUrl(bareUrl(task.title)!)}</span>
          </span>
        {:else}
          {task.title}
        {/if}
      </p>

      <!-- Meta row -->
      {#if task.tags?.length || task.time_estimate_minutes || (task.source && task.source !== 'manual') || isRecurring}
        <div class="flex flex-wrap gap-1 mt-1.5">
          {#if (task.tags ?? []).length}
            <!-- Tags as colour dots only (matches TaskCard) — names live in the
                 task detail view; title shows them on long-press. -->
            <span class="inline-flex items-center gap-1" title={(task.tags ?? []).join(', ')}>
              {#each task.tags ?? [] as tag}
                <span class="shrink-0 rounded-full" style="width: 7px; height: 7px; background-color: {tagStore.colorFor(tag)};"></span>
              {/each}
            </span>
          {/if}
          {#if task.time_estimate_minutes}
            <span class="rounded px-1.5 py-0.5 text-[10.5px] font-mono"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-text-dim);">
              {formatMinutes(task.time_estimate_minutes)}
            </span>
          {/if}
          {#if task.source && task.source !== 'manual'}
            <span class="rounded px-1.5 py-0.5 text-[10.5px]"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
              {sourceLabel[task.source] ?? task.source}
            </span>
          {/if}
          {#if isRecurring}
            <span class="rounded px-1.5 py-0.5 text-[10.5px]"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-text-dim);"
                  title="Recurring">&#8634;</span>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Actions (visible on mobile, no hover needed) -->
    <div class="flex shrink-0 items-center gap-1">
      {#if onFocusClick && !isDone}
        <button onclick={(e) => { e.stopPropagation(); onFocusClick?.(task.id, task.title); }}
                class="rounded-lg p-1.5" style="color: var(--sempa-text-dim);"
                title="Start focus timer">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
          </svg>
        </button>
      {/if}
      {#if onTrash}
        <button onclick={(e) => { e.stopPropagation(); onTrash?.(task.id, task.title); }}
                class="rounded-lg p-1.5" style="color: var(--sempa-text-dim);"
                title="Delete task">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
          </svg>
        </button>
      {/if}
    </div>
  </div>
</div>
