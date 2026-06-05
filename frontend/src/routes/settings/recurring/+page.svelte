<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';

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

  let templates = $state<Task[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try { templates = await api.recurring.list(); }
    finally { loading = false; }
  });

  async function remove(id: string) {
    if (!confirm('Delete this recurring task? Future instances will not be generated.')) return;
    await api.recurring.delete(id);
    templates = templates.filter(t => t.id !== id);
  }
</script>

<div class="mx-auto max-w-xl px-6 py-8">
  <a href="/settings/integrations"
     class="mb-6 inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="mb-1 text-xl font-semibold text-gray-900 dark:text-gray-50">Recurring Tasks</h1>
  <p class="mb-6 text-sm text-gray-500 dark:text-gray-400">
    Recurring templates automatically generate a task each day they're due.
    If you didn't complete yesterday's, it carries forward — you'll never see duplicates.
  </p>

  {#if loading}
    <p class="text-sm text-gray-400">Loading…</p>
  {:else if templates.length === 0}
    <div class="rounded-xl border border-dashed border-gray-300 p-8 text-center dark:border-gray-700">
      <p class="text-sm text-gray-500 dark:text-gray-400">No recurring tasks yet.</p>
      <p class="mt-1 text-xs text-gray-400 dark:text-gray-600">
        Create one from the kanban board by choosing a repeat schedule when adding a task.
      </p>
    </div>
  {:else}
    <div class="flex flex-col gap-2">
      {#each templates as tmpl (tmpl.id)}
        <div class="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3
                    dark:border-gray-700 dark:bg-gray-800">
          <span class="text-lg leading-none text-violet-500">↺</span>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate">{tmpl.title}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">{ruleLabel(tmpl.recurrence_rule)}</p>
          </div>
          {#if tmpl.tags?.length}
            <div class="flex gap-1">
              {#each tmpl.tags as tag}
                <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">{tag}</span>
              {/each}
            </div>
          {/if}
          <button onclick={() => remove(tmpl.id)}
                  class="text-gray-400 hover:text-red-500 transition-colors dark:text-gray-600 dark:hover:text-red-400">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M19 7l-.867 12.142A2 2 0 0 1 16.138 21H7.862a2 2 0 0 1-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v3M4 7h16"/>
            </svg>
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>
