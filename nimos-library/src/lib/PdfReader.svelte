<script>
  // Lector de PDF propio (PDF.js) con barra editorial — offline, mismo look en todo
  // navegador. Reemplaza el visor nativo de Chrome (feo) en las fichas de Archives.
  import * as pdfjsLib from 'pdfjs-dist';
  import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
  import { onDestroy } from 'svelte';
  import { t } from './i18n.svelte.js';

  pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;

  let { url, title = '' } = $props();

  let doc = $state(null);
  let numPages = $state(0);
  let page = $state(1);
  let scale = $state(1);
  let fitMode = $state('page'); // 'page' (entra entera) | 'width' (ajusta al ancho) | 'custom' (zoom)
  let spread = $state(true);    // dos páginas lado a lado (libro abierto) por defecto
  // Página derecha del pliego: la portada (pág 1) va SOLA como en un libro real; a
  // partir de ahí pliegos [2,3],[4,5]… `page` es la página IZQUIERDA (ancla).
  const spreadRight = $derived((spread && page >= 2 && page + 1 <= numPages) ? page + 1 : null);
  let loading = $state(true);
  let errMsg = $state('');
  let canvasEl = $state(null);
  let stageEl = $state(null);
  let rootEl = $state(null);
  let pageInput = $state('1');
  let renderScale = $state(1); // escala efectiva mostrada (para el %)

  let loadingTask = null, lastUrl = '';
  let rendering = false, pending = false, lastW = 0, resizeTimer = null;

  async function loadDoc(u) {
    loading = true; errMsg = ''; doc = null; numPages = 0;
    try {
      // disableStream/disableAutoFetch: descarga el PDF completo de una (sin range
      // requests). El render necesita el stream de contenido de la página; si el
      // servidor de media responde raro a los rangos, el render se cuelga esperando.
      // Para ficheros locales offline bajar el fichero entero es instantáneo.
      // wasmUrl/cMapUrl/standardFontDataUrl: auxiliares que pdf.js baja en runtime
      // (los copia el plugin pdfjs-assets de vite.config a /pdfjs/). Sin wasmUrl los
      // escaneos JPEG2000/JBIG2 antiguos NO decodifican → páginas en blanco.
      loadingTask = pdfjsLib.getDocument({
        url: u, disableStream: true, disableAutoFetch: true,
        // pdf.js hace su propio fetch: sin withCredentials NO manda la cookie de
        // sesión cross-origin (cliente en otro puerto que el server) → 403. El
        // <video>/<img> nativos sí la mandan solos; pdf.js hay que decírselo.
        withCredentials: true,
        wasmUrl: '/pdfjs/wasm/', cMapUrl: '/pdfjs/cmaps/', cMapPacked: true,
        standardFontDataUrl: '/pdfjs/standard_fonts/',
      });
      const d = await loadingTask.promise;
      doc = d; numPages = d.numPages; page = 1; pageInput = '1';
    } catch (e) {
      errMsg = e?.message || 'No se pudo abrir el PDF';
    }
    loading = false;
  }

  // Cerrojo: NUNCA dos render() a la vez sobre el mismo canvas (pdfjs se cuelga si se
  // solapan). Si llega una petición mientras se pinta, se marca `pending` y al acabar
  // se re-renderiza con el estado más nuevo. Sin cancelaciones (que dejaban cuelgues).
  function renderPage() {
    if (rendering) { pending = true; return; }
    rendering = true;
    (async () => {
      do { pending = false; try { await doRender(); } catch (e) {} } while (pending);
      rendering = false;
    })();
  }
  async function doRender() {
    if (!doc || !canvasEl || !stageEl) return;
    const R = spreadRight;                       // null si portada sola o modo 1 página
    const pgL = await doc.getPage(page);
    const pgR = R ? await doc.getPage(R) : null;
    const vpL1 = pgL.getViewport({ scale: 1 });
    const vpR1 = pgR ? pgR.getViewport({ scale: 1 }) : null;
    const totalW = vpL1.width + (vpR1 ? vpR1.width : 0);
    const maxH = Math.max(vpL1.height, vpR1 ? vpR1.height : 0);
    let s = scale;
    if (fitMode !== 'custom') {
      const availW = Math.max(200, stageEl.clientWidth - 48);
      const availH = Math.max(200, stageEl.clientHeight - 48);
      // 'page' = el pliego ENTERO entra en la ventana (min ancho/alto); 'width' = llena
      // el ancho. Con dos páginas el ancho total es la suma → aprovecha toda la ventana.
      s = Math.max(0.12, fitMode === 'page' ? Math.min(availW / totalW, availH / maxH) : availW / totalW);
    }
    renderScale = s;
    // pdfjs v6: `canvas` en params, DPR en la escala del viewport. Cada página se pinta
    // en un canvas propio y se compone en el visible → sin líos de clear entre renders.
    const dpr = window.devicePixelRatio || 1;
    const cw = Math.floor(totalW * s * dpr), ch = Math.floor(maxH * s * dpr);
    canvasEl.width = cw; canvasEl.height = ch;
    canvasEl.style.width = Math.floor(totalW * s) + 'px';
    canvasEl.style.height = Math.floor(maxH * s) + 'px';
    const ctx = canvasEl.getContext('2d');
    ctx.clearRect(0, 0, cw, ch);
    let x = 0;
    for (const pg of [pgL, pgR]) {
      if (!pg) continue;
      const vp = pg.getViewport({ scale: s * dpr });
      const tmp = document.createElement('canvas');
      tmp.width = Math.max(1, Math.floor(vp.width)); tmp.height = Math.max(1, Math.floor(vp.height));
      await pg.render({ canvas: tmp, viewport: vp }).promise;
      ctx.drawImage(tmp, x, 0);
      x += Math.floor(vp.width);
    }
  }

  // Cargar al cambiar la URL.
  $effect(() => { const u = url; if (u && u !== lastUrl) { lastUrl = u; loadDoc(u); } });
  // Re-render al cambiar página / zoom / ajuste (o al cargar el doc).
  $effect(() => { page; scale; fitMode; spread; doc; canvasEl; if (doc && canvasEl) renderPage(); });
  // Re-render (debounced) si cambia el TAMAÑO del escenario y estamos en un modo de
  // ajuste (page/width dependen del ancho y del alto → cualquier resize recalcula).
  $effect(() => {
    if (!stageEl) return;
    let ro;
    try {
      ro = new ResizeObserver(() => {
        if (fitMode === 'custom') return;
        clearTimeout(resizeTimer); resizeTimer = setTimeout(renderPage, 120);
      });
      ro.observe(stageEl);
    } catch (e) {}
    return () => ro?.disconnect();
  });
  $effect(() => { pageInput = String(page); });

  // Ancla de pliego: portada (1) sola; luego pares con IZQUIERDA par (2,4,6…).
  const anchorOf = (n) => (n <= 1 ? 1 : (n % 2 === 0 ? n : n - 1));
  function prev() {
    if (spread) page = page === 2 ? 1 : Math.max(1, page - 2);
    else if (page > 1) page--;
    stageEl?.scrollTo({ top: 0 });
  }
  function next() {
    if (spread) { const na = page === 1 ? 2 : page + 2; if (na <= numPages) page = na; }
    else if (page < numPages) page++;
    stageEl?.scrollTo({ top: 0 });
  }
  function commitPageInput() {
    const n = parseInt(pageInput, 10);
    if (n >= 1 && n <= numPages) page = spread ? anchorOf(n) : n;
    else pageInput = String(page);
  }
  function toggleSpread() { spread = !spread; if (spread) page = anchorOf(page); }
  // Zoom manual: parte de la escala mostrada (venga de fit o no) y sale del modo fit.
  function zoomIn() { const base = renderScale; fitMode = 'custom'; scale = Math.min(6, +(base + 0.2).toFixed(2)); }
  function zoomOut() { const base = renderScale; fitMode = 'custom'; scale = Math.max(0.2, +(base - 0.2).toFixed(2)); }
  // El botón central alterna "entra entera" ↔ "ajusta al ancho".
  function toggleFit() { fitMode = fitMode === 'page' ? 'width' : 'page'; }
  function fs() { const el = rootEl; if (!el) return; if (!document.fullscreenElement) el.requestFullscreen?.(); else document.exitFullscreen?.(); }
  function onKey(e) {
    if (e.target?.tagName === 'INPUT') return;
    if (e.key === 'ArrowRight' || e.key === 'PageDown') { e.preventDefault(); next(); }
    else if (e.key === 'ArrowLeft' || e.key === 'PageUp') { e.preventDefault(); prev(); }
  }

  onDestroy(() => { try { loadingTask?.destroy?.(); } catch (e) {} });
</script>

<div class="pdf" bind:this={rootEl}>
  <div class="pbar">
    <div class="pnav">
      <button onclick={prev} disabled={page <= 1} aria-label={t('cabinet.prev')} title={t('cabinet.prev')}>‹</button>
      <span class="pnum">
        <input value={pageInput} oninput={(e) => (pageInput = e.currentTarget.value)} onchange={commitPageInput}
               onkeydown={(e) => { if (e.key === 'Enter') e.currentTarget.blur(); }} aria-label={t('item.pdfPage')} />
        <em>/ {numPages || '—'}</em>
      </span>
      <button onclick={next} disabled={spread ? ((page === 1 ? 2 : page + 2) > numPages) : (page >= numPages)} aria-label={t('cabinet.next')} title={t('cabinet.next')}>›</button>
    </div>
    <div class="pfile">{title}</div>
    <div class="pzoom">
      <button class="pspread" class:on={spread} onclick={toggleSpread}
              aria-label={spread ? t('item.singlePage') : t('item.twoPage')} title={spread ? t('item.singlePage') : t('item.twoPage')}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"><rect x="3" y="5" width="8" height="14" rx="1"/><rect x="13" y="5" width="8" height="14" rx="1"/></svg>
      </button>
      <button onclick={zoomOut} aria-label={t('item.zoomOut')} title={t('item.zoomOut')}>−</button>
      <button class="pct" onclick={toggleFit} class:on={fitMode !== 'custom'}
              title={fitMode === 'page' ? t('item.pdfFitPage') : fitMode === 'width' ? t('item.pdfFit') : t('item.zoom')}>{Math.round(renderScale * 100)}%</button>
      <button onclick={zoomIn} aria-label={t('item.zoomIn')} title={t('item.zoomIn')}>+</button>
      <button onclick={fs} aria-label={t('item.fullscreen')} title={t('item.fullscreen')}>⛶</button>
    </div>
  </div>
  <div class="pstage" bind:this={stageEl} tabindex="0" onkeydown={onKey}>
    {#if loading}
      <div class="pstate"><span class="pspin"></span> {t('common.loading')}</div>
    {:else if errMsg}
      <div class="pstate err">{errMsg}</div>
    {/if}
    <canvas bind:this={canvasEl} style:visibility={loading || errMsg ? 'hidden' : 'visible'}></canvas>
  </div>
</div>

<style>
  .pdf { display: flex; flex-direction: column; height: 78vh; min-height: 460px; background: var(--a-panel, #17181d); border: 1px solid var(--a-line, #2a2d35); border-radius: 4px; overflow: hidden; }
  :global(.pdf:fullscreen) { height: 100vh; border-radius: 0; }

  /* Barra editorial */
  .pbar { height: 50px; flex: none; display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 12px; padding: 0 12px;
    background: var(--a-panel, #17181d); border-bottom: 1px solid var(--a-line, #2a2d35); color: var(--a-dim, #aaa69e); font-size: 12.5px; }
  .pnav, .pzoom { display: flex; align-items: center; gap: 6px; }
  .pzoom { justify-content: flex-end; }
  .pbar button { min-width: 32px; height: 32px; padding: 0 8px; display: grid; place-items: center; border: 1px solid var(--a-line, #2a2d35);
    background: transparent; color: var(--a-ink, #f2efe8); border-radius: 7px; cursor: pointer; font-size: 16px; line-height: 1; transition: border-color .12s, color .12s, background .12s; }
  .pbar button:hover:not(:disabled) { border-color: var(--a-accent, #e0a867); color: var(--a-accent, #e0a867); }
  .pbar button:disabled { opacity: .38; cursor: default; }
  .pbar button.pct { font-size: 12px; min-width: 52px; color: var(--a-dim, #aaa69e); }
  .pbar button.pct.on, .pbar button.pspread.on { border-color: var(--a-accent, #e0a867); color: var(--a-accent, #e0a867); }
  .pnum { display: flex; align-items: center; gap: 7px; color: var(--a-faint, #77746f); }
  .pnum input { width: 42px; height: 32px; text-align: center; background: var(--a-bg, #101114); border: 1px solid var(--a-line, #2a2d35);
    border-radius: 7px; color: var(--a-ink, #f2efe8); font: inherit; font-size: 12.5px; }
  .pnum input:focus { outline: 0; border-color: var(--a-accent, #e0a867); }
  .pnum em { font-style: normal; }
  .pfile { text-align: center; color: var(--a-faint, #77746f); font-family: var(--a-serif, Georgia), serif; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  /* Escenario que FUNDE con el color de la app (charcoal editorial) + un susurro
     dorado detrás de la página → armoniza con la identidad, no una caja gris pegada.
     `safe` evita que al hacer zoom (página > ventana) se recorte el borde superior. */
  .pstage { flex: 1; min-height: 0; overflow: auto; display: flex; align-items: safe center; justify-content: safe center; padding: 28px;
    background:
      radial-gradient(90% 78% at 50% 30%, color-mix(in srgb, var(--a-accent, #e0a867) 7%, transparent), transparent 55%),
      radial-gradient(130% 120% at 50% 32%, var(--a-panel, #17181d), var(--a-bg, #101114) 72%);
    scrollbar-width: thin; scrollbar-color: color-mix(in srgb, var(--a-accent, #e0a867) 42%, transparent) transparent; }
  .pstage::-webkit-scrollbar { width: 11px; height: 11px; }
  .pstage::-webkit-scrollbar-track { background: transparent; }
  .pstage::-webkit-scrollbar-thumb { background: color-mix(in srgb, var(--a-accent, #e0a867) 38%, transparent); border-radius: 10px; border: 3px solid transparent; background-clip: padding-box; }
  .pstage::-webkit-scrollbar-thumb:hover { background: color-mix(in srgb, var(--a-accent, #e0a867) 62%, transparent); background-clip: padding-box; }
  .pstage::-webkit-scrollbar-corner { background: transparent; }
  .pstage canvas { display: block; box-shadow: 0 18px 46px rgba(0,0,0,.5); background: #fff; border-radius: 2px; }
  .pstage canvas.hidden { display: none; }
  .pstate { position: absolute; display: flex; align-items: center; gap: 10px; color: #d9d5cc; font-size: 13.5px; margin-top: 60px; }
  .pstate.err { color: #e89; }
  .pspin { width: 15px; height: 15px; border-radius: 50%; border: 2px solid rgba(255,255,255,.25); border-top-color: var(--a-accent, #e0a867); animation: pspin .7s linear infinite; }
  @keyframes pspin { to { transform: rotate(360deg); } }

  @media (max-width: 620px) {
    .pbar { grid-template-columns: auto 1fr; } .pfile { display: none; }
  }
</style>
