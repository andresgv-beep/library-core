// videoProgress.svelte.js — progreso de reproducción de vídeos (para "Seguir
// viendo"). Reactivo (runes) y persistido en localStorage. MomentsWatch lo va
// guardando mientras se ve un vídeo; Moments lo lee para la sección.
const KEY = 'nimos-yt-progress';

function loadMap() {
  try { return JSON.parse(localStorage.getItem(KEY) || '{}'); } catch { return {}; }
}

// { map: { <itemId>: { t: segundos, d: duración, at: timestamp } } }
export const videoProgress = $state({ map: loadMap() });

// Guarda el progreso; si está casi al final (o al principio) lo quita (no es
// "seguir viendo"). Reasignar .map dispara la reactividad en Moments.
export function saveVideoProgress(id, t, d) {
  if (!id || !d || !Number.isFinite(t)) return;
  const m = { ...videoProgress.map };
  if (t > 5 && t < d - 10) m[id] = { t, d, at: Date.now() };
  else if (t >= d - 10 && m[id]) delete m[id]; // terminado → fuera de la lista
  else return;
  videoProgress.map = m;
  try { localStorage.setItem(KEY, JSON.stringify(m)); } catch {}
}

export const getVideoProgress = (id) => videoProgress.map[id];

// Vacía todo el historial de reproducciones ("Seguir viendo").
export function clearAllProgress() {
  videoProgress.map = {};
  try { localStorage.removeItem(KEY); } catch {}
}
