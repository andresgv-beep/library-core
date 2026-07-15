const STORAGE_KEY = 'nimos-library-server';
const SESSION_KEY = 'nimos-library-session';

function normalizeBase(value) {
  return String(value || '').trim().replace(/\/+$/, '');
}

function initialBase() {
  if (typeof window !== 'undefined') {
    const injected = window.__NIMOS_LIBRARY_SERVER__;
    if (injected) return normalizeBase(injected);
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved) return normalizeBase(saved);
    } catch (e) {}
  }
  return normalizeBase(import.meta.env.VITE_LIBRARY_SERVER || '');
}

let serverBase = initialBase();

export function getServerBase() {
  return serverBase;
}

export function setServerBase(value) {
  const next = normalizeBase(value);
  if (next !== serverBase) setSessionToken('');
  serverBase = next;
  try {
    if (serverBase) localStorage.setItem(STORAGE_KEY, serverBase);
    else localStorage.removeItem(STORAGE_KEY);
  } catch (e) {}
  return serverBase;
}

export function serverUrl(path) {
  const value = String(path || '');
  if (!value || /^(?:[a-z][a-z0-9+.-]*:)?\/\//i.test(value) || value.startsWith('data:') || value.startsWith('blob:')) {
    return value;
  }
  if (!serverBase) return value;
  const full = `${serverBase}${value.startsWith('/') ? '' : '/'}${value}`;
  return withMediaToken(full, value.startsWith('/') ? value : `/${value}`);
}

// withMediaToken añade el token de sesión (?st=) a las URLs de /media y /content:
// esas se cargan como src de <video>/<embed>/<img>/<iframe>, que no pueden mandar
// la cabecera Authorization y, cross-origin, tampoco siempre la cookie. Solo esas
// dos rutas (no /api, que va por Bearer) para no filtrar el token de más.
function withMediaToken(fullUrl, path) {
  const token = getSessionToken();
  if (!token) return fullUrl;
  if (!path.startsWith('/media') && !path.startsWith('/content')) return fullUrl;
  return `${fullUrl}${fullUrl.includes('?') ? '&' : '?'}st=${encodeURIComponent(token)}`;
}

export function serverFetch(input, init = {}) {
  const target = typeof input === 'string' || input instanceof URL ? serverUrl(input) : input;
  const headers = new Headers(init.headers || {});
  const token = getSessionToken();
  if (token && !headers.has('Authorization')) headers.set('Authorization', `Bearer ${token}`);
  return fetch(target, { credentials: 'include', ...init, headers });
}

export function getSessionToken() {
  try { return localStorage.getItem(SESSION_KEY) || ''; } catch (e) { return ''; }
}

export function setSessionToken(token) {
  try {
    if (token) localStorage.setItem(SESSION_KEY, token);
    else localStorage.removeItem(SESSION_KEY);
  } catch (e) {}
}

export function serverPath(value) {
  const url = String(value || '');
  if (url.startsWith('/')) return url;
  try {
    const parsed = new URL(url);
    if (!serverBase || parsed.origin === new URL(serverBase).origin) {
      return `${parsed.pathname}${parsed.search}${parsed.hash}`;
    }
  } catch (e) {}
  return url;
}

export function resolveServerPayload(value) {
  if (Array.isArray(value)) return value.map(resolveServerPayload);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, resolveServerPayload(entry)]));
  }
  if (typeof value === 'string' && value.startsWith('/')) return serverUrl(value);
  return value;
}
