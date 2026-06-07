/**
 * Reactive viewport store — tracks the *visual* viewport so bottom sheets and
 * dialogs can stay above the Android soft keyboard.
 *
 * On Android (Capacitor `adjustResize`) and in mobile browsers the visual
 * viewport shrinks when the keyboard opens. We expose that height plus a
 * derived `keyboardOpen` flag so components can size themselves correctly and
 * keep their footers / inputs reachable while typing.
 */
class ViewportStore {
  /** Current visual-viewport height in px (falls back to innerHeight). */
  height = $state(typeof window !== 'undefined' ? window.innerHeight : 800);
  /** True when the soft keyboard is (very likely) open. */
  keyboardOpen = $state(false);

  private baseline = 0;
  private started = false;

  init() {
    if (typeof window === 'undefined' || this.started) return;
    this.started = true;

    const vv = window.visualViewport;
    this.baseline = window.innerHeight;

    const update = () => {
      const h = Math.round(vv?.height ?? window.innerHeight);
      this.height = h;
      // The keyboard is open when the visual viewport is meaningfully shorter
      // than the tallest layout we've seen (covers both adjustResize + browser).
      this.keyboardOpen = this.baseline - h > 150;
    };

    if (vv) {
      vv.addEventListener('resize', update);
      vv.addEventListener('scroll', update);
    }
    // Orientation / window changes raise the baseline (true full height).
    window.addEventListener('resize', () => {
      this.baseline = Math.max(this.baseline, window.innerHeight);
      update();
    });

    update();
  }
}

export const viewport = new ViewportStore();
