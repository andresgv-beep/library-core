// sites.svelte.js — qué "sitios" (apps Vídeos/Archivo local + colecciones ZIM)
// aparecen en el launcher "Tus sitios" del inicio. Reactivo (runes) + persistido en
// localStorage. Modelo OPT-OUT: por defecto se muestran TODOS; el usuario oculta con
// la estrella del sidebar los que no quiera (así un ZIM recién importado aparece
// solo). Guardamos el conjunto OCULTO (no el mostrado) para que lo nuevo sea visible.
const KEY = 'nimos-sites-hidden';

function load() {
  try { const v = JSON.parse(localStorage.getItem(KEY) || '[]'); return Array.isArray(v) ? v : []; }
  catch { return []; }
}

export const siteHidden = $state({ list: load() });

// ¿el sitio `key` está en "Tus sitios"? (true = visible en el inicio).
export const siteShown = (key) => !siteHidden.list.includes(key);

// Alterna si el sitio aparece en "Tus sitios".
export function toggleSite(key) {
  if (!key) return;
  const s = new Set(siteHidden.list);
  s.has(key) ? s.delete(key) : s.add(key);
  siteHidden.list = [...s];
  try { localStorage.setItem(KEY, JSON.stringify(siteHidden.list)); } catch {}
}
