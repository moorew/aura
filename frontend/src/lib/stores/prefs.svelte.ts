// User-facing display preferences that live purely on the client (localStorage).
// Keep this separate from theme.svelte so feature toggles don't bloat the theme
// store. Mirrors the same runes + init() pattern.

const CONTEXTUAL_KEY = 'sempa-contextual-reflections';

function createPrefsStore() {
  // When true, intentions/reflections/week-review summaries are surfaced inline
  // on the day and week screens. The Journal page is unaffected by this toggle.
  let contextualReflections = $state(true);

  function init() {
    if (typeof localStorage === 'undefined') return;
    const saved = localStorage.getItem(CONTEXTUAL_KEY);
    if (saved !== null) contextualReflections = saved === '1';
  }

  function setContextualReflections(on: boolean) {
    contextualReflections = on;
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(CONTEXTUAL_KEY, on ? '1' : '0');
    }
  }

  return {
    get contextualReflections() { return contextualReflections; },
    init,
    setContextualReflections,
    toggleContextualReflections: () => setContextualReflections(!contextualReflections),
  };
}

export const prefs = createPrefsStore();
