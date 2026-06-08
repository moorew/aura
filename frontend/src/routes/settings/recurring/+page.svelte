<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import SempaSelect from '$lib/components/ui/SempaSelect.svelte';
  import { tagStore } from '$lib/stores/tags.svelte';

  const RULE_LABELS: Record<string, string> = {
    daily: 'Every day',
    weekdays: 'Every weekday',
    weekends: 'Every weekend',
  };

  function ruleLabel(rule: string | null): string {
    if (!rule) return '';
    if (RULE_LABELS[rule]) return RULE_LABELS[rule];
    if (rule.startsWith('weekly:')) {
      const days = ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'];
      const nums = rule.replace('weekly:','').split(',');
      return 'Weekly: ' + nums.map(n => days[parseInt(n)] ?? n).join(', ');
    }
    if (rule.startsWith('monthly:')) {
      return `Monthly on the ${rule.replace('monthly:','')}th`;
    }
    return rule;
  }

  // "Roughly at" options — every 30 min, plus "No time". Visual-order hint only.
  const TIME_OPTIONS = [
    { value: null as string | null, label: 'No set time' },
    ...Array.from({ length: 48 }, (_, i) => {
      const h = Math.floor(i / 2), m = (i % 2) * 30;
      const hh = String(h).padStart(2, '0'), mm = String(m).padStart(2, '0');
      const period = h < 12 ? 'AM' : 'PM';
      const h12 = h === 0 ? 12 : h > 12 ? h - 12 : h;
      return { value: `${hh}:${mm}`, label: `${h12}:${mm} ${period}` };
    }),
  ];

  let templates = $state<Task[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try { templates = await api.recurring.list(); }
    finally { loading = false; }
  });

  async function setRoughlyAt(tmpl: Task, value: string | null) {
    tmpl.roughly_at = value;            // optimistic
    templates = templates;
    try {
      await api.tasks.update(tmpl.id, { roughly_at: value });
    } catch { /* keep optimistic value; will reconcile on reload */ }
  }

  async function remove(id: string) {
    if (!confirm('Delete this recurring task? Future instances will not be generated.')) return;
    await api.recurring.delete(id);
    templates = templates.filter(t => t.id !== id);
  }
</script>

<div class="mx-auto max-w-xl px-6 py-8">
  <a href="/settings/accounts"
     class="mb-6 inline-flex items-center gap-1.5 text-sm transition-colors"
     style="color: var(--sempa-text-soft);">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">Recurring Tasks</h1>
  <p class="mb-6 text-sm leading-relaxed" style="color: var(--sempa-text-soft);">
    Each template generates a task on the days it's due. If you don't finish one and
    haven't touched it, it's quietly replaced by the next day's fresh copy. If you've
    added notes or sub-tasks, it carries forward so you keep your work. Set a
    <em>roughly at</em> time to order it in the day — it won't lock to a calendar block.
  </p>

  {#if loading}
    <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
  {:else if templates.length === 0}
    <div class="rounded-xl border border-dashed p-8 text-center" style="border-color: var(--sempa-border);">
      <p class="text-sm" style="color: var(--sempa-text-soft);">No recurring tasks yet.</p>
      <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">
        Create one from the board by choosing a repeat schedule when adding a task.
      </p>
    </div>
  {:else}
    <div class="flex flex-col gap-2">
      {#each templates as tmpl (tmpl.id)}
        <div class="rounded-xl border px-4 py-3"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          <div class="flex items-center gap-3">
            <span class="text-lg leading-none" style="color: var(--sempa-accent);">↺</span>
            <div class="flex-1 min-w-0">
              <p class="truncate text-sm font-medium" style="color: var(--sempa-text);">{tmpl.title}</p>
              <p class="text-xs" style="color: var(--sempa-text-dim);">{ruleLabel(tmpl.recurrence_rule)}</p>
            </div>
            {#if tmpl.tags?.length}
              <div class="flex gap-1">
                {#each tmpl.tags as tag}
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium text-white"
                        style="background-color: {tagStore.colorFor(tag)}">{tag}</span>
                {/each}
              </div>
            {/if}
            <button onclick={() => remove(tmpl.id)} aria-label="Delete recurring task"
                    class="transition-colors" style="color: var(--sempa-text-dim);">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M19 7l-.867 12.142A2 2 0 0 1 16.138 21H7.862a2 2 0 0 1-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v3M4 7h16"/>
              </svg>
            </button>
          </div>

          <!-- Roughly at -->
          <div class="mt-3 flex items-center gap-2 pl-8">
            <span class="text-xs" style="color: var(--sempa-text-dim);">Roughly at</span>
            <div class="w-40">
              <SempaSelect
                value={tmpl.roughly_at}
                options={TIME_OPTIONS}
                placeholder="No set time"
                onchange={(v) => setRoughlyAt(tmpl, (v as string | null) ?? null)} />
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
