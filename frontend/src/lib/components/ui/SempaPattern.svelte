<script lang="ts">
  /**
   * SempaPattern — calming thin-stroke line-art illustrations.
   *
   * Purely decorative geometry rendered in the Sempa accent colour at very low
   * opacity. Drawn with `currentColor` driven by `--sempa-accent`, so the art
   * brightens automatically in dark mode with no separate handling.
   *
   * Always render these inside a `position: relative; overflow: hidden` parent,
   * with `pointer-events: none` and `z-index: 0`; real content sits at z-index 1.
   */
  type Motif = 'aurora' | 'rings' | 'meridian' | 'scatter' | 'garden' | 'cradle';

  let {
    motif,
    opacity = 1.0,
    class: className = '',
    style = '',
  }: {
    motif: Motif;
    opacity?: number;
    class?: string;
    style?: string;
  } = $props();

  const o = (base: number) => (base * opacity).toFixed(4);

  // Aurora — concentric quarter-circle arcs from the bottom-left corner.
  const AURORA = [
    { r: 100, w: 1.2,  op: 0.14 },
    { r: 180, w: 1.0,  op: 0.11 },
    { r: 260, w: 0.9,  op: 0.08 },
    { r: 340, w: 0.8,  op: 0.06 },
    { r: 420, w: 0.75, op: 0.05 },
    { r: 500, w: 0.75, op: 0.04 },
  ];

  // Rings — concentric circles around the centre.
  const RINGS = [
    { r: 60,  w: 1.2,  op: 0.14 },
    { r: 110, w: 1.0,  op: 0.10 },
    { r: 165, w: 0.9,  op: 0.08 },
    { r: 220, w: 0.75, op: 0.06 },
    { r: 280, w: 0.75, op: 0.04 },
  ];

  // Meridian — horizontal quadratic wave lines.
  const MERIDIAN = [
    { y: 60,  w: 0.75, op: 0.10 },
    { y: 104, w: 0.75, op: 0.09 },
    { y: 148, w: 1.0,  op: 0.13 },
    { y: 192, w: 0.75, op: 0.09 },
    { y: 236, w: 0.75, op: 0.08 },
    { y: 280, w: 0.75, op: 0.07 },
    { y: 324, w: 0.75, op: 0.06 },
  ];

  // Scatter — offset dot grid for background tiling.
  const SCATTER = [
    { cx: 0,  cy: 16, op: 0.10 }, { cx: 32, cy: 16, op: 0.08 }, { cx: 64, cy: 16, op: 0.12 },
    { cx: 16, cy: 44, op: 0.09 }, { cx: 48, cy: 44, op: 0.14 }, { cx: 80, cy: 44, op: 0.10 },
    { cx: 0,  cy: 72, op: 0.08 }, { cx: 32, cy: 72, op: 0.11 }, { cx: 64, cy: 72, op: 0.09 },
  ];

  // Cradle — upward-opening semicircles extending the Sempa logo mark.
  const CRADLE = [
    { r: 120, w: 1.25, op: 0.14 },
    { r: 150, w: 0.9,  op: 0.09 },
    { r: 180, w: 0.75, op: 0.07 },
  ];
</script>

{#if motif === 'aurora'}
  <svg viewBox="0 0 500 500" fill="none" aria-hidden="true"
       preserveAspectRatio="xMidYMid slice"
       class={className} style="color: var(--sempa-accent); {style}">
    {#each AURORA as a}
      <path d="M0,500 A{a.r},{a.r},0,0,1,{a.r},{500 - a.r}"
            stroke="currentColor" stroke-width={a.w} stroke-opacity={o(a.op)} />
    {/each}
    <circle cx="102" cy="396" r="3.5" fill="currentColor" fill-opacity={o(0.18)} />
    <circle cx="62"  cy="260" r="2"   fill="currentColor" fill-opacity={o(0.12)} />
  </svg>

{:else if motif === 'rings'}
  <svg viewBox="0 0 500 500" fill="none" aria-hidden="true"
       preserveAspectRatio="xMidYMid slice"
       class={className} style="color: var(--sempa-accent); {style}">
    {#each RINGS as r}
      <circle cx="250" cy="250" r={r.r}
              stroke="currentColor" stroke-width={r.w} stroke-opacity={o(r.op)} />
    {/each}
    <circle cx="250" cy="250" r="6"   fill="currentColor" fill-opacity={o(0.25)} />
    <circle cx="310" cy="250" r="2.5" fill="currentColor" fill-opacity={o(0.16)} />
    <circle cx="190" cy="210" r="2"   fill="currentColor" fill-opacity={o(0.14)} />
    <circle cx="260" cy="168" r="1.5" fill="currentColor" fill-opacity={o(0.12)} />
  </svg>

{:else if motif === 'meridian'}
  <svg viewBox="0 0 500 400" fill="none" aria-hidden="true"
       preserveAspectRatio="xMidYMid slice"
       class={className} style="color: var(--sempa-accent); {style}">
    {#each MERIDIAN as m}
      <path d="M0,{m.y} Q125,{m.y - 12} 250,{m.y} T500,{m.y}"
            stroke="currentColor" stroke-width={m.w} stroke-opacity={o(m.op)} />
    {/each}
    <circle cx="124" cy="148" r="2.5" fill="currentColor" fill-opacity={o(0.20)} />
    <circle cx="250" cy="104" r="2"   fill="currentColor" fill-opacity={o(0.15)} />
    <circle cx="376" cy="192" r="1.5" fill="currentColor" fill-opacity={o(0.12)} />
  </svg>

{:else if motif === 'scatter'}
  <svg viewBox="0 0 96 96" fill="none" aria-hidden="true"
       preserveAspectRatio="xMidYMid meet"
       class={className} style="color: var(--sempa-accent); {style}">
    {#each SCATTER as d}
      <circle cx={d.cx} cy={d.cy} r="1.5" fill="currentColor" fill-opacity={o(d.op)} />
    {/each}
    <circle cx="48" cy="48" r="2.5" fill="currentColor" fill-opacity={o(0.18)} />
  </svg>

{:else if motif === 'garden'}
  <svg viewBox="0 0 400 400" fill="none" aria-hidden="true"
       preserveAspectRatio="xMidYMid meet"
       class={className} style="color: var(--sempa-accent); {style}">
    <!-- Main stem -->
    <path d="M40,400 C90,320 180,260 280,180"
          stroke="currentColor" stroke-width="1.2" stroke-opacity={o(0.13)} />
    <!-- Branch -->
    <path d="M40,400 C70,360 100,330 160,300"
          stroke="currentColor" stroke-width="0.9" stroke-opacity={o(0.10)} />
    <!-- Secondary branch -->
    <path d="M160,300 C220,270 260,240 300,200"
          stroke="currentColor" stroke-width="0.9" stroke-opacity={o(0.09)} />
    <!-- Leaf arcs -->
    <path d="M120,340 C135,316 165,310 148,340"
          stroke="currentColor" stroke-width="0.85" stroke-opacity={o(0.10)} />
    <path d="M195,295 C210,271 240,265 223,295"
          stroke="currentColor" stroke-width="0.85" stroke-opacity={o(0.10)} />
    <path d="M252,244 C267,220 297,214 280,244"
          stroke="currentColor" stroke-width="0.85" stroke-opacity={o(0.10)} />
    <!-- Nodes -->
    <circle cx="120" cy="340" r="2"   fill="currentColor" fill-opacity={o(0.12)} />
    <circle cx="195" cy="295" r="2"   fill="currentColor" fill-opacity={o(0.14)} />
    <circle cx="252" cy="244" r="2.5" fill="currentColor" fill-opacity={o(0.16)} />
    <circle cx="280" cy="180" r="2.5" fill="currentColor" fill-opacity={o(0.18)} />
  </svg>

{:else if motif === 'cradle'}
  <svg viewBox="0 0 400 360" fill="none" aria-hidden="true"
       preserveAspectRatio="xMidYMid meet"
       class={className} style="color: var(--sempa-accent); {style}">
    {#each CRADLE as c}
      <path d="M{200 - c.r},220 a{c.r},{c.r},0,0,0,{2 * c.r},0"
            stroke="currentColor" stroke-width={c.w} stroke-opacity={o(c.op)} />
    {/each}
    <circle cx="200" cy="168" r="12"  stroke="currentColor" stroke-width="1.25" stroke-opacity={o(0.14)} />
    <circle cx="200" cy="168" r="4"   fill="currentColor" fill-opacity={o(0.22)} />
    <circle cx="292" cy="196" r="2.5" fill="currentColor" fill-opacity={o(0.14)} />
    <circle cx="110" cy="200" r="2"   fill="currentColor" fill-opacity={o(0.12)} />
  </svg>
{/if}
