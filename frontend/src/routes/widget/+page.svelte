<script lang="ts">
  import { onMount } from 'svelte';
  import { isTauri } from '$lib/tauri/bridge';
  import { query } from '$lib/tauri/db';
  import { today } from '$lib/utils';
  import { Check } from 'lucide-svelte';

  interface WidgetTask {
    id: string;
    title: string;
    status: string;
  }

  let tasks = $state<WidgetTask[]>([]);
  let doneCount = $derived(tasks.filter((t) => t.status === 'done').length);
  let totalCount = $derived(tasks.length);
  let progress = $derived(totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0);

  onMount(() => {
    if (!isTauri()) return;
    loadTasks();
    const interval = setInterval(loadTasks, 30000);
    return () => clearInterval(interval);
  });

  async function loadTasks() {
    try {
      const todayDate = today();
      tasks = await query<WidgetTask[]>(
        `SELECT id, title, status FROM tasks
         WHERE planned_date = ? AND status != 'cancelled'
         ORDER BY position ASC
         LIMIT 8`,
        [todayDate],
      );
    } catch {
      // silently fail in widget context
    }
  }
</script>

<svelte:head>
  <style>
    body { background: transparent !important; overflow: hidden; }
  </style>
</svelte:head>

<div class="widget" data-tauri-drag-region>
  <!-- Header -->
  <div class="widget-header">
    <span class="widget-logo">sempa</span>
    <span class="widget-progress">{progress}%</span>
  </div>

  <!-- Progress bar -->
  <div class="widget-bar">
    <div class="widget-bar-fill" style="width: {progress}%"></div>
  </div>

  <!-- Task grid: 4x2 -->
  <div class="widget-grid">
    {#each tasks.slice(0, 8) as task}
      <div class="widget-cell" class:done={task.status === 'done'} class:active={task.status === 'in_progress'}>
        {#if task.status === 'done'}
          <Check size={10} strokeWidth={3} />
        {:else if task.status === 'in_progress'}
          <div class="pulse-dot"></div>
        {:else}
          <div class="empty-dot"></div>
        {/if}
        <span class="widget-cell-title">{task.title}</span>
      </div>
    {/each}
  </div>

  <!-- Footer -->
  <div class="widget-footer">
    {doneCount}/{totalCount} tasks done
  </div>
</div>

<style>
  .widget {
    width: 304px;
    padding: 12px;
    border-radius: 12px;
    background: var(--sempa-bg-panel);
    border: 1px solid var(--sempa-border);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: var(--sempa-text);
    cursor: grab;
  }

  .widget-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .widget-logo {
    font-size: 13px;
    font-weight: 600;
    letter-spacing: -0.02em;
    color: var(--sempa-accent);
  }

  .widget-progress {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    font-weight: 600;
    color: var(--sempa-text-soft);
  }

  .widget-bar {
    height: 4px;
    border-radius: 2px;
    background: var(--sempa-border);
    margin-bottom: 12px;
    overflow: hidden;
  }

  .widget-bar-fill {
    height: 100%;
    border-radius: 2px;
    background: var(--sempa-accent);
    transition: width 500ms ease-out;
  }

  .widget-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4px;
  }

  .widget-cell {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 8px;
    border-radius: 6px;
    background: var(--sempa-bg-main);
    border: 1px solid var(--sempa-border);
    min-height: 28px;
  }

  .widget-cell.done {
    opacity: 0.5;
  }

  .widget-cell.active {
    border-color: var(--sempa-accent);
    background: var(--sempa-accent-bg);
  }

  .widget-cell-title {
    font-size: 10px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
  }

  .empty-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    border: 1.5px solid var(--sempa-text-dim);
    flex-shrink: 0;
  }

  .pulse-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--sempa-accent);
    flex-shrink: 0;
    animation: pulse 2s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .widget-footer {
    margin-top: 8px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    color: var(--sempa-text-dim);
    text-align: center;
  }

  @media (prefers-reduced-motion: reduce) {
    .pulse-dot { animation: none; }
    .widget-bar-fill { transition: none; }
  }
</style>
