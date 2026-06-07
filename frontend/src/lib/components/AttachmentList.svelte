<script lang="ts">
  import type { Attachment } from '$lib/types';
  import { api } from '$lib/api';

  let { ownerType, ownerId }: { ownerType: 'task' | 'objective'; ownerId: string } = $props();

  const MAX_BYTES = 500 * 1024 * 1024; // 500 MB

  let items = $state<Attachment[]>([]);
  let loading = $state(true);
  let error = $state('');
  let uploadPct = $state<number | null>(null);
  let uploadingName = $state('');
  let fileInput: HTMLInputElement | undefined = $state();
  let dragOver = $state(false);

  $effect(() => {
    // Re-load whenever the owner changes.
    const id = ownerId;
    if (!id) { items = []; loading = false; return; }
    loading = true;
    const load = ownerType === 'task'
      ? api.attachments.listForTask(id)
      : api.attachments.listForObjective(id);
    load.then((a) => { items = a; }).catch(() => { items = []; }).finally(() => { loading = false; });
  });

  function formatBytes(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
    return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  const isImage = (m: string) => m.startsWith('image/');

  function fileIcon(m: string): string {
    if (m === 'application/pdf') return '📄';
    if (m.startsWith('image/')) return '🖼️';
    if (m.startsWith('video/')) return '🎬';
    if (m.startsWith('audio/')) return '🎵';
    if (m.startsWith('text/')) return '📝';
    if (m.includes('zip') || m.includes('compressed') || m.includes('tar')) return '🗜️';
    if (m.includes('sheet') || m.includes('excel') || m.includes('csv')) return '📊';
    if (m.includes('word') || m.includes('document')) return '📃';
    return '📎';
  }

  async function uploadFiles(files: FileList | File[]) {
    error = '';
    for (const file of Array.from(files)) {
      if (file.size > MAX_BYTES) {
        error = `"${file.name}" is ${formatBytes(file.size)} — over the 500 MB limit.`;
        continue;
      }
      uploadingName = file.name;
      uploadPct = 0;
      try {
        const saved = ownerType === 'task'
          ? await api.attachments.uploadToTask(ownerId, file, (p) => (uploadPct = p))
          : await api.attachments.uploadToObjective(ownerId, file, (p) => (uploadPct = p));
        items = [...items, saved];
      } catch (e) {
        error = e instanceof Error ? e.message : 'Upload failed';
      } finally {
        uploadPct = null;
        uploadingName = '';
      }
    }
  }

  function onPick(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files?.length) uploadFiles(input.files);
    input.value = '';
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    if (e.dataTransfer?.files?.length) uploadFiles(e.dataTransfer.files);
  }

  async function remove(att: Attachment) {
    const prev = items;
    items = items.filter((a) => a.id !== att.id); // optimistic
    try {
      await api.attachments.delete(att.id);
    } catch {
      items = prev; // restore on failure
      error = 'Failed to delete attachment';
    }
  }
</script>

<div>
  <div class="mb-1.5 flex items-center justify-between">
    <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">
      Attachments
      {#if items.length}<span class="font-normal text-gray-400 dark:text-gray-600"> · {items.length}</span>{/if}
    </label>
    <button type="button" onclick={() => fileInput?.click()}
            class="text-xs font-medium text-blue-500 hover:text-blue-600 dark:hover:text-blue-400">
      + Add file
    </button>
  </div>

  <input bind:this={fileInput} type="file" multiple class="hidden" onchange={onPick} />

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div role="button" tabindex="0"
       class="rounded-lg border border-dashed px-3 py-3 transition-colors
              {dragOver ? 'border-blue-400 bg-blue-50/60 dark:bg-blue-950/30' : 'border-gray-200 dark:border-gray-700'}"
       ondragover={(e) => { e.preventDefault(); dragOver = true; }}
       ondragleave={() => dragOver = false}
       ondrop={onDrop}>

    {#if loading}
      <p class="text-center text-xs text-gray-400 dark:text-gray-600">Loading…</p>
    {:else if items.length === 0 && uploadPct === null}
      <button type="button" onclick={() => fileInput?.click()}
              class="w-full text-center text-xs text-gray-400 dark:text-gray-600 hover:text-gray-500">
        Drop files here or click to attach <span class="text-gray-300 dark:text-gray-700">· up to 500 MB</span>
      </button>
    {:else}
      <div class="flex flex-col gap-2">
        {#each items as att (att.id)}
          <div class="group flex items-center gap-3 rounded-lg bg-gray-50 px-2.5 py-2 dark:bg-gray-800/60">
            {#if isImage(att.mime_type)}
              <a href={api.attachments.downloadUrl(att.id)} target="_blank" rel="noopener" class="shrink-0">
                <img src={api.attachments.downloadUrl(att.id)} alt={att.filename}
                     class="h-10 w-10 rounded-md object-cover" loading="lazy" />
              </a>
            {:else}
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-white text-lg dark:bg-gray-700">
                {fileIcon(att.mime_type)}
              </span>
            {/if}
            <a href={api.attachments.downloadUrl(att.id)} target="_blank" rel="noopener"
               class="min-w-0 flex-1 hover:underline">
              <p class="truncate text-xs font-medium text-gray-700 dark:text-gray-200">{att.filename}</p>
              <p class="text-[10px] text-gray-400 dark:text-gray-600">{formatBytes(att.size_bytes)}</p>
            </a>
            <button type="button" onclick={() => remove(att)}
                    class="shrink-0 rounded p-1 text-gray-300 opacity-0 transition-opacity hover:text-red-500
                           group-hover:opacity-100 dark:text-gray-600"
                    aria-label="Remove attachment">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        {/each}

        {#if uploadPct !== null}
          <div class="rounded-lg bg-gray-50 px-2.5 py-2 dark:bg-gray-800/60">
            <p class="mb-1 truncate text-[11px] text-gray-500 dark:text-gray-400">
              Uploading {uploadingName}… {uploadPct}%
            </p>
            <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
              <div class="h-full rounded-full bg-blue-500 transition-all" style="width: {uploadPct}%"></div>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  {#if error}
    <p class="mt-1.5 text-xs text-red-500 dark:text-red-400">{error}</p>
  {/if}
</div>
