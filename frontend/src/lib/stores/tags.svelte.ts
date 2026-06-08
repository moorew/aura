import { api } from '$lib/api';
import type { TagDefinition } from '$lib/types';

function createTagStore() {
  let definitions = $state<TagDefinition[]>([]);
  let loaded = false;
  let loading = false;

  // Load tag definitions once. Safe to call repeatedly (e.g. from onMount and
  // afterNavigate): the loading guard dedupes concurrent calls, and because
  // `loaded` is only set on success, a call that failed before auth was ready
  // (returning 401 right after login) will retry on the next invocation rather
  // than leaving every tag stuck on the grey fallback colour.
  async function load() {
    if (loaded || loading) return;
    loading = true;
    try {
      definitions = await api.tags.list();
      loaded = true;
    } catch {
      // non-fatal — tags just won't have colours until loaded
    } finally {
      loading = false;
    }
  }

  // Force a refresh regardless of the load-once guard (e.g. a tag:change event
  // from another client edited a colour).
  async function reload() {
    if (loading) return;
    loading = true;
    try {
      definitions = await api.tags.list();
      loaded = true;
    } catch {
      // non-fatal
    } finally {
      loading = false;
    }
  }

  function colorFor(name: string): string {
    const d = definitions.find(t => t.name.toLowerCase() === name.toLowerCase());
    return d?.color ?? '#6b7280';
  }

  function add(tag: TagDefinition) {
    const idx = definitions.findIndex(t => t.id === tag.id);
    if (idx >= 0) definitions[idx] = tag;
    else definitions = [...definitions, tag];
  }

  function remove(id: string) {
    definitions = definitions.filter(t => t.id !== id);
  }

  return {
    get definitions() { return definitions; },
    load,
    reload,
    colorFor,
    add,
    remove,
  };
}

export const tagStore = createTagStore();
