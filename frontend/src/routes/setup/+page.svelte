<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import { today } from '$lib/utils';

  let step = $state(1);
  const TOTAL = 3;

  // Integration connection state (read from the API)
  let gmailConnected   = $state(false);
  let fastmailConnected = $state(false);
  let jiraConnected    = $state(false);
  let loading          = $state(true);

  onMount(async () => {
    const [gmail, fastmail, jira] = await Promise.all([
      api.integrations.gmail.get().catch(() => ({ connected: false })),
      api.integrations.fastmail.get().catch(() => ({ connected: false })),
      api.integrations.jira.get().catch(() => ({ connected: false })),
    ]);
    gmailConnected    = gmail.connected;
    fastmailConnected = fastmail.connected;
    jiraConnected     = jira.connected;
    loading = false;
  });

  async function finish() {
    await api.setup.complete();
    goto('/day/' + today());
  }

  const progressPct = $derived(((step - 1) / TOTAL) * 100);
</script>

<svelte:head><title>Welcome to Sempa</title></svelte:head>

<div class="flex min-h-screen flex-col items-center justify-center px-4 py-12"
     style="background: var(--sempa-bg-main);">

  <!-- Progress bar -->
  <div class="mb-10 w-full max-w-md">
    <div class="h-1 rounded-full overflow-hidden" style="background: var(--sempa-border);">
      <div class="h-full rounded-full transition-all duration-500"
           style="width: {progressPct}%; background: var(--sempa-accent);"></div>
    </div>
    <p class="mt-2 text-right text-xs" style="color: var(--sempa-text-dim);">Step {step} of {TOTAL}</p>
  </div>

  <div class="w-full max-w-md">

    <!-- ── Step 1: Welcome ───────────────────────────────────────────────── -->
    {#if step === 1}
      <div class="text-center">
        <div class="mb-6 inline-flex h-16 w-16 items-center justify-center rounded-2xl shadow-lg"
             style="background: var(--sempa-accent);">
          <svg width="36" height="36" viewBox="0 0 100 100" fill="none" aria-hidden="true">
            <path d="M22,40 a28,28 0 0 0 56,0" stroke="white" stroke-width="10" stroke-linecap="round"/>
            <circle cx="50" cy="35" r="8" fill="white"/>
          </svg>
        </div>

        <h1 class="mb-2 text-2xl font-semibold" style="color: var(--sempa-text);">
          Welcome to Sempa
        </h1>
        <p class="mb-8 text-sm leading-relaxed" style="color: var(--sempa-text-soft);">
          Your personal daily planner — plan your day, track your work,<br>
          and end each day with intention.
        </p>

        <div class="mb-8 space-y-3 rounded-xl border p-5 text-left"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each [
            ['📅', 'Daily Kanban', 'Plan what you\'re working on each day with a simple board.'],
            ['📬', 'Email → Tasks', 'Import starred emails from Gmail or Fastmail directly as tasks.'],
            ['⏱', 'Pomodoro + timeboxing', 'Schedule focused blocks and track sessions per task.'],
            ['🔁', 'Shutdown ritual', 'End your day deliberately — review what got done, plan tomorrow.'],
          ] as [icon, title, desc]}
            <div class="flex items-start gap-3">
              <span class="text-lg leading-none mt-0.5">{icon}</span>
              <div>
                <p class="text-sm font-medium" style="color: var(--sempa-text);">{title}</p>
                <p class="text-xs" style="color: var(--sempa-text-soft);">{desc}</p>
              </div>
            </div>
          {/each}
        </div>

        <button onclick={() => step = 2}
                class="w-full rounded-xl py-3 text-sm font-semibold text-white transition-all hover:opacity-90 active:scale-[.98]"
                style="background: var(--sempa-accent);">
          Get started →
        </button>
      </div>

    <!-- ── Step 2: Connect tools ─────────────────────────────────────────── -->
    {:else if step === 2}
      <div>
        <h2 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">Connect your tools</h2>
        <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">
          All integrations are optional — you can set them up any time in Settings.
        </p>

        {#if loading}
          <div class="flex justify-center py-8">
            <div class="h-5 w-5 animate-spin rounded-full border-2"
                 style="border-color: var(--sempa-border); border-top-color: var(--sempa-accent);"></div>
          </div>
        {:else}
          <div class="space-y-3">

            <!-- Gmail -->
            <div class="flex items-center gap-4 rounded-xl border p-4"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-red-50 dark:bg-red-950">
                <svg class="h-5 w-5 text-red-500" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
                </svg>
              </div>
              <div class="flex-1">
                <p class="text-sm font-medium" style="color: var(--sempa-text);">Gmail</p>
                <p class="text-xs" style="color: var(--sempa-text-soft);">Import starred emails as tasks</p>
              </div>
              {#if gmailConnected}
                <span class="text-xs font-medium text-green-600 dark:text-green-400">Connected ✓</span>
              {:else}
                <a href={api.integrations.gmail.authUrl(false)}
                   class="rounded-lg px-3 py-1.5 text-xs font-medium text-white"
                   style="background: var(--sempa-accent);">
                  Connect
                </a>
              {/if}
            </div>

            <!-- Fastmail -->
            <div class="flex items-center gap-4 rounded-xl border p-4"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
                   style="background: var(--sempa-accent-bg);">
                <svg class="h-5 w-5" style="color: var(--sempa-accent);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
                </svg>
              </div>
              <div class="flex-1">
                <p class="text-sm font-medium" style="color: var(--sempa-text);">Fastmail</p>
                <p class="text-xs" style="color: var(--sempa-text-soft);">Email + calendar sync</p>
              </div>
              {#if fastmailConnected}
                <span class="text-xs font-medium text-green-600 dark:text-green-400">Connected ✓</span>
              {:else}
                <a href="/settings/accounts"
                   class="rounded-lg border px-3 py-1.5 text-xs font-medium"
                   style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                  Set up →
                </a>
              {/if}
            </div>

            <!-- Jira -->
            <div class="flex items-center gap-4 rounded-xl border p-4"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
                   style="background: var(--sempa-accent-bg);">
                <svg class="h-5 w-5" style="color: var(--sempa-accent);" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
                </svg>
              </div>
              <div class="flex-1">
                <p class="text-sm font-medium" style="color: var(--sempa-text);">Jira</p>
                <p class="text-xs" style="color: var(--sempa-text-soft);">Sync assigned issues as tasks</p>
              </div>
              {#if jiraConnected}
                <span class="text-xs font-medium text-green-600 dark:text-green-400">Connected ✓</span>
              {:else}
                <a href="/settings/integrations/jira"
                   class="rounded-lg border px-3 py-1.5 text-xs font-medium"
                   style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
                  Set up →
                </a>
              {/if}
            </div>

          </div>
        {/if}

        <div class="mt-6 flex gap-3">
          <button onclick={() => step = 1}
                  class="rounded-xl border px-4 py-2.5 text-sm font-medium transition-colors"
                  style="border-color: var(--sempa-border); color: var(--sempa-text-soft);">
            ← Back
          </button>
          <button onclick={() => step = 3}
                  class="flex-1 rounded-xl py-2.5 text-sm font-semibold text-white transition-all hover:opacity-90"
                  style="background: var(--sempa-accent);">
            Continue →
          </button>
        </div>
        <button onclick={finish}
                class="mt-3 w-full text-center text-xs transition-colors"
                style="color: var(--sempa-text-dim);">
          Skip and go to the app
        </button>
      </div>

    <!-- ── Step 3: All set ───────────────────────────────────────────────── -->
    {:else if step === 3}
      <div class="text-center">
        <div class="mb-6 text-5xl">🎉</div>

        <h2 class="mb-2 text-2xl font-semibold" style="color: var(--sempa-text);">You're all set!</h2>
        <p class="mb-8 text-sm" style="color: var(--sempa-text-soft);">
          Here are a few things to try first.
        </p>

        <div class="mb-8 space-y-2 rounded-xl border p-5 text-left"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each [
            ['n', 'Press  to create a new task on the Today view'],
            ['?', 'Press  to see all keyboard shortcuts'],
            ['→', 'Drag emails from the right panel onto a day column to create tasks'],
            ['⚙', 'Visit Settings to connect more integrations any time'],
          ] as [key, desc]}
            <div class="flex items-center gap-3 py-1">
              <kbd class="shrink-0 rounded px-1.5 py-0.5 font-mono text-xs"
                   style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">{key}</kbd>
              <span class="text-sm" style="color: var(--sempa-text-soft);">{desc}</span>
            </div>
          {/each}
        </div>

        <button onclick={finish}
                class="w-full rounded-xl py-3 text-sm font-semibold text-white transition-all hover:opacity-90 active:scale-[.98]"
                style="background: var(--sempa-accent);">
          Start planning →
        </button>

        <button onclick={() => step = 2}
                class="mt-3 text-xs transition-colors"
                style="color: var(--sempa-text-dim);">
          ← Back to integrations
        </button>
      </div>
    {/if}

  </div>
</div>
