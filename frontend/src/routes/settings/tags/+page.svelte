<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { tagStore } from '$lib/stores/tags.svelte';
  import type { TagDefinition } from '$lib/types';

  const PALETTE = [
    '#3b82f6','#10b981','#f59e0b','#ef4444',
    '#8b5cf6','#ec4899','#06b6d4','#84cc16',
  ];

  let tags = $state<TagDefinition[]>([]);
  let newName = $state('');
  let newColor = $state(PALETTE[0]);
  let saving = $state(false);
  let error = $state('');

  onMount(async () => {
    tags = await api.tags.list();
  });

  async function create() {
    if (!newName.trim()) return;
    saving = true; error = '';
    try {
      const tag = await api.tags.create(newName.trim(), newColor);
      tags = [...tags, tag];
      tagStore.add(tag);
      newName = '';
      newColor = PALETTE[tags.length % PALETTE.length];
    } catch (e) { error = (e as Error).message; }
    finally { saving = false; }
  }

  async function updateColor(tag: TagDefinition, color: string) {
    const updated = await api.tags.update(tag.id, color);
    tags = tags.map(t => t.id === updated.id ? updated : t);
    tagStore.add(updated);
  }

  async function remove(id: string) {
    await api.tags.delete(id);
    tags = tags.filter(t => t.id !== id);
    tagStore.remove(id);
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

  <h1 class="mb-1 text-xl font-semibold text-gray-900 dark:text-gray-50">Tags</h1>
  <p class="mb-6 text-sm text-gray-500 dark:text-gray-400">
    Create tags to categorise tasks. Use them in task titles as <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">#work</code> or pick from the list.
  </p>

  <!-- Existing tags -->
  <div class="mb-6 flex flex-col gap-2">
    {#each tags as tag (tag.id)}
      <div class="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3
                  dark:border-gray-700 dark:bg-gray-800">
        <input type="color" value={tag.color}
               onchange={(e) => updateColor(tag, (e.target as HTMLInputElement).value)}
               class="h-7 w-7 cursor-pointer rounded border-0 bg-transparent p-0" />
        <span class="flex-1 text-sm font-medium text-gray-800 dark:text-gray-100">{tag.name}</span>
        <span class="rounded-full px-2.5 py-0.5 text-xs text-white font-medium"
              style="background-color: {tag.color}">{tag.name}</span>
        <button onclick={() => remove(tag.id)}
                class="text-gray-400 hover:text-red-500 transition-colors dark:text-gray-600 dark:hover:text-red-400">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M6 18 18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    {:else}
      <p class="text-sm text-gray-400 dark:text-gray-600">No tags yet.</p>
    {/each}
  </div>

  <!-- Create new tag -->
  <div class="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800">
    <p class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-200">New tag</p>
    <div class="flex gap-3">
      <input type="color" bind:value={newColor}
             class="h-10 w-10 cursor-pointer rounded-lg border border-gray-200 bg-transparent p-1
                    dark:border-gray-600" />
      <input type="text" bind:value={newName}
             onkeydown={(e) => e.key === 'Enter' && create()}
             placeholder="Tag name (e.g. work, personal)"
             class="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none
                    focus:border-blue-500 focus:ring-2 focus:ring-blue-100
                    dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 dark:placeholder-gray-500" />
      <button onclick={create} disabled={saving || !newName.trim()}
              class="rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white
                     hover:bg-blue-600 disabled:opacity-40 transition-colors">
        Add
      </button>
    </div>
    {#if error}<p class="mt-2 text-sm text-red-600 dark:text-red-400">{error}</p>{/if}
  </div>
</div>
