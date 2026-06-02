<script lang="ts">
  import { today } from '$lib/utils';

  let { date }: { date: string } = $props();

  const todayStr = today();
  let viewYear  = $state(new Date().getFullYear());
  let viewMonth = $state(new Date().getMonth()); // 0-indexed

  const DAYS   = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'];
  const MONTHS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
  ];

  let cells = $derived.by(() => {
    const first = new Date(viewYear, viewMonth, 1).getDay();
    const days  = new Date(viewYear, viewMonth + 1, 0).getDate();
    const out: (number | null)[] = Array(first).fill(null);
    for (let d = 1; d <= days; d++) out.push(d);
    return out;
  });

  function toStr(y: number, m: number, d: number) {
    return `${y}-${String(m + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
  }

  function prev() {
    if (viewMonth === 0) { viewMonth = 11; viewYear--; } else viewMonth--;
  }
  function next() {
    if (viewMonth === 11) { viewMonth = 0; viewYear++; } else viewMonth++;
  }
</script>

<div class="select-none px-4 py-3">
  <!-- Month header -->
  <div class="mb-3 flex items-center justify-between">
    <span class="text-xs font-semibold text-gray-700 dark:text-gray-200">
      {MONTHS[viewMonth]} {viewYear}
    </span>
    <div class="flex gap-0.5">
      <button onclick={prev} aria-label="Previous month"
              class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:hover:bg-gray-700 dark:hover:text-gray-300">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <button onclick={next} aria-label="Next month"
              class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:hover:bg-gray-700 dark:hover:text-gray-300">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- Day-of-week headers -->
  <div class="mb-1 grid grid-cols-7">
    {#each DAYS as day}
      <div class="flex items-center justify-center py-0.5 text-center text-[10px] font-medium
                  text-gray-400 dark:text-gray-600">{day}</div>
    {/each}
  </div>

  <!-- Day cells -->
  <div class="grid grid-cols-7 gap-y-0.5">
    {#each cells as cell}
      {#if cell === null}
        <div></div>
      {:else}
        {@const ds = toStr(viewYear, viewMonth, cell)}
        {@const isToday = ds === todayStr}
        {@const isSel = ds === date}
        <div class="flex items-center justify-center">
          <span class="flex h-6 w-6 items-center justify-center rounded-full text-[11px]
                       {isToday && isSel
                         ? 'bg-blue-500 text-white font-bold'
                         : isToday
                           ? 'bg-blue-100 text-blue-700 font-semibold dark:bg-blue-900/60 dark:text-blue-300'
                           : isSel
                             ? 'bg-gray-200 text-gray-800 font-medium dark:bg-gray-600 dark:text-gray-100'
                             : 'text-gray-500 dark:text-gray-400'}">
            {cell}
          </span>
        </div>
      {/if}
    {/each}
  </div>
</div>
