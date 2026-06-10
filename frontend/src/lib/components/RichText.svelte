<script lang="ts">
  // Renders free-text notes with smart link handling:
  //  • Markdown links — [label](https://…) — stay INLINE as hyperlinked text.
  //  • Bare pasted URLs — https://… on their own — are pulled OUT of the prose
  //    and shown as rich preview cards in a "Links" section below, so a long
  //    URL never stretches the layout and you get a thumbnail/title preview.
  // Everything else renders as plain, wrapped text.
  import LinkPreview from './LinkPreview.svelte';

  let { text }: { text: string } = $props();

  type Node = { t: 'text'; v: string } | { t: 'link'; label: string; url: string };

  const parsed = $derived.by(() => {
    const nodes: Node[] = [];
    const links: string[] = [];
    const seen = new Set<string>();

    // Pass 1: split out markdown links so they survive as inline hyperlinks.
    const MD = /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g;
    const segments: ({ md: { label: string; url: string } } | { text: string })[] = [];
    let i = 0;
    let m: RegExpExecArray | null;
    while ((m = MD.exec(text)) !== null) {
      if (m.index > i) segments.push({ text: text.slice(i, m.index) });
      segments.push({ md: { label: m[1], url: m[2] } });
      i = m.index + m[0].length;
    }
    if (i < text.length) segments.push({ text: text.slice(i) });

    // Pass 2: in the plain-text segments, lift bare URLs into the links section.
    for (const seg of segments) {
      if ('md' in seg) {
        nodes.push({ t: 'link', label: seg.md.label, url: seg.md.url });
        continue;
      }
      const URL_RE = /(https?:\/\/[^\s)]+)/g;
      let last = 0;
      let rebuilt = '';
      let um: RegExpExecArray | null;
      while ((um = URL_RE.exec(seg.text)) !== null) {
        let raw = um[0];
        let trailing = '';
        const tm = raw.match(/[).,;:!?\]]+$/);
        if (tm) { trailing = tm[0]; raw = raw.slice(0, -trailing.length); }
        rebuilt += seg.text.slice(last, um.index) + trailing; // keep surrounding text, drop the URL
        if (!seen.has(raw)) { seen.add(raw); links.push(raw); }
        last = um.index + um[0].length;
      }
      rebuilt += seg.text.slice(last);
      nodes.push({ t: 'text', v: rebuilt });
    }

    // Tidy whitespace left where URLs were removed (trailing spaces, blank lines).
    for (const n of nodes) {
      if (n.t === 'text') n.v = n.v.replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n');
    }
    if (nodes.length && nodes[0].t === 'text') nodes[0].v = nodes[0].v.replace(/^\s+/, '');
    const lastN = nodes[nodes.length - 1];
    if (lastN && lastN.t === 'text') lastN.v = lastN.v.replace(/\s+$/, '');

    const hasProse = nodes.some((n) => (n.t === 'text' ? n.v.trim() !== '' : true));
    return { nodes, links, hasProse };
  });
</script>

{#if parsed.hasProse}<p
    class="m-0"
    style="overflow-wrap:anywhere; word-break:break-word; white-space:pre-wrap;"
  >{#each parsed.nodes as n}{#if n.t === 'text'}{n.v}{:else}<a
        href={n.url}
        target="_blank"
        rel="noopener noreferrer"
        onclick={(e) => e.stopPropagation()}
        class="underline decoration-1 underline-offset-2"
        style="color: var(--sempa-accent);"
      >{n.label}</a>{/if}{/each}</p>{/if}

{#if parsed.links.length}
  <div class="mt-3 flex flex-col gap-2">
    {#each parsed.links as url (url)}
      <LinkPreview {url} />
    {/each}
  </div>
{/if}
