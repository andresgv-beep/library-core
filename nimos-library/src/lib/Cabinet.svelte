<script>
  // Cabinet.svelte — portada EDITORIAL del archivo documental local
  // (offline). Identidad propia de museo/biblioteca (serif + pergamino + dorado),
  // distinta del resto de Library — decisión del usuario 2026-07-12, norte =
  // (maquetación editorial propia).
  //
  // Solo BROWSE: home editorial → vista filtrada (formato/tema/época/búsqueda);
  // un item se abre como PESTAÑA del lector (ItemPage) vía onOpenItem, nunca en un
  // navegador anidado. Importar/gestionar cola vive en el Panel de Control.

  import { onMount } from 'svelte';
  import { getCollections, getCollectionItems } from './libraryApi.js';
  import { getRecent } from './readerStateApi.js';
  import { t } from './i18n.svelte.js';

  let { onOpenItem } = $props();

  let items = $state([]);
  let collections = $state([]);
  let loading = $state(true);
  let errMsg = $state('');
  let query = $state('');
  let scroller = $state(null);
  // browse = null → home; si no, { mode:'type'|'theme'|'era'|'search', kinds?, value?, label }
  let browse = $state(null);
  let recentOpened = $state([]);   // "Seguir leyendo": textos ya abiertos (historial)
  // Rieles con flechas doradas (sin barra de scroll). Un ref + estado de flechas por riel.
  let recentEl = $state(null), continueEl = $state(null);
  let recNav = $state({ l: false, r: false });
  let contNav = $state({ l: false, r: false });
  function updNav(el, nav) { if (!el) return; nav.l = el.scrollLeft > 4; nav.r = el.scrollLeft + el.clientWidth < el.scrollWidth - 4; }
  function railScroll(el, dir) { el?.scrollBy({ left: dir * Math.round((el.clientWidth || 600) * 0.82), behavior: 'smooth' }); }
  // Action: al montar (y al redimensionar) provoca el mismo camino reactivo que el
  // scroll manual disparando un evento 'scroll' → el onscroll del riel recalcula las
  // flechas. (Llamar updNav directo desde aquí no reflejaba el estado al montar.)
  function railwatch(node) {
    const kick = () => node.dispatchEvent(new Event('scroll'));
    requestAnimationFrame(kick); setTimeout(kick, 60);
    let ro; try { ro = new ResizeObserver(kick); ro.observe(node); } catch (e) {}
    return { destroy() { ro?.disconnect(); } };
  }

  async function load() {
    loading = true; errMsg = '';
    try {
      collections = (await getCollections()).filter((c) => c.kind === 'media');
      const groups = await Promise.all(collections.map(async (collection) => {
        const list = await getCollectionItems(collection.id);
        return list.map((it) => ({ ...it, sectionId: collection.id, sectionName: collection.title }));
      }));
      // Dedup por id (las colecciones media se solapan: la raíz contiene los mismos
      // items que sus subcarpetas). Los vídeos NO son de aquí (viven en Moments).
      const seen = new Set();
      items = groups.flat()
        .filter((it) => it.source?.provider === 'cabinet')
        .filter((it) => (seen.has(it.id) ? false : (seen.add(it.id), true)));
      pickFeatured(); // rota el destacado en cada carga de Cabinet
      await loadContinue();
    }
    catch (e) { errMsg = e.message || t('cabinet.error'); }
    finally { loading = false; }
  }
  onMount(load);

  // Seguir leyendo: textos/documentos ya abiertos (del historial reciente), en el
  // orden en que se abrieron. Excluye audio/vídeo (aquí es "leer").
  async function loadContinue() {
    try {
      const rows = await getRecent();
      const byId = new Map(items.map((it) => [it.id, it]));
      const seenC = new Set();
      recentOpened = (rows || [])
        .map((r) => r && r.itemId && byId.get(r.itemId))
        .filter(Boolean)
        .filter((it) => !isAudio(it) && !isVideo(it))
        .filter((it) => (seenC.has(it.id) ? false : (seenC.add(it.id), true)))
        .slice(0, 14);
    } catch (e) { recentOpened = []; }
  }


  // ── Derivados de datos reales ───────────────────────────────────────────────
  const previewURL = (it) => it?.preview?.kind === 'image' ? it.preview.url : '';
  const openMode = (it) => it?.open?.mode || it?.kind;
  const isVideo = (it) => it?.kind === 'video' || openMode(it) === 'video';
  const isAudio = (it) => it?.kind === 'audio' || openMode(it) === 'audio';
  const isImage = (it) => it?.kind === 'image' || openMode(it) === 'image';
  const artClass = (it) => (isVideo(it) ? 'wide' : isImage(it) ? 'sq' : 'book');
  const TYPE_KEY = { video: 'cabinet.kind.video', pdf: 'cabinet.kind.text', document: 'cabinet.kind.text', image: 'cabinet.kind.image', audio: 'cabinet.kind.audio' };
  const kindLabel = (kind) => t(TYPE_KEY[kind] || 'cabinet.kind.doc');
  const authors = (it) => (it.authors || []).join(', ');
  const sub = (it) => [authors(it), kindLabel(it.kind)].filter(Boolean).join(' · ');
  const yearOf = (it) => { const m = /(\d{4})/.exec(it?.date || ''); return m ? +m[1] : null; };
  const pieces = (n) => `${n} ${t(n === 1 ? 'cabinet.pieces.one' : 'cabinet.pieces.other')}`;

  function matches(it, value) {
    const needle = (value || '').trim().toLocaleLowerCase();
    if (!needle) return true;
    return [it.title, it.description, ...(it.authors || []), ...(it.tags || [])].filter(Boolean).join(' ').toLocaleLowerCase().includes(needle);
  }

  const FORMATS = [
    { key: 'text',  kinds: ['pdf', 'document'], glyph: 'Aa', label: () => t('cabinet.kind.text'),  sub: () => t('cabinet.textsSub') },
    { key: 'video', kinds: ['video'],           glyph: '▷',  label: () => t('cabinet.kind.video'), sub: () => t('cabinet.videoSub') },
    { key: 'audio', kinds: ['audio'],           glyph: '♫',  label: () => t('cabinet.kind.audio'), sub: () => t('cabinet.audioSub') },
    { key: 'all',   kinds: null,                glyph: '◇',  label: () => t('cabinet.allArchive'), sub: () => t('cabinet.allArchiveSub') },
  ];
  const fmtCount = (f) => f.kinds ? items.filter((it) => f.kinds.includes(it.kind)).length : items.length;

  // Recién incorporado: orden de escaneo (lo último que entró al pool va primero
  // dentro de cada colección; sin timestamp de alta usamos el orden de carga).
  let recent = $derived(items.slice(0, 12));

  let typeCount = $derived(new Set(items.map((it) => TYPE_KEY[it.kind] || 'cabinet.kind.doc')).size);

  // Temas: agregación de etiquetas del editor → rutas de exploración.
  let themes = $derived.by(() => {
    const m = new Map();
    for (const it of items) for (const tg of (it.tags || [])) {
      const k = (tg || '').trim(); if (!k) continue;
      m.set(k, (m.get(k) || 0) + 1);
    }
    return [...m.entries()].sort((a, b) => b[1] - a[1]).map(([tag, n]) => ({ tag, n }));
  });
  let topThemes = $derived(themes.slice(0, 14));

  // Por época: buckets de medio siglo, altura ∝ nº de piezas.
  let eras = $derived.by(() => {
    const m = new Map();
    for (const it of items) { const y = yearOf(it); if (y == null) continue; const e = Math.floor(y / 50) * 50; m.set(e, (m.get(e) || 0) + 1); }
    const arr = [...m.entries()].sort((a, b) => a[0] - b[0]);
    const max = Math.max(1, ...arr.map(([, n]) => n));
    return arr.map(([era, n]) => ({ era, n, pct: Math.max(14, Math.round(n / max * 100)) }));
  });

  // Pieza destacada: SOLO textos/libros/archivos (nada de audio ni vídeo — el hero
  // es el "escaparate de libro"). ROTA EN CADA VISITA: al cargar Cabinet se elige
  // una al azar entre los textos con portada (para que luzca); se guarda la última
  // en localStorage para no repetirla dos veces seguidas. Si no hay texto → sin hero.
  let featured = $state(null);
  function pickFeatured() {
    const texts = items.filter((it) => !isAudio(it) && !isVideo(it));
    const pool = texts.filter((it) => previewURL(it));
    const base = pool.length ? pool : texts;
    if (!base.length) { featured = null; return; }
    let choices = base;
    try {
      const last = localStorage.getItem('nimos-arch-featured');
      if (last && base.length > 1) { const f = base.filter((it) => it.id !== last); if (f.length) choices = f; }
    } catch (e) {}
    const pick = choices[Math.floor(Math.random() * choices.length)];
    featured = pick;
    try { localStorage.setItem('nimos-arch-featured', pick.id); } catch (e) {}
  }

  // Fondo del hero teñido con el color dominante de la portada: se dibuja la tapa
  // en un canvas pequeño (mismo origen → sin taint), se promedia el color y se
  // construye un degradado elegante con ese tono. Cada libro → su propio ambiente.
  let heroTint = $state(null);   // {r,g,b} o null
  let lastTintId = '';
  function sampleCover(url) {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.onload = () => {
      try {
        const c = document.createElement('canvas');
        const w = c.width = 24, h = c.height = 32;
        const ctx = c.getContext('2d', { willReadFrequently: true });
        ctx.drawImage(img, 0, 0, w, h);
        const d = ctx.getImageData(0, 0, w, h).data;
        let r = 0, g = 0, b = 0, n = 0;
        for (let i = 0; i < d.length; i += 4) {
          if (d[i + 3] < 128) continue;            // ignora transparente
          r += d[i]; g += d[i + 1]; b += d[i + 2]; n++;
        }
        heroTint = n ? { r: Math.round(r / n), g: Math.round(g / n), b: Math.round(b / n) } : null;
      } catch (e) { heroTint = null; }
    };
    img.onerror = () => { heroTint = null; };
    img.src = url;
  }
  $effect(() => {
    const f = featured;
    const id = f?.id || '';
    if (id === lastTintId) return;
    lastTintId = id;
    heroTint = null;
    const url = f && previewURL(f);
    if (url) sampleCover(url);
  });
  // Degradado del panel del hero a partir del tinte (o dorado por defecto). Incluye
  // el pinstripe diagonal: una línea CLARA fina + otra más marcada, teñidas con el
  // color de la portada (doradas en marrón, azuladas en azul…), sobre bandas
  // oscuras del propio fondo → líneas más claras y más oscuras alternadas.
  const pinstripe = (line) =>
    `repeating-linear-gradient(98deg,rgba(${line},.55) 0 1px,transparent 1px 22px,rgba(${line},.26) 22px 24px,transparent 24px 47px)`;
  let heroBg = $derived.by(() => {
    const t = heroTint;
    if (!t) return `${pinstripe('224,178,110')}, radial-gradient(circle at 32% 36%,rgba(224,168,103,.14),transparent 34%), linear-gradient(125deg,#2a2118,#0e0f12 72%)`;
    const { r, g, b } = t;
    // Color de la línea = tinte "subido" para que brille sobre el fondo oscuro.
    const line = `${Math.min(255, Math.round(r * 1.5) + 55)},${Math.min(255, Math.round(g * 1.5) + 55)},${Math.min(255, Math.round(b * 1.5) + 45)}`;
    const glow = `rgba(${r},${g},${b},.30)`;
    const mid = `rgb(${Math.round(r * 0.40)},${Math.round(g * 0.40)},${Math.round(b * 0.40)})`;
    const dark = `rgb(${Math.round(r * 0.12) + 7},${Math.round(g * 0.12) + 7},${Math.round(b * 0.12) + 8})`;
    return `${pinstripe(line)}, radial-gradient(circle at 32% 36%,${glow},transparent 32%), linear-gradient(125deg,${mid},${dark} 72%)`;
  });

  // ── Vista filtrada (browse) = catálogo con facetas ──────────────────────────
  // filtered = conjunto BASE (lo que abrió el apartado: formato/tema/época/búsqueda).
  let filtered = $derived.by(() => {
    if (!browse) return [];
    let list = items;
    if (browse.mode === 'type' && browse.kinds) list = list.filter((it) => browse.kinds.includes(it.kind));
    else if (browse.mode === 'theme') list = list.filter((it) => (it.tags || []).some((tg) => (tg || '').toLowerCase() === browse.value.toLowerCase()));
    else if (browse.mode === 'era') list = list.filter((it) => { const y = yearOf(it); return y != null && y >= browse.value && y < browse.value + 50; });
    else if (browse.mode === 'search') list = list.filter((it) => matches(it, browse.value));
    return list;
  });

  // Facetas del sidebar (autor / fuente / temas / año), calculadas sobre el BASE.
  let fFrom = $state(null), fTo = $state(null);
  let fAuthors = $state([]), fSources = $state([]), fThemes = $state([]);
  let sortBy = $state('recent');
  function resetFacets() { fFrom = null; fTo = null; fAuthors = []; fSources = []; fThemes = []; sortBy = 'recent'; }
  const toggleIn = (arr, v) => (arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v]);
  // Togglear una faceta acorta/alarga la lista → la página encoge y el navegador
  // reajusta el scroll (salta arriba). Preservamos la posición del contenedor.
  function toggleFacet(which, v) {
    const y = scroller ? scroller.scrollTop : 0;
    if (which === 'author') fAuthors = toggleIn(fAuthors, v);
    else if (which === 'source') fSources = toggleIn(fSources, v);
    else fThemes = toggleIn(fThemes, v);
    requestAnimationFrame(() => { if (scroller) scroller.scrollTop = y; });
  }
  const facetHasSel = $derived(fAuthors.length || fSources.length || fThemes.length || fFrom != null || fTo != null);

  function aggregate(list, keyer, limit = 12) {
    const m = new Map();
    for (const it of list) for (const v of keyer(it)) { const k = (v || '').toString().trim(); if (!k) continue; m.set(k, (m.get(k) || 0) + 1); }
    return [...m.entries()].sort((a, b) => b[1] - a[1]).slice(0, limit).map(([value, n]) => ({ value, n }));
  }
  let facetAuthors = $derived(aggregate(filtered, (it) => it.authors || [], 10));
  let facetSources = $derived(aggregate(filtered, (it) => (it.sectionName ? [it.sectionName] : []), 10));
  let facetThemes = $derived(aggregate(filtered, (it) => it.tags || [], 12));
  let yearBounds = $derived.by(() => { const ys = filtered.map(yearOf).filter((v) => v != null); return ys.length ? { min: Math.min(...ys), max: Math.max(...ys) } : null; });

  // Resultado final: base + facetas + orden.
  let catalogItems = $derived.by(() => {
    const list = filtered.filter((it) => {
      const y = yearOf(it);
      if ((fFrom != null || fTo != null)) { if (y == null) return false; if (fFrom != null && y < fFrom) return false; if (fTo != null && y > fTo) return false; }
      if (fAuthors.length && !(it.authors || []).some((a) => fAuthors.includes(a))) return false;
      if (fSources.length && !(it.sectionName && fSources.includes(it.sectionName))) return false;
      if (fThemes.length && !(it.tags || []).some((tg) => fThemes.includes(tg))) return false;
      return true;
    });
    const cmp = {
      recent: (a, b) => (yearOf(b) || 0) - (yearOf(a) || 0),
      old: (a, b) => (yearOf(a) || 9999) - (yearOf(b) || 9999),
      az: (a, b) => (a.title || '').localeCompare(b.title || ''),
    }[sortBy] || (() => 0);
    return [...list].sort(cmp);
  });

  // ── Navegación interna ──────────────────────────────────────────────────────
  function toTop() { if (scroller) scroller.scrollTop = 0; }
  function toHome() { browse = null; query = ''; resetFacets(); toTop(); }
  function openFormat(f) { resetFacets(); browse = { mode: 'type', kinds: f.kinds, label: f.label() }; toTop(); }
  function openTheme(tag) { resetFacets(); browse = { mode: 'theme', value: tag, label: tag }; toTop(); }
  function openEra(era) { resetFacets(); browse = { mode: 'era', value: era, label: `${era}–${era + 49}` }; toTop(); }
  function doSearch(e) {
    e?.preventDefault();
    const q = query.trim();
    if (!q) { toHome(); return; }
    resetFacets();
    browse = { mode: 'search', value: q, label: `${t('cabinet.resultsFor')} «${q}»` };
    toTop();
  }
  function openItem(it) { if (it) onOpenItem?.(it.id); }
  function onKey(e) { if (e.key === 'Escape' && browse) toHome(); }
</script>

<svelte:window onkeydown={onKey} />

<div class="aw scroll" bind:this={scroller}>
  <div class="wrap">
    <!-- Masthead editorial (identidad, sin controles de navegación: esos viven en
         el navbar del lector para no anidar chrome) -->
    <header class="amast">
      <button class="brand" onclick={toHome}><i>Nimos</i> {t('cabinet.brand')}</button>
      <span class="crumb">{t('cabinet.localLibrary')} · {pieces(items.length)}</span>
    </header>

    <!-- Tarjeta editorial reutilizable (rieles + grid) -->
    {#snippet card(it)}
      <article class="acard" onclick={() => openItem(it)}>
        <div class="acover {artClass(it)}" class:img={previewURL(it)}>
          {#if previewURL(it)}<img src={previewURL(it)} alt={it.title} loading="lazy" />{/if}
          {#if yearOf(it)}<span class="ayear">{yearOf(it)}</span>{/if}
          {#if isVideo(it)}<span class="aplay">▶</span>{/if}
        </div>
        <h3>{it.title}</h3>
        <p>{sub(it)}</p>
      </article>
    {/snippet}

    {#if errMsg}
      <div class="err">{errMsg}</div>
    {:else if loading}
      <div class="hint">{t('cabinet.loading')}</div>
    {:else if items.length === 0}
      <div class="empty">
        <div class="ei">▤</div>
        <div class="et">{t('cabinet.emptyTitle')}</div>
        <div class="eb">{t('cabinet.emptyBody')}</div>
      </div>

    {:else if browse}
      <!-- ══ CATÁLOGO FACETADO (vista de lista) ══ -->
      <header class="cataloghead">
        <button class="aback" onclick={toHome}>← {t('cabinet.brand')}</button>
        <div>
          <h1>{browse.label}</h1>
          <p>{pieces(filtered.length)} · {t('cabinet.organized')}</p>
        </div>
      </header>
      <div class="cataloggrid">
        <aside class="filterside" aria-label={t('cabinet.sort')}>
          {#if yearBounds}
            <div class="facet">
              <h3>{t('cabinet.fYear')}</h3>
              <div class="range">
                <input type="number" bind:value={fFrom} placeholder={yearBounds.min} aria-label="{t('cabinet.fYear')} {t('cabinet.fFrom')}" />
                <input type="number" bind:value={fTo} placeholder={yearBounds.max} aria-label="{t('cabinet.fYear')} {t('cabinet.fTo')}" />
              </div>
            </div>
          {/if}
          {#if facetAuthors.length}
            <div class="facet">
              <h3>{t('cabinet.fAuthor')}</h3>
              {#each facetAuthors as a}
                <label><input type="checkbox" checked={fAuthors.includes(a.value)} onchange={() => toggleFacet('author', a.value)} /> {a.value} <span>{a.n}</span></label>
              {/each}
            </div>
          {/if}
          {#if facetSources.length}
            <div class="facet">
              <h3>{t('cabinet.fSource')}</h3>
              {#each facetSources as s}
                <label><input type="checkbox" checked={fSources.includes(s.value)} onchange={() => toggleFacet('source', s.value)} /> {s.value} <span>{s.n}</span></label>
              {/each}
            </div>
          {/if}
          {#if facetThemes.length}
            <div class="facet">
              <h3>{t('cabinet.themes')}</h3>
              {#each facetThemes as th}
                <label><input type="checkbox" checked={fThemes.includes(th.value)} onchange={() => toggleFacet('theme', th.value)} /> {th.value} <span>{th.n}</span></label>
              {/each}
            </div>
          {/if}
        </aside>
        <div>
          <div class="resulttools">
            <span><b>{catalogItems.length}</b> {t('cabinet.resultsWord')}</span>
            {#if facetHasSel}<button class="clearf" onclick={resetFacets}>{t('cabinet.clearFilters')}</button>{/if}
            <select bind:value={sortBy} aria-label={t('cabinet.sort')}>
              <option value="recent">{t('cabinet.sortRecent')}</option>
              <option value="old">{t('cabinet.sortOld')}</option>
              <option value="az">{t('cabinet.sortAZ')}</option>
            </select>
          </div>
          {#if catalogItems.length === 0}
            <div class="hint">{t('cabinet.noResults')}</div>
          {:else}
            <div class="resultlist">
              {#each catalogItems as it (it.id)}
                <article class="result" onclick={() => openItem(it)}>
                  <div class="thumb {artClass(it)}" class:img={previewURL(it)}>
                    {#if previewURL(it)}<img src={previewURL(it)} alt={it.title} loading="lazy" />{/if}
                    {#if isVideo(it)}<span class="tplay">▶</span>{/if}
                  </div>
                  <div class="rinfo">
                    <h2>{it.title}</h2>
                    {#if authors(it)}<div class="rby">{authors(it)}</div>{/if}
                    {#if it.description}<p class="rdesc">{it.description}</p>{/if}
                    <div class="rdata">
                      {#if yearOf(it)}<b>{yearOf(it)}</b>{/if}
                      {#if it.sectionName}<span>{it.sectionName}</span>{/if}
                      {#if (it.tags || []).length}<span>{it.tags.slice(0, 3).join(' · ')}</span>{/if}
                      <span>{kindLabel(it.kind)}</span>
                    </div>
                  </div>
                  <span class="status">● {t('cabinet.offlineStatus')}</span>
                </article>
              {/each}
            </div>
          {/if}
        </div>
      </div>

    {:else}
      <!-- ══ PORTADA EDITORIAL ══ -->
      {#if featured}
        <section class="ahero">
          <div class="aheroart" style="background:{heroBg}">
            <div class="abook" class:cover={previewURL(featured)}>
              {#if previewURL(featured)}
                <img src={previewURL(featured)} alt={featured.title} />
              {:else}
                <b>{featured.title}</b>{#if authors(featured)}<small>{authors(featured)}</small>{/if}
              {/if}
            </div>
          </div>
          <div class="aherocopy">
            <span class="kicker">{yearOf(featured) ? `${t('cabinet.featured')} · ${yearOf(featured)}` : t('cabinet.featured')}</span>
            <h1>{featured.title}</h1>
            {#if featured.description}<p>{featured.description}</p>{/if}
            <div class="hmeta">
              <span>{kindLabel(featured.kind)}</span>
              {#if featured.language}<span>{featured.language}</span>{/if}
              {#if authors(featured)}<span>{authors(featured)}</span>{/if}
            </div>
            <button class="primary" onclick={() => openItem(featured)}>{t('cabinet.openPiece')}</button>
          </div>
        </section>
      {/if}

      <form class="asearch" onsubmit={doSearch}>
        <span class="sic">⌕</span>
        <input aria-label={t('cabinet.searchPlaceholder')} placeholder={t('cabinet.searchPlaceholder')} bind:value={query} />
        <button type="submit">{t('cabinet.searchBtn')}</button>
      </form>

      <nav class="aformats">
        {#each FORMATS as f}
          <button class="aformat" onclick={() => openFormat(f)}>
            <span class="fic">{f.glyph}</span>
            <span class="ftxt"><b>{f.label()}</b><small>{f.sub()}</small></span>
            <em>{fmtCount(f)}</em>
          </button>
        {/each}
      </nav>

      {#if recent.length}
        <section class="asection">
          <div class="ahead"><h2>{t('cabinet.recent')}</h2><p>{t('cabinet.heroSub')}</p></div>
          <div class="railwrap">
            <button class="railnav l" class:on={recNav.l} onclick={() => railScroll(recentEl, -1)} aria-label={t('cabinet.prev')}>‹</button>
            <div class="arail" bind:this={recentEl} use:railwatch onscroll={(e) => updNav(e.currentTarget, recNav)}>
              {#each recent as it (it.id)}{@render card(it)}{/each}
            </div>
            <button class="railnav r" class:on={recNav.r} onclick={() => railScroll(recentEl, 1)} aria-label={t('cabinet.next')}>›</button>
          </div>
        </section>
      {/if}

      {#if recentOpened.length}
        <section class="asection">
          <div class="ahead"><h2>{t('cabinet.continueReading')}</h2><p>{t('cabinet.continueReadingSub')}</p></div>
          <div class="railwrap">
            <button class="railnav l" class:on={contNav.l} onclick={() => railScroll(continueEl, -1)} aria-label={t('cabinet.prev')}>‹</button>
            <div class="arail" bind:this={continueEl} use:railwatch onscroll={(e) => updNav(e.currentTarget, contNav)}>
              {#each recentOpened as it (it.id)}{@render card(it)}{/each}
            </div>
            <button class="railnav r" class:on={contNav.r} onclick={() => railScroll(continueEl, 1)} aria-label={t('cabinet.next')}>›</button>
          </div>
        </section>
      {/if}

      {#if topThemes.length || eras.length}
        <section class="asection">
          <div class="ahead"><h2>{t('cabinet.explore')}</h2><p>{t('cabinet.exploreSub')}</p></div>
          <div class="abrowse">
            {#if topThemes.length}
              <div>
                <h3>{t('cabinet.themes')}</h3>
                <div class="atags">
                  {#each topThemes as th}
                    <button class="atag" onclick={() => openTheme(th.tag)}>{th.tag} <small>{th.n}</small></button>
                  {/each}
                </div>
                <div class="logic">{t('cabinet.themesLogic')}</div>
              </div>
            {/if}
            {#if eras.length}
              <div>
                <h3>{t('cabinet.byEra')}</h3>
                <div class="aeras" style="--cols:{eras.length}">
                  {#each eras as e}
                    <button class="abar" style="height:{e.pct}%" title={pieces(e.n)} onclick={() => openEra(e.era)}><span>{e.era}</span></button>
                  {/each}
                </div>
                <div class="logic">{t('cabinet.eraLogic')}</div>
              </div>
            {/if}
          </div>
        </section>
      {/if}

      <div class="afoot">
        <span>{items.length} {t('cabinet.footConserved')} · {typeCount} {t('cabinet.footTypes')} · {themes.length} {t('cabinet.footThemes')} · {t('cabinet.offline')}</span>
      </div>
    {/if}
  </div>
</div>

<style>
  /* ── Identidad editorial de Cabinet (paleta cálida propia, dark, serif) ── */
  .aw {
    --a-bg: #101114; --a-panel: #17181d; --a-panel2: #1d1f25; --a-line: #2a2d35;
    --a-ink: #f2efe8; --a-dim: #aaa69e; --a-faint: #77746f;
    --a-accent: #e0a867; --a-accent2: #89a898;
    --a-serif: Georgia, "Times New Roman", serif;
    flex: 1; min-width: 0; height: 100%; overflow-y: auto;
    background: var(--a-bg); color: var(--a-ink);
    font-size: 14px;
  }
  .wrap { max-width: 1480px; margin: 0 auto; padding: 20px 28px 70px; }
  .err { font-size: 13px; color: #da6b74; background: color-mix(in srgb,#da6b74 12%,transparent); border: 1px solid color-mix(in srgb,#da6b74 34%,var(--a-line)); border-radius: 8px; padding: 11px 13px; margin-top: 18px; }
  .hint { color: var(--a-faint); font-size: 14px; padding: 46px 4px; text-align: center; }
  .empty { text-align: center; padding: 66px 20px; color: var(--a-dim); }
  .empty .ei { font-size: 42px; color: var(--a-faint); } .empty .et { font-family: var(--a-serif); font-size: 22px; color: var(--a-ink); margin: 14px 0 8px; }
  .empty .eb { font-size: 13.5px; max-width: 440px; margin: 0 auto; line-height: 1.6; color: var(--a-faint); }

  /* Masthead */
  .amast { height: 54px; display: flex; align-items: center; gap: 16px; border-bottom: 1px solid var(--a-line); margin-bottom: 26px; }
  .brand { font-family: var(--a-serif); font-size: 19px; letter-spacing: .02em; color: var(--a-ink); background: none; border: 0; padding: 0; cursor: pointer; }
  .brand i { font-style: normal; color: var(--a-accent); }
  .crumb { color: var(--a-faint); font-size: 12px; }

  /* Hero destacado */
  .ahero { display: grid; grid-template-columns: minmax(0,1.2fr) minmax(360px,.8fr); min-height: 360px; border: 1px solid var(--a-line); background: var(--a-panel); overflow: hidden; border-radius: 4px; }
  /* Panel del hero: fondo teñido con la portada (inline) + libro recto y centrado.
     La altura la marca la fila (el texto), no la imagen. */
  .aheroart { position: relative; overflow: hidden; transition: background .4s ease; }
  .abook { position: absolute; left: 50%; top: 50%; transform: translate(-50%,-50%); width: 200px; aspect-ratio: 3/4;
    box-shadow: 0 26px 54px rgba(0,0,0,.55); border: 1px solid rgba(0,0,0,.35); border-radius: 2px; }
  /* Variante libro de texto (sin portada): tapa de cuero con título */
  .abook:not(.cover) { background: linear-gradient(90deg,#362519 0 8%,#88613b 8% 10%,#684929 10% 94%,#bd8a53 94%);
    padding: 30px 22px; color: #e7d2ae; text-align: center; font-family: var(--a-serif); display: flex; flex-direction: column; justify-content: center; }
  .abook:not(.cover) b { display: block; font-size: 19px; line-height: 1.15; }
  .abook:not(.cover) small { display: block; margin-top: 12px; letter-spacing: .14em; font-size: 11px; color: #cbb89a; opacity: .85; }
  /* Variante con portada real: marco de libro con la imagen */
  .abook.cover { overflow: hidden; background: #0b0c0e; }
  .abook.cover img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .aherocopy { padding: 38px 40px; display: flex; flex-direction: column; justify-content: center; background: linear-gradient(145deg,var(--a-panel2),var(--a-panel)); }
  .kicker { color: var(--a-accent); text-transform: uppercase; letter-spacing: .16em; font-size: 11px; font-weight: 700; }
  .aherocopy h1 { font-family: var(--a-serif); font-weight: 400; font-size: clamp(26px,2.7vw,40px); line-height: 1.06; margin: 12px 0 11px; }
  .aherocopy p { color: var(--a-dim); line-height: 1.62; margin: 0; max-width: 520px; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
  .hmeta { display: flex; flex-wrap: wrap; gap: 16px; color: var(--a-faint); font-size: 12px; margin-top: 17px; }
  .primary { align-self: flex-start; margin-top: 26px; border: 0; background: var(--a-ink); color: #151515; border-radius: 8px; padding: 12px 18px; font-weight: 750; cursor: pointer; transition: transform .12s; }
  .primary:hover { transform: translateY(-2px); }

  /* Buscador elevado */
  .asearch { display: flex; align-items: center; gap: 12px; max-width: 840px; margin: -25px auto 0; position: relative; z-index: 2;
    background: #202127; border: 1px solid #3a3c45; border-radius: 12px; padding: 8px 9px 8px 18px; box-shadow: 0 16px 35px rgba(0,0,0,.34); }
  .asearch .sic { color: var(--a-faint); font-size: 18px; }
  .asearch input { flex: 1; min-width: 0; border: 0; outline: 0; background: transparent; color: var(--a-ink); font: inherit; font-size: 15px; }
  .asearch input::placeholder { color: var(--a-faint); }
  .asearch button { border: 0; background: var(--a-accent); color: #1b160f; border-radius: 8px; padding: 11px 17px; font-weight: 750; cursor: pointer; }

  /* Selector de formato */
  .aformats { display: grid; grid-template-columns: repeat(4,1fr); margin-top: 40px; border-top: 1px solid var(--a-line); border-bottom: 1px solid var(--a-line); }
  .aformat { display: grid; grid-template-columns: 42px 1fr auto; gap: 12px; align-items: center; padding: 18px 20px; border: 0; border-right: 1px solid var(--a-line); background: transparent; color: var(--a-ink); text-align: left; cursor: pointer; transition: background .12s; }
  .aformat:last-child { border-right: 0; }
  .aformat:hover { background: var(--a-panel); }
  .fic { width: 40px; height: 40px; border-radius: 50%; background: #24262c; display: grid; place-items: center; color: var(--a-accent); font-family: var(--a-serif); font-size: 18px; }
  .ftxt b { display: block; font-size: 14px; }
  .ftxt small { color: var(--a-faint); font-size: 12px; }
  .aformat em { font-style: normal; color: var(--a-faint); font-family: var(--a-serif); font-size: 16px; }

  /* Secciones */
  .asection { margin-top: 38px; }
  .ahead { display: flex; align-items: baseline; margin-bottom: 16px; }
  .ahead h2 { font-family: var(--a-serif); font-weight: 400; font-size: 26px; margin: 0; }
  .ahead p { color: var(--a-faint); margin: 0 0 0 14px; font-size: 12.5px; }

  /* Riel + tarjetas editoriales — sin barra de scroll, flechas doradas a los lados */
  .railwrap { position: relative; }
  .arail { display: flex; gap: 18px; overflow-x: auto; scroll-behavior: smooth; padding: 4px 2px 12px; scrollbar-width: none; -ms-overflow-style: none; }
  .arail::-webkit-scrollbar { display: none; }
  .arail .acard { width: 168px; flex: none; }
  .railnav { position: absolute; top: 42%; transform: translateY(-50%); z-index: 4; width: 42px; height: 42px; border-radius: 50%;
    display: none; place-items: center; cursor: pointer; font-size: 24px; line-height: 1; font-family: var(--a-serif);
    background: rgba(16,17,20,.82); border: 1px solid var(--a-accent); color: var(--a-accent);
    box-shadow: 0 6px 20px rgba(0,0,0,.55); transition: background .12s, color .12s, transform .12s; }
  .railnav.on { display: grid; }
  .railnav:hover { background: var(--a-accent); color: #1b160f; transform: translateY(-50%) scale(1.06); }
  .railnav.l { left: -8px; } .railnav.r { right: -8px; }
  .agrid { display: grid; grid-template-columns: repeat(auto-fill,minmax(158px,1fr)); gap: 20px; }
  .acard { min-width: 0; cursor: pointer; background: none; border: 0; padding: 0; color: inherit; text-align: left; }
  .acover { aspect-ratio: 3/4; border: 1px solid var(--a-line); position: relative; overflow: hidden; background: var(--a-panel2); transition: transform .18s; border-radius: 3px; }
  .acard:hover .acover { transform: translateY(-4px); }
  .acover:after { content: ""; position: absolute; inset: 0; box-shadow: inset 0 -60px 50px -42px #000; pointer-events: none; }
  .acover.book { background: linear-gradient(150deg,#6d5334,#241a12); }
  .acover.wide { background: linear-gradient(150deg,#3a5236,#141d16); }
  .acover.sq   { background: linear-gradient(150deg,#4a4636,#16150f); }
  .acover.img  { background: #0b0c0e; }
  .acover img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .ayear { position: absolute; top: 10px; right: 10px; background: rgba(12,12,12,.72); padding: 4px 7px; border-radius: 4px; color: #ddd; font-size: 10px; z-index: 1; }
  .aplay { position: absolute; inset: 0; display: grid; place-items: center; font-size: 30px; color: #fff; text-shadow: 0 2px 12px #000; }
  .acard h3 { font-size: 13px; line-height: 1.35; margin: 10px 0 3px; font-weight: 600; color: var(--a-ink); display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .acard p { font-size: 11px; color: var(--a-faint); margin: 0; line-height: 1.45; }

  /* Explorar (temas + época) */
  .abrowse { display: grid; grid-template-columns: 1.2fr .8fr; gap: 26px; border-top: 1px solid var(--a-line); padding-top: 22px; }
  .abrowse h3 { font-family: var(--a-serif); font-size: 18px; font-weight: 400; margin: 0 0 13px; }
  .atags { display: flex; flex-wrap: wrap; gap: 9px; }
  .atag { border: 1px solid var(--a-line); background: var(--a-panel); color: var(--a-dim); border-radius: 99px; padding: 9px 13px; cursor: pointer; font-size: 13px; transition: border-color .12s, color .12s, background .12s; }
  .atag:hover { border-color: var(--a-accent); color: var(--a-ink); background: #211d19; }
  .atag small { color: var(--a-faint); margin-left: 6px; }
  .aeras { display: grid; grid-template-columns: repeat(var(--cols),1fr); gap: 7px; align-items: end; height: 106px; border-bottom: 1px solid var(--a-line); padding: 0 5px; }
  .abar { border: 0; background: linear-gradient(180deg,#a47b4f,#4b3928); min-height: 22px; color: var(--a-ink); cursor: pointer; position: relative; border-radius: 2px 2px 0 0; transition: filter .12s; }
  .abar:hover { filter: brightness(1.18); }
  .abar span { position: absolute; top: calc(100% + 8px); left: 50%; transform: translateX(-50%); font-size: 10px; color: var(--a-faint); }
  .logic { margin-top: 26px; color: var(--a-faint); font-size: 12px; display: flex; align-items: center; gap: 9px; }
  .logic:before { content: "AUTO"; font-size: 9px; letter-spacing: .12em; color: var(--a-accent); border: 1px solid #72583b; border-radius: 4px; padding: 3px 5px; }

  /* Pie */
  .afoot { margin-top: 44px; padding-top: 18px; border-top: 1px solid var(--a-line); display: flex; color: var(--a-faint); font-size: 11px; }

  /* ── Catálogo facetado (vista de lista) ── */
  .cataloghead { display: flex; align-items: flex-end; gap: 16px; margin: 8px 0 24px; }
  .aback { border: 1px solid var(--a-line); background: var(--a-panel); color: var(--a-dim); border-radius: 8px; padding: 9px 12px; cursor: pointer; }
  .aback:hover { border-color: #484b55; color: var(--a-ink); }
  .cataloghead h1 { font-family: var(--a-serif); font-weight: 400; font-size: 34px; margin: 0; }
  .cataloghead p { color: var(--a-faint); margin: 0 0 5px; font-size: 13px; }

  .cataloggrid { display: grid; grid-template-columns: 250px minmax(0,1fr); gap: 28px; align-items: start; }
  .filterside { position: sticky; top: 12px; border-top: 1px solid var(--a-line); }
  .facet { padding: 18px 0; border-bottom: 1px solid var(--a-line); }
  .facet h3 { font-size: 11px; text-transform: uppercase; letter-spacing: .13em; color: var(--a-faint); margin: 0 0 12px; }
  .facet label { display: flex; align-items: center; gap: 9px; color: var(--a-dim); padding: 6px 0; cursor: pointer; font-size: 13px; }
  .facet label span { margin-left: auto; color: var(--a-faint); font-size: 11px; }
  .facet input[type="checkbox"] { accent-color: var(--a-accent); flex: none; }
  .range { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .range input, .resulttools select { background: var(--a-panel); border: 1px solid var(--a-line); border-radius: 7px; color: var(--a-ink); padding: 9px; font: inherit; width: 100%; }
  .range input { min-width: 0; }

  .resulttools { display: flex; align-items: center; gap: 12px; padding-bottom: 13px; border-bottom: 1px solid var(--a-line); color: var(--a-faint); font-size: 12px; }
  .resulttools b { color: var(--a-ink); font-weight: 600; }
  .resulttools select { width: auto; margin-left: auto; cursor: pointer; }
  .clearf { background: none; border: 1px solid var(--a-line); color: var(--a-dim); border-radius: 7px; padding: 6px 10px; font-size: 12px; cursor: pointer; }
  .clearf:hover { border-color: var(--a-accent); color: var(--a-ink); }

  .resultlist { display: flex; flex-direction: column; }
  .result { display: grid; grid-template-columns: 76px minmax(0,1fr) auto; gap: 18px; padding: 17px 0; border-bottom: 1px solid var(--a-line); align-items: center; cursor: pointer; text-align: left; background: none; border-left: 0; border-right: 0; border-top: 0; color: inherit; width: 100%; }
  .result:hover { background: linear-gradient(90deg,transparent,var(--a-panel),transparent); }
  .thumb { width: 76px; aspect-ratio: 3/4; border: 1px solid var(--a-line); box-shadow: 0 7px 16px rgba(0,0,0,.25); overflow: hidden; position: relative; flex: none; border-radius: 2px; }
  .thumb.book { background: linear-gradient(150deg,#6d5334,#241a12); }
  .thumb.wide { background: linear-gradient(150deg,#3a5236,#141d16); }
  .thumb.sq   { background: linear-gradient(150deg,#4a4636,#16150f); }
  .thumb.img  { background: #0b0c0e; }
  .thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .tplay { position: absolute; inset: 0; display: grid; place-items: center; font-size: 18px; color: #fff; text-shadow: 0 2px 8px #000; }
  .rinfo { min-width: 0; }
  .rinfo h2 { font-family: var(--a-serif); font-size: 20px; font-weight: 400; margin: 0 0 5px; line-height: 1.2; }
  .rby { color: var(--a-dim); font-size: 12px; }
  .rdesc { color: var(--a-faint); font-size: 12px; line-height: 1.5; margin: 8px 0; max-width: 760px; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .rdata { display: flex; gap: 13px; flex-wrap: wrap; font-size: 11px; color: var(--a-faint); }
  .rdata b { font-weight: 400; color: var(--a-dim); }
  .status { align-self: start; color: var(--a-accent2); font-size: 11px; padding: 5px 7px; border: 1px solid #35493f; border-radius: 5px; white-space: nowrap; }

  @media (max-width: 980px) {
    .ahero { grid-template-columns: 1fr; } .aheroart { min-height: 260px; } .aherocopy { padding: 32px; }
    .abrowse { grid-template-columns: 1fr; }
    .aformats { grid-template-columns: repeat(2,1fr); }
    .aformat:nth-child(2) { border-right: 0; }
    .aformat:nth-child(-n+2) { border-bottom: 1px solid var(--a-line); }
  }
  @media (max-width: 760px) {
    .cataloggrid { grid-template-columns: 1fr; }
    .filterside { position: static; display: grid; grid-template-columns: repeat(2,1fr); gap: 0 18px; }
    .result { grid-template-columns: 58px minmax(0,1fr); }
    .thumb { width: 58px; } .status { display: none; }
    .cataloghead { align-items: flex-start; flex-wrap: wrap; }
  }
  @media (max-width: 620px) {
    .wrap { padding: 14px 16px 60px; } .aheroart { min-height: 220px; }
    .abook { width: 150px; }
    .asearch { margin: 16px 0 0; } .aformats { margin-top: 22px; }
    .aformat { grid-template-columns: 34px 1fr; padding: 13px 10px; } .aformat em { display: none; }
    .fic { width: 32px; height: 32px; font-size: 16px; }
    .crumb { display: none; }
  }
</style>
