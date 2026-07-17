<script>
  import { onMount } from 'svelte';
  import { getLibraries, getItem, resolveProviderItem, itemSearch, mapSearch } from './lib/libraryApi.js';
  import { parseLibraryAddress } from './lib/libraryAddress.js';
  import * as readerState from './lib/readerStateApi.js';
  import Tabs from './lib/Tabs.svelte';
  import NavBar from './lib/NavBar.svelte';
  import Sidebar from './lib/Sidebar.svelte';
  import Reader from './lib/Reader.svelte';
  import BookmarksBar from './lib/BookmarksBar.svelte';
  import NoteEditor from './lib/NoteEditor.svelte';
  import TagEditor from './lib/TagEditor.svelte';
  import AccountModal from './lib/AccountModal.svelte';
  import { auth, refreshAuth, loginPrompt } from './lib/auth.svelte.js';
  import { t } from './lib/i18n.svelte.js';
  import { serverPath } from './lib/connection.js';
  import { initShell } from './lib/shell.svelte.js';
  import './lib/theme.svelte.js'; // inicializa el tema (data-theme) + listener del sistema

  const loadBool = (k, def) => { try { const v = localStorage.getItem(k); return v === null ? def : v === '1'; } catch (e) { return def; } };
  const saveBool = (k, v) => { try { localStorage.setItem(k, v ? '1' : '0'); } catch (e) {} };

  let libraries = $state([]);
  let accountOpen = $state(false);
  let sidebarOpen = $state(loadBool('nimos-sidebar', true));

  // Al iniciar/cerrar sesión cambia TODO lo dependiente del usuario: el catálogo
  // (visibilidad por nivel/edad) y el estado personal (favoritos, marcas de nota/etiqueta).
  async function reloadLibraries() { try { libraries = await getLibraries(); } catch (e) {} }
  async function reloadPersonal() {
    try { favorites = await readerState.getFavorites(); } catch (e) {}
    try { const ns = await readerState.listNotes(); notedPaths = new Set(ns.map((n) => noteKey(n.lib, n.path, n.itemId))); } catch (e) {}
    try { taggedPaths = new Set(await readerState.getTaggedKeys()); } catch (e) {}
  }
  async function onAuthChanged() { await reloadLibraries(); await reloadPersonal(); }
  let indexOpen = $state(loadBool('nimos-index', true));
  let tabs = $state([]);
  let activeId = $state(null);
  let uid = 1;

  let active = $derived(tabs.find((tb) => tb.id === activeId) || null);
  // La barra de marcadores (páginas guardadas) vive a nivel de app: visible en
  // TODAS las pestañas mientras haya algún favorito. Al final de la barra vive el
  // menú Favoritos, así que si no hay favoritos, barra y menú desaparecen juntos.
  // Se funde con el navbar (le quitamos su borde inferior).
  let barOn = $derived(favorites.length > 0);
  let searchLib = $derived(
    (active && active.lib) || (libraries[0] && libraries[0].id) || null
  );

  // Favoritos: páginas concretas guardadas (persisten en el navegador; Fase 2 → SQLite del shim).
  let favorites = $state([]);
  const stateKey = (itemId, lib, path) => itemId || ((lib || '') + '\n' + (path || ''));
  let activeStateKey = $derived(active ? stateKey(active.itemId, active.lib, active.path) : '');
  let activeFav = $derived(
    !!(activeStateKey && (active?.kind === 'article' || active?.kind === 'item') && favorites.some((f) => stateKey(f.itemId, f.lib, f.path) === activeStateKey))
  );
  const HOME_MAX = 9; // tope 3×3 en el inicio
  const homeCount = () => favorites.filter((f) => f.onHome).length;
  const bookOf = (lib) => libraries.find((l) => l.id === lib)?.name || lib;
  const activeBook = () => active?.kind === 'article' ? bookOf(active.lib) : (active?.open?.title || active?.title || 'Library');
  const activeTarget = () => active ? {
    itemId: active.itemId || '',
    lib: active.lib || '',
    path: active.path || '',
    title: active.title || active.path || active.itemId,
    book: activeBook()
  } : null;
  function toggleFav() {
    if (!activeStateKey || (active.kind !== 'article' && active.kind !== 'item')) return;
    const i = favorites.findIndex((f) => stateKey(f.itemId, f.lib, f.path) === activeStateKey);
    if (i >= 0) {
      const f = favorites[i];
      favorites.splice(i, 1);
      readerState.deleteFavorite(f.lib, f.path, f.itemId);
    } else {
      const target = activeTarget();
      const f = { itemId: target.itemId, lib: target.lib, path: target.path, title: target.title, book: target.book, onHome: homeCount() < HOME_MAX };
      favorites.push(f);
      readerState.putFavorite(f);
    }
  }
  function toggleHome(fav) {
    const key = stateKey(fav.itemId, fav.lib, fav.path);
    const f = favorites.find((x) => stateKey(x.itemId, x.lib, x.path) === key);
    if (!f) return;
    if (!f.onHome && homeCount() >= HOME_MAX) return; // inicio lleno
    f.onHome = !f.onHome;
    readerState.putFavorite({ itemId: f.itemId, lib: f.lib, path: f.path, title: f.title, book: f.book, onHome: f.onHome });
  }
  function removeFav(fav) {
    const key = stateKey(fav.itemId, fav.lib, fav.path);
    const i = favorites.findIndex((f) => stateKey(f.itemId, f.lib, f.path) === key);
    if (i >= 0) { const f = favorites[i]; favorites.splice(i, 1); readerState.deleteFavorite(f.lib, f.path, f.itemId); }
  }
  function openFav(fav) { if (fav.itemId) openItemById(fav.itemId); else openArticle(fav.lib, fav.path); }
  function toggleSidebar() { sidebarOpen = !sidebarOpen; saveBool('nimos-sidebar', sidebarOpen); }
  function toggleIndex() { indexOpen = !indexOpen; saveBool('nimos-index', indexOpen); }

  // ── Vistas del sidebar (Favoritos/Reciente/Historial/Notas/…) ────────────────
  // Qué ítem del sidebar resaltar según la pestaña activa.
  let activeView = $derived(
    !active ? 'home' : active.kind === 'home' ? 'home' : active.kind === 'view' ? active.view : null
  );

  // ── Notas: una por artículo (lib+path). notedPaths marca cuáles tienen nota. ─
  let notedPaths = $state(new Set());
  let noteEditing = $state(null); // {lib, path, title, book, body, updated} mientras el modal está abierto
  let notesVersion = $state(0);   // sube al guardar/borrar → refresca la vista Notas
  const noteKey = (lib, path, itemId = '') => stateKey(itemId, lib, path);
  let activeNoted = $derived(
    !!(activeStateKey && (active?.kind === 'article' || active?.kind === 'item') && notedPaths.has(activeStateKey))
  );

  async function openNoteFor(target) {
    let existing = null;
    try { existing = await readerState.getNote(target.lib, target.path, target.itemId); } catch (e) {}
    noteEditing = existing || { itemId: target.itemId || '', lib: target.lib || '', path: target.path || '', title: target.title || target.path, book: target.book || bookOf(target.lib), body: '', updated: 0 };
  }
  function openActiveNote() {
    if (!active || (active.kind !== 'article' && active.kind !== 'item')) return;
    openNoteFor(activeTarget());
  }
  function openNoteItem(item) { openNoteFor({ itemId: item.itemId || '', lib: item.lib || '', path: item.path || '', title: item.title, book: item.book }); }
  async function saveNote(body) {
    const n = noteEditing; if (!n) return;
    await readerState.putNote({ itemId: n.itemId, lib: n.lib, path: n.path, title: n.title, book: n.book, body });
    const next = new Set(notedPaths);
    if (body.trim()) next.add(noteKey(n.lib, n.path, n.itemId)); else next.delete(noteKey(n.lib, n.path, n.itemId));
    notedPaths = next;
    notesVersion++;
    noteEditing = null;
  }
  function closeNote() { noteEditing = null; }
  async function deleteNoteItem(item) {
    await readerState.deleteNote(item.lib, item.path, item.itemId);
    const next = new Set(notedPaths); next.delete(noteKey(item.lib, item.path, item.itemId)); notedPaths = next;
    notesVersion++;
  }

  // ── Etiquetas: varias por página. taggedPaths marca cuáles tienen alguna. ─────
  let taggedPaths = $state(new Set());
  let tagEditing = $state(null); // {lib, path, title, book} mientras el modal está abierto
  let tagsVersion = $state(0);   // sube al cambiar etiquetas → refresca la vista Etiquetas
  let activeTagged = $derived(
    !!(activeStateKey && (active?.kind === 'article' || active?.kind === 'item') && taggedPaths.has(activeStateKey))
  );
  function openActiveTags() {
    if (!active || (active.kind !== 'article' && active.kind !== 'item')) return;
    tagEditing = activeTarget();
  }
  function closeTags() { tagEditing = null; }
  async function onTagsChanged() {
    tagsVersion++;
    try { taggedPaths = new Set(await readerState.getTaggedKeys()); } catch (e) {}
  }

  onMount(async () => {
    initShell(); // ¿corremos dentro de la app de escritorio? (window.runtime de Wails)
    newTab();
    refreshAuth(); // sesión actual (para la cuenta del sidebar)
    try { libraries = await getLibraries(); } catch (e) { /* motor caído: home vacío */ }
    // Favoritos desde el shim (SQLite); migra una vez los de localStorage si existían.
    try {
      favorites = await readerState.getFavorites();
      const localRaw = localStorage.getItem('nimos-favs');
      if (favorites.length === 0 && localRaw) {
        for (const f of JSON.parse(localRaw)) {
          await readerState.putFavorite({ lib: f.lib, path: f.path, title: f.title, book: f.book || bookOf(f.lib), onHome: f.onHome !== false });
        }
        favorites = await readerState.getFavorites();
      }
      if (localRaw) localStorage.removeItem('nimos-favs');
    } catch (e) {}
    // Qué artículos tienen nota (para marcar el botón Nota en la barra).
    try {
      const ns = await readerState.listNotes();
      notedPaths = new Set(ns.map((n) => noteKey(n.lib, n.path, n.itemId)));
    } catch (e) {}
    // Qué artículos tienen alguna etiqueta (para marcar el botón Etiquetas).
    try { taggedPaths = new Set(await readerState.getTaggedKeys()); } catch (e) {}
  });

  function emptySearch() {
    return {
      q: '', mode: 'all', results: [], groups: [], images: [], searched: false, loading: false,
      location: { status: 'idle', result: null, radius: 2500, selectedPoi: null },
    };
  }
  function makeHome() {
    return { id: uid++, kind: 'home', lib: null, path: null, titleKey: 'tab.home', title: 'Inicio', article: null, toc: [], loading: false, error: null, back: [], fwd: [], search: emptySearch() };
  }
  function newTab() {
    const tab = makeHome();
    tabs.push(tab);
    activeId = tab.id;
  }
  function activate(id) { activeId = id; }
  function closeTab(id) {
    const i = tabs.findIndex((tb) => tb.id === id);
    if (i < 0) return;
    tabs.splice(i, 1);
    if (activeId === id) activeId = tabs.length ? tabs[Math.max(0, i - 1)].id : null;
    if (!tabs.length) newTab();
  }

  // Carga un artículo: fija ruta y dispara el iframe (que muestra el contenido
  // original del ZIM). El título e historial llegan del iframe al cargar (frameNav).
  function load(tab, lib, path, itemId = '') {
    tab.lib = lib;
    tab.path = path;
    tab.itemId = itemId || '';
    tab.error = null;
    tab.titleKey = null; // título literal (viene del iframe); no se re-traduce
    const seg = decodeURIComponent((path || '').split('/').pop() || '');
    tab.title = seg || bookOf(lib) || t('tab.article'); // provisional hasta que cargue el iframe
    tab.nav = (tab.nav || 0) + 1;
  }

  // ── Historial por pestaña (back/fwd), incluye el inicio y las vistas ─────────
  const snapshot = (tb) => ({ kind: tb.kind, view: tb.view, lib: tb.lib, path: tb.path, itemId: tb.itemId, open: tb.open, source: tb.source, title: tb.title });
  function pushHistory(tab) { tab.back.push(snapshot(tab)); tab.fwd = []; }
  function setHome(tab) {
    tab.kind = 'home'; tab.titleKey = 'tab.home'; tab.title = 'Inicio'; tab.article = null; tab.error = null; tab.toc = [];
    if (!tab.search) tab.search = emptySearch();
  }
  // titleKey → la pestaña se re-traduce sola al cambiar de idioma (Tabs resuelve con t()).
  const viewTitleKey = (view) => (view === 'settings' ? 'settings.title' : 'menu.' + view);
  function setView(tab, view) {
    tab.kind = 'view'; tab.view = view; tab.titleKey = viewTitleKey(view); tab.title = view; tab.error = null;
  }
  function restore(tab, st) {
    if (st.kind === 'home') setHome(tab);
    else if (st.kind === 'view') setView(tab, st.view);
    else if (st.kind === 'item') setItem(tab, st.itemId, st.open, st.title, st.source);
    else { tab.kind = 'article'; load(tab, st.lib, st.path, st.itemId); }
  }

  // Abrir una vista del sidebar en la pestaña activa (empuja historial para poder volver).
  function openView(view) {
    if (!active) { newTab(); }
    const tb = active;
    if (tb.kind === 'view' && tb.view === view) return; // ya estamos ahí
    pushHistory(tb);
    if (tb.search) tb.search = emptySearch();
    setView(tb, view);
  }

  // Abrir un artículo: en la pestaña activa (empujando historial) o en una nueva.
  function openArticle(lib, path, { inNew = false, itemId = '' } = {}) {
    if (inNew || !active) {
      const tab = { id: uid++, kind: 'article', lib, path, itemId, title: '…', error: null, back: [], fwd: [], nav: 0 };
      tabs.push(tab); activeId = tab.id;
      load(tab, lib, path, itemId);
    } else {
      pushHistory(active); // recuerda dónde estábamos (inicio o artículo)
      active.kind = 'article';
      load(active, lib, path, itemId);
    }
  }
  function setItem(tab, itemId, open, title, source = null) {
    tab.kind = 'item';
    tab.itemId = itemId;
    tab.open = open;
    tab.source = source;
    tab.lib = null;
    tab.path = null;
    tab.titleKey = null;
    tab.title = title || open?.title || t('tab.article');
    tab.error = null;
  }
  async function openItemById(itemId) {
    if (!itemId) return;
    let item;
    try {
      item = await getItem(itemId);
    } catch (e) {
      return;
    }
    const open = item.open;
    const openPath = serverPath(open?.url);
    if (open?.mode === 'iframe' && openPath.startsWith('/content/')) {
      const rest = decodeURIComponent(openPath.slice('/content/'.length));
      const i = rest.indexOf('/');
      if (i >= 0) {
        // Un artículo ZIM se identifica por lib/path, no por su itemId de Item.
        // No propagamos el itemId: así la estrella/nota/etiqueta usan la misma
        // clave (lib/path) tanto si llegas por link como por búsqueda/favorito.
        openArticle(rest.slice(0, i), rest.slice(i + 1));
        return;
      }
    }
    if (!active) newTab();
    const tb = active;
    pushHistory(tb);
    setItem(tb, itemId, open, item.title || open?.title, item.source);
    readerState.addHistory({ itemId, title: open?.title || itemId, book: open?.title || 'Library' });
  }

  async function navigateAddress(text) {
    const addr = parseLibraryAddress(text);
    if (addr.kind === 'home') { goHome(); return; }
    if (addr.kind === 'view') { openView(addr.view); return; }
    if (addr.kind === 'article' && addr.lib) { openArticle(addr.lib, addr.path); return; }
    if (addr.kind === 'item' && addr.itemId) { await openItemById(addr.itemId); return; }
    if (addr.kind === 'provider' && addr.sourceId) {
      try {
        const item = await resolveProviderItem(addr.provider, addr.sourceId);
        await openItemById(item.id);
      } catch (e) { /* no está importado: se conserva la pantalla actual */ }
      return;
    }
    if (addr.kind === 'search' && addr.query) {
      if (!active) newTab();
      if (active.kind !== 'home') { pushHistory(active); setHome(active); }
      const search = active.search || (active.search = emptySearch());
      search.q = addr.query; search.mode = 'all'; search.loading = true; search.searched = false;
      search.location = { status: 'loading', result: null, radius: 2500, selectedPoi: null };
      const [items, location] = await Promise.allSettled([itemSearch(addr.query), mapSearch(addr.query, 2500)]);
      search.results = items.status === 'fulfilled' ? items.value : [];
      search.searched = true;
      if (location.status === 'fulfilled' && location.value?.available) {
        search.location = { ...search.location, status: 'ready', result: location.value };
      } else {
        search.location = { ...search.location, status: location.status === 'rejected' ? 'error' : (location.value?.reason === 'no_match' ? 'empty' : 'unavailable'), result: null };
      }
      search.loading = false;
    }
  }
  function navigate(lib, path) { openArticle(lib, path); }        // click en link interno
  function openLibrary(lib) { openArticle(lib.id, ''); }          // página principal de la colección
  function back() {
    const tb = active; if (!tb || !tb.back.length) return;
    tb.fwd.push(snapshot(tb));
    restore(tb, tb.back.pop());
  }
  function forward() {
    const tb = active; if (!tb || !tb.fwd.length) return;
    tb.back.push(snapshot(tb));
    restore(tb, tb.fwd.pop());
  }
  // Recargar: el artículo ZIM recarga su iframe (load sube tab.nav → re-fija el src).
  // Las demás superficies (home, vistas, ficha de item, vídeo) son componentes Svelte:
  // se recargan remontándolas → suben tab.rev, y un {#key} en Reader las reconstruye,
  // lo que re-dispara su fetch de datos.
  function reload() {
    if (!active) return;
    if (active.kind === 'article') { load(active, active.lib, active.path, active.itemId); return; }
    active.rev = (active.rev || 0) + 1;
  }
  // El iframe navegó por dentro → sincroniza la ruta actual para que estrella,
  // dirección y título reflejen la página real.
  function frameNav(lib, path, title) {
    if (!active) return;
    const sameItemTarget = active.itemId && active.lib === lib && active.path === path;
    const itemId = sameItemTarget ? active.itemId : '';
    active.lib = lib;
    active.path = path;
    active.itemId = itemId;
    if (title) active.title = title;
    readerState.addHistory({ itemId, lib, path, title: title || path, book: bookOf(lib) }); // registrar visita
  }
  function goHome() {
    if (!active || active.kind === 'home') return;
    pushHistory(active);
    setHome(active);
    active.search = emptySearch(); // "Inicio" = inicio limpio → muestra las páginas guardadas
  }
</script>

<div class="app" class:side-hidden={!sidebarOpen} class:bookmarks-open={barOn}>
  <div class="r-top"><Tabs {tabs} {activeId} onActivate={activate} onClose={closeTab} onNew={newTab} /></div>
  <div class="r-nav">
    <NavBar {active} {sidebarOpen} {indexOpen} user={auth.user}
      onToggleSidebar={toggleSidebar} onToggleIndex={toggleIndex}
      onBack={back} onForward={forward} onReload={reload} onHome={goHome}
      onNavigateAddress={navigateAddress}
      starred={activeFav} onToggleFav={toggleFav} noted={activeNoted} onOpenNote={openActiveNote}
      tagged={activeTagged} onOpenTags={openActiveTags}
      onAccount={() => (accountOpen = true)} />
  </div>
  <div class="r-side"><Sidebar {libraries} activeLib={active?.lib} {activeView} user={auth.user}
      onOpenLibrary={openLibrary} onOpenHome={goHome} onOpenView={openView} onAccount={() => (accountOpen = true)} /></div>
  <div class="r-main">
    {#if barOn}
      <BookmarksBar {favorites} {libraries} onOpen={openFav} onToggleHome={toggleHome} onRemoveFav={removeFav} />
    {/if}
    {#if active}
      <div class="r-reader">
        <Reader tab={active} {libraries} {favorites} {indexOpen} {notesVersion} {tagsVersion} onNavigate={navigate}
          onOpenItem={openItemById} onOpenView={openView} onToggleHome={toggleHome} onFrameNav={frameNav}
          onRemoveFav={removeFav} onOpenNote={openNoteItem} onDeleteNote={deleteNoteItem} />
      </div>
    {/if}
  </div>
</div>

{#if noteEditing}
  <NoteEditor note={noteEditing} onSave={saveNote} onClose={closeNote} />
{/if}

{#if tagEditing}
  <TagEditor target={tagEditing} onChanged={onTagsChanged} onClose={closeTags} />
{/if}

{#if accountOpen || loginPrompt.open}
  <AccountModal
    reason={loginPrompt.open ? loginPrompt.reason : ''}
    onClose={() => { accountOpen = false; loginPrompt.open = false; }}
    onChanged={onAuthChanged} />
{/if}

<style>
  .app{
    height:100vh;display:grid;
    grid-template-rows:46px 52px 1fr;
    grid-template-columns:262px 1fr;
    grid-template-areas:"top top" "nav nav" "side main";
    transition:grid-template-columns .2s ease;
  }
  .app.side-hidden{grid-template-columns:0 1fr}
  /* Con la barra de marcadores del inicio abierta, el navbar pierde su borde
     inferior para que barra y navbar se lean como un solo bloque superior. */
  :global(.app.bookmarks-open .nav){border-bottom-color:transparent}
  .r-top{grid-area:top;min-width:0}
  .r-nav{grid-area:nav;min-width:0}
  .r-side{grid-area:side;overflow:hidden}
  .app.side-hidden .r-side{border-right:none}
  .r-main{grid-area:main;min-width:0;overflow:hidden;display:flex;flex-direction:column}
  .r-reader{flex:1;min-height:0;min-width:0}
</style>
