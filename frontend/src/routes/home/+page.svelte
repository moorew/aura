<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import type { Objective, Task, WeekReview } from '$lib/types';
  import { today, weekStart, compareTasksForDay } from '$lib/utils';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { tagStore } from '$lib/stores/tags.svelte';

  const todayDate = today();
  const thisWeek  = weekStart(todayDate);

  let todayTasks  = $state<Task[]>([]);
  let objectives  = $state<Objective[]>([]);
  let loading     = $state(true);

  // Only count top-level tasks — sub-tasks inherit their parent's planned_date
  // but render nested inside the parent, so they shouldn't inflate the ring.
  const topLevel     = $derived(todayTasks.filter(t => !t.parent_task_id));
  const doneTasks    = $derived(topLevel.filter(t => t.status === 'done'));
  const activeTasks  = $derived(topLevel.filter(t => t.status !== 'cancelled'));
  const doneFraction = $derived(activeTasks.length > 0 ? doneTasks.length / activeTasks.length : 0);

  const weekTasks       = $derived(objectives.flatMap(() => [])); // placeholder; objectives drive progress
  const doneObjectives  = $derived(objectives.filter(o => o.status === 'completed').length);
  const openObjectives  = $derived(objectives.filter(o => o.status !== 'completed').length);

  const timeEstimate = $derived(
    activeTasks.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0)
  );

  // Open tasks for today: scheduled time blocks first, then by "roughly at"
  // sort hint (recurring tasks), then manual position.
  const openToday = $derived(
    topLevel
      .filter(t => t.status !== 'done' && t.status !== 'cancelled')
      .sort((a, b) => {
        if (a.scheduled_start && b.scheduled_start) return a.scheduled_start < b.scheduled_start ? -1 : 1;
        if (a.scheduled_start) return -1;
        if (b.scheduled_start) return 1;
        return compareTasksForDay(a, b);
      })
  );

  function schedTime(iso: string | null): string {
    if (!iso) return '';
    const d = new Date(iso);
    return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  }

  const circumference = 2 * Math.PI * 30;
  const ringOffset    = $derived(circumference * (1 - doneFraction));

  function greetingPrefix(): string {
    const h = new Date().getHours();
    if (h < 12) return 'Good morning';
    if (h < 17) return 'Good afternoon';
    return 'Good evening';
  }

  function formatDate(d: string): string {
    const dt = new Date(d + 'T12:00:00');
    return dt.toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' });
  }

  function formatMins(mins: number): string {
    if (mins < 60) return `${mins}m`;
    const h = Math.floor(mins / 60);
    const m = mins % 60;
    return m > 0 ? `${h}h ${m}m` : `${h}h`;
  }

  let weekReview = $state<WeekReview | null>(null);
  const reviewDone = $derived(
    !!weekReview && (
      !!weekReview.next_focus?.trim() ||
      (!!weekReview.wins && weekReview.wins !== '[]' && weekReview.wins !== '[""]') ||
      (!!weekReview.challenges && weekReview.challenges !== '[]' && weekReview.challenges !== '[""]')
    )
  );

  async function loadData() {
    [todayTasks, objectives, weekReview] = await Promise.all([
      api.tasks.listByDate(todayDate),
      api.objectives.listByWeek(thisWeek),
      api.weeks.getReview(thisWeek).catch(() => null),
    ]);
    loading = false;
  }

  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'task:change' || ev.type === 'objective:change') void loadData();
  });

  onMount(async () => {
    if (!mobile.value) {
      goto(`/day/${todayDate}`, { replaceState: true });
      return;
    }
    await loadData();
  });
</script>

<svelte:head><title>Today — Sempa</title></svelte:head>

<div class="animate-fade-in" style="padding-top: env(safe-area-inset-top, 0px);">

  <!-- Greeting header -->
  <div class="flex items-start justify-between px-5 pt-5 pb-4">
    <div>
      <p class="text-[11px] font-mono font-semibold uppercase tracking-widest mb-1"
         style="color: var(--sempa-text-dim);">{formatDate(todayDate)}</p>
      <h1 class="text-2xl font-bold" style="color: var(--sempa-text);">{greetingPrefix()}</h1>
    </div>
    <a href="/settings/accounts"
       class="flex h-9 w-9 items-center justify-center rounded-full transition-colors"
       style="background: var(--sempa-bg-panel); color: var(--sempa-text-dim);">
      <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="3"/>
        <path stroke-linecap="round" d="M19.07 4.93A10 10 0 1 0 4.93 19.07M12 2v2m0 18v-2m8-8h2M2 12h2m13.66-7.07 1.41-1.41M4.93 19.07l1.41-1.41M19.07 19.07l1.41 1.41M4.93 4.93 3.51 3.51"/>
      </svg>
    </a>
  </div>

  {#if loading}
    <div class="flex h-40 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>
  {:else}

  <div class="px-4 space-y-3 pb-8">

    <!-- Today card -->
    <a href="/day/{todayDate}"
       class="flex items-center gap-5 rounded-2xl px-5 py-5 transition-opacity active:opacity-70"
       style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">

      <!-- Progress ring -->
      <div class="shrink-0">
        <svg width="74" height="74" viewBox="0 0 74 74" fill="none">
          <!-- Track -->
          <circle cx="37" cy="37" r="30"
                  stroke="var(--sempa-border)" stroke-width="6" fill="none"/>
          <!-- Progress -->
          <circle cx="37" cy="37" r="30"
                  stroke="var(--sempa-accent)" stroke-width="6" fill="none"
                  stroke-linecap="round"
                  transform="rotate(-90 37 37)"
                  stroke-dasharray="{circumference}"
                  stroke-dashoffset="{ringOffset}"
                  style="transition: stroke-dashoffset 600ms ease-out;"/>
          <!-- Count label -->
          <text x="37" y="33" text-anchor="middle" fill="var(--sempa-text)"
                font-size="14" font-weight="700"
                font-family="Plus Jakarta Sans, sans-serif">
            {doneTasks.length}
          </text>
          <text x="37" y="47" text-anchor="middle" fill="var(--sempa-text-dim)"
                font-size="10"
                font-family="Plus Jakarta Sans, sans-serif">
            of {activeTasks.length}
          </text>
        </svg>
      </div>

      <div class="flex-1 min-w-0">
        <p class="text-base font-semibold" style="color: var(--sempa-text);">Today</p>
        <p class="text-sm mt-0.5" style="color: var(--sempa-text-soft);">
          {doneTasks.length} of {activeTasks.length} tasks done
        </p>
        {#if timeEstimate > 0}
          <p class="text-xs mt-1" style="color: var(--sempa-text-dim);">
            ~{formatMins(timeEstimate)} estimated
          </p>
        {/if}
      </div>

      <svg class="h-4 w-4 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
      </svg>
    </a>

    <!-- Today's tasks -->
    {#if openToday.length > 0}
      <div>
        <p class="mb-2 px-1 text-[10.5px] font-bold uppercase tracking-widest"
           style="font-family:monospace; color: var(--sempa-text-dim);">Up Next</p>
        <div class="flex flex-col gap-2">
          {#each openToday.slice(0, 5) as task (task.id)}
            <a href="/day/{todayDate}"
               class="flex items-center gap-3 rounded-xl px-4 py-3 transition-opacity active:opacity-70"
               style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
              <!-- Status dot -->
              <span class="h-2 w-2 shrink-0 rounded-full"
                    style="background: {task.status === 'in_progress' ? 'var(--sempa-accent)' : 'var(--sempa-text-dim)'};"></span>
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium truncate" style="color: var(--sempa-text);">{task.title}</p>
                {#if task.scheduled_start || task.time_estimate_minutes || task.tags.length}
                  <div class="mt-0.5 flex items-center gap-2 text-[11px]" style="color: var(--sempa-text-dim);">
                    {#if task.scheduled_start}<span>{schedTime(task.scheduled_start)}</span>{/if}
                    {#if task.time_estimate_minutes}<span>~{formatMins(task.time_estimate_minutes)}</span>{/if}
                    {#each task.tags.slice(0, 3) as tag}
                      <span class="h-2 w-2 rounded-full" style="background: {tagStore.colorFor(tag)};"></span>
                    {/each}
                  </div>
                {/if}
              </div>
            </a>
          {/each}
          {#if openToday.length > 5}
            <a href="/day/{todayDate}" class="px-1 py-1 text-xs font-medium" style="color: var(--sempa-accent);">
              +{openToday.length - 5} more →
            </a>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Week card -->
    <a href="/week/{thisWeek}"
       class="block rounded-2xl px-5 py-4 transition-opacity active:opacity-70"
       style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
      <div class="flex items-center justify-between mb-3">
        <p class="text-sm font-semibold" style="color: var(--sempa-text);">This Week</p>
        <svg class="h-4 w-4" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </div>
      <!-- Progress bar -->
      <div class="h-2 rounded-full overflow-hidden mb-3" style="background: var(--sempa-border);">
        <div style="width:{objectives.length > 0 ? Math.round((doneObjectives/objectives.length)*100) : 0}%;
                    height:100%; border-radius:9999px; background: var(--sempa-accent);
                    transition: width 500ms ease-out;"></div>
      </div>
      <!-- Stat chips -->
      <div class="flex gap-2">
        <span class="rounded-full px-2.5 py-1 text-xs font-medium"
              style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
          {doneObjectives} done
        </span>
        <span class="rounded-full px-2.5 py-1 text-xs font-medium"
              style="background: var(--sempa-border); color: var(--sempa-text-soft);">
          {openObjectives} open
        </span>
        <span class="rounded-full px-2.5 py-1 text-xs font-medium"
              style="background: var(--sempa-border); color: var(--sempa-text-soft);">
          {objectives.length} objectives
        </span>
      </div>
    </a>

    <!-- Quick actions row -->
    <div class="flex gap-2">
      <a href="/day/{todayDate}"
         class="flex flex-1 items-center justify-center gap-1.5 rounded-xl py-3 font-medium text-sm transition-opacity active:opacity-70"
         style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
        </svg>
        Plan Day
      </a>
      <a href="/week/{thisWeek}/review"
         class="flex flex-1 items-center justify-center gap-1.5 rounded-xl py-3 text-sm transition-opacity active:opacity-70"
         style="background: {reviewDone ? 'var(--sempa-accent-bg)' : 'var(--sempa-bg-panel)'};
                border: 1px solid {reviewDone ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
                color: {reviewDone ? 'var(--sempa-accent)' : 'var(--sempa-text-soft)'};">
        {#if reviewDone}
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
          </svg>
          Reviewed
        {:else}
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
          </svg>
          Review
        {/if}
      </a>
      <a href="/email"
         class="flex flex-1 items-center justify-center gap-1.5 rounded-xl py-3 text-sm transition-opacity active:opacity-70"
         style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
        </svg>
        Inbox
      </a>
    </div>

    <!-- Weekly objectives -->
    {#if objectives.length > 0}
      <div>
        <p class="mb-2 px-1 text-[10.5px] font-bold uppercase tracking-widest"
           style="font-family:monospace; color: var(--sempa-text-dim);">Weekly Objectives</p>
        <div class="flex flex-col gap-2">
          {#each objectives.slice(0, 4) as obj (obj.id)}
            {@const isDone = obj.status === 'completed'}
            <a href="/week/{thisWeek}"
               class="flex items-center gap-3 rounded-xl px-4 py-3 transition-opacity active:opacity-70"
               style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
              <!-- Checkbox -->
              <div class="h-5 w-5 shrink-0 rounded-full border-2 flex items-center justify-center transition-colors
                          {isDone ? 'border-green-500 bg-green-500' : ''}"
                   style="{!isDone ? 'border-color: var(--sempa-border);' : ''}">
                {#if isDone}
                  <svg class="h-3 w-3 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                  </svg>
                {/if}
              </div>
              <!-- Title + progress -->
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium truncate {isDone ? 'line-through' : ''}"
                   style="color: {isDone ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'};">
                  {obj.title}
                </p>
              </div>
              <svg class="h-4 w-4 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
              </svg>
            </a>
          {/each}
        </div>
      </div>
    {:else}
      <a href="/week/{thisWeek}/plan"
         class="flex items-center justify-center gap-2 rounded-xl py-5 transition-opacity active:opacity-70"
         style="border: 2px dashed var(--sempa-border); color: var(--sempa-accent);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
        </svg>
        <span class="text-sm font-medium">Plan your week</span>
      </a>
    {/if}

  </div>
  {/if}
</div>
