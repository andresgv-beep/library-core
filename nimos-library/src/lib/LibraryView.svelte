<script>
  import Icon from './Icon.svelte';
  import ZimIcon from './ZimIcon.svelte';
  import * as readerState from './readerStateApi.js';
  import { t, tn, relTime } from './i18n.svelte.js';

  // Vistas del sidebar (Favoritos/Reciente/Historial/Notas + placeholders).
  let { view, libraries = [], favorites = [], notesVersion = 0,
        onNavigate, onOpenItem, onRemoveFav, onToggleHome, onOpenNote, onDeleteNote } = $props();

  const ICONS = { favorites: 'star', recent: 'clock', history: 'history', notes: 'note', tags: 'tag' };
  let icon = $derived(ICONS[view] || 'star');
  // Título/empty/hint salen de i18n (menu.* y view.<vista>.*) → re-render al cambiar idioma.
  let title = $derived(t('menu.' + view));
  let emptyMsg = $derived(t('view.' + view + '.empty'));
  let emptyHint = $derived(t('view.' + view + '.hint'));

  const bookName = (id) => libraries.find((l) => l.id === id)?.name || id;
  const iconOf = (id) => libraries.find((l) => l.id === id)?.icon;

  // Datos: favoritos vienen por prop (vivos); reciente/historial/notas se cargan del shim.
  let loaded = $state([]);
  let loading = $state(false);
  let confirming = $state(false); // confirmación en línea del "Vaciar historial"
  $effect(() => {
    view; notesVersion; // deps: recargar al cambiar de vista o al guardar/borrar nota
    confirming = false;
    if (view === 'recent' || view === 'history' || view === 'notes') load(view);
    else { loaded = []; loading = false; }
  });
  async function load(v) {
    loading = true;
    let data = [];
    try {
      if (v === 'recent') data = await readerState.getRecent();
      else if (v === 'history') data = await readerState.getHistory();
      else if (v === 'notes') data = await readerState.listNotes();
    } catch (e) {}
    if (view === v) { loaded = data; loading = false; }
  }

  // Borrado manual del historial (sin auto-limpieza). Historial = por fila (id);
  // Recientes = quita la página entera (todas sus visitas). Ambos comparten tabla.
  async function delVisit(v) {
    if (view === 'history') await readerState.deleteHistoryEntry(v.id);
    else await readerState.deleteHistoryPage(v.lib, v.path, v.itemId);
    loaded = loaded.filter((x) => x !== v);
  }
  async function clearAll() {
    await readerState.clearHistory();
    confirming = false;
    loaded = [];
  }

  let rows = $derived(view === 'favorites' ? favorites : loaded);
  let isEmpty = $derived(!loading && rows.length === 0);
  const snippet = (body) => (body || '').replace(/\s+/g, ' ').trim();
  const openRow = (row) => row.itemId ? onOpenItem?.(row.itemId) : onNavigate?.(row.lib, row.path);
</script>

<div class="view scroll">
  <header class="vhead">
    <span class="vtile"><Icon name={icon} size={20} /></span>
    <div class="vtitle">
      <h1>{title}</h1>
      {#if !isEmpty && (view === 'favorites' || view === 'recent' || view === 'history' || view === 'notes')}
        <span class="vcount">{tn('view.count', rows.length)}</span>
      {/if}
    </div>
    {#if view === 'history' && !isEmpty}
      <div class="vactions">
        {#if confirming}
          <span class="confirmq">{t('view.clearConfirm')}</span>
          <button class="hbtn ghost" onclick={() => (confirming = false)}>{t('common.cancel')}</button>
          <button class="hbtn danger" onclick={clearAll}><Icon name="trash" size={15} /> {t('view.clear')}</button>
        {:else}
          <button class="hbtn" onclick={() => (confirming = true)}><Icon name="trash" size={15} /> {t('view.clearHistory')}</button>
        {/if}
      </div>
    {/if}
  </header>

  {#if loading}
    <div class="empty"><Icon name="clock" size={26} /><p>{t('common.loading')}</p></div>
  {:else if isEmpty}
    <div class="empty">
      <Icon name={icon} size={30} />
      <p>{emptyMsg}</p>
      <span>{emptyHint}</span>
    </div>

  {:else if view === 'favorites'}
    <ul class="rows">
      {#each rows as f}
        <li class="row">
          <button class="open" onclick={() => openRow(f)}>
            <ZimIcon icon={iconOf(f.lib)} name={f.book || bookName(f.lib)} size={38} radius={10} />
            <span class="info"><b>{f.title}</b><small>{f.book || bookName(f.lib)}</small></span>
          </button>
          <div class="acts">
            <button class="act" class:on={f.onHome} title={f.onHome ? t('view.pinned') : t('view.pin')} onclick={() => onToggleHome?.(f)}><Icon name="pin" size={15} /></button>
            <button class="act" title={t('view.note')} onclick={() => onOpenNote?.(f)}><Icon name="note" size={15} /></button>
            <button class="act danger" title={t('nav.removeFav')} onclick={() => onRemoveFav?.(f)}><Icon name="trash" size={15} /></button>
          </div>
        </li>
      {/each}
    </ul>

  {:else if view === 'recent' || view === 'history'}
    <ul class="rows">
      {#each rows as v}
        <li class="row">
          <button class="open" onclick={() => openRow(v)}>
            <ZimIcon icon={iconOf(v.lib)} name={v.book || bookName(v.lib)} size={38} radius={10} />
            <span class="info"><b>{v.title || v.path}</b><small>{v.book || bookName(v.lib)}</small></span>
          </button>
          <div class="tail">
            <span class="when">{relTime(v.visited)}</span>
            <button class="act danger" title={view === 'history' ? t('view.removeVisit') : t('view.removeRecent')} onclick={() => delVisit(v)}><Icon name="close" size={15} /></button>
          </div>
        </li>
      {/each}
    </ul>

  {:else if view === 'notes'}
    <ul class="rows">
      {#each rows as n}
        <li class="row note">
          <button class="open note" onclick={() => onOpenNote?.(n)}>
            <ZimIcon icon={iconOf(n.lib)} name={n.book || bookName(n.lib)} size={38} radius={10} />
            <span class="info">
              <b>{n.title || n.path}</b>
              <span class="body">{snippet(n.body)}</span>
              <small>{n.book || bookName(n.lib)} · {relTime(n.updated)}</small>
            </span>
          </button>
          <div class="acts">
            <button class="act" title={t('view.goArticle')} onclick={() => openRow(n)}><Icon name="forward" size={15} /></button>
            <button class="act danger" title={t('view.deleteNote')} onclick={() => onDeleteNote?.(n)}><Icon name="trash" size={15} /></button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .view{background:var(--ground);min-height:100%;overflow-y:auto}
  .vhead{max-width:860px;margin:0 auto;padding:34px 40px 18px;display:flex;align-items:center;gap:14px}
  .vtile{width:44px;height:44px;border-radius:12px;flex:none;display:grid;place-items:center;background:color-mix(in srgb,var(--accent) 16%,transparent);color:var(--accent-2)}
  .vtitle{display:flex;flex-direction:column;gap:2px}
  .vtitle h1{font-size:24px;font-weight:680;letter-spacing:-.4px}
  .vcount{font-size:12.5px;color:var(--faint);font-variant-numeric:tabular-nums}
  .vactions{margin-left:auto;display:flex;align-items:center;gap:8px}
  .confirmq{font-size:13px;color:var(--ink-dim)}
  .hbtn{display:flex;align-items:center;gap:7px;height:34px;padding:0 13px;border-radius:9px;font-size:13px;color:var(--ink-dim);border:1px solid var(--border);background:var(--card);transition:background .12s,color .12s,border-color .12s}
  .hbtn:hover{background:var(--raise);color:var(--ink)}
  .hbtn :global(.ic){color:var(--muted)}
  .hbtn.ghost{border-color:transparent;background:transparent}
  .hbtn.danger{background:color-mix(in srgb,#e5484d 20%,transparent);border-color:color-mix(in srgb,#e5484d 40%,transparent);color:#ff9a9d}
  .hbtn.danger:hover{background:color-mix(in srgb,#e5484d 30%,transparent);color:#ffb3b5}
  .hbtn.danger :global(.ic){color:#ff9a9d}

  .rows{max-width:860px;margin:0 auto;padding:4px 40px 90px;display:flex;flex-direction:column;gap:4px;list-style:none}
  .row{display:flex;align-items:center;gap:6px;background:var(--card);border:1px solid var(--border);border-radius:12px;transition:border-color .12s,background .12s}
  .row:hover{border-color:color-mix(in srgb,var(--accent) 40%,var(--border));background:var(--raise)}
  .row.note{align-items:stretch}
  .open{flex:1;min-width:0;display:flex;align-items:center;gap:13px;padding:12px 14px;text-align:left}
  .open.note{align-items:flex-start}
  .info{display:flex;flex-direction:column;min-width:0;gap:2px}
  .info b{font-size:14.5px;color:var(--ink);font-weight:580;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .info small{font-size:12px;color:var(--muted);font-variant-numeric:tabular-nums}
  .info .body{font-size:13px;color:var(--ink-dim);line-height:1.5;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
  .tail{display:flex;align-items:center;gap:6px;flex:none;padding:0 12px 0 8px}
  .when{font-size:12.5px;color:var(--faint);font-variant-numeric:tabular-nums;white-space:nowrap}
  .tail .act{opacity:0}
  .row:hover .tail .act{opacity:1}
  .acts{display:flex;align-items:center;gap:2px;padding:0 10px 0 4px;flex:none}
  .act{width:30px;height:30px;border-radius:8px;display:grid;place-items:center;color:var(--muted);transition:background .12s,color .12s}
  .act:hover{background:var(--border);color:var(--ink)}
  .act.on{color:var(--accent-2)}
  .act.on :global(.ic){fill:color-mix(in srgb,var(--accent-2) 22%,transparent)}
  .act.danger:hover{background:color-mix(in srgb,#e5484d 22%,transparent);color:#ff9a9d}

  .empty{max-width:480px;margin:40px auto;text-align:center;color:var(--muted);display:flex;flex-direction:column;align-items:center;padding:0 30px}
  .empty :global(.ic){color:var(--accent-2);opacity:.7}
  .empty p{font-size:15.5px;color:var(--ink-dim);margin:16px 0 7px;font-weight:550}
  .empty span{font-size:13.5px;line-height:1.55}
</style>
