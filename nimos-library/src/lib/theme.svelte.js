// Tema del shell (claro/oscuro/sistema). SOLO afecta a la interfaz: el contenido
// de los ZIM se muestra en su iframe con su propio estilo (no se oscurece). El tema
// se aplica como atributo data-theme en <html>; app.css trae los tokens claros en
// :root[data-theme="light"]. El anti-parpadeo inicial lo hace un script en index.html.

const STORE_KEY = 'nimos-theme';

export const THEMES = [
  { code: 'system', labelKey: 'settings.themeSystem', icon: 'contrast' },
  { code: 'light', labelKey: 'settings.themeLight', icon: 'sun' },
  { code: 'dark', labelKey: 'settings.themeDark', icon: 'moon' },
];

function saved() {
  try { const v = localStorage.getItem(STORE_KEY); if (v) return v; } catch (e) {}
  return 'dark';
}
function systemDark() {
  try { return matchMedia('(prefers-color-scheme: dark)').matches; } catch (e) { return true; }
}
const resolve = (choice) => (choice === 'system' ? (systemDark() ? 'dark' : 'light') : choice);

// Estado reactivo compartido (runes). `choice` = lo elegido; `resolved` = lo aplicado.
export const theme = $state({ choice: saved(), resolved: resolve(saved()) });

function apply() {
  theme.resolved = resolve(theme.choice);
  try { document.documentElement.setAttribute('data-theme', theme.resolved); } catch (e) {}
}

export function setTheme(code) {
  theme.choice = code;
  try { localStorage.setItem(STORE_KEY, code); } catch (e) {}
  apply();
}

// Init + seguir al sistema en vivo cuando la elección es "system".
apply();
try {
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (theme.choice === 'system') apply();
  });
} catch (e) {}
