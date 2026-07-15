// Deduce la "app" de origen de un favorito/ítem a partir de su itemId. Los ítems
// de media tienen itemId "media:"+base64(rutaSidecar); el prefijo de la ruta
// (Moments/… · Cabinet/…) identifica la app. Los artículos ZIM no tienen ese
// prefijo → devuelve '' (sin marca; se usa el icono del ZIM).
export function sourceOfItemId(id) {
  if (typeof id !== 'string' || !id.startsWith('media:')) return '';
  let b = id.slice(6).replace(/-/g, '+').replace(/_/g, '/');
  b += '='.repeat((4 - (b.length % 4)) % 4);
  let path = '';
  try { path = decodeURIComponent(escape(atob(b))); }
  catch (e) { try { path = atob(b); } catch (e2) { return ''; } }
  if (path.startsWith('Moments/')) return 'moments';
  if (path.startsWith('Cabinet/')) return 'cabinet';
  return '';
}
