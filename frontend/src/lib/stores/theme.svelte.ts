const DARK_KEY       = 'sempa-theme';        // 'dark' | 'light' — light/dark preference
const THEME_NAME_KEY = 'sempa-theme-name';   // which of the six interface themes
const SCALE_KEY      = 'sempa-text-scale';   // root font-size percent
const ACCENT_KEY     = 'sempa-accent';       // legacy (15-swatch accent) — migrated away

export type ThemeName = 'terracotta' | 'forest' | 'plum' | 'slate' | 'oled' | 'ocean';

export type ThemeMeta = {
  id: ThemeName;
  label: string;
  sublabel: string;
  /** OLED is a dark-only theme — the light/dark toggle is hidden/disabled for it. */
  darkOnly?: boolean;
};

/** Curated full-interface themes (colour values live in src/themes.css). */
export const THEMES: ThemeMeta[] = [
  { id: 'terracotta', label: 'Terracotta', sublabel: 'Warm clay' },
  { id: 'forest',     label: 'Forest',     sublabel: 'Pine green' },
  { id: 'plum',       label: 'Plum',       sublabel: 'Aubergine' },
  { id: 'slate',      label: 'Slate',      sublabel: 'Graphite' },
  { id: 'oled',       label: 'OLED Black', sublabel: 'Dark only', darkOnly: true },
  { id: 'ocean',      label: 'Ocean',      sublabel: 'Marine blue' },
];

const THEME_IDS = THEMES.map((t) => t.id);
const isTheme = (v: string | null): v is ThemeName => !!v && (THEME_IDS as string[]).includes(v);

function createThemeStore() {
  let dark      = $state(false);
  let themeName = $state<ThemeName>('terracotta');
  let textScale = $state(100); // percent, e.g. 90 / 100 / 110

  function init() {
    if (typeof localStorage === 'undefined') return;

    // ── One-time migration: the old 15-swatch accent picker → themes. Any
    // legacy accent maps to the terracotta default (the themes aren't 1:1 with
    // the old accents). Then drop the legacy key for good.
    if (!localStorage.getItem(THEME_NAME_KEY) && localStorage.getItem(ACCENT_KEY)) {
      localStorage.setItem(THEME_NAME_KEY, 'terracotta');
    }
    localStorage.removeItem(ACCENT_KEY);

    const savedTheme = localStorage.getItem(THEME_NAME_KEY);
    if (isTheme(savedTheme)) themeName = savedTheme;
    applyTheme(themeName);

    const savedDark = localStorage.getItem(DARK_KEY);
    const prefersDark = typeof window !== 'undefined'
      && window.matchMedia?.('(prefers-color-scheme: dark)').matches;
    // OLED forces dark regardless of the saved light/dark preference (which is
    // preserved untouched so it returns when the user picks another theme).
    dark = themeName === 'oled' || savedDark === 'dark' || (savedDark === null && !!prefersDark);
    applyDark();

    const savedScale = localStorage.getItem(SCALE_KEY);
    if (savedScale) {
      const n = parseInt(savedScale, 10);
      if (n >= 80 && n <= 130) textScale = n;
    }
    applyScale(textScale);
    applyThemeColor();
  }

  function applyTheme(name: ThemeName) {
    if (typeof document === 'undefined') return;
    document.documentElement.dataset.theme = name;
  }

  function applyDark() {
    if (typeof document === 'undefined') return;
    document.documentElement.classList.toggle('dark', dark);
  }

  function applyScale(pct: number) {
    if (typeof document === 'undefined') return;
    document.documentElement.style.fontSize = `${pct}%`;
  }

  // Keep the browser/PWA chrome (address bar, Android status bar) in step with
  // the active theme by syncing <meta name="theme-color"> to the surface colour.
  // Runs after the theme/dark attrs are set, so the computed var is current.
  function applyThemeColor() {
    if (typeof document === 'undefined') return;
    const meta = document.querySelector('meta[name="theme-color"]');
    if (!meta) return;
    const c = getComputedStyle(document.documentElement).getPropertyValue('--sempa-bg-main').trim();
    if (c) meta.setAttribute('content', c);
  }

  /** True for the active light/dark preference, ignoring an OLED override. */
  function savedDarkPref(): boolean {
    const saved = localStorage.getItem(DARK_KEY);
    const prefersDark = typeof window !== 'undefined'
      && window.matchMedia?.('(prefers-color-scheme: dark)').matches;
    return saved === 'dark' || (saved === null && !!prefersDark);
  }

  function setTheme(name: ThemeName) {
    themeName = name;
    localStorage.setItem(THEME_NAME_KEY, name);
    applyTheme(name);

    if (name === 'oled') {
      // Dark-only — force dark for the session WITHOUT clobbering the saved
      // light/dark preference, so it's restored when leaving OLED.
      if (!dark) { dark = true; applyDark(); }
    } else {
      // Restore whatever light/dark preference was saved before (OLED never
      // overwrote it).
      const wantDark = savedDarkPref();
      if (dark !== wantDark) { dark = wantDark; applyDark(); }
    }
    applyThemeColor();
  }

  function setScale(pct: number) {
    textScale = Math.min(130, Math.max(80, pct));
    localStorage.setItem(SCALE_KEY, String(textScale));
    applyScale(textScale);
  }

  function toggleDark() {
    if (themeName === 'oled') return; // OLED is dark-only — toggle is a no-op
    dark = !dark;
    localStorage.setItem(DARK_KEY, dark ? 'dark' : 'light');
    applyDark();
    applyThemeColor();
  }

  return {
    get dark()      { return dark; },
    get theme()     { return themeName; },
    get textScale() { return textScale; },
    /** True when the active theme can't switch modes (OLED). */
    get darkOnly()  { return themeName === 'oled'; },
    THEMES,
    init,
    toggle: toggleDark,
    setTheme,
    setScale,
  };
}

export const theme = createThemeStore();
