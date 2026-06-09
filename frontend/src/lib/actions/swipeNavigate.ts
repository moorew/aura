/**
 * `swipeNavigate` — horizontal navigation gestures for the day/week views.
 *
 * Adds, on top of the existing ‹ › buttons:
 *   • Desktop: trackpad horizontal scroll (deltaX) and shift+wheel. One flick =
 *     one step, with a cooldown so it doesn't fly through several.
 *   • Android/touch: swipe left/right (→ prev, ← next), gated so it doesn't
 *     steal vertical scrolls.
 *
 * Edge guards (both inputs): the gesture is ignored when it starts inside a
 * horizontally-scrollable element that can still consume it in that direction
 * (e.g. the week grid with overflow-x:auto on a narrow window). Only when that
 * scroller is at its edge — or there is none — does a gesture navigate.
 *
 * `onPrev`/`onNext` are called with the navigation intent. Keyboard ArrowLeft/
 * Right are handled separately by the page (they need focus-target checks).
 */
export interface SwipeNavOptions {
  onPrev: () => void;
  onNext: () => void;
  /** Accumulated wheel deltaX (px) before a step fires. Default 120. */
  wheelThreshold?: number;
  /** Min horizontal touch travel (px) to count as a swipe. Default 60. */
  swipeThreshold?: number;
  /** Cooldown (ms) after a step before another can fire. Default 400. */
  cooldown?: number;
}

export function swipeNavigate(node: HTMLElement, options: SwipeNavOptions) {
  let opts = options;
  const wheelThreshold = () => opts.wheelThreshold ?? 120;
  const swipeThreshold = () => opts.swipeThreshold ?? 60;
  const cooldown = () => opts.cooldown ?? 400;

  let accX = 0;
  let lastStep = 0;
  let resetTimer: ReturnType<typeof setTimeout> | null = null;

  // Walk up from the event target looking for a scroller that can still move in
  // `dir` (-1 = content moves right / showing earlier, +1 = later). Returns true
  // if such a scroller exists (so we must NOT navigate — let it scroll).
  function scrollerCanConsume(target: EventTarget | null, dir: number): boolean {
    let el = target instanceof Element ? target : null;
    while (el && el !== node.parentElement) {
      if (el instanceof HTMLElement && el.scrollWidth > el.clientWidth + 1) {
        const atStart = el.scrollLeft <= 0;
        const atEnd = el.scrollLeft >= el.scrollWidth - el.clientWidth - 1;
        // dir > 0 means we want to go "next" (gesture scrolls content left, i.e.
        // increasing scrollLeft). It can consume unless already at the end.
        if (dir > 0 && !atEnd) return true;
        if (dir < 0 && !atStart) return true;
      }
      el = el.parentElement;
    }
    return false;
  }

  function now(): number {
    // performance.now is available in webviews; avoids Date.now (banned in some
    // contexts) and is monotonic.
    return typeof performance !== 'undefined' ? performance.now() : 0;
  }

  function step(dir: number) {
    const t = now();
    if (t - lastStep < cooldown()) return;
    lastStep = t;
    accX = 0;
    if (dir < 0) opts.onPrev(); else opts.onNext();
  }

  function onWheel(e: WheelEvent) {
    // Treat shift+wheel (reported as deltaY) as horizontal intent.
    const dx = Math.abs(e.deltaX) >= Math.abs(e.deltaY) ? e.deltaX
             : (e.shiftKey ? e.deltaY : 0);
    if (dx === 0) return;
    const dir = dx > 0 ? 1 : -1;
    if (scrollerCanConsume(e.target, dir)) return; // let the grid scroll
    e.preventDefault();
    // Reset the accumulator if the gesture paused.
    if (resetTimer) clearTimeout(resetTimer);
    resetTimer = setTimeout(() => { accX = 0; }, 150);
    accX += dx;
    if (Math.abs(accX) >= wheelThreshold()) step(accX > 0 ? 1 : -1);
  }

  // ── Touch ──
  let startX = 0, startY = 0, tracking = false, startTarget: EventTarget | null = null;

  function onTouchStart(e: TouchEvent) {
    if (e.touches.length !== 1) { tracking = false; return; }
    startX = e.touches[0].clientX;
    startY = e.touches[0].clientY;
    startTarget = e.target;
    tracking = true;
  }
  function onTouchEnd(e: TouchEvent) {
    if (!tracking) return;
    tracking = false;
    const t = e.changedTouches[0];
    const dx = t.clientX - startX;
    const dy = t.clientY - startY;
    if (Math.abs(dx) < swipeThreshold()) return;
    if (Math.abs(dx) < 1.5 * Math.abs(dy)) return; // mostly-vertical → scroll
    // Swiping the content right (dx > 0) reveals earlier dates = "prev".
    const dir = dx > 0 ? -1 : 1;
    if (scrollerCanConsume(startTarget, dir)) return;
    // Don't navigate if the swipe began on a draggable task card.
    if (startTarget instanceof Element && startTarget.closest('[draggable="true"]')) return;
    step(dir);
  }

  node.addEventListener('wheel', onWheel, { passive: false });
  node.addEventListener('touchstart', onTouchStart, { passive: true });
  node.addEventListener('touchend', onTouchEnd, { passive: true });

  return {
    update(next: SwipeNavOptions) { opts = next; },
    destroy() {
      node.removeEventListener('wheel', onWheel);
      node.removeEventListener('touchstart', onTouchStart);
      node.removeEventListener('touchend', onTouchEnd);
      if (resetTimer) clearTimeout(resetTimer);
    },
  };
}
