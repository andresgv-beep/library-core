// shell.svelte.js — detección del shell nativo (Wails) y controles de ventana.
//
// La misma SPA corre en navegador y dentro de la app de escritorio. Cuando Wails
// inyecta su runtime, window.runtime existe → mostramos barra arrastrable y los
// botones min/max/cerrar. En navegador normal, shell.desktop = false y nada de
// esto aparece.
export const shell = $state({ desktop: false });

export function initShell() {
  if (typeof window !== 'undefined' && window.runtime && typeof window.runtime.WindowMinimise === 'function') {
    shell.desktop = true;
  }
}

export const win = {
  minimise() { window.runtime?.WindowMinimise?.(); },
  toggleMaximise() { window.runtime?.WindowToggleMaximise?.(); },
  close() { window.runtime?.Quit?.(); },
};
