// Persists dark/light preference in localStorage and applies the 'dark' class to <html>.
const STORAGE_KEY = 'sempa-theme';

function createThemeStore() {
  let dark = $state(false);

  function init() {
    const saved = typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) : null;
    const prefersDark = typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
    dark = saved === 'dark' || (saved === null && prefersDark);
    apply();
  }

  function apply() {
    if (typeof document !== 'undefined') {
      document.documentElement.classList.toggle('dark', dark);
    }
  }

  function toggle() {
    dark = !dark;
    localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light');
    apply();
  }

  return {
    get dark() { return dark; },
    init,
    toggle,
  };
}

export const theme = createThemeStore();
