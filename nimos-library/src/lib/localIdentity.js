// Separa el estado que vive en el navegador cuando varias cuentas utilizan el
// mismo cliente de Windows. Las preferencias globales (idioma/tema) se conservan;
// búsquedas, perfil, sitios y progreso se guardan por servidor + identidad.

import { getServerBase } from './connection.js';

const OWNER_KEY = 'nimos-local-owner-v1';
const PRIVATE_KEYS = [
  'nimos-recent-searches',
  'nimos-yt-faves',
  'nimos-yt-progress',
  'nimos-sites-hidden',
  'nimos-profile',
];

function identityKey(user) {
  const server = getServerBase() || 'same-origin';
  // El ID evita que una cuenta borrada y recreada con el mismo nombre herede
  // búsquedas o progreso local de la persona anterior.
  const who = user?.id != null ? `user:${user.id}` : 'guest';
  return `${server}|${who}`;
}

function scoped(owner, key) {
  return `nimos-private:${encodeURIComponent(owner)}:${key}`;
}

// Devuelve true cuando ha cambiado de identidad y los módulos reactivos que ya
// habían leído localStorage deben recargarse.
export function syncLocalIdentity(user) {
  try {
    const next = identityKey(user);
    const current = localStorage.getItem(OWNER_KEY);
    if (!current) {
      // Primera ejecución tras actualizar: los datos existentes pertenecen a la
      // identidad que el servidor acaba de resolver.
      localStorage.setItem(OWNER_KEY, next);
      return false;
    }
    if (current === next) return false;

    for (const key of PRIVATE_KEYS) {
      const value = localStorage.getItem(key);
      if (value == null) localStorage.removeItem(scoped(current, key));
      else localStorage.setItem(scoped(current, key), value);
      localStorage.removeItem(key);
    }
    for (const key of PRIVATE_KEYS) {
      const value = localStorage.getItem(scoped(next, key));
      if (value != null) localStorage.setItem(key, value);
    }
    localStorage.setItem(OWNER_KEY, next);
    return true;
  } catch (e) {
    return false;
  }
}
