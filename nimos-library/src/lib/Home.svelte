<script>
  import Icon from './Icon.svelte';
  import Logo from './Logo.svelte';
  import Wordmark from './Wordmark.svelte';
  import ZimIcon from './ZimIcon.svelte';
  import BrandIcon from './BrandIcon.svelte';
  import LocationSearchResult from './LocationSearchResult.svelte';
  import { siteShown } from './sites.svelte.js';
  import { itemSearch, globalImages, mapSearch } from './libraryApi.js';
  import { t, tn, i18n } from './i18n.svelte.js';

  let { tab, libraries = [], favorites = [], onNavigate, onOpenItem, onOpenView, onToggleHome } = $props();
  const bookName = (id) => libraries.find((l) => l.id === id)?.name || id;
  const iconOf = (id) => libraries.find((l) => l.id === id)?.icon;
  const TYPE_KEY = { article: 'tab.article', video: 'cabinet.kind.video', pdf: 'cabinet.kind.text', document: 'cabinet.kind.text', image: 'cabinet.kind.image', audio: 'cabinet.kind.audio' };
  const kindLabel = (kind) => t(TYPE_KEY[kind] || 'cabinet.kind.doc');
  // Sitios visibles en el launcher (los que el usuario mantiene con la estrella).
  let anySite = $derived(siteShown('moments') || siteShown('cabinet') || libraries.some((l) => siteShown(l.id)));

  // El estado de búsqueda vive en la pestaña → se conserva al volver atrás.
  let s = $derived(tab.search);
  let timer;                   // debounce del full-text (caro)
  let settleTimer;             // registra la búsqueda "asentada" en recientes
  let ctrl;                    // AbortController de la búsqueda en curso
  let mapCtrl;                 // Maps no bloquea ni comparte cancelación con ZIM
  let focused = $state(false); // input enfocado → mostrar el desplegable

  function ensureLocation() {
    if (!s.location) s.location = { status: 'idle', result: null, radius: 2500, selectedPoi: null };
    return s.location;
  }
  function locationStatus(response) {
    if (response?.available) return 'ready';
    return response?.reason === 'no_match' ? 'empty' : 'unavailable';
  }

  // Búsquedas recientes (historial en el navegador). Instantáneas; re-buscar una
  // tira del cache del shim → resultado inmediato. Estilo omnibox del navegador.
  const RECENTS_KEY = 'nimos-recent-searches';
  const RECENTS_MAX = 8;
  function loadRecents() { try { return JSON.parse(localStorage.getItem(RECENTS_KEY)) || []; } catch (e) { return []; } }
  let recents = $state(loadRecents());
  function recordRecent(term) {
    term = term.trim();
    if (!term) return;
    recents = [term, ...recents.filter((t) => t.toLowerCase() !== term.toLowerCase())].slice(0, RECENTS_MAX);
    try { localStorage.setItem(RECENTS_KEY, JSON.stringify(recents)); } catch (e) {}
  }
  function removeRecent(term) {
    recents = recents.filter((t) => t !== term);
    try { localStorage.setItem(RECENTS_KEY, JSON.stringify(recents)); } catch (e) {}
  }
  // Solo al escribir: filtra el historial por PREFIJO (empieza-por, predictivo
  // como el autocompletado del navegador). Con la caja vacía NO se muestra nada.
  let shownRecents = $derived.by(() => {
    const q = s.q.trim().toLowerCase();
    if (!q) return [];
    return recents.filter((t) => { const l = t.toLowerCase(); return l.startsWith(q) && l !== q; });
  });
  let showRecents = $derived(focused && shownRecents.length > 0);

  function onInput() {
    clearTimeout(timer);
    clearTimeout(settleTimer);
    const term = s.q;
    if (!term.trim()) {
      ctrl?.abort();
      mapCtrl?.abort();
      s.results = []; s.groups = []; s.images = []; s.searched = false; s.loading = false;
      s.location = { status: 'idle', result: null, radius: ensureLocation().radius, selectedPoi: null };
      return;
    }
    s.loading = true;
    timer = setTimeout(() => run(term), 320);
  }
  function submit(e) { e?.preventDefault(); clearTimeout(timer); focused = false; if (s.q.trim()) { recordRecent(s.q); run(s.q); } }
  function run(term) {
    ctrl?.abort();
    mapCtrl?.abort();
    ctrl = new AbortController();
    s.loading = true;
    const m = s.mode;
    if (m === 'images') {
      s.location = { status: 'idle', result: null, radius: ensureLocation().radius, selectedPoi: null };
      (async () => { try {
        const res = await globalImages(term, { signal: ctrl.signal });
        if (term === s.q && s.mode === 'images') { s.images = res; s.searched = true; s.loading = false; scheduleRecord(term); }
      } catch (e) { if (e?.name !== 'AbortError' && term === s.q) { s.images = []; s.searched = true; s.loading = false; } } })();
      return;
    }

    const radius = ensureLocation().radius;
    s.location = { status: 'loading', result: null, radius, selectedPoi: null };
    mapCtrl = new AbortController();
    (async () => { try {
        const res = await itemSearch(term, { signal: ctrl.signal });
        if (term === s.q && s.mode === 'all') { s.results = res; s.groups = []; s.searched = true; s.loading = false; scheduleRecord(term); }
      } catch (e) { if (e?.name !== 'AbortError' && term === s.q) { s.results = []; s.searched = true; s.loading = false; } } })();
    (async () => { try {
      const response = await mapSearch(term, radius, { signal: mapCtrl.signal });
      if (term === s.q && s.mode === 'all') {
        s.location = { status: locationStatus(response), result: response.available ? response : null, radius, selectedPoi: null };
      }
    } catch (e) {
      if (e?.name !== 'AbortError' && term === s.q && s.mode === 'all') s.location = { status: 'error', result: null, radius, selectedPoi: null };
    } })();
  }
  // Registra el término solo si el usuario se "asienta" en él (no cada prefijo).
  function scheduleRecord(term) {
    clearTimeout(settleTimer);
    settleTimer = setTimeout(() => { if (term === s.q) recordRecent(term); }, 1200);
  }
  function setMode(m) { if (m === s.mode) return; s.mode = m; if (s.q.trim()) { s.searched = false; run(s.q); } }
  function clear() { ctrl?.abort(); mapCtrl?.abort(); clearTimeout(settleTimer); s.q = ''; s.results = []; s.groups = []; s.images = []; s.searched = false; s.loading = false; s.location = { status: 'idle', result: null, radius: ensureLocation().radius, selectedPoi: null }; }
  function pickRecent(term) { s.q = term; focused = false; recordRecent(term); run(term); }
  async function changeRadius(radius) {
    const term = s.q.trim();
    if (!term || s.mode !== 'all') return;
    mapCtrl?.abort();
    mapCtrl = new AbortController();
    const previous = ensureLocation().result;
    s.location = { ...ensureLocation(), status: 'loading', radius, result: previous };
    try {
      const response = await mapSearch(term, radius, { signal: mapCtrl.signal });
      if (term === s.q.trim() && s.mode === 'all') s.location = { status: locationStatus(response), result: response.available ? response : null, radius, selectedPoi: null };
    } catch (e) {
      if (e?.name !== 'AbortError' && term === s.q.trim()) s.location = { status: 'error', result: previous, radius, selectedPoi: null };
    }
  }
  function normalizeResults(results) {
    return (results || []).map((hit) => {
      const book = hit.subtitle || hit.collectionId || '';
      return {
        ...hit,
        thumb: hit.preview?.kind === 'image' ? hit.preview.url : '',
        book,
        // Restaura el icono real del ZIM: el resultado trae el nombre de la
        // colección (subtitle), que casa con la librería del catálogo.
        icon: libraries.find((l) => l.name === book)?.icon,
      };
    }).sort((a, b) => (b.score || 0) - (a.score || 0));
  }

  let activeView = $derived(s.searched || s.loading);
  let pageResults = $derived(normalizeResults(s.results || []));
</script>

<div class="home scroll">
  <div class="top" class:compact={activeView}>
    {#if !activeView}
      <div class="brand">
        <div class="lockup">
          <Logo size={104} />
          <Wordmark height={72} />
        </div>
      </div>
    {/if}
    <div class="searchwrap">
      <form class="bigsearch" onsubmit={submit}>
        <Icon name="search" size={18} />
        <input placeholder={t('home.searchPlaceholder')} value={s.q}
          oninput={(e) => { s.q = e.currentTarget.value; onInput(); }} autocomplete="off"
          onfocus={() => { recents = loadRecents(); focused = true; }} onblur={() => setTimeout(() => focused = false, 120)} />
        {#if s.q}<button type="button" class="clear" onclick={clear} title={t('home.clear')}><Icon name="close" size={16} /></button>{/if}
      </form>
      {#if showRecents}
        <div class="suggest">
          {#each shownRecents as term}
            <div class="sug" role="button" tabindex="-1" onmousedown={(e) => { e.preventDefault(); pickRecent(term); }}>
              <Icon name="clock" size={16} />
              <span class="sgt">{term}</span>
              <button type="button" class="sgx" title={t('home.clear')} onmousedown={(e) => { e.preventDefault(); e.stopPropagation(); removeRecent(term); }}><Icon name="close" size={14} /></button>
            </div>
          {/each}
        </div>
      {/if}
    </div>
    {#if s.q.trim()}
      <div class="modes">
        <button class:on={s.mode === 'all'} onclick={() => setMode('all')}><Icon name="search" size={15} /> {t('home.all')}</button>
        <button class:on={s.mode === 'images'} onclick={() => setMode('images')}><Icon name="image" size={15} /> {t('home.images')}</button>
      </div>
    {/if}
  </div>

  {#if !activeView}
    <div class="browse">
      <div class="blabel">{t('home.sites')}</div>
      {#if anySite}
        <div class="sitegrid">
          {#if siteShown('moments')}
            <button class="site" onclick={() => onOpenView?.('moments')} title={t('home.openSite', { name: t('menu.moments') })}>
              <BrandIcon kind="moments" size={42} radius={21} />
              <span class="sname">{t('menu.moments')}</span>
            </button>
          {/if}
          {#if siteShown('cabinet')}
            <button class="site" onclick={() => onOpenView?.('cabinet')} title={t('home.openSite', { name: t('menu.cabinet') })}>
              <BrandIcon kind="cabinet" size={42} radius={21} />
              <span class="sname">{t('menu.cabinet')}</span>
            </button>
          {/if}
          {#each libraries.filter((l) => siteShown(l.id)) as lib}
            <button class="site" onclick={() => onNavigate?.(lib.id, '')} title={lib.name}>
              <ZimIcon icon={lib.icon} name={lib.name} size={42} radius={21} />
              <span class="sname">{lib.name}</span>
            </button>
          {/each}
        </div>
      {:else}
        <div class="sitesempty">{t('home.sitesEmpty')}</div>
      {/if}
    </div>
  {:else if s.mode === 'images'}
    {#if s.searched && s.images.length}
      <div class="imggrid">
        {#each s.images as im}
          <button class="imgcard" onclick={() => im.itemId ? onOpenItem?.(im.itemId) : onNavigate?.(im.lib, im.path)} title={im.title}>
            <img src={im.thumb} alt={im.title} loading="lazy" />
            <div class="imgcap"><b>{im.title}</b><small class="src"><ZimIcon icon={iconOf(im.lib)} name={im.book} size={15} radius={4} /> {im.book}</small></div>
          </button>
        {/each}
      </div>
    {:else if s.searched}
      <div class="status">{t('home.noImages', { q: s.q })}</div>
    {:else}
      <div class="status">{t('home.searchingImages', { n: libraries.length })}</div>
    {/if}
  {:else}
    {#if s.location?.result}
      <LocationSearchResult locationState={s.location} onRadiusChange={changeRadius} />
    {/if}
    {#if s.searched && pageResults.length}
      <div class="results">
        <div class="resulthead"><b>{t('home.map.collectionResults')}</b><span>{tn('home.results', pageResults.length, { n: pageResults.length.toLocaleString(i18n.locale) })}</span></div>
        <div class="hits">
          {#each pageResults as hit}
            <button class="hit" class:hasthumb={hit.thumb} onclick={() => onOpenItem?.(hit.itemId)}>
              <div class="hbody">
                <div class="htitle">{hit.title}</div>
                {#if hit.snippet}<div class="hsnippet">{hit.snippet}</div>{/if}
                <div class="hmeta">
                  <ZimIcon icon={hit.icon} name={hit.book || hit.kind} size={16} radius={4} />
                  <span>{hit.book}</span>
                  {#if hit.kind}<span class="dot">-</span><span>{kindLabel(hit.kind)}</span>{/if}
                </div>
              </div>
              {#if hit.thumb}<img class="hthumb" src={hit.thumb} alt="" loading="lazy" />{/if}
            </button>
          {/each}
        </div>
      </div>
    {:else if s.searched && !s.location?.result}
      <div class="status">{t('home.noResults', { q: s.q })}</div>
    {:else if !s.searched}
      <div class="status">{t('home.searching', { n: libraries.length })}</div>
    {/if}
  {/if}
</div>

<style>
  .home{background:var(--ground);min-height:100%}
  .top{display:flex;flex-direction:column;align-items:center;padding:54px 24px 10px;transition:padding .2s ease}
  .top.compact{padding:26px 24px 8px;position:sticky;top:0;background:linear-gradient(var(--ground) 78%,transparent);z-index:5}
  .brand{text-align:center;margin-bottom:28px;display:flex;flex-direction:column;align-items:center}
  .lockup{display:flex;flex-direction:column;align-items:center;gap:13px}
  @media (max-width:560px){.top{padding-top:40px}.lockup{gap:11px}}
  .bigsearch{display:flex;align-items:center;gap:10px;width:100%;max-width:600px;height:46px;padding:0 16px;background:var(--card);border:1px solid var(--border);border-radius:12px;box-shadow:var(--shadow);transition:border-color .12s}
  .bigsearch:focus-within{border-color:color-mix(in srgb,var(--accent) 55%,var(--border))}
  .bigsearch :global(.ic){color:var(--muted)}
  .bigsearch input{flex:1;background:none;border:none;outline:none;color:var(--ink);font-size:15px}
  .bigsearch input::placeholder{color:var(--muted)}
  .clear{width:28px;height:28px;border-radius:7px;display:grid;place-items:center;color:var(--muted)}
  .clear:hover{background:var(--raise);color:var(--ink)}
  .searchwrap{position:relative;width:100%;max-width:600px}
  .suggest{position:absolute;top:calc(100% + 8px);left:0;right:0;z-index:20;background:var(--card);border:1px solid var(--border);border-radius:12px;box-shadow:var(--shadow);padding:6px;display:flex;flex-direction:column;gap:2px}
  .sug{display:flex;align-items:center;gap:11px;padding:9px 11px;border-radius:8px;text-align:left;width:100%;cursor:pointer;transition:background .1s}
  .sug:hover{background:var(--raise)}
  .sug :global(.ic){color:var(--muted);flex:none}
  .sug .sgt{flex:1;color:var(--ink-dim);font-size:14.5px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .sug:hover .sgt{color:var(--ink)}
  .sgx{width:24px;height:24px;border-radius:6px;display:grid;place-items:center;color:var(--faint);flex:none;opacity:0;transition:opacity .1s,background .1s}
  .sug:hover .sgx{opacity:1}
  .sgx:hover{background:var(--border);color:var(--ink)}
  .modes{display:flex;gap:4px;margin-top:14px;background:var(--card);border:1px solid var(--border);border-radius:10px;padding:4px}
  .modes button{display:flex;align-items:center;gap:7px;padding:7px 16px;border-radius:7px;font-size:13.5px;color:var(--muted);transition:background .12s,color .12s}
  .modes button:hover{color:var(--ink)}
  .modes button.on{background:color-mix(in srgb,var(--accent) 20%,transparent);color:var(--accent-2);font-weight:550}

  .imggrid{max-width:1600px;margin:0 auto;padding:18px 40px 90px;display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:16px}
  .imgcard{display:flex;flex-direction:column;background:var(--card);border:1px solid var(--border);border-radius:12px;overflow:hidden;text-align:left;transition:border-color .12s,transform .12s;box-shadow:var(--shadow)}
  .imgcard:hover{border-color:color-mix(in srgb,var(--accent) 45%,var(--border));transform:translateY(-2px)}
  .imgcard img{width:100%;height:234px;object-fit:cover;display:block;background:var(--raise)}
  .imgcap{padding:9px 12px}
  .imgcap b{display:block;font-size:13.5px;color:var(--ink);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-weight:600}
  .imgcap small{font-size:11.5px;color:var(--muted)}
  .imgcap small.src{display:flex;align-items:center;gap:5px;margin-top:2px}

  .status{max-width:1560px;margin:30px auto;padding:0 40px;color:var(--muted);text-align:center}
  .results{max-width:1400px;margin:0 auto;padding:10px 40px 90px;display:flex;flex-direction:column;gap:14px}
  .resulthead{display:flex;align-items:baseline;justify-content:space-between;gap:14px;padding:0 14px;color:var(--faint);font-size:13px}
  .resulthead b{color:var(--ink);font-size:14px;font-weight:650}
  .hits{display:flex;flex-direction:column;gap:8px}
  .hit{text-align:left;padding:14px;border-radius:11px;transition:background .12s;width:100%;display:grid;grid-template-columns:minmax(0,1fr);gap:6px}
  .hit.hasthumb{grid-template-columns:minmax(0,1fr) 126px;column-gap:18px;align-items:start}
  .hit:hover{background:var(--card)}
  .hbody{min-width:0;display:flex;flex-direction:column;gap:6px}
  .htitle{color:var(--link);font-size:19px;font-weight:560}
  .hit:hover .htitle{text-decoration:underline}
  .hsnippet{color:var(--muted);font-size:15.5px;line-height:1.58;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
  .hmeta{display:flex;align-items:center;gap:6px;color:var(--faint);font-size:13px;line-height:1.2;min-height:18px}
  .hmeta .dot{color:var(--border);padding:0 1px}
  .hthumb{width:126px;height:94px;object-fit:cover;border-radius:9px;background:var(--raise);border:1px solid var(--border);align-self:start}
  @media (max-width:720px){.hit.hasthumb{grid-template-columns:minmax(0,1fr) 92px;column-gap:12px}.hthumb{width:92px;height:72px}.hsnippet{-webkit-line-clamp:3}}

  /* Barra de marcadores (páginas guardadas) — estilo barra de favoritos del navegador. */

  /* Accesos directos a los sitios (ZIMs · Moments · Cabinet). */
  .browse{max-width:1200px;margin:0 auto;padding:30px 24px 60px}
  .blabel{font-size:12px;font-weight:650;letter-spacing:.7px;text-transform:uppercase;color:var(--faint);text-align:center;margin-bottom:18px}
  /* Launcher: icono redondo + nombre centrado debajo. */
  .sitegrid{display:flex;flex-wrap:wrap;justify-content:center;gap:20px 10px}
  .site{display:flex;flex-direction:column;align-items:center;gap:8px;width:78px;padding:8px 4px 6px;border-radius:14px;transition:background .12s}
  .site:hover{background:var(--raise)}
  .site :global(.zt),.site :global(.bi){transition:transform .12s,box-shadow .12s}
  .site:hover :global(.zt),.site:hover :global(.bi){transform:translateY(-2px);box-shadow:0 6px 16px color-mix(in srgb,var(--accent) 22%,transparent)}
  .sname{font-size:12.5px;color:var(--ink-dim);text-align:center;line-height:1.3;max-width:100%;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .site:hover .sname{color:var(--ink)}
  .sitesempty{text-align:center;color:var(--muted);font-size:13px;padding:14px 0 4px;line-height:1.55}
</style>
