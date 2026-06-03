/** Native haptic feedback bridge for Android WebView. */

declare global {
  interface Window {
    SempaHaptics?: {
      click(): void;
      tick(): void;
      heavyClick(): void;
    };
  }
}

export function hapticClick() {
  window.SempaHaptics?.click();
}

export function hapticTick() {
  window.SempaHaptics?.tick();
}

export function hapticHeavyClick() {
  window.SempaHaptics?.heavyClick();
}
