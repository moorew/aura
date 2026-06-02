const DARK_KEY   = 'sempa-theme';
const ACCENT_KEY = 'sempa-accent';

export type AccentName =
  | 'blue' | 'sky' | 'indigo'
  | 'violet' | 'purple' | 'pink'
  | 'rose' | 'orange' | 'amber'
  | 'emerald' | 'teal' | 'cyan'
  | 'slate' | 'zinc';

type Preset = { label: string; swatch: string; a50: string; a100: string; a200: string; a400: string; a500: string; a600: string; a700: string; a900: string; a950: string };

export const ACCENT_PRESETS: Record<AccentName, Preset> = {
  // ── Blues ──────────────────────────────────────────────────────────────
  sky: {
    label: 'Sky', swatch: '#0ea5e9',
    a50:'#f0f9ff', a100:'#e0f2fe', a200:'#bae6fd', a400:'#38bdf8',
    a500:'#0ea5e9', a600:'#0284c7', a700:'#0369a1', a900:'#0c4a6e', a950:'#082f49',
  },
  blue: {
    label: 'Blue', swatch: '#3b82f6',
    a50:'#eff6ff', a100:'#dbeafe', a200:'#bfdbfe', a400:'#60a5fa',
    a500:'#3b82f6', a600:'#2563eb', a700:'#1d4ed8', a900:'#1e3a8a', a950:'#172554',
  },
  indigo: {
    label: 'Indigo', swatch: '#6366f1',
    a50:'#eef2ff', a100:'#e0e7ff', a200:'#c7d2fe', a400:'#818cf8',
    a500:'#6366f1', a600:'#4f46e5', a700:'#4338ca', a900:'#312e81', a950:'#1e1b4b',
  },
  // ── Purples ────────────────────────────────────────────────────────────
  violet: {
    label: 'Violet', swatch: '#8b5cf6',
    a50:'#f5f3ff', a100:'#ede9fe', a200:'#ddd6fe', a400:'#a78bfa',
    a500:'#8b5cf6', a600:'#7c3aed', a700:'#6d28d9', a900:'#4c1d95', a950:'#2e1065',
  },
  purple: {
    label: 'Purple', swatch: '#a855f7',
    a50:'#faf5ff', a100:'#f3e8ff', a200:'#e9d5ff', a400:'#c084fc',
    a500:'#a855f7', a600:'#9333ea', a700:'#7e22ce', a900:'#581c87', a950:'#3b0764',
  },
  pink: {
    label: 'Pink', swatch: '#ec4899',
    a50:'#fdf2f8', a100:'#fce7f3', a200:'#fbcfe8', a400:'#f472b6',
    a500:'#ec4899', a600:'#db2777', a700:'#be185d', a900:'#831843', a950:'#500724',
  },
  // ── Reds / Oranges ─────────────────────────────────────────────────────
  rose: {
    label: 'Rose', swatch: '#f43f5e',
    a50:'#fff1f2', a100:'#ffe4e6', a200:'#fecdd3', a400:'#fb7185',
    a500:'#f43f5e', a600:'#e11d48', a700:'#be123c', a900:'#881337', a950:'#4c0519',
  },
  orange: {
    label: 'Orange', swatch: '#f97316',
    a50:'#fff7ed', a100:'#ffedd5', a200:'#fed7aa', a400:'#fb923c',
    a500:'#f97316', a600:'#ea580c', a700:'#c2410c', a900:'#7c2d12', a950:'#431407',
  },
  amber: {
    label: 'Amber', swatch: '#f59e0b',
    a50:'#fffbeb', a100:'#fef3c7', a200:'#fde68a', a400:'#fbbf24',
    a500:'#f59e0b', a600:'#d97706', a700:'#b45309', a900:'#78350f', a950:'#451a03',
  },
  // ── Greens ─────────────────────────────────────────────────────────────
  emerald: {
    label: 'Emerald', swatch: '#10b981',
    a50:'#ecfdf5', a100:'#d1fae5', a200:'#a7f3d0', a400:'#34d399',
    a500:'#10b981', a600:'#059669', a700:'#047857', a900:'#064e3b', a950:'#022c22',
  },
  teal: {
    label: 'Teal', swatch: '#14b8a6',
    a50:'#f0fdfa', a100:'#ccfbf1', a200:'#99f6e4', a400:'#2dd4bf',
    a500:'#14b8a6', a600:'#0d9488', a700:'#0f766e', a900:'#134e4a', a950:'#042f2e',
  },
  cyan: {
    label: 'Cyan', swatch: '#06b6d4',
    a50:'#ecfeff', a100:'#cffafe', a200:'#a5f3fc', a400:'#22d3ee',
    a500:'#06b6d4', a600:'#0891b2', a700:'#0e7490', a900:'#164e63', a950:'#083344',
  },
  // ── Neutrals ───────────────────────────────────────────────────────────
  slate: {
    label: 'Slate', swatch: '#475569',
    a50:'#f8fafc', a100:'#f1f5f9', a200:'#e2e8f0', a400:'#94a3b8',
    a500:'#64748b', a600:'#475569', a700:'#334155', a900:'#0f172a', a950:'#020617',
  },
  zinc: {
    label: 'Zinc', swatch: '#71717a',
    a50:'#fafafa', a100:'#f4f4f5', a200:'#e4e4e7', a400:'#a1a1aa',
    a500:'#71717a', a600:'#52525b', a700:'#3f3f46', a900:'#18181b', a950:'#09090b',
  },
};

function createThemeStore() {
  let dark       = $state(false);
  let accentName = $state<AccentName>('blue');

  function init() {
    if (typeof localStorage === 'undefined') return;
    const savedDark   = localStorage.getItem(DARK_KEY);
    const prefersDark = typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
    dark = savedDark === 'dark' || (savedDark === null && prefersDark);
    applyDark();

    const savedAccent = localStorage.getItem(ACCENT_KEY) as AccentName | null;
    if (savedAccent && ACCENT_PRESETS[savedAccent]) accentName = savedAccent;
    applyAccent(accentName);
  }

  function applyDark() {
    if (typeof document === 'undefined') return;
    document.documentElement.classList.toggle('dark', dark);
  }

  function applyAccent(name: AccentName) {
    if (typeof document === 'undefined') return;
    const p = ACCENT_PRESETS[name];
    const r = document.documentElement.style;
    r.setProperty('--a50',  p.a50);
    r.setProperty('--a100', p.a100);
    r.setProperty('--a200', p.a200);
    r.setProperty('--a400', p.a400);
    r.setProperty('--a500', p.a500);
    r.setProperty('--a600', p.a600);
    r.setProperty('--a700', p.a700);
    r.setProperty('--a900', p.a900);
    r.setProperty('--a950', p.a950);
  }

  function toggleDark() {
    dark = !dark;
    localStorage.setItem(DARK_KEY, dark ? 'dark' : 'light');
    applyDark();
  }

  function setAccent(name: AccentName) {
    accentName = name;
    localStorage.setItem(ACCENT_KEY, name);
    applyAccent(name);
  }

  return {
    get dark()   { return dark; },
    get accent() { return accentName; },
    init,
    toggle: toggleDark,
    setAccent,
  };
}

export const theme = createThemeStore();
