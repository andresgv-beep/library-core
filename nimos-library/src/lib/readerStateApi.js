// API de estado personal del lector: favoritos, notas, historial y etiquetas.

import { JSONH } from './http.js';
import { serverFetch } from './connection.js';

function identityQuery(lib, path, itemId) {
  const q = new URLSearchParams();
  if (itemId) q.set('itemId', itemId);
  else {
    q.set('lib', lib || '');
    q.set('path', path || '');
  }
  return q.toString();
}

export async function getFavorites() { const r = await serverFetch('/api/favorites'); return r.ok ? r.json() : []; }
export async function putFavorite(f) { try { await serverFetch('/api/favorites', { method: 'PUT', headers: JSONH, body: JSON.stringify(f) }); } catch (e) {} }
export async function deleteFavorite(lib, path, itemId = '') { try { await serverFetch(`/api/favorites?${identityQuery(lib, path, itemId)}`, { method: 'DELETE' }); } catch (e) {} }

export async function listNotes() { const r = await serverFetch('/api/notes'); return r.ok ? r.json() : []; }
export async function getNote(lib, path, itemId = '') { const r = await serverFetch(`/api/notes?${identityQuery(lib, path, itemId)}`); return r.ok ? r.json() : null; }
export async function putNote(n) { try { await serverFetch('/api/notes', { method: 'PUT', headers: JSONH, body: JSON.stringify(n) }); } catch (e) {} }
export async function deleteNote(lib, path, itemId = '') { try { await serverFetch(`/api/notes?${identityQuery(lib, path, itemId)}`, { method: 'DELETE' }); } catch (e) {} }

export async function addHistory(v) { try { serverFetch('/api/history', { method: 'POST', headers: JSONH, body: JSON.stringify(v) }); } catch (e) {} }
export async function getRecent() { const r = await serverFetch('/api/recent'); return r.ok ? r.json() : []; }
export async function getHistory() { const r = await serverFetch('/api/history'); return r.ok ? r.json() : []; }
export async function deleteHistoryEntry(id) { try { await serverFetch(`/api/history?id=${encodeURIComponent(id)}`, { method: 'DELETE' }); } catch (e) {} }
export async function deleteHistoryPage(lib, path, itemId = '') { try { await serverFetch(`/api/history?${identityQuery(lib, path, itemId)}`, { method: 'DELETE' }); } catch (e) {} }
export async function clearHistory() { try { await serverFetch('/api/history', { method: 'DELETE' }); } catch (e) {} }

export async function listTags() { const r = await serverFetch('/api/tags'); return r.ok ? r.json() : []; }
export async function getTagPages(tag) { const r = await serverFetch(`/api/tags?tag=${encodeURIComponent(tag)}`); return r.ok ? r.json() : []; }
export async function getPageTags(lib, path, itemId = '') { const r = await serverFetch(`/api/tags?${identityQuery(lib, path, itemId)}`); return r.ok ? r.json() : []; }
export async function getTaggedKeys() { const r = await serverFetch('/api/tags?keys=1'); return r.ok ? r.json() : []; }
export async function addTag(t) { try { await serverFetch('/api/tags', { method: 'PUT', headers: JSONH, body: JSON.stringify(t) }); } catch (e) {} }
export async function removeTag(lib, path, tag, itemId = '') {
  try {
    const q = new URLSearchParams(identityQuery(lib, path, itemId));
    q.set('tag', tag);
    await serverFetch(`/api/tags?${q.toString()}`, { method: 'DELETE' });
  } catch (e) {}
}
export async function deleteTag(tag) { try { await serverFetch(`/api/tags?tag=${encodeURIComponent(tag)}`, { method: 'DELETE' }); } catch (e) {} }
