<script module lang="ts">
  import { api } from '$lib/api';
  import type { LinkUnfurl } from '$lib/types';

  // Dedupe + cache unfurl requests for the session so the same URL shown in
  // several places (or re-rendered) only hits the API once. The backend caches
  // too; this just avoids redundant round-trips within a page.
  const cache = new Map<string, Promise<LinkUnfurl>>();
  export function unfurlCached(url: string): Promise<LinkUnfurl> {
    let p = cache.get(url);
    if (!p) {
      p = api.unfurl(url);
      cache.set(url, p);
    }
    return p;
  }
</script>

<script lang="ts">
  import { prettyUrl } from '$lib/utils';

  let { url }: { url: string } = $props();

  let imgFailed = $state(false);
  let favFailed = $state(false);

  const host = $derived.by(() => {
    try { return new URL(url).hostname.replace(/^www\./, ''); } catch { return url; }
  });
  const fallbackLabel = $derived.by(() => {
    try { return prettyUrl(new URL(url)); } catch { return url; }
  });
  const favicon = $derived(`https://www.google.com/s2/favicons?domain=${host}&sz=64`);
  const clamp2 = 'display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;';
</script>

<a href={url} target="_blank" rel="noopener noreferrer"
   onclick={(e) => e.stopPropagation()}
   class="block overflow-hidden rounded-xl no-underline transition-colors"
   style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);">
  {#await unfurlCached(url)}
    <!-- Skeleton while fetching -->
    <div class="flex items-center gap-2.5 px-3 py-3">
      <div class="h-8 w-8 shrink-0 rounded-lg" style="background: var(--sempa-border); opacity:.5;"></div>
      <div class="flex-1 space-y-1.5">
        <div class="h-2.5 w-1/2 rounded" style="background: var(--sempa-border); opacity:.5;"></div>
        <div class="h-2.5 w-3/4 rounded" style="background: var(--sempa-border); opacity:.35;"></div>
      </div>
    </div>
  {:then data}
    {#if data.image_url && !imgFailed}
      <img src={data.image_url} alt="" loading="lazy" referrerpolicy="no-referrer"
           class="w-full object-cover" style="max-height: 150px; background: var(--sempa-bg-panel);"
           onerror={() => (imgFailed = true)} />
    {/if}
    <div class="px-3 py-2.5">
      <div class="mb-1 flex items-center gap-1.5">
        {#if !favFailed}
          <img src={favicon} alt="" class="h-3.5 w-3.5 shrink-0 rounded-sm" onerror={() => (favFailed = true)} />
        {:else}
          <svg class="h-3.5 w-3.5 shrink-0 opacity-60" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" style="color: var(--sempa-text-dim);">
            <path stroke-linecap="round" stroke-linejoin="round" d="M10 13a5 5 0 007.07 0l3-3a5 5 0 00-7.07-7.07l-1.72 1.71M14 11a5 5 0 00-7.07 0l-3 3a5 5 0 007.07 7.07l1.71-1.71"/>
          </svg>
        {/if}
        <span class="truncate text-[11px]" style="color: var(--sempa-text-dim);">{data.site_name || host}</span>
      </div>
      <p class="text-sm font-medium leading-snug" style="color: var(--sempa-text); {clamp2}">
        {data.ok && data.title ? data.title : fallbackLabel}
      </p>
      {#if data.description}
        <p class="mt-0.5 text-xs leading-snug" style="color: var(--sempa-text-soft); {clamp2}">{data.description}</p>
      {/if}
    </div>
  {:catch}
    <!-- Unfurl failed → minimal chip so the link is still visible/tappable. -->
    <div class="flex items-center gap-1.5 px-3 py-2.5">
      <svg class="h-3.5 w-3.5 shrink-0 opacity-60" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" style="color: var(--sempa-accent);">
        <path stroke-linecap="round" stroke-linejoin="round" d="M10 13a5 5 0 007.07 0l3-3a5 5 0 00-7.07-7.07l-1.72 1.71M14 11a5 5 0 00-7.07 0l-3 3a5 5 0 007.07 7.07l1.71-1.71"/>
      </svg>
      <span class="truncate text-[13px]" style="color: var(--sempa-accent);">{fallbackLabel}</span>
    </div>
  {/await}
</a>
