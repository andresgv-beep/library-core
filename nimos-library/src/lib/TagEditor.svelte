<script>
  import Icon from './Icon.svelte';
  import * as readerState from './readerStateApi.js';
  import { t } from './i18n.svelte.js';

  // Editor de etiquetas de una página (modal). Persiste al instante cada cambio
  // y avisa a App (onChanged) para refrescar el marcado del botón y la vista.
  let { target, onChanged, onClose } = $props();

  let tags = $state([]);      // etiquetas actuales de la página
  let all = $state([]);       // todas las etiquetas existentes (sugerencias)
  let input = $state('');
  let inp;

  $effect(() => { load(); });
  async function load() {
    try { tags = await readerState.getPageTags(target.lib, target.path, target.itemId); } catch (e) { tags = []; }
    try { all = (await readerState.listTags()).map((x) => x.tag); } catch (e) { all = []; }
    if (inp) inp.focus();
  }

  let suggestions = $derived(
    all.filter((x) => !tags.includes(x) && (!input.trim() || x.toLowerCase().includes(input.trim().toLowerCase()))).slice(0, 12)
  );

  async function add(name) {
    const tag = (name || '').trim();
    if (!tag || tags.includes(tag)) { input = ''; return; }
    tags = [...tags, tag];
    input = '';
    await readerState.addTag({ itemId: target.itemId, lib: target.lib, path: target.path, tag, title: target.title, book: target.book });
    if (!all.includes(tag)) all = [...all, tag];
    onChanged?.();
  }
  async function remove(tag) {
    tags = tags.filter((x) => x !== tag);
    await readerState.removeTag(target.lib, target.path, tag, target.itemId);
    onChanged?.();
  }
  function onKey(e) {
    if (e.key === 'Enter') { e.preventDefault(); add(input); }
    else if (e.key === 'Backspace' && !input && tags.length) { remove(tags[tags.length - 1]); }
    else if (e.key === 'Escape') { e.preventDefault(); onClose?.(); }
  }
</script>

<div class="scrim" onclick={() => onClose?.()} role="presentation">
  <div class="card" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label={t('tag.aria')}>
    <header class="thead">
      <span class="ttile"><Icon name="tag" size={17} /></span>
      <div class="ttitle">
        <b>{t('tag.heading')}</b>
        <small>{target?.title || target?.path}</small>
      </div>
      <button class="x" title={t('note.close')} onclick={() => onClose?.()}><Icon name="close" size={17} /></button>
    </header>

    <div class="field">
      {#each tags as tg}
        <span class="chip"><span class="ct">{tg}</span><button class="cx" title={t('tag.remove')} onclick={() => remove(tg)}><Icon name="close" size={12} /></button></span>
      {/each}
      <input bind:this={inp} bind:value={input} onkeydown={onKey} placeholder={tags.length ? '' : t('tag.placeholder')} autocomplete="off" />
    </div>

    {#if tags.length === 0 && suggestions.length === 0}
      <div class="hint">{t('tag.none')}</div>
    {/if}

    {#if suggestions.length}
      <div class="sugwrap">
        <div class="suglabel">{t('tag.existing')}</div>
        <div class="sugs">
          {#each suggestions as sg}
            <button class="sug" onclick={() => add(sg)}><Icon name="plus" size={13} /> {sg}</button>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .scrim{position:fixed;inset:0;z-index:100;background:color-mix(in srgb,#000 55%,transparent);backdrop-filter:blur(2px);display:grid;place-items:center;padding:24px;animation:fade .12s ease}
  @keyframes fade{from{opacity:0}to{opacity:1}}
  .card{width:100%;max-width:560px;background:var(--panel);border:1px solid var(--border);border-radius:16px;box-shadow:0 24px 70px rgba(0,0,0,.5);display:flex;flex-direction:column;overflow:hidden;animation:pop .14s ease}
  @keyframes pop{from{transform:translateY(8px) scale(.98);opacity:0}to{transform:none;opacity:1}}
  .thead{display:flex;align-items:center;gap:12px;padding:16px 16px 14px;border-bottom:1px solid var(--border)}
  .ttile{width:34px;height:34px;border-radius:9px;flex:none;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .ttitle{flex:1;min-width:0;display:flex;flex-direction:column;gap:2px}
  .ttitle b{font-size:15px;color:var(--ink)}
  .ttitle small{font-size:12px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .x{width:30px;height:30px;border-radius:8px;display:grid;place-items:center;color:var(--muted);flex:none}
  .x:hover{background:var(--raise);color:var(--ink)}

  .field{margin:16px;display:flex;flex-wrap:wrap;align-items:center;gap:7px;background:var(--ground);border:1px solid var(--border);border-radius:11px;padding:10px 12px;min-height:52px}
  .field:focus-within{border-color:color-mix(in srgb,var(--accent) 55%,var(--border))}
  .chip{display:inline-flex;align-items:center;gap:4px;background:color-mix(in srgb,var(--accent) 18%,transparent);color:var(--ink);border-radius:8px;padding:4px 4px 4px 10px;font-size:13px;font-weight:520}
  .chip .cx{width:18px;height:18px;border-radius:5px;display:grid;place-items:center;color:var(--muted)}
  .chip .cx:hover{background:color-mix(in srgb,var(--accent) 30%,transparent);color:var(--ink)}
  .field input{flex:1;min-width:120px;background:none;border:none;outline:none;color:var(--ink);font-size:14px;height:26px}
  .field input::placeholder{color:var(--muted)}

  .hint{margin:0 16px 8px;color:var(--muted);font-size:13px}
  .sugwrap{margin:0 16px 18px}
  .suglabel{font-size:11px;font-weight:650;letter-spacing:.5px;text-transform:uppercase;color:var(--faint);margin-bottom:9px}
  .sugs{display:flex;flex-wrap:wrap;gap:7px}
  .sug{display:inline-flex;align-items:center;gap:5px;background:var(--card);border:1px solid var(--border);border-radius:8px;padding:6px 11px;font-size:13px;color:var(--ink-dim);transition:background .12s,border-color .12s,color .12s}
  .sug:hover{background:var(--raise);border-color:color-mix(in srgb,var(--accent) 40%,var(--border));color:var(--ink)}
  .sug :global(.ic){color:var(--muted)}
</style>
