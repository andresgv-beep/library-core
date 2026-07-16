// Cliente del Panel de Control → habla con el shim (Library Core) por /api.
// Solo dominios de administración: pool/almacenamiento, colecciones, modelos de
// traducción, catálogo ZIM y cola de descargas.

async function getJSON(url, opts) {
  const r = await fetch(url, opts)
  if (!r.ok) {
    const msg = await r.text().catch(() => r.statusText)
    try { throw new Error(JSON.parse(msg).error || msg) } catch (e) {
      if (e instanceof SyntaxError) throw new Error(msg || `HTTP ${r.status}`)
      throw e
    }
  }
  return r.json()
}

// Almacenamiento (pool) — POOL-CONTRACT.md §6
export const getStorage = () => getJSON('/api/storage')
export const setStorageRoot = (root) =>
  getJSON('/api/storage', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ contentRoot: root }),
  })

// Colecciones normalizadas (ZIM + contenido publicado)
export const getCollections = () =>
  getJSON('/api/collections').then((d) => d.collections || [])

// Idiomas/modelos de traducción instalados (vía translate-wrap)
export const getLanguages = () =>
  getJSON('/api/translate/languages').catch(() => ({ available: false }))

// Salud del Core + motor
export const getHealth = () => getJSON('/api/health').catch(() => ({ shim: 'down', engine: 'down' }))
export const getServiceStatus = () =>
  getJSON('/api/admin/service').catch(() => ({ supervised: false }))
export const restartLibraryServer = () =>
  postJSON('/api/admin/service', {})

// Mapas offline: catalogo regional, extraccion PMTiles y mapa activo.
export const getMaps = () => getJSON('/api/admin/maps')
export const downloadMap = (regionId, maxZoom) =>
  getJSON('/api/admin/maps/download', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ regionId, maxZoom }) })
export const cancelMapDownload = () =>
  getJSON('/api/admin/maps/cancel', { method: 'POST' })
export const activateMap = (file) =>
  getJSON('/api/admin/maps/activate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ file }) })
export const deleteMap = (file) =>
  getJSON('/api/admin/maps/delete', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ file }) })

// ── Auth / usuarios ──
export const authMe = () => getJSON('/api/auth/me').catch(() => ({ setupNeeded: false, user: null }))
export const authRegister = (username, password, age) =>
  postJSON('/api/auth/register', { username, password, age })
export const authLogin = (username, password) => postJSON('/api/auth/login', { username, password })
export const authLogout = () => postJSON('/api/auth/logout', {})
export const listUsers = () => getJSON('/api/admin/users').then((d) => d.users || [])
export const createUser = (u) => postJSON('/api/admin/users', u)
export const deleteUser = (id) =>
  fetch(`/api/admin/users/${id}`, { method: 'DELETE' })
// Restablecer la contraseña de un usuario (a petición, por olvido). La temporal
// debe cumplir la política (10 caracteres + 1 especial); el usuario la cambia
// luego desde su interfaz del lector.
export const resetPassword = (id, password) =>
  fetch(`/api/admin/users/${id}/password`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })

// ── Acceso por colección ──
export const getAccessMap = () => getJSON('/api/admin/collections/access').then((d) => d.access || {})
export function setAccess(collectionId, access, minAge, allowDownload) {
  return fetch('/api/admin/collections/access', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ collectionId, access, minAge, allowDownload: !!allowDownload }),
  })
}

// Gestión de ZIM (Panel) — registrar/quitar colecciones en library.xml
export const getAdminZim = () => getJSON('/api/admin/zim')

function postJSON(url, payload) {
  return fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}
export const registerZim = (file) => postJSON('/api/admin/zim/register', { file })
export const unregisterZim = (id) => postJSON('/api/admin/zim/unregister', { id })

// Catálogo remoto de Kiwix (Panel) — explorar y descargar al pool
export const catalogCategories = () =>
  getJSON('/api/admin/catalog/categories').then((d) => d.categories || [])
export function catalogEntries({ category = '', lang = '', q = '', count = 60 } = {}) {
  const p = new URLSearchParams()
  if (category) p.set('category', category)
  if (lang) p.set('lang', lang)
  if (q) p.set('q', q)
  p.set('count', String(count))
  return getJSON(`/api/admin/catalog/entries?${p.toString()}`).then((d) => d.entries || [])
}
export const catalogDownload = (url, filename) =>
  postJSON('/api/admin/catalog/download', { url, filename })

// Gestión de modelos de traducción (Panel) — vía translate-wrap
export const translateAvailable = () => getJSON('/api/admin/translate/available')
export const translateDownload = (id) => postJSON('/api/admin/translate/download', { id })
export const translateRemove = (id) => postJSON('/api/admin/translate/remove', { id })

// Contenido del pool (medios locales) · listar y eliminar
export const getMedia = () =>
  getJSON('/api/media').then((d) => d.items || []).catch(() => [])
export const deleteMedia = (id) => postJSON('/api/admin/media/delete', { id })

// Carga manual (Moments/Cabinet): multipart con fichero + metadatos + imágenes.
// files: { file, cover, channel_avatar }. Devuelve la respuesta cruda.
export function uploadContent(fields, files = {}, url = '/api/admin/upload') {
  const fd = new FormData()
  for (const [k, v] of Object.entries(fields)) {
    if (v != null && String(v).trim() !== '') fd.append(k, v)
  }
  for (const [k, f] of Object.entries(files)) {
    if (f) fd.append(k, f)
  }
  return fetch(url, { method: 'POST', body: fd })
}

// Editar la ficha de un item ya en el pool (metadatos + imágenes + visibilidad).
// El fichero de media no se cambia. id = ID del item (ruta del sidecar).
export function updateContent(id, fields, files = {}) {
  return uploadContent({ ...fields, id }, files, '/api/admin/media/update')
}

// Importar · Cola de descargas administrativas
export const listDownloads = () =>
  getJSON('/api/downloads').then((d) => d.jobs || []).catch(() => [])

export function enqueueDownload({ url, owner_kind = 'manual', owner_id = '', dest_dir = '', filename = '' }) {
  return fetch('/api/downloads', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, owner_kind, owner_id, dest_dir, filename }),
  })
}

const downloadOp = (id, action) =>
  fetch(`/api/downloads/${encodeURIComponent(id)}/${action}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
export const pauseDownload = (id) => downloadOp(id, 'pause')
export const resumeDownload = (id) => downloadOp(id, 'resume')
export const cancelDownload = (id) => downloadOp(id, 'cancel')

// Limpia el historial de descargas terminadas (done/error/cancelled).
export const clearDownloads = () =>
  fetch('/api/downloads/clear', { method: 'POST', headers: { 'Content-Type': 'application/json' } })
