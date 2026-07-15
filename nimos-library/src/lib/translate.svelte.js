// Preferencias de traducción del lector (dropdown del icono).
//
// El idioma NATIVO del usuario es el de la UI (Ajustes → i18n.locale): de momento
// ES/EN. El modo AUTO traduce al idioma nativo cuando el artículo está en otro
// (p. ej. artículo en inglés + nativo español → traduce a español, y viceversa).
// La lista del dropdown permite fijar un idioma destino manual.
//
// Este módulo solo mantiene la preferencia; el disparo de la traducción (recorrer
// el iframe → /api/translate) se cablea en la Fase 2 (TRANSLATE.md §6).

import { i18n } from './i18n.svelte.js';

const STORE_KEY = 'nimos-translate';

function load() {
  try {
    return JSON.parse(localStorage.getItem(STORE_KEY) || '{}');
  } catch (e) {
    return {};
  }
}
const saved = load();

// Estado reactivo compartido: leerlo en un template lo suscribe al cambio.
// `run`/`original` son nonces-comando: el menú (NavBar) los incrementa y el
// Reader (dueño del iframe) los observa para traducir o restaurar el original,
// sin acoplar componentes por props.
export const tstate = $state({
  auto: saved.auto ?? false,
  target: saved.target || '', // '' = usar el idioma nativo (locale)
  run: 0,
  original: 0,
});

// Comandos que dispara el dropdown; el Reader reacciona.
export function requestTranslate() {
  tstate.run++;
}
export function requestOriginal() {
  tstate.original++;
}

// norm2: códigos ISO 639-2 → 639-1 (mismo mapa que el shim). "mul"/desconocido → ''.
const LANG2 = {
  eng: 'en', spa: 'es', fra: 'fr', fre: 'fr', deu: 'de', ger: 'de',
  ita: 'it', por: 'pt', rus: 'ru', cat: 'ca', nld: 'nl', dut: 'nl',
};
export function norm2(code) {
  code = (code || '').toLowerCase().trim();
  if (!code) return '';
  if (LANG2[code]) return LANG2[code];
  if (code.length === 2) return code;
  return '';
}

// detectLang: detección ligera es/en por señales léxicas (stopwords + ortografía),
// para autotraducir cuando el item NO trae metadato de idioma. Solo distingue los
// dos locales soportados. Devuelve 'es' | 'en' | '' (incierto → no se autotraduce).
const ES_WORDS = new Set(['el','la','los','las','de','que','y','en','un','una','por','con','para','del','se','es','su','al','lo','como','más','pero','este','esta','sus','no','ha','han','fue','le','ese','esa','entre','sobre','cuando','muy','sin','también','así','porque','desde','hay','son','está']);
const EN_WORDS = new Set(['the','of','and','to','in','is','that','for','with','on','as','by','this','are','was','from','it','an','be','or','at','which','has','his','their','they','you','not','but','have','more','when','who','will','all','can','about','was','were','been']);
export function detectLang(text) {
  const s = (text || '').toLowerCase();
  if (s.trim().length < 8) return '';
  const words = s.split(/[^a-záéíóúñü]+/).filter(Boolean);
  let es = 0, en = 0;
  for (const w of words) { if (ES_WORDS.has(w)) es++; else if (EN_WORDS.has(w)) en++; }
  if (/[ñ¿¡]/.test(s)) es += 3;   // señales casi seguras de español
  if (/[áéíóú]/.test(s)) es += 1;
  if (es === 0 && en === 0) return '';
  if (es >= 2 && es >= en * 1.25) return 'es';
  if (en >= 2 && en >= es * 1.25) return 'en';
  return '';
}

function persist() {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify({ auto: tstate.auto, target: tstate.target }));
  } catch (e) {}
}

// Idioma nativo del usuario = idioma de la UI (elegido en Ajustes).
export function nativeLang() {
  return i18n.locale;
}

// Idioma destino efectivo: el manual elegido, o el nativo si no hay.
export function targetLang() {
  return tstate.target || i18n.locale;
}

export function setAuto(v) {
  tstate.auto = !!v;
  persist();
}

export function setTarget(code) {
  tstate.target = code;
  tstate.auto = false; // elegir un destino manual desactiva el auto
  persist();
}
