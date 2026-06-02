<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { formatDate, formatMinutes, isToday } from '$lib/utils';

  let date = $derived($page.params.date ?? new Date().toISOString().split('T')[0]);

  let step       = $state<1 | 2 | 3>(1);
  let doneTasks  = $state<Task[]>([]);
  let pendingTasks = $state<Task[]>([]);
  let wins       = $state<string[]>(['']);
  let reflection = $state('');
  let saving     = $state(false);
  let error      = $state<string | null>(null);

  let winInputs: (HTMLInputElement | undefined)[] = $state([]);

  onMount(async () => {
    try {
      const tasks = await api.tasks.listByDate(date);
      doneTasks    = tasks.filter(t => t.status === 'done');
      pendingTasks = tasks.filter(t => t.status !== 'done' && t.status !== 'cancelled');

      // Load existing plan data if any
      try {
        const plan = await api.plans.get(date);
        if (plan.reflection) reflection = plan.reflection;
        if (plan.wins) {
          const parsed = JSON.parse(plan.wins) as string[];
          wins = parsed.length > 0 ? parsed : [''];
        }
      } catch { /* 404 expected */ }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    }
  });

  function updateWin(i: number, val: string) {
    wins = wins.map((w, idx) => idx === i ? val : w);
  }

  function addWin() {
    wins = [...wins, ''];
    setTimeout(() => winInputs[wins.length - 1]?.focus(), 0);
  }

  function handleWinKeydown(e: KeyboardEvent, i: number) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (i === wins.length - 1) {
        addWin();
      } else {
        winInputs[i + 1]?.focus();
      }
    }
    if (e.key === 'Backspace' && wins[i] === '' && wins.length > 1) {
      e.preventDefault();
      wins = wins.filter((_, idx) => idx !== i);
      setTimeout(() => winInputs[Math.max(0, i - 1)]?.focus(), 0);
    }
  }

  const totalMinutes = $derived(doneTasks.reduce((s, t) => s + (t.time_actual_minutes ?? t.time_estimate_minutes ?? 0), 0));

  async function finish() {
    saving = true;
    error  = null;
    try {
      const cleanWins = wins.filter(w => w.trim());
      await api.plans.upsert(date, {
        status:     'shutdown_complete',
        reflection: reflection.trim() || null,
        wins:       cleanWins.length > 0 ? JSON.stringify(cleanWins) : null,
        shutdown_at: new Date().toISOString(),
      });
      goto(`/day/${date}`);
    } catch (e) {
      error  = e instanceof Error ? e.message : 'Failed to save';
      saving = false;
    }
  }
</script>

<svelte:head><title>Shutdown {isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<div class="flex min-h-full flex-col items-center justify-center px-4 py-12">
  <!-- Step indicators -->
  <div class="mb-10 flex items-center gap-2">
    {#each ['Review', 'Wins', 'Reflect'] as label, i}
      <div class="flex items-center gap-2">
        <div class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold
                    {i + 1 <= step ? 'bg-indigo-500 text-white' : 'bg-gray-100 text-gray-400'}">
          {#if i + 1 < step}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {:else}
            {i + 1}
          {/if}
        </div>
        <span class="text-xs {i + 1 === step ? 'font-medium text-gray-700' : 'text-gray-400'}">{label}</span>
        {#if i < 2}
          <div class="h-px w-8 bg-gray-200"></div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="w-full max-w-md">
    <!-- ── Step 1: Review ───────────────────────────────────────────────── -->
    {#if step === 1}
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        <p class="mb-1 text-2xl">🌙</p>
        <h1 class="mb-1 text-xl font-semibold text-gray-900">
          {doneTasks.length > 0 ? 'Great work today!' : 'Wrapping up…'}
        </h1>
        <p class="mb-6 text-sm text-gray-500">
          {doneTasks.length} task{doneTasks.length !== 1 ? 's' : ''} completed
          {#if totalMinutes > 0}· {formatMinutes(totalMinutes)} logged{/if}
        </p>

        {#if doneTasks.length > 0}
          <div class="mb-4 flex flex-col gap-1.5">
            {#each doneTasks as t (t.id)}
              <div class="flex items-center gap-2 rounded-lg bg-green-50 px-3 py-2">
                <svg class="h-3.5 w-3.5 shrink-0 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
                <span class="text-sm text-gray-700">{t.title}</span>
              </div>
            {/each}
          </div>
        {/if}

        {#if pendingTasks.length > 0}
          <details class="mb-4">
            <summary class="cursor-pointer text-xs text-gray-400 hover:text-gray-600">
              {pendingTasks.length} task{pendingTasks.length !== 1 ? 's' : ''} not completed
            </summary>
            <div class="mt-2 flex flex-col gap-1">
              {#each pendingTasks as t (t.id)}
                <p class="text-xs text-gray-400 pl-2">· {t.title}</p>
              {/each}
            </div>
          </details>
        {/if}

        <div class="flex justify-end">
          <button
            onclick={() => step = 2}
            class="flex items-center gap-1.5 rounded-xl bg-indigo-500 px-5 py-2.5 text-sm font-medium
                   text-white hover:bg-indigo-600 transition-colors"
          >
            Continue
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
      </div>

    <!-- ── Step 2: Wins ──────────────────────────────────────────────────── -->
    {:else if step === 2}
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        <p class="mb-1 text-2xl">🏆</p>
        <h1 class="mb-1 text-xl font-semibold text-gray-900">What went well?</h1>
        <p class="mb-6 text-sm text-gray-500">Write down your wins — big or small.</p>

        <div class="flex flex-col gap-2 mb-4">
          {#each wins as win, i}
            <div class="flex items-center gap-2">
              <span class="text-sm text-gray-300">·</span>
              <input
                bind:this={winInputs[i]}
                value={win}
                oninput={(e) => updateWin(i, (e.target as HTMLInputElement).value)}
                onkeydown={(e) => handleWinKeydown(e, i)}
                type="text"
                placeholder="e.g. Fixed the auth bug faster than expected"
                class="flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm
                       text-gray-800 placeholder-gray-400 outline-none focus:border-indigo-300
                       focus:bg-white focus:ring-2 focus:ring-indigo-100"
              />
            </div>
          {/each}
        </div>

        <button onclick={addWin} class="text-xs text-gray-400 hover:text-gray-600 transition-colors">
          + Add win (or press Enter)
        </button>

        <div class="mt-6 flex justify-between">
          <button onclick={() => step = 1} class="px-4 py-2 text-sm text-gray-400 hover:text-gray-600 transition-colors">
            ← Back
          </button>
          <button
            onclick={() => step = 3}
            class="flex items-center gap-1.5 rounded-xl bg-indigo-500 px-5 py-2.5 text-sm font-medium
                   text-white hover:bg-indigo-600 transition-colors"
          >
            Continue
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
      </div>

    <!-- ── Step 3: Reflect ───────────────────────────────────────────────── -->
    {:else}
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        <p class="mb-1 text-2xl">💭</p>
        <h1 class="mb-1 text-xl font-semibold text-gray-900">Any reflections?</h1>
        <p class="mb-6 text-sm text-gray-500">Blockers, learnings, or things to improve tomorrow.</p>

        <textarea
          bind:value={reflection}
          rows="4"
          placeholder="Optional — what would you do differently?"
          class="w-full resize-none rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm
                 text-gray-800 placeholder-gray-400 outline-none focus:border-indigo-300 focus:bg-white
                 focus:ring-2 focus:ring-indigo-100"
        ></textarea>

        {#if error}<p class="mt-2 text-xs text-red-500">{error}</p>{/if}

        <div class="mt-6 flex justify-between">
          <button onclick={() => step = 2} class="px-4 py-2 text-sm text-gray-400 hover:text-gray-600 transition-colors">
            ← Back
          </button>
          <button
            onclick={finish}
            disabled={saving}
            class="flex items-center gap-2 rounded-xl bg-indigo-500 px-6 py-2.5 text-sm font-medium
                   text-white hover:bg-indigo-600 disabled:opacity-50 transition-colors"
          >
            {saving ? 'Saving…' : 'Finish day'}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
