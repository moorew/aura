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

<div class="mx-auto max-w-xl px-6 py-10 space-y-8 animate-fade-in"
     style="padding-top: calc(env(safe-area-inset-top, 0px) + 40px);">

  <!-- Header -->
  <div>
    <a href="/week/{ws}" class="text-xs transition-colors"
       style="color: var(--sempa-text-dim);"
       onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-accent)'}
       onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
      ← Back to week
    </a>
    <h1 class="mt-2 text-xl font-bold" style="color: var(--sempa-text);">Weekly Review</h1>
    <p class="text-sm" style="color: var(--sempa-text-soft);">{formatWeekRange(ws)}</p>
  </div>

  <!-- Step indicators -->
  <div class="flex items-center gap-2">
    {#each STEPS as s}
      <button onclick={() => step = s.n}
              class="flex items-center gap-1.5 transition-colors"
              style={step === s.n
                ? 'background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:9999px; padding:4px 14px; font-size:12px; font-weight:500;'
                : step > s.n
                  ? 'background: var(--sempa-success-soft); color: var(--sempa-success); border-radius:9999px; padding:4px 14px; font-size:12px; font-weight:500;'
                  : 'background: var(--sempa-accent-bg); color: var(--sempa-text-dim); border-radius:9999px; padding:4px 14px; font-size:12px;'}>
        {s.icon} {s.label}
      </button>
      {#if s.n < STEPS.length}
        <div class="h-px flex-1" style="background: var(--sempa-border);"></div>
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
      <h2 class="text-base font-semibold" style="color: var(--sempa-text);">This week at a glance</h2>

      <div class="grid grid-cols-3 gap-3">
        <div class="rounded-xl p-4 text-center" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <p class="text-2xl font-bold" style="color: var(--sempa-accent);">{doneTasks.length}</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">of {totalTasks.length} tasks done</p>
        </div>
        <div class="rounded-xl p-4 text-center" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <p class="text-2xl font-bold text-green-600">{doneObjs}</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">of {objectives.length} objectives</p>
        </div>
        <div class="rounded-xl p-4 text-center" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <p class="text-2xl font-bold" style="color: var(--sempa-amber);">{totalMins > 0 ? formatMinutes(totalMins) : '—'}</p>
          <p class="text-xs" style="color: var(--sempa-text-soft);">time logged</p>
        </div>
      </div>

      {#if doneTasks.length > 0}
        <div>
          <p class="mb-2" style="font-family:monospace; font-size:10px; font-weight:700; letter-spacing:0.12em;
                   text-transform:uppercase; color:var(--sempa-text-dim)">Completed tasks</p>
          <ul class="space-y-1">
            {#each doneTasks.slice(0, 8) as t}
              <li class="flex items-center gap-2 text-sm" style="color: var(--sempa-text-soft);">
                <svg class="h-3.5 w-3.5 shrink-0 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
                <span class="truncate">{t.title}</span>
              </li>
            {/each}
            {#if doneTasks.length > 8}
              <li class="text-xs pl-5" style="color: var(--sempa-text-dim);">+{doneTasks.length - 8} more</li>
            {/if}
          </ul>
        </div>
      {/if}

      <button onclick={() => step = 2}
              class="w-full transition-colors"
              style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                     padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;"
              onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
              onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
        Continue →
      </button>
    </div>

  <!-- Step 2: Reflection -->
  {:else if step === 2}
    <div class="space-y-6">
      <h2 class="text-base font-semibold" style="color: var(--sempa-text);">How did the week go?</h2>

      <div>
        <label class="mb-2 block text-sm font-medium" style="color: var(--sempa-text-soft);">
          🏆 What went well?
        </label>
        <div class="space-y-1.5 rounded-xl px-3 py-2"
             style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);">
          {#each wins as w, i}
            <div class="flex items-center gap-2">
              <span class="text-sm" style="color: var(--sempa-text-dim);">·</span>
              <input value={w}
                     oninput={(e) => updateBullet(wins, i, (e.target as HTMLInputElement).value, v => wins = v)}
                     onkeydown={(e) => handleBulletKey(e, wins, i, v => wins = v)}
                     type="text"
                     placeholder="Something that went well…"
                     class="flex-1 bg-transparent text-sm outline-none"
                     style="color: var(--sempa-text);" />
            </div>
          {/each}
          <button onclick={() => addBullet(wins, v => wins = v)}
                  class="text-xs pl-4" style="color: var(--sempa-accent);">
            + Add
          </button>
        </div>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium" style="color: var(--sempa-text-soft);">
          🚧 What was challenging?
        </label>
        <div class="space-y-1.5 rounded-xl px-3 py-2"
             style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);">
          {#each challenges as c, i}
            <div class="flex items-center gap-2">
              <span class="text-sm" style="color: var(--sempa-text-dim);">·</span>
              <input value={c}
                     oninput={(e) => updateBullet(challenges, i, (e.target as HTMLInputElement).value, v => challenges = v)}
                     onkeydown={(e) => handleBulletKey(e, challenges, i, v => challenges = v)}
                     type="text"
                     placeholder="Something that was hard…"
                     class="flex-1 bg-transparent text-sm outline-none"
                     style="color: var(--sempa-text);" />
            </div>
          {/each}
          <button onclick={() => addBullet(challenges, v => challenges = v)}
                  class="text-xs pl-4" style="color: var(--sempa-accent);">
            + Add
          </button>
        </div>
      </div>

      <div class="flex gap-2">
        <button onclick={() => step = 1}
                class="flex-1 transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: transparent; border-radius:12px; padding:10px; font-size:14px;">
          ← Back
        </button>
        <button onclick={() => step = 3}
                class="flex-1 transition-colors"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                       padding:10px; font-size:14px; font-weight:500; border:none; cursor:pointer;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          Continue →
        </button>
      </div>
    </div>

  <!-- Step 3: Next week -->
  {:else if step === 3}
    <div class="space-y-6">
      <h2 class="text-base font-semibold" style="color: var(--sempa-text);">Looking ahead</h2>

      <div>
        <label class="mb-2 block text-sm font-medium" style="color: var(--sempa-text-soft);" for="next-focus">
          🎯 What is your focus for next week?
        </label>
        <textarea id="next-focus" bind:value={nextFocus} rows="4"
                  placeholder="Your intention for the week ahead…"
                  onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 2.5px rgba(179,89,46,0.12)'; }}
                  onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }}
                  style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);
                         border-radius:12px; padding:14px; resize:none; width:100%;
                         font-size:14px; line-height:1.65; color:var(--sempa-text); outline:none;"></textarea>
      </div>

      {#if error}
        <p class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>
      {/if}

      <div class="flex gap-2">
        <button onclick={() => step = 2}
                class="flex-1 transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);
                       background: transparent; border-radius:12px; padding:10px; font-size:14px;">
          ← Back
        </button>
        <button onclick={save} disabled={saving}
                class="flex-1 disabled:opacity-40 transition-colors"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                       padding:10px; font-size:14px; font-weight:500; border:none; cursor:pointer;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          {saving ? 'Saving…' : 'Complete review ✓'}
        </button>
      </div>
    </div>
  {/if}
</div>
