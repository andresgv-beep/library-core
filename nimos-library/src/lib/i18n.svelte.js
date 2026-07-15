// i18n ligero y propio (sin dependencias, coherente con el ethos del proyecto).
// `locale` es estado reactivo de módulo (runes) → cambiar idioma re-renderiza toda
// la UI en vivo. Añadir un idioma = añadir su diccionario en messages.js + una
// entrada en LANGS. El CONTENIDO de los ZIM no se traduce; esto es solo el shell.

import { messages } from './messages.js';

export const LANGS = [
  { code: 'es', label: 'Español', flag: '🇪🇸' },
  { code: 'en', label: 'English', flag: '🇬🇧' },
];

const STORE_KEY = 'nimos-lang';

function detectInitial() {
  try {
    const saved = localStorage.getItem(STORE_KEY);
    if (saved && messages[saved]) return saved;
  } catch (e) {}
  try {
    const b = (navigator.language || 'es').slice(0, 2).toLowerCase();
    if (messages[b]) return b;
  } catch (e) {}
  return 'es';
}

// Estado reactivo compartido. Leer i18n.locale en un template lo suscribe al cambio.
export const i18n = $state({ locale: detectInitial() });

export function setLocale(code) {
  if (!messages[code]) return;
  i18n.locale = code;
  try { localStorage.setItem(STORE_KEY, code); } catch (e) {}
}

// t(clave, params?) → cadena traducida. Fallback: idioma actual → español → la clave.
// Interpola {marcadores} con params: t('home.searching', { n: 5 }).
export function t(key, params) {
  const dict = messages[i18n.locale] || messages.es;
  let s = dict[key];
  if (s == null) s = messages.es[key];
  if (s == null) return key;
  if (params) for (const k in params) s = s.replaceAll('{' + k + '}', params[k]);
  return s;
}

// Plural simple (es/en comparten regla n===1). Claves: `${base}.one` / `${base}.other`.
export function tn(base, n, params = {}) {
  return t(`${base}.${n === 1 ? 'one' : 'other'}`, { n, ...params });
}

// Tiempo relativo localizado sin diccionario (Intl). "hace 5 min" / "5 min ago"…
export function relTime(sec) {
  if (!sec) return '';
  const diff = sec - Math.floor(Date.now() / 1000); // negativo = pasado
  const abs = Math.abs(diff);
  const rtf = new Intl.RelativeTimeFormat(i18n.locale, { numeric: 'auto' });
  if (abs < 45) return t('time.now');
  if (abs < 3600) return rtf.format(Math.round(diff / 60), 'minute');
  if (abs < 86400) return rtf.format(Math.round(diff / 3600), 'hour');
  if (abs < 2592000) return rtf.format(Math.round(diff / 86400), 'day');
  if (abs < 31536000) return rtf.format(Math.round(diff / 2592000), 'month');
  return new Date(sec * 1000).toLocaleDateString(i18n.locale, { day: 'numeric', month: 'short', year: 'numeric' });
}
