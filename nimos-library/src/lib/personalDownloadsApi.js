// Descarga de cliente: baja al equipo del propio usuario un fichero que Library
// YA sirve (/media/*, /content/*), usando el navegador — como descargar de
// cualquier web. NO adquiere nada nuevo, no crea Items, no usa la cola admin.
// Importar proveedores externos al servidor pertenece exclusivamente al Panel.

// ¿El Item ofrece un fichero descargable?
import { serverUrl } from './connection.js';

export function canDownload(item) {
  return !!(item && (item.capabilities?.download || item.open?.url));
}

// URL del fichero servido por Library para este Item/OpenTarget.
export function downloadUrlFor(item) {
  return item?.open?.url || item?.url || '';
}

// Dispara la descarga del navegador ("guardar como") sobre una URL que Library
// ya sirve. El destino lo decide el navegador del cliente (su carpeta Descargas).
export function downloadFile(url, filename = '') {
  if (!url) return;
  const a = document.createElement('a');
  a.href = serverUrl(url);
  a.download = filename || ''; // sugiere nombre; el Core puede reforzar con Content-Disposition
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
}

// Atajo: descargar el fichero de un Item.
export function downloadItem(item) {
  const url = downloadUrlFor(item);
  if (!url) return;
  const name = item?.open?.title || item?.title || '';
  downloadFile(url, name);
}
