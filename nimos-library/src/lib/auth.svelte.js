// Auth del lector: sesión del usuario para el enforcement de acceso/edad.
// El backend ya filtra librerías/contenido por sesión; aquí solo iniciamos y
// cerramos sesión, y sabemos quién somos para pintar la cuenta.

import { serverFetch, setMediaToken, setSessionToken, serverUrl } from './connection.js';
import { syncLocalIdentity } from './localIdentity.js';

export const auth = $state({ user: null, setupNeeded: false, loaded: false });

// loginPrompt: lo activa un intento de descarga que el servidor rechaza por falta
// de cuenta (403 loginToDownload). App lo observa y abre el modal de login. Así el
// botón "↓" se queda visible para el anónimo, pero al pulsarlo pide iniciar sesión
// en vez de bajar (modelo pedido: ver público, descargar exige cuenta salvo que el
// admin marque descarga anónima).
export const loginPrompt = $state({ open: false, reason: '' });

let mediaRefreshTimer = null;

async function refreshMediaToken() {
  if (!auth.user) { setMediaToken(''); return; }
  const r = await serverFetch('/api/auth/media-token', { method: 'POST' });
  if (!r.ok) { setMediaToken(''); return; }
  const data = await r.json();
  setMediaToken(data.token || '');
  if (!mediaRefreshTimer && typeof window !== 'undefined') {
    mediaRefreshTimer = window.setInterval(() => { if (auth.user) refreshMediaToken().catch(() => setMediaToken('')); }, 10 * 60 * 1000);
  }
}

async function rotateSession() {
  const r = await serverFetch('/api/auth/refresh', { method: 'POST' });
  if (!r.ok) return false;
  const data = await r.json();
  if (data.sessionToken) setSessionToken(data.sessionToken);
  return true;
}

export async function refreshAuth() {
  try {
    const r = await serverFetch('/api/auth/me');
    const d = await r.json();
    auth.user = d.user || null;
    auth.setupNeeded = !!d.setupNeeded;
    if (auth.user) await rotateSession();
    await refreshMediaToken();
    if (syncLocalIdentity(auth.user)) setTimeout(() => window.location.reload(), 0);
  } catch (e) {
    auth.user = null;
    setMediaToken('');
  }
  auth.loaded = true;
}

export async function login(username, password) {
  const r = await serverFetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'no se pudo entrar');
  const data = await r.json();
  setSessionToken(data.sessionToken || '');
  const identityChanged = syncLocalIdentity(data.user || null);
  await refreshAuth();
  if (identityChanged) setTimeout(() => window.location.reload(), 0);
}

export async function logout() {
  await serverFetch('/api/auth/logout', { method: 'POST' });
  setSessionToken('');
  setMediaToken('');
  const identityChanged = syncLocalIdentity(null);
  await refreshAuth();
  if (identityChanged) setTimeout(() => window.location.reload(), 0);
}

export async function logoutAll() {
  await serverFetch('/api/auth/logout-all', { method: 'POST' });
  setSessionToken('');
  setMediaToken('');
  const identityChanged = syncLocalIdentity(null);
  await refreshAuth();
  if (identityChanged) setTimeout(() => window.location.reload(), 0);
}

// changePassword: el usuario cambia SU propia contraseña. Exige la actual (no es
// un reset: eso lo hace el admin desde el Panel). El servidor renueva la sesión
// de esta petición y cierra las demás; guardamos el token nuevo para no quedar
// fuera. La regla (10 caracteres + 1 especial) la valida el servidor.
export async function changePassword(current, next) {
  const r = await serverFetch('/api/auth/password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current, new: next }),
  });
  if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'no se pudo cambiar la contraseña');
  const data = await r.json().catch(() => ({}));
  if (data.sessionToken) setSessionToken(data.sessionToken);
}

// downloadMedia intenta descargar un fichero de /media con intención explícita
// (?dl=1). El servidor decide: si la colección no permite descarga anónima y no
// hay sesión, responde 403 con loginToDownload → abrimos el modal de login en vez
// de bajar nada. Si autoriza, disparamos la descarga real (el navegador guarda por
// el Content-Disposition que pone el servidor).
//
// `mediaUrl` es la URL /media que ya trae el item (puede llevar ?st=). Le añadimos
// dl=1 y hacemos una comprobación con fetch; si pasa, navegamos a la misma URL para
// que el navegador la guarde.
export async function downloadMedia(mediaUrl, filename) {
  if (!mediaUrl) return;
  const sep = mediaUrl.includes('?') ? '&' : '?';
  const dlUrl = `${mediaUrl}${sep}dl=1`;

  // Sonda: ¿me deja el servidor? Pedimos un rango mínimo para no bajar el fichero
  // entero solo para comprobar el permiso.
  let resp;
  try {
    resp = await fetch(dlUrl, { credentials: 'include', headers: { Range: 'bytes=0-0' } });
  } catch (e) {
    // Error de red: intentamos la descarga directa igualmente (mejor que nada).
    triggerBrowserDownload(dlUrl, filename);
    return;
  }
  if (resp.status === 403) {
    let body = {};
    try { body = await resp.json(); } catch (e) {}
    if (body.loginToDownload) {
      loginPrompt.reason = 'download';
      loginPrompt.open = true;
      return;
    }
    // 403 sin loginToDownload: sin acceso a la colección; no hay descarga posible.
    return;
  }
  // Autorizado (200/206): navegar a la URL para que el navegador guarde el fichero.
  triggerBrowserDownload(dlUrl, filename);
}

function triggerBrowserDownload(url, filename) {
  const a = document.createElement('a');
  a.href = url;
  if (filename) a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}
