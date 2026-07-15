// Formateo compartido del Panel.

export function bytes(n) {
  if (!n || n < 0) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0, v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`
}

export function num(n) {
  return (n || 0).toLocaleString('es-ES')
}

// Metadatos de presentación por sección del pool (icono/color/etiqueta).
export const SECTION_META = {
  zim:       { label: 'Colecciones ZIM', glyph: 'W', color: 'var(--info)' },
  models:    { label: 'Modelos de traducción', glyph: '文', color: 'var(--magenta)' },
  downloads: { label: 'Descargas / media', glyph: '▼', color: 'var(--orange)' },
  maps:      { label: 'Mapas', glyph: '◈', color: 'var(--signal)' },
  db:        { label: 'Estado (bases de datos)', glyph: '≡', color: 'var(--ink-mute)' },
}
