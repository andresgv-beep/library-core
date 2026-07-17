<script>
  import Home from './Home.svelte';
  import LibraryView from './LibraryView.svelte';
  import Settings from './Settings.svelte';
  import ThirdParty from './ThirdParty.svelte';
  import TagsView from './TagsView.svelte';
  import Cabinet from './Cabinet.svelte';
  import Moments from './Moments.svelte';
  import ItemPage from './ItemPage.svelte';
  import MomentsWatch from './MomentsWatch.svelte';
  import Icon from './Icon.svelte';
  import { untrack } from 'svelte';
  import { fade } from 'svelte/transition';
  import { t, i18n } from './i18n.svelte.js';
  import { translateSegments } from './libraryApi.js';
  import { tstate, targetLang, nativeLang, norm2 } from './translate.svelte.js';
  import { serverUrl } from './connection.js';

  let { tab, libraries = [], favorites = [], indexOpen = false, notesVersion = 0, tagsVersion = 0,
        onNavigate, onOpenItem, onOpenView, onToggleHome, onFrameNav, onRemoveFav, onOpenNote, onDeleteNote } = $props();

  let frameEl = $state(null);
  let toc = $state([]);

  // ── Traducción in situ (TRANSLATE.md §6) ─────────────────────────────────────
  // El artículo se carga SIEMPRE original; la traducción cae encima cuando está
  // lista y se puede volver al original. El idioma origen sale del ZIM (catálogo).
  let translating = $state(false);
  let translated = $state(false);
  let badgeOn = $state(false); // "Traducido" visible; se desvanece a los 3s
  let badgeTimer = null;
  let translateCtrl = null;
  let lastRun = 0, lastOrig = 0;

  function flashBadge() {
    badgeOn = true;
    clearTimeout(badgeTimer);
    badgeTimer = setTimeout(() => { badgeOn = false; }, 3000);
  }

  const SEG_SEL = 'p, li, h1, h2, h3, h4, blockquote, figcaption, caption, dd, dt';
  let srcLang = $derived(norm2(libraries.find((l) => l.id === tab.lib)?.lang || ''));

  // Limpia el ruido antes de traducir (TRANSLATE.md §8): fuera marcadores de
  // referencia [1], enlaces [editar] y nodos no-prosa. Devuelve HTML (no texto
  // pelado) para conservar los enlaces `<a>` al traducir con --html; si no, el
  // reemplazo destruiría la navegación interna del artículo.
  function cleanSource(el) {
    const clone = el.cloneNode(true);
    clone.querySelectorAll('sup, .reference, .mw-editsection, .noprint, style, script').forEach((n) => n.remove());
    return (clone.innerHTML || '')
      .replace(/\[\s*\d+\s*\]/g, '')
      .replace(/\[\s*(edit|editar|cita requerida|citation needed)\s*\]/gi, '')
      .replace(/\s+/g, ' ')
      .trim();
  }

  // Recoge bloques de prosa con un id estable (índice de documento) para cachear.
  function collectSegments(doc) {
    const segs = [], map = [];
    doc.querySelectorAll(SEG_SEL).forEach((el, i) => {
      if (el.closest('table, style, script')) return; // fuera infoboxes/navboxes
      const text = cleanSource(el);
      if (text.length < 2) return;
      const id = 's' + i;
      segs.push({ id, text });
      map.push({ id, el });
    });
    return { segs, map };
  }

  async function translateArticle(to) {
    const doc = frameEl?.contentDocument;
    if (!doc || !tab.lib || !to || to === srcLang) return;
    const { segs, map } = collectSegments(doc);
    if (!segs.length) return;
    translateCtrl?.abort();
    translateCtrl = new AbortController();
    translating = true;
    try {
      const out = await translateSegments(
        { lib: tab.lib, path: tab.path, to, sourceHint: srcLang, html: true, segments: segs },
        { signal: translateCtrl.signal });
      const byId = new Map(out.map((s) => [s.id, s.text]));
      for (const m of map) {
        const tx = byId.get(m.id);
        if (tx == null) continue;
        // Guarda el HTML original (con enlaces) y pon el traducido conservándolos.
        if (m.el.dataset.torig == null) m.el.dataset.torig = m.el.innerHTML;
        m.el.innerHTML = tx;
      }
      translated = true;
      flashBadge();
    } catch (e) { /* abortada o error → se queda el original */ }
    translating = false;
  }

  function showOriginal() {
    const doc = frameEl?.contentDocument;
    if (!doc) return;
    translateCtrl?.abort();
    doc.querySelectorAll('[data-torig]').forEach((el) => {
      el.innerHTML = el.dataset.torig;
      delete el.dataset.torig;
    });
    translated = false;
  }

  // Comandos del dropdown (nonces): traducir al destino elegido / ver original.
  $effect(() => {
    const r = tstate.run;
    untrack(() => {
      if (r === lastRun) return;
      lastRun = r;
      translateArticle(targetLang());
    });
  });
  $effect(() => {
    const o = tstate.original;
    untrack(() => {
      if (o === lastOrig) return;
      lastOrig = o;
      showOriginal();
    });
  });

  // Índice: extrae los encabezados de la página original (mismo origen).
  function extractToc(doc) {
    const items = [];
    try {
      doc.querySelectorAll('h2, h3').forEach((h) => {
        const text = (h.textContent || '').replace(/\s*\[\s*editar\s*\]\s*/gi, '').replace(/\s+/g, ' ').trim();
        if (!text || text.length > 90) return;
        let id = h.id || h.querySelector('[id]')?.id;
        if (!id) { id = 'nimostoc-' + items.length; h.id = id; }
        items.push({ level: h.tagName === 'H3' ? 3 : 2, id, text });
      });
    } catch (e) {}
    return items;
  }
  function scrollToHeading(id) {
    // Nota: 'smooth' no funciona en iframe cross-frame → scroll instantáneo.
    try { frameEl?.contentDocument?.getElementById(id)?.scrollIntoView({ block: 'start' }); } catch (e) {}
  }

  // El contenido se muestra SIEMPRE original (el ZIM ya trae su diseño). Nosotros
  // somos el navegador/gestor por encima. El iframe es del mismo origen.

  // src del iframe: se re-fija solo en navegación intencionada (cambia pestaña o
  // sube el contador tab.nav). La ruta se lee con untrack para que onFrameNav
  // (sync desde dentro del iframe) NO re-fije el src → sin recargas en bucle.
  let frameSrc = $state('');
  $effect(() => {
    tab.id; tab.nav; // deps de navegación intencionada
    frameSrc = untrack(() => serverUrl('/content/' + tab.lib + '/' + encodeURI(tab.path || '')));
  });

  // Enlaces EXTERNOS (a la web viva): no deben secuestrar el lector offline
  // (algunos sitios devuelven un challenge/WAF dentro del iframe). Se abren en una
  // pestaña real del navegador; los internos del ZIM navegan normal.
  function onFrameClick(e) {
    const a = e.target?.closest?.('a[href]');
    if (!a) return;
    let url;
    try { url = new URL(a.href); } catch (err) { return; }
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return; // mailto/tel/#…
    if (url.origin === location.origin) return; // enlace interno del ZIM
    e.preventDefault();
    window.open(url.href, '_blank', 'noopener');
  }

  // El iframe navegó (carga inicial o link interno): sincroniza ruta/título/historial.
  function onFrameLoad(e) {
    const frame = e.target;
    // Documento nuevo → el original ya está en pantalla; se reinicia el estado.
    translated = false;
    try { frame.contentDocument.addEventListener('click', onFrameClick); } catch (err) {}
    try { toc = extractToc(frame.contentDocument); } catch (err) {}
    try {
      const p = decodeURIComponent(frame.contentWindow.location.pathname);
      if (!p.startsWith('/content/')) return;
      const rest = p.slice('/content/'.length);
      const i = rest.indexOf('/');
      if (i < 0) return;
      onFrameNav?.(rest.slice(0, i), rest.slice(i + 1), frame.contentDocument?.title || rest);
    } catch (err) { /* mismo origen: no debería fallar */ }
    // Modo auto: si el artículo está en otro idioma, se traduce al nativo. Si el
    // catálogo aún no cargó (srcLang vacío), se intenta igual y el shim deduce el
    // idioma origen; si coincide con el destino, translateArticle no hace nada.
    if (tstate.auto) {
      const to = nativeLang();
      if (srcLang !== to) translateArticle(to);
    }
  }
</script>

<div class="main">
  <!-- tab.rev sube al pulsar Recargar en una superficie Svelte (no-artículo) →
       {#key} la remonta y re-dispara su fetch. El artículo (iframe, más abajo) no
       usa esta llave: recarga por su cuenta vía tab.nav. -->
  {#key (tab.rev || 0)}
  {#if tab.kind === 'home'}
    <div class="reader scroll"><Home {tab} {libraries} {favorites} {onNavigate} {onOpenItem} {onOpenView} {onToggleHome} /></div>
  {:else if tab.kind === 'view' && tab.view === 'settings'}
    <div class="reader"><Settings /></div>
  {:else if tab.kind === 'view' && tab.view === 'information'}
    <div class="reader"><ThirdParty /></div>
  {:else if tab.kind === 'view' && tab.view === 'tags'}
    <div class="reader"><TagsView {libraries} {tagsVersion} {onNavigate} {onOpenItem} /></div>
  {:else if tab.kind === 'view' && tab.view === 'maps'}
    <iframe class="pluginframe" title={t('menu.maps')} src={serverUrl(`/maps/?v=compact-panel-3&lang=${encodeURIComponent(i18n.locale)}`)}></iframe>
  {:else if tab.kind === 'view' && tab.view === 'cabinet'}
    <Cabinet {onOpenItem} />
  {:else if tab.kind === 'view' && tab.view === 'moments'}
    <Moments {onOpenItem} />
  {:else if tab.kind === 'item'}
    {#if tab.open?.provider === 'moments'}
      <MomentsWatch {tab} {onOpenItem} {onOpenView} />
    {:else}
      <ItemPage {tab} {onOpenItem} />
    {/if}
  {:else if tab.kind === 'view'}
    <div class="reader"><LibraryView view={tab.view} {libraries} {favorites} {notesVersion}
      {onNavigate} {onOpenItem} {onToggleHome} {onRemoveFav} {onOpenNote} {onDeleteNote} /></div>
  {:else if tab.error}
    <div class="reader"><div class="state err"><Icon name="close" /> {tab.error}</div></div>
  {:else}
    <div class="framebox">
      <iframe class="orig" title={t('reader.content')} bind:this={frameEl} src={frameSrc} onload={onFrameLoad}></iframe>
      {#if translating}
        <div class="tbadge"><span class="spin"></span> {t('translate.working')}</div>
      {:else if translated && badgeOn}
        <div class="tbadge done" out:fade={{ duration: 600 }}><Icon name="translate" size={13} /> {t('translate.done')}</div>
      {/if}
    </div>
    {#if indexOpen && toc.length}
      <aside class="toc-col scroll thin">
        <div class="toc-h">{t('reader.index')}</div>
        <nav class="toc">
          {#each toc as t}
            <button class="tocitem" class:lvl3={t.level === 3} onclick={() => scrollToHeading(t.id)}>{t.text}</button>
          {/each}
        </nav>
      </aside>
    {/if}
  {/if}
  {/key}
</div>

<style>
  .main{min-height:0;overflow:hidden;background:var(--ground);height:100%;display:flex}
  .reader{flex:1;overflow-y:auto;min-width:0}
  .pluginframe{flex:1;width:100%;height:100%;border:0;display:block;background:var(--ground)}
  .framebox{--content-zoom:1.08;position:relative;flex:1;min-width:0;height:100%;overflow:hidden;background:#fff}
  .tbadge{position:absolute;top:12px;right:16px;display:flex;align-items:center;gap:7px;padding:7px 12px;border-radius:20px;background:var(--card);border:1px solid var(--border);box-shadow:var(--shadow);color:var(--ink-dim);font-size:12.5px;font-weight:520;z-index:5}
  .tbadge.done{color:var(--accent-2)}
  .tbadge.done :global(.ic){color:var(--accent-2)}
  .spin{width:13px;height:13px;border-radius:50%;border:2px solid var(--border);border-top-color:var(--accent);animation:tspin .7s linear infinite}
  @keyframes tspin{to{transform:rotate(360deg)}}
  .orig{width:calc(100% / var(--content-zoom));height:calc(100% / var(--content-zoom));border:0;display:block;background:#fff;transform:scale(var(--content-zoom));transform-origin:top left}
  .state{padding:48px;color:var(--muted)}
  .state.err{display:flex;align-items:center;gap:8px;color:#e88}
  .state.err :global(.ic){color:#e88}
  .toc-col{width:300px;flex:none;border-left:1px solid var(--border);background:var(--panel);overflow-y:auto;padding:18px 14px}
  .toc-h{font-size:12px;font-weight:650;letter-spacing:.6px;text-transform:uppercase;color:var(--faint);padding:0 10px 10px}
  .toc{display:flex;flex-direction:column;gap:1px}
  .tocitem{display:block;padding:7px 11px;border-radius:8px;color:var(--muted);font-size:13.5px;text-align:left;border-left:2px solid transparent;line-height:1.35;width:100%;transition:background .12s,color .12s}
  .tocitem:hover{background:var(--raise);color:var(--ink)}
  .tocitem.lvl3{padding-left:22px;font-size:12.5px}
</style>
