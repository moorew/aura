// User-facing display preferences that live purely on the client (localStorage).
// Keep this separate from theme.svelte so feature toggles don't bloat the theme
// store. Mirrors the same runes + init() pattern.

const CONTEXTUAL_KEY = 'sempa-contextual-reflections';
const NAV_GROUPING_KEY = 'sempa.navGrouping';
const NAV_SECTIONS_KEY = 'sempa.navSections';

// How the desktop navigation rail is organised. 'spaces' (default) groups by
// place; 'rhythm' groups by plan→focus→review; 'flat' is the original one-list.
export type NavGrouping = 'spaces' | 'rhythm' | 'flat';
// Whether grouped rails show mono-caps section names or quiet dividers.
export type NavSections = 'labels' | 'dividers';

const isGrouping = (v: string | null): v is NavGrouping =>
  v === 'spaces' || v === 'rhythm' || v === 'flat';
const isSections = (v: string | null): v is NavSections =>
  v === 'labels' || v === 'dividers';

function createPrefsStore() {
  // When true, intentions/reflections/week-review summaries are surfaced inline
  // on the day and week screens. The Journal page is unaffected by this toggle.
  let contextualReflections = $state(true);
  let navGrouping = $state<NavGrouping>('spaces');
  let navSections = $state<NavSections>('labels');

  function init() {
    if (typeof localStorage === 'undefined') return;
    const saved = localStorage.getItem(CONTEXTUAL_KEY);
    if (saved !== null) contextualReflections = saved === '1';
    const g = localStorage.getItem(NAV_GROUPING_KEY);
    if (isGrouping(g)) navGrouping = g;
    const s = localStorage.getItem(NAV_SECTIONS_KEY);
    if (isSections(s)) navSections = s;
  }

  function setContextualReflections(on: boolean) {
    contextualReflections = on;
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(CONTEXTUAL_KEY, on ? '1' : '0');
    }
  }

  function setNavGrouping(v: NavGrouping) {
    navGrouping = v;
    if (typeof localStorage !== 'undefined') localStorage.setItem(NAV_GROUPING_KEY, v);
  }

  function setNavSections(v: NavSections) {
    navSections = v;
    if (typeof localStorage !== 'undefined') localStorage.setItem(NAV_SECTIONS_KEY, v);
  }

  return {
    get contextualReflections() { return contextualReflections; },
    get navGrouping() { return navGrouping; },
    get navSections() { return navSections; },
    init,
    setContextualReflections,
    toggleContextualReflections: () => setContextualReflections(!contextualReflections),
    setNavGrouping,
    setNavSections,
  };
}

export const prefs = createPrefsStore();
