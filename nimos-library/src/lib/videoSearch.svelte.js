// Búsqueda de la app "Vídeos" (Moments). Store compartido para que la cabecera
// reutilizable (MomentsHeader) filtre en la superficie de Vídeos y, al buscar desde
// la ficha de un vídeo, arrastre la consulta al volver a la cuadrícula.
export const videoSearch = $state({ q: '' });
