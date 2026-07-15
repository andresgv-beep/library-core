<script>
  import Icon from './Icon.svelte';
  import ZimIcon from './ZimIcon.svelte';
  import * as readerState from './readerStateApi.js';
  import { t, tn, relTime } from './i18n.svelte.js';

  let { libraries = [], tagsVersion = 0, onNavigate, onOpenItem } = $props();

  const bookName = (id) => libraries.find((l) => l.id === id)?.name || id;
  const iconOf = (id) => libraries.find((l) => l.id === id)?.icon;

  let tags = $state([]);       // [{tag, count}]
  let selected = $state(null); // etiqueta abierta
  let pages = $state([]);      // páginas de la etiqueta abierta
  let loading = $state(false);
  let confirming = $state(false);

  $effect(() => { tagsVersion; loadTags(); });
  async function loadTags() {
    loading = true;
    try { tags = await readerState.listTags(); } catch (e) { tags = []; }
    loading = false;
    // si la etiqueta abierta ya no existe, volver a la nube
    if (selected && !tags.some((x) => x.tag === selected)) { selected = null; pages = []; }
    else if (selected) loadPages(selected);
  }
  async function open(tag) {
    selected = tag; confirming = false;
    loadPages(tag);
  }
  async function loadPages(tag) {
    try { pages = await readerState.getTagPages(tag); } catch (e) { pages = []; }
  }
  function backToCloud() { selected = null; pages = []; confirming = false; }

  async function removeFromPage(p) {
    await readerState.removeTag(p.lib, p.path, selected, p.itemId);
    pages = pages.filter((x) => x !== p);
    if (pages.length === 0) { await loadTags(); backToCloud(); }
  }
  async function deleteWholeTag() {
    const tag = selected;
    await readerState.deleteTag(tag);
    confirming = false;
    backToCloud();
    await loadTags();
  }

  let isEmpty = $derived(!loading && tags.length === 0);
</script>

<div class="view scroll">
  <header class="vhead">
    <span class="vtile"><Icon name="tag" size={20} /></span>
    <div class="vtitle">
      <h1>{t('menu.tags')}</h1>
      {#if !isEmpty && !selected}<span class="vcount">{tn('view.count', tags.length)}</span>{/if}
    </div>
  </header>

  {#if loading && !tags.length}
    <div class="empty"><Icon name="clock" size={26} /><p>{t('common.loading')}</p></div>
  {:else if isEmpty}
    <div class="empty">
      <Icon name="tag" size={30} />
      <p>{t('view.tags.empty')}</p>
      <span>{t('view.tags.hint')}</span>
    </div>

  {:else if !selected}
    <div class="cloud">
      {#each tags as tc}
        <button class="tagchip" onclick={() => open(tc.tag)}>
          <Icon name="tag" size={14} />
          <span class="tn">{tc.tag}</span>
          <span class="cnt">{tc.count}</span>
        </button>
      {/each}
    </div>

  {:else}
    <div class="detail">
      <div class="dbar">
        <button class="back" onclick={backToCloud}><Icon name="back" size={15} /> {t('tags.back')}</button>
        <span class="dtitle">{t('tags.pagesFor', { tag: selected })}</span>
        <span class="dcount">{tn('tags.pages', pages.length)}</span>
        {#if confirming}
          <button class="hbtn ghost" onclick={() => (confirming = false)}>{t('common.cancel')}</button>
          <button class="hbtn danger" onclick={deleteWholeTag}><Icon name="trash" size={14} /> {t('view.clear')}</button>
        {:else}
          <button class="hbtn" title={t('tags.confirmDelete', { tag: selected })} onclick={() => (confirming = true)}><Icon name="trash" size={14} /> {t('tags.deleteTag')}</button>
        {/if}
      </div>
      <ul class="rows">
        {#each pages as p}
          <li class="row">
            <button class="open" onclick={() => p.itemId ? onOpenItem?.(p.itemId) : onNavigate?.(p.lib, p.path)}>
              <ZimIcon icon={iconOf(p.lib)} name={p.book || bookName(p.lib)} size={38} radius={10} />
              <span class="info"><b>{p.title || p.path}</b><small>{p.book || bookName(p.lib)} · {relTime(p.created)}</small></span>
            </button>
            <div class="acts">
              <button class="act danger" title={t('tags.removeFromPage')} onclick={() => removeFromPage(p)}><Icon name="close" size={15} /></button>
            </div>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</div>

<style>
  .view{background:var(--ground);min-height:100%;overflow-y:auto}
  .vhead{max-width:860px;margin:0 auto;padding:34px 40px 18px;display:flex;align-items:center;gap:14px}
  .vtile{width:44px;height:44px;border-radius:12px;flex:none;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .vtitle{display:flex;flex-direction:column;gap:2px}
  .vtitle h1{font-size:24px;font-weight:680;letter-spacing:-.4px}
  .vcount{font-size:12.5px;color:var(--faint);font-variant-numeric:tabular-nums}

  .cloud{max-width:860px;margin:0 auto;padding:6px 40px 90px;display:flex;flex-wrap:wrap;gap:10px}
  .tagchip{display:inline-flex;align-items:center;gap:8px;padding:9px 12px;background:var(--card);border:1px solid var(--border);border-radius:11px;color:var(--ink-dim);font-size:14px;transition:background .12s,border-color .12s,color .12s}
  .tagchip:hover{background:var(--raise);border-color:color-mix(in srgb,var(--accent) 45%,var(--border));color:var(--ink)}
  .tagchip :global(.ic){color:var(--accent-2)}
  .tagchip .tn{font-weight:540}
  .tagchip .cnt{min-width:20px;height:20px;padding:0 6px;border-radius:10px;background:color-mix(in srgb,var(--accent) 20%,transparent);color:var(--accent-2);font-size:11.5px;font-weight:650;display:grid;place-items:center;font-variant-numeric:tabular-nums}

  .detail{max-width:860px;margin:0 auto;padding:0 40px 90px}
  .dbar{display:flex;align-items:center;gap:12px;padding:2px 0 16px;flex-wrap:wrap}
  .back{display:inline-flex;align-items:center;gap:6px;height:32px;padding:0 12px;border-radius:9px;background:var(--card);border:1px solid var(--border);color:var(--ink-dim);font-size:13px;transition:background .12s,color .12s}
  .back:hover{background:var(--raise);color:var(--ink)}
  .back :global(.ic){color:var(--muted)}
  .dtitle{font-weight:640;font-size:16px;color:var(--ink)}
  .dcount{font-size:12.5px;color:var(--faint);font-variant-numeric:tabular-nums}
  .hbtn{margin-left:auto;display:flex;align-items:center;gap:7px;height:32px;padding:0 12px;border-radius:9px;font-size:12.5px;color:var(--ink-dim);border:1px solid var(--border);background:var(--card);transition:background .12s,color .12s,border-color .12s}
  .hbtn:not(.danger):not(.ghost):hover{background:var(--raise);color:var(--ink)}
  .hbtn :global(.ic){color:var(--muted)}
  .hbtn.ghost{margin-left:0;border-color:transparent;background:transparent}
  .hbtn.danger{margin-left:0;background:color-mix(in srgb,#e5484d 20%,transparent);border-color:color-mix(in srgb,#e5484d 40%,transparent);color:#ff9a9d}
  .hbtn.danger :global(.ic){color:#ff9a9d}

  .rows{display:flex;flex-direction:column;gap:4px;list-style:none}
  .row{display:flex;align-items:center;gap:6px;background:var(--card);border:1px solid var(--border);border-radius:12px;transition:border-color .12s,background .12s}
  .row:hover{border-color:color-mix(in srgb,var(--accent) 40%,var(--border));background:var(--raise)}
  .open{flex:1;min-width:0;display:flex;align-items:center;gap:13px;padding:12px 14px;text-align:left}
  .info{display:flex;flex-direction:column;min-width:0;gap:2px}
  .info b{font-size:14.5px;color:var(--ink);font-weight:580;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .info small{font-size:12px;color:var(--muted);font-variant-numeric:tabular-nums}
  .acts{display:flex;align-items:center;padding:0 10px 0 4px;flex:none}
  .act{width:30px;height:30px;border-radius:8px;display:grid;place-items:center;color:var(--muted);opacity:0;transition:background .12s,color .12s,opacity .12s}
  .row:hover .act{opacity:1}
  .act.danger:hover{background:color-mix(in srgb,#e5484d 22%,transparent);color:#ff9a9d}

  .empty{max-width:480px;margin:40px auto;text-align:center;color:var(--muted);display:flex;flex-direction:column;align-items:center;padding:0 30px}
  .empty :global(.ic){color:var(--accent-2);opacity:.7}
  .empty p{font-size:15.5px;color:var(--ink-dim);margin:16px 0 7px;font-weight:550}
  .empty span{font-size:13.5px;line-height:1.55}
</style>
