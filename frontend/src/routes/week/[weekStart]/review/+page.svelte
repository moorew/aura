<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Objective, Task, WeekReview } from '$lib/types';
  import { formatMinutes, formatWeekRange, weekStart as calcWeekStart } from '$lib/utils';

  let ws = $derived($page.params.weekStart ?? calcWeekStart(new Date().toISOString().split('T')[0]));

  let step = $state(1); // 1=stats, 2=reflection, 3=next-week
  let loading = $state(true);
  let saving  = $state(false);
  let error   = $state('');

  let tasks      = $state<Task[]>([]);
  let objectives = $state<Objective[]>([]);
  let review     = $state<WeekReview | null>(null);

  // Step 2 inputs (wins / challenges as bullet arrays)
  let wins       = $state<string[]>(['']);
  let challenges = $state<string[]>(['']);
  let nextFocus  = $state('');

  async function load() {
    loading = true;
    try {
      [tasks, objectives, review] = await Promise.all([
        api.tasks.listByWeek(ws),
        api.objectives.listByWeek(ws),
        api.weeks.getReview(ws).catch(() => null),
      ]);
      if (review) {
        wins       = review.wins       ? JSON.parse(review.wins)       : [''];
        challenges = review.challenges ? JSON.parse(review.challenges) : [''];
        nextFocus  = review.next_focus ?? '';
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  // Stats
  const doneTasks  = $derived(tasks.filter(t => t.status === 'done'));
  const totalTasks = $derived(tasks.filter(t => t.status !== 'cancelled'));
  const totalMins  = $derived(doneTasks.reduce((s, t) => s + (t.time_actual_minutes ?? t.time_estimate_minutes ?? 0), 0));
  const doneObjs   = $derived(objectives.filter(o => o.status === 'completed').length);

  function addBullet(arr: string[], setter: (v: string[]) => void) {
    setter([...arr, '']);
  }
  function updateBullet(arr: string[], i: number, val: string, setter: (v: string[]) => void) {
    const copy = [...arr]; copy[i] = val; setter(copy);
  }
  function removeBullet(arr: string[], i: number, setter: (v: string[]) => void) {
    if (arr.length <= 1) { setter(['']); return; }
    setter(arr.filter((_, j) => j !== i));
  }
  function handleBulletKey(e: KeyboardEvent, arr: string[], i: number, setter: (v: string[]) => void) {
    if (e.key === 'Enter') { e.preventDefault(); addBullet(arr, setter); }
    if (e.key === 'Backspace' && (e.target as HTMLInputElement).value === '' && arr.length > 1) {
      e.preventDefault(); removeBullet(arr, i, setter);
    }
  }

  async function save() {
    saving = true;
    try {
      const w = wins.filter(s => s.trim());
      const c = challenges.filter(s => s.trim());
      await api.weeks.upsertReview(ws, {
        wins:       w.length ? JSON.stringify(w) : null,
        challenges: c.length ? JSON.stringify(c) : null,
        next_focus: nextFocus.trim() || null,
      });
      goto(`/week/${ws}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    } finally {
      saving = false;
    }
  }

  const STEPS = [
    { n: 1, icon: '📊', label: 'Stats' },
    { n: 2, icon: '💭', label: 'Reflect' },
    { n: 3, icon: '🎯', label: 'Next week' },
  ];
</script>

<svelte:head><title>Weekly Review — {formatWeekRange(ws)}</title></svelte:head>

<div class="mx-auto max-w-xl px-6 py-10 space-y-8">

  <!-- Header -->
  <div>
    <a href="/week/{ws}" class="text-xs text-gray-400 hover:text-gray-600 dark:text-gray-600 dark:hover:text-gray-400">
      ← Back to week
    </a>
    <h1 class="mt-2 text-xl font-bold text-gray-900 dark:text-gray-50">Weekly Review</h1>
    <p class="text-sm text-gray-500 dark:text-gray-500">{formatWeekRange(ws)}</p>
  </div>

  <!-- Step indicators -->
  <div class="flex items-center gap-2">
    {#each STEPS as s}
      <button onclick={() => step = s.n}
              class="flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors
                     {step === s.n
                       ? 'bg-blue-500 text-white'
                       : step > s.n
                         ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-400'
                         : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500'}">
        {s.icon} {s.label}
      </button>
      {#if s.n < STEPS.length}
        <div class="h-px flex-1 bg-gray-200 dark:bg-gray-800"></div>
      {/if}
    {/each}
  </div>

  {#if loading}
    <div class="flex h-40 items-center justify-center text-sm text-gray-400">Loading…</div>
  {:else if error}
    <p class="rounded-xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>

  <!-- Step 1: Stats -->
  {:else if step === 1}
    <div class="space-y-4">
      <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100">This week at a glance</h2>

      <div class="grid grid-cols-3 gap-3">
        <div class="rounded-xl border border-gray-100 bg-gray-50 p-4 text-center dark:border-gray-800 dark:bg-gray-800/60">
          <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">{doneTasks.length}</p>
          <p class="text-xs text-gray-500 dark:text-gray-500">of {totalTasks.length} tasks done</p>
        </div>
        <div class="rounded-xl border border-gray-100 bg-gray-50 p-4 text-center dark:border-gray-800 dark:bg-gray-800/60">
          <p class="text-2xl font-bold text-green-600 dark:text-green-400">{doneObjs}</p>
          <p class="text-xs text-gray-500 dark:text-gray-500">of {objectives.length} objectives</p>
        </div>
        <div class="rounded-xl border border-gray-100 bg-gray-50 p-4 text-center dark:border-gray-800 dark:bg-gray-800/60">
          <p class="text-2xl font-bold text-purple-600 dark:text-purple-400">{totalMins > 0 ? formatMinutes(totalMins) : '—'}</p>
          <p class="text-xs text-gray-500 dark:text-gray-500">time logged</p>
        </div>
      </div>

      {#if doneTasks.length > 0}
        <div>
          <p class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-400">Completed tasks</p>
          <ul class="space-y-1">
            {#each doneTasks.slice(0, 8) as t}
              <li class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
                <svg class="h-3.5 w-3.5 shrink-0 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
                <span class="truncate">{t.title}</span>
              </li>
            {/each}
            {#if doneTasks.length > 8}
              <li class="text-xs text-gray-400 pl-5">+{doneTasks.length - 8} more</li>
            {/if}
          </ul>
        </div>
      {/if}

      <button onclick={() => step = 2}
              class="w-full rounded-xl bg-blue-500 py-2.5 text-sm font-medium text-white hover:bg-blue-600 transition-colors">
        Continue →
      </button>
    </div>

  <!-- Step 2: Reflection -->
  {:else if step === 2}
    <div class="space-y-6">
      <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100">How did the week go?</h2>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          🏆 What went well?
        </label>
        <div class="space-y-1.5 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-gray-800/60">
          {#each wins as w, i}
            <div class="flex items-center gap-2">
              <span class="text-gray-300 dark:text-gray-600 text-sm">·</span>
              <input value={w}
                     oninput={(e) => updateBullet(wins, i, (e.target as HTMLInputElement).value, v => wins = v)}
                     onkeydown={(e) => handleBulletKey(e, wins, i, v => wins = v)}
                     type="text"
                     placeholder="Something that went well…"
                     class="flex-1 bg-transparent text-sm text-gray-700 placeholder-gray-400 outline-none
                            dark:text-gray-200 dark:placeholder-gray-600" />
            </div>
          {/each}
          <button onclick={() => addBullet(wins, v => wins = v)}
                  class="text-xs text-blue-500 hover:text-blue-700 dark:text-blue-400 pl-4">
            + Add
          </button>
        </div>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          🚧 What was challenging?
        </label>
        <div class="space-y-1.5 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-gray-800/60">
          {#each challenges as c, i}
            <div class="flex items-center gap-2">
              <span class="text-gray-300 dark:text-gray-600 text-sm">·</span>
              <input value={c}
                     oninput={(e) => updateBullet(challenges, i, (e.target as HTMLInputElement).value, v => challenges = v)}
                     onkeydown={(e) => handleBulletKey(e, challenges, i, v => challenges = v)}
                     type="text"
                     placeholder="Something that was hard…"
                     class="flex-1 bg-transparent text-sm text-gray-700 placeholder-gray-400 outline-none
                            dark:text-gray-200 dark:placeholder-gray-600" />
            </div>
          {/each}
          <button onclick={() => addBullet(challenges, v => challenges = v)}
                  class="text-xs text-blue-500 hover:text-blue-700 dark:text-blue-400 pl-4">
            + Add
          </button>
        </div>
      </div>

      <div class="flex gap-2">
        <button onclick={() => step = 1}
                class="flex-1 rounded-xl border border-gray-200 py-2.5 text-sm text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          ← Back
        </button>
        <button onclick={() => step = 3}
                class="flex-1 rounded-xl bg-blue-500 py-2.5 text-sm font-medium text-white hover:bg-blue-600 transition-colors">
          Continue →
        </button>
      </div>
    </div>

  <!-- Step 3: Next week -->
  {:else if step === 3}
    <div class="space-y-6">
      <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100">Looking ahead</h2>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300" for="next-focus">
          🎯 What is your focus for next week?
        </label>
        <textarea id="next-focus" bind:value={nextFocus} rows="4"
                  placeholder="Your intention for the week ahead…"
                  class="w-full resize-none rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm
                         text-gray-800 placeholder-gray-400 outline-none leading-relaxed
                         focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                         dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-100 dark:placeholder-gray-600"></textarea>
      </div>

      {#if error}
        <p class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>
      {/if}

      <div class="flex gap-2">
        <button onclick={() => step = 2}
                class="flex-1 rounded-xl border border-gray-200 py-2.5 text-sm text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          ← Back
        </button>
        <button onclick={save} disabled={saving}
                class="flex-1 rounded-xl bg-blue-500 py-2.5 text-sm font-medium text-white
                       hover:bg-blue-600 disabled:opacity-40 transition-colors">
          {saving ? 'Saving…' : 'Complete review ✓'}
        </button>
      </div>
    </div>
  {/if}
</div>
