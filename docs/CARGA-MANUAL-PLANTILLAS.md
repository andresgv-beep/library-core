# Carga manual — plantillas del Panel de Control

Las dos apps del lector se alimentan de aquí: **Moments** (vídeos propios) y
**Cabinet** (archivo documental). Cada formulario publica en la carpeta de su app;
el carril `source` enruta cada item a su superficie.

El Panel ofrece **formularios de carga manual**: el admin sube SU fichero +
metadatos y Library escribe la ficha `<fichero>.json` (sidecar) junto al media.
El buscador (FTS5) y las plantillas de render la recogen automáticamente. Todo
el contenido del pool lo aporta el operador; Library solo lo almacena, indexa y
sirve offline.

## Andamiaje existente que se REUTILIZA
- `sidecar` struct (`library-server/core/sidecar.go`) = la ficha. Es literalmente
  la plantilla del formulario.
- `templateForExt(path)` → deriva `template`/tipo de colección de la EXTENSIÓN del
  fichero: `.mp4/.webm/.m4v/.mov`→`video`, `.mp3/.ogg/.flac/.m4a/.wav`→`audio`,
  `.jpg/.png/.gif/.webp`→`gallery`, `.pdf`→`pdf`, `.epub/.md/.txt`→`reader`.
- `writeJSONFileIfAbsent` (atómico, no pisa ediciones del usuario).
- `collection.json` por carpeta (`collectionMeta`): la carpeta ES la colección.
- Escáner de media (`media.go scan`) + servir `/media` + gate de acceso: intactos.

---

## FORMULARIO 1 — Vídeo → app **Moments**  (template `video`)

| Campo JSON | Etiqueta UI | Control | Req | Notas |
|---|---|---|---|---|
| `media` | Fichero de vídeo | file `.mp4/.webm/.mkv/.m4v/.mov` | ✅ | se guarda en la carpeta; define el `template` |
| `title` | Nombre | text | ✅ | |
| `author` | Canal / autor | text | – | |
| `description` | Descripción | textarea | – | |
| `tags` | Tags | chips (lista) | – | visibles, filtrables |
| `keywords` | *(auto)* | — | — | derivados de `tags` (minúsculas, sin duplicados) |
| `date` | Fecha | date `YYYY-MM-DD` | – | |
| `duration` | Duración (s) | number | – | auto vía ffprobe del fichero, o manual |
| `cover` | Miniatura | file `.jpg/.png` | – | portada de la tarjeta (se guarda `<base>.cover.jpg`) |
| `channel_avatar` | Avatar del canal | file `.jpg` | – | opcional (`channel.jpg` en la carpeta) |
| `subtitles` | Subtítulos | file[] `.vtt` + `lang` | – | opcional |
| `chapters` | Capítulos | lista `{start(s), title}` | – | opcional |
| `source_url` | Enlace de referencia | url | – | procedencia, opcional |
| `source` | *(fijo)* | — | — | `"moments"` (carpeta `Moments/<canal>/`) |
| `template` | *(fijo)* | — | — | `"video"` |

## FORMULARIO 2 — Documento / Archivo → app **Cabinet**  (template por extensión)

Sirve para PDF, EPUB, audio, imágenes:

| Campo JSON | Etiqueta UI | Control | Req | Notas |
|---|---|---|---|---|
| `media` | Archivo | file `.pdf/.epub/.mp3/.jpg/…` | ✅ | define el `template` por extensión |
| `title` | Título | text | ✅ | |
| `author` | Autor | text | – | |
| `date` | Año | text/number | – | |
| `description` | Descripción | textarea | – | |
| `tags` | Categorías / temas | chips | – | |
| `keywords` | *(auto)* | — | — | derivados de `tags` |
| `language` | Idioma | select/text | – | |
| `contributor` | Contribuidor | text | – | p. ej. una biblioteca |
| `license` | Licencia | text/select | – | |
| `cover` | Portada | file `.jpg` | – | |
| `text` | Texto / OCR | file `.txt` | – | opcional; habilita búsqueda dentro del documento |
| `source_url` | Procedencia | url | – | opcional |
| `source` | *(fijo)* | — | — | `"cabinet"` (carpeta `Cabinet/<colección>/`) |

## Común a ambos — la colección (carpeta)

Cada subida va a una carpeta = colección, con su `collection.json`:

| Campo JSON | Etiqueta UI | Notas |
|---|---|---|
| `title` | Nombre de la colección | = nombre de la carpeta |
| `type`/`template` | *(auto)* | del tipo de fichero |
| `description` | Descripción | opcional |
| `source` | *(fijo)* | `"moments"` o `"cabinet"` según la app |

El formulario necesita un selector **"colección"**: elegir una existente o crear
una nueva (carpeta nueva en el pool).

## Audiolibro (avanzado, opcional)

`tracks: [{title, media, waveform?}]` para varias pistas en una ficha. Se deja
para una segunda fase.

---

## Endpoint — ✅ IMPLEMENTADO (verificado 2026-07-18)

`POST /api/admin/upload` (multipart, admin — ruta real; este doc decía antes
`/api/upload`). Registrado en `library-server/core/main.go:345` →
`handleUpload` (`library-server/core/upload.go`). Recibe fichero(s) + campos →
guarda en `pool/<colección>/`, escribe `<fichero>.json` (sidecar) y
`collection.json` si falta. Reusa `templateForExt` + `writeJSONFileIfAbsent`. El
escáner lo recoge y el buscador lo indexa igual que el resto del pool.
