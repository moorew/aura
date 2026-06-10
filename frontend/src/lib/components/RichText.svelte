<script lang="ts">
  // Renders free-text notes with any URLs turned into clean, tappable link
  // chips (favicon + host/path) instead of long raw strings that overflow the
  // layout. Plain text keeps its line breaks; everything wraps. Shared by the
  // mobile task detail view and the focus screen so it behaves the same on
  // every platform.
  import { prettyUrl } from '$lib/utils';

  let { text }: { text: string } = $props();

  // Split on URLs, trimming trailing punctuation that's clearly not part of the
  // link (e.g. a period or closing paren at the end of a sentence).
  const parts = $derived.by(() => {
    const out: { url: boolean; value: string }[] = [];
    const re = /(https?:\/\/[^\s]+)/g;
    let last = 0;
    let m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
      let raw = m[0];
      let trailing = '';
      const tm = raw.match(/[).,;:!?\]]+$/);
      if (tm) { trailing = tm[0]; raw = raw.slice(0, -trailing.length); }
      if (m.index > last) out.push({ url: false, value: text.slice(last, m.index) });
      out.push({ url: true, value: raw });
      if (trailing) out.push({ url: false, value: trailing });
      last = m.index + m[0].length;
    }
    if (last < text.length) out.push({ url: false, value: text.slice(last) });
    return out;
  });

  function hostOf(u: string): string {
    try { return new URL(u).hostname; } catch { return ''; }
  }
  function label(u: string): string {
    try { return prettyUrl(new URL(u)); } catch { return u; }
  }
  const favicon = (host: string) => `https://www.google.com/s2/favicons?domain=${host}&sz=64`;

  // URLs whose favicon failed to load (offline / blocked) → fall back to a glyph.
  let noFav = $state<Set<string>>(new Set());
</script>

<span style="overflow-wrap:anywhere; word-break:break-word; white-space:pre-wrap;">{#each parts as part}{#if part.url}<a
        href={part.value}
        target="_blank"
        rel="noopener noreferrer"
        onclick={(e) => e.stopPropagation()}
        class="mx-0.5 inline-flex max-w-full items-center gap-1.5 rounded-lg px-2 py-1 align-middle text-[13px] no-underline"
        style="background: var(--sempa-bg-main); border: 1px solid var(--sempa-border); color: var(--sempa-accent); vertical-align: middle;"
      >{#if noFav.has(part.value)}<svg class="h-3.5 w-3.5 shrink-0 opacity-70" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M10 13a5 5 0 007.07 0l3-3a5 5 0 00-7.07-7.07l-1.72 1.71M14 11a5 5 0 00-7.07 0l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg>{:else}<img
            src={favicon(hostOf(part.value))}
            alt=""
            class="h-4 w-4 shrink-0 rounded-sm"
            onerror={() => { const s = new Set(noFav); s.add(part.value); noFav = s; }}
          />{/if}<span class="truncate">{label(part.value)}</span></a>{:else}{part.value}{/if}{/each}</span>
