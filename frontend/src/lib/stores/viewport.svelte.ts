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
  /**
   * Height of the area the soft keyboard covers, in px (0 when closed).
   *
   * On Android in pan mode (and iOS browsers) the *layout* viewport stays full
   * height while only the *visual* viewport shrinks, so a `position: fixed;
   * bottom: 0` element sits BEHIND the keyboard. Bottom sheets add this as a
   * bottom offset to lift their footer (Save/Cancel) above the keyboard.
   */
  keyboardHeight = $state(0);

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
      // How much the layout viewport overhangs the visual viewport at the
      // bottom = keyboard coverage. (offsetTop accounts for any top inset so we
      // don't double-count.) Clamped to >= 0.
      const covered = vv ? Math.round(window.innerHeight - vv.height - vv.offsetTop) : 0;
      this.keyboardHeight = Math.max(0, covered);
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

    // Belt-and-braces for Android WebViews that don't reliably fire a
    // visualViewport `resize` when the soft keyboard *closes* — which left
    // keyboardHeight/height stale and sheets stuck at half size. Blurring an
    // input (or focusing one) closes/opens the keyboard; re-measure a couple of
    // times as the animation settles so the values always recover.
    const recheck = () => { update(); setTimeout(update, 150); setTimeout(update, 400); };
    window.addEventListener('focusout', recheck);
    window.addEventListener('focusin', recheck);

    update();
  }
}

export const viewport = new ViewportStore();
