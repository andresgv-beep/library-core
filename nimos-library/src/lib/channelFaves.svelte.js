// channelFaves.svelte.js — canales guardados (favoritos) de Vídeos. Reactivo
// (runes) + persistido en localStorage. La estrella de la cuadrícula los añade;
// la card lateral de "Favoritos" los lista.
const KEY = 'nimos-yt-faves';

function load() {
  try { const v = JSON.parse(localStorage.getItem(KEY) || '[]'); return Array.isArray(v) ? v : []; }
  catch { return []; }
}

export const channelFaves = $state({ list: load() });

export function toggleFave(name) {
  if (!name) return;
  const s = new Set(channelFaves.list);
  s.has(name) ? s.delete(name) : s.add(name);
  channelFaves.list = [...s];
  try { localStorage.setItem(KEY, JSON.stringify(channelFaves.list)); } catch {}
}

export const isFave = (name) => channelFaves.list.includes(name);
