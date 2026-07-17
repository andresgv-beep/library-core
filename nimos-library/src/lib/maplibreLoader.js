import { serverUrl } from './connection.js';

let loading;

export function loadMapLibre() {
  if (typeof window === 'undefined') return Promise.reject(new Error('MapLibre necesita navegador'));
  if (window.maplibregl) return Promise.resolve(window.maplibregl);
  if (loading) return loading;
  loading = new Promise((resolve, reject) => {
    const cssHref = serverUrl('/maps/vendor/maplibre-gl.css');
    if (!document.querySelector(`link[data-nimos-maplibre]`)) {
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = cssHref;
      link.dataset.nimosMaplibre = 'true';
      document.head.appendChild(link);
    }
    const existing = document.querySelector('script[data-nimos-maplibre]');
    if (existing) {
      existing.addEventListener('load', () => resolve(window.maplibregl), { once: true });
      existing.addEventListener('error', () => reject(new Error('No se pudo cargar MapLibre')), { once: true });
      return;
    }
    const script = document.createElement('script');
    script.src = serverUrl('/maps/vendor/maplibre-gl.js');
    script.async = true;
    script.dataset.nimosMaplibre = 'true';
    script.onload = () => window.maplibregl ? resolve(window.maplibregl) : reject(new Error('MapLibre no disponible'));
    script.onerror = () => reject(new Error('No se pudo cargar MapLibre'));
    document.head.appendChild(script);
  });
  return loading;
}
