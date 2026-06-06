<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { formatDate, formatMinutes, isToday } from '$lib/utils';
  import { Moon, Star, MessageSquare } from 'lucide-svelte';
  import SempaPattern from '$lib/components/ui/SempaPattern.svelte';

  let date = $derived($page.params.date ?? new Date().toISOString().split('T')[0]);

  let step       = $state<1 | 2 | 3>(1);
  let doneTasks  = $state<Task[]>([]);
  let pendingTasks = $state<Task[]>([]);
  let wins       = $state<string[]>(['']);
  let reflection = $state('');
  let saving     = $state(false);
  let error      = $state<string | null>(null);

  let winInputs: (HTMLInputElement | undefined)[] = $state([]);
  let pendingOpen = $state(false);

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

<div class="flex min-h-full flex-col items-center justify-center px-4 py-12 animate-fade-in">
  <!-- Step indicators -->
  <div class="mb-10 flex items-center gap-2">
    {#each ['Review', 'Wins', 'Reflect'] as label, i}
      <div class="flex items-center gap-2">
        <div class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
             style={i + 1 <= step
               ? 'background: var(--sempa-accent); color: var(--sempa-btn-fg);'
               : 'background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text-dim);'}>
          {#if i + 1 < step}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          {:else}
            {i + 1}
          {/if}
        </div>
        <span class="text-xs {i + 1 === step ? 'font-medium' : ''}"
              style="color: {i + 1 === step ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'};">{label}</span>
        {#if i < 2}
          <div class="h-px w-8" style="background: var(--sempa-border);"></div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="w-full max-w-md">
    <!-- ── Step 1: Review ───────────────────────────────────────────────── -->
    {#if step === 1}
      {#key step}
      <div class="animate-fade-in relative overflow-hidden"
           style="border-radius:16px; border: 1px solid var(--sempa-border);
                  background: var(--sempa-bg-panel); padding:28px 28px 24px;">
        <div class="absolute bottom-0 left-0 w-48 h-48 pointer-events-none z-0">
          <SempaPattern motif="garden" class="w-full h-full" opacity={0.8} />
        </div>
        <div class="relative z-10">
        <Moon size={28} style="color:var(--sempa-accent);margin-bottom:8px"/>
        <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">
          {doneTasks.length > 0 ? 'Great work today!' : 'Wrapping up…'}
        </h1>
        <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">
          {doneTasks.length} task{doneTasks.length !== 1 ? 's' : ''} completed
          {#if totalMinutes > 0}· {formatMinutes(totalMinutes)} logged{/if}
        </p>

        {#if doneTasks.length > 0}
          <div class="mb-4 flex flex-col gap-1.5">
            {#each doneTasks as t (t.id)}
              <div style="display:flex; align-items:center; gap:10px; border-radius:9px;
                          background: var(--sempa-accent-bg); padding:9px 12px; margin-bottom:6px;">
                <svg class="h-3.5 w-3.5 shrink-0" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
                <span class="text-sm" style="color: var(--sempa-text);">{t.title}</span>
              </div>
            {/each}
          </div>
        {/if}

        {#if pendingTasks.length > 0}
          <div class="mb-4">
            <button type="button" onclick={() => pendingOpen = !pendingOpen}
                    class="flex items-center gap-1.5 transition-colors"
                    style="color: var(--sempa-text-dim); font-size:12px;"
                    onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-soft)'}
                    onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
              <svg class="h-3.5 w-3.5 transition-transform" style="transform: rotate({pendingOpen ? 90 : 0}deg);"
                   fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
              </svg>
              {pendingTasks.length} task{pendingTasks.length !== 1 ? 's' : ''} not completed
            </button>
            {#if pendingOpen}
              <div class="mt-2 flex flex-col gap-1 pl-5">
                {#each pendingTasks as t (t.id)}
                  <p style="color: var(--sempa-text-dim); font-size:12px;">· {t.title}</p>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <div class="flex justify-end">
          <button
            onclick={() => step = 2}
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                   padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;
                   display:inline-flex; align-items:center; gap:8px;"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}
          >
            Continue
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
        </div><!-- end z-10 -->
      </div>
      {/key}

    <!-- ── Step 2: Wins ──────────────────────────────────────────────────── -->
    {:else if step === 2}
      {#key step}
      <div class="animate-fade-in relative overflow-hidden"
           style="border-radius:16px; border: 1px solid var(--sempa-border);
                  background: var(--sempa-bg-panel); padding:28px 28px 24px;">
        <div class="absolute bottom-0 left-0 w-48 h-48 pointer-events-none z-0">
          <SempaPattern motif="garden" class="w-full h-full" opacity={0.8} />
        </div>
        <div class="relative z-10">
        <Star size={28} style="color:var(--sempa-amber);margin-bottom:8px"/>
        <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">What went well?</h1>
        <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">Write down your wins — big or small.</p>

        <div class="flex flex-col gap-2 mb-4">
          {#each wins as win, i}
            <div class="flex items-center gap-2">
              <span class="text-sm" style="color: var(--sempa-text-dim);">·</span>
              <input
                bind:this={winInputs[i]}
                value={win}
                oninput={(e) => updateWin(i, (e.target as HTMLInputElement).value)}
                onkeydown={(e) => handleWinKeydown(e, i)}
                onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
                onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}
                type="text"
                placeholder="e.g. Fixed the auth bug faster than expected"
                class="flex-1 outline-none"
                style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);
                       border-radius:9px; padding:9px 13px; font-size:14px; color:var(--sempa-text);"
              />
            </div>
          {/each}
        </div>

        <button onclick={addWin} class="text-xs transition-colors" style="color: var(--sempa-text-dim);">
          + Add win (or press Enter)
        </button>

        <div class="mt-6 flex justify-between">
          <button onclick={() => step = 1} class="px-4 py-2 text-sm transition-colors" style="color: var(--sempa-text-dim);">
            ← Back
          </button>
          <button
            onclick={() => step = 3}
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                   padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;
                   display:inline-flex; align-items:center; gap:8px;"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}
          >
            Continue
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </button>
        </div>
        </div><!-- end z-10 -->
      </div>
      {/key}

    <!-- ── Step 3: Reflect ───────────────────────────────────────────────── -->
    {:else}
      {#key step}
      <div class="animate-fade-in relative overflow-hidden"
           style="border-radius:16px; border: 1px solid var(--sempa-border);
                  background: var(--sempa-bg-panel); padding:28px 28px 24px;">
        <div class="absolute bottom-0 left-0 w-48 h-48 pointer-events-none z-0">
          <SempaPattern motif="garden" class="w-full h-full" opacity={0.8} />
        </div>
        <div class="relative z-10">
        <MessageSquare size={28} style="color:var(--sempa-text-soft);margin-bottom:8px"/>
        <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">Any reflections?</h1>
        <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">Blockers, learnings, or things to improve tomorrow.</p>

        <textarea
          bind:value={reflection}
          rows="4"
          placeholder="Optional — what would you do differently?"
          onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
          onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);
                 border-radius:12px; padding:14px; resize:none; width:100%;
                 font-size:14px; color: var(--sempa-text); outline:none;"
        ></textarea>

        {#if error}<p class="mt-2 text-xs text-red-500">{error}</p>{/if}

        <div class="mt-6 flex justify-between">
          <button onclick={() => step = 2} class="px-4 py-2 text-sm transition-colors" style="color: var(--sempa-text-dim);">
            ← Back
          </button>
          <button
            onclick={finish}
            disabled={saving}
            class="disabled:opacity-50"
            style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); border-radius:12px;
                   padding:10px 20px; font-size:14px; font-weight:500; border:none; cursor:pointer;
                   display:inline-flex; align-items:center; gap:8px;"
            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}
          >
            {saving ? 'Saving…' : 'Finish day'}
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
          </button>
        </div>
        </div><!-- end z-10 -->
      </div>
      {/key}
    {/if}
  </div>
</div>
