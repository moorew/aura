<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { localApi } from '$lib/tauri/local-api';
  import { hasLocalDb } from '$lib/platform';
  import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
  import type { Task } from '$lib/types';
  import { Bell, BellRing, Check, Clock, X } from 'lucide-svelte';

  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let now = $state(Date.now());

  // Tick every 30s so countdowns stay fresh and a reminder slides from
  // "upcoming" to "rang" without a manual reload.
  let tick: ReturnType<typeof setInterval> | null = null;

  async function load() {
    try {
      tasks = hasLocalDb()
        ? await localApi.tasks.withReminders()
        : await api.tasks.listWithReminders();
    } catch {
      tasks = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
    tick = setInterval(() => (now = Date.now()), 30_000);
  });
  onDestroy(() => {
    if (tick) clearInterval(tick);
  });

  const withTime = $derived(
    tasks
      .filter((t) => t.remind_at)
      .map((t) => ({ task: t, at: new Date(t.remind_at as string).getTime() }))
      .filter((r) => !isNaN(r.at)),
  );
  const upcoming = $derived(withTime.filter((r) => r.at > now).sort((a, b) => a.at - b.at));
  const rang     = $derived(withTime.filter((r) => r.at <= now).sort((a, b) => b.at - a.at));

  function relUpcoming(at: number): string {
    const mins = Math.round((at - now) / 60000);
    if (mins < 1) return 'in <1 min';
    if (mins < 60) return `in ${mins} min`;
    const hrs = Math.round(mins / 60);
    if (hrs < 24) return `in ${hrs}h`;
    const days = Math.round(hrs / 24);
    return `in ${days}d`;
  }
  function relPast(at: number): string {
    const mins = Math.round((now - at) / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins} min ago`;
    const hrs = Math.round(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.round(hrs / 24);
    return `${days}d ago`;
  }
  function clockTime(at: number): string {
    const d = new Date(at);
    return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }
  function dayLabel(at: number): string {
    const d = new Date(at);
    const today = new Date();
    const t0 = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime();
    const d0 = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
    const diff = Math.round((d0 - t0) / 86400000);
    if (diff === 0) return 'Today';
    if (diff === 1) return 'Tomorrow';
    if (diff === -1) return 'Yesterday';
    return d.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
  }

  function open(taskId: string) {
    goto(`/focus/${taskId}`);
  }
  async function markDone(taskId: string) {
    tasks = tasks.filter((t) => t.id !== taskId);
    reminderAlerts.dismiss(taskId);
    await api.tasks.update(taskId, { status: 'done' }).catch(() => {});
  }
  async function clearReminder(taskId: string) {
    tasks = tasks.filter((t) => t.id !== taskId);
    reminderAlerts.dismiss(taskId);
    await api.tasks.update(taskId, { remind_at: '' }).catch(() => {});
  }
</script>

<div class="mx-auto flex h-full max-w-xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <div class="flex h-8 w-8 items-center justify-center rounded-lg"
         style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
      <Bell size={17} strokeWidth={2} />
    </div>
    <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Reminders</h1>
    <a href="/settings/notifications"
       class="ml-auto rounded-lg px-2.5 py-1.5 text-[12.5px] font-medium transition-colors"
       style="color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);">
      Settings
    </a>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-20">
    {#if loading}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if withTime.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <div class="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl"
             style="background: var(--sempa-bg-panel); color: var(--sempa-text-dim);">
          <Bell size={22} strokeWidth={1.75} />
        </div>
        <p class="font-semibold" style="font-size: 14px; color: var(--sempa-text);">No reminders set</p>
        <p class="mt-1 max-w-xs" style="font-size: 12.5px; color: var(--sempa-text-soft);">
          Set "Remind me" on any task and it'll show up here.
        </p>
      </div>
    {:else}

      {#snippet sectionLabel(text: string, count: number)}
        <div class="mb-3 flex items-center gap-2">
          <p style="font-family:monospace; font-size:10.5px; font-weight:700; letter-spacing:0.12em;
             text-transform:uppercase; color:var(--sempa-text-dim)">{text}</p>
          <span class="rounded-full px-1.5 text-[10px] font-semibold"
                style="background: var(--sempa-bg-panel); color: var(--sempa-text-dim);">{count}</span>
        </div>
      {/snippet}

      {#snippet row(task: Task, at: number, fired: boolean)}
        <div class="flex items-center gap-3 px-4 py-3" style:opacity={fired ? 0.92 : 1}
             style:border-bottom="1px solid var(--sempa-border)">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
               style="background: {fired ? 'var(--sempa-accent-bg)' : 'var(--sempa-bg-main)'};
                      color: {fired ? 'var(--sempa-accent)' : 'var(--sempa-text-dim)'};
                      border: 1px solid var(--sempa-border);">
            {#if fired}<BellRing size={15} strokeWidth={2} />{:else}<Clock size={15} strokeWidth={2} />{/if}
          </div>

          <button class="min-w-0 flex-1 text-left" onclick={() => open(task.id)}>
            <p class="truncate font-semibold" style="font-size: 13.5px; color: var(--sempa-text);">{task.title}</p>
            <p style="font-size: 11.5px; color: {fired ? 'var(--sempa-accent)' : 'var(--sempa-text-soft)'};">
              {clockTime(at)} · {dayLabel(at)} · {fired ? `rang ${relPast(at)}` : relUpcoming(at)}
            </p>
          </button>

          {#if fired}
            <button onclick={() => markDone(task.id)} aria-label="Mark done"
                    class="shrink-0 rounded-lg p-1.5 transition-colors"
                    style="color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);"
                    title="Mark done">
              <Check size={15} />
            </button>
          {/if}
          <button onclick={() => clearReminder(task.id)} aria-label="Clear reminder"
                  class="shrink-0 rounded-lg p-1.5 transition-colors"
                  style="color: var(--sempa-text-dim);"
                  title="Clear reminder">
            <X size={15} />
          </button>
        </div>
      {/snippet}

      {#if rang.length > 0}
        {@render sectionLabel('Rang', rang.length)}
        <section class="mb-7 overflow-hidden rounded-xl border"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each rang as r (r.task.id)}
            {@render row(r.task, r.at, true)}
          {/each}
        </section>
      {/if}

      {#if upcoming.length > 0}
        {@render sectionLabel('Upcoming', upcoming.length)}
        <section class="mb-7 overflow-hidden rounded-xl border"
                 style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          {#each upcoming as r (r.task.id)}
            {@render row(r.task, r.at, false)}
          {/each}
        </section>
      {/if}

    {/if}
  </div>
</div>
