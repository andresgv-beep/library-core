// API de consumo: colecciones, busqueda, salud y contenido ya publicado.

import { jsonOrError, JSONH } from './http.js';
import { serverFetch } from './connection.js';

// ─── Traducción (TRANSLATE.md) ────────────────────────────────────────────────

// ¿Hay motor de traducción disponible? { available, pairs }
export async function translateLanguages() {
  try {
    const r = await serverFetch('/api/translate/languages');
    return await jsonOrError(r);
  } catch (e) {
    return { available: false };
  }
}

// Traduce segmentos {id,text} a `to`. Devuelve [{id,text}] (mezcla caché + motor).
// html:true → los textos son fragmentos HTML y se preservan los tags (enlaces).
export async function translateSegments({ lib, path, to, sourceHint, html, segments }, { signal } = {}) {
  const r = await serverFetch('/api/translate', {
    method: 'POST',
    headers: JSONH,
    body: JSON.stringify({ lib, path, to, sourceHint, html, segments }),
    signal,
  });
  const data = await jsonOrError(r);
  return data.segments || [];
}

export async function getLibraries() {
  const r = await serverFetch('/api/libraries');
  if (!r.ok) throw new Error('No se pudieron cargar las colecciones');
  return jsonOrError(r);
}

export async function getCollections() {
  const r = await serverFetch('/api/collections');
  const data = await jsonOrError(r);
  return data.collections || [];
}

export async function getCollection(id) {
  const r = await serverFetch(`/api/collections/${encodeURIComponent(id)}`);
  return jsonOrError(r);
}

export async function getCollectionItems(id) {
  const r = await serverFetch(`/api/collections/${encodeURIComponent(id)}/items`);
  const data = await jsonOrError(r);
  return data.items || [];
}

export async function getItem(id) {
  const r = await serverFetch(`/api/items/${encodeURIComponent(id)}`);
  return jsonOrError(r);
}

export async function resolveProviderItem(provider, sourceId) {
  const q = new URLSearchParams({ provider, sourceId });
  const r = await serverFetch(`/api/items/resolve?${q.toString()}`);
  return jsonOrError(r);
}

export async function openItem(id) {
  const r = await serverFetch(`/api/items/${encodeURIComponent(id)}/open`);
  return jsonOrError(r);
}

export async function previewItem(id) {
  const r = await serverFetch(`/api/items/${encodeURIComponent(id)}/preview`);
  return jsonOrError(r);
}

export async function itemSearch(q, { signal } = {}) {
  if (!q || !q.trim()) return [];
  const r = await serverFetch(`/api/items/search?q=${encodeURIComponent(q)}`, { signal });
  const data = await jsonOrError(r);
  return data.results || [];
}

export async function suggest(lib, q) {
  if (!q || !q.trim()) return [];
  const r = await serverFetch(`/api/libraries/${encodeURIComponent(lib)}/search?q=${encodeURIComponent(q)}`);
  if (!r.ok) return [];
  return jsonOrError(r);
}

export async function globalSearch(q, { signal } = {}) {
  if (!q || !q.trim()) return [];
  const r = await serverFetch(`/api/search?q=${encodeURIComponent(q)}`, { signal });
  if (!r.ok) return [];
  return jsonOrError(r);
}

export async function globalImages(q, { signal } = {}) {
  if (!q || !q.trim()) return [];
  const r = await serverFetch(`/api/images?q=${encodeURIComponent(q)}`, { signal });
  if (!r.ok) return [];
  return jsonOrError(r);
}

export async function health() {
  const r = await serverFetch('/api/health');
  return r.ok ? jsonOrError(r) : { shim: 'down' };
}

export async function getMedia(collection = '') {
  const q = collection ? `?collection=${encodeURIComponent(collection)}` : '';
  const r = await serverFetch(`/api/media${q}`);
  const data = await jsonOrError(r);
  return data.items || [];
}

export function fmtSize(bytes) {
  if (!bytes) return '';
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0, n = bytes;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return `${n.toFixed(n < 10 && i > 2 ? 1 : 0)} ${u[i]}`;
}
