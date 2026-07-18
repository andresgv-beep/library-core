# Plan de instalación limpia y aprovisionamiento de dependencias

Estado: propuesta de diseño. No implementado todavía.
Objetivo: que una instalación desde cero sea reproducible, que el Panel diga
qué falta y por qué, que las piezas externas se instalen solas desde su fuente
oficial, y que todo sea legalmente limpio al ser este un repositorio público.

## 1. Problema

El sistema depende de piezas externas que **no vienen en el repositorio** y que
**no se documentan ni se detectan de forma útil**. En una instalación limpia el
operador se encuentra con un Panel que dice `available: false` sin explicar qué
falta, de dónde sacarlo, ni cómo instalarlo. Además, varios valores por defecto
son rutas de la máquina Windows donde se compiló (`C:\Users\asus\...`), inútiles
en cualquier otro equipo, y no existe ninguna ruta de instalación fuera de
Windows: los scripts son `build.ps1` / `install-*.ps1` en PowerShell.

Consecuencia: instalar esto es artesanal y depende de conocer el código.

## 2. Inventario de dependencias externas

| Capacidad | Pieza externa | Fuente oficial | Licencia (software) | Licencia (datos) | ARM64 | Detección actual |
|---|---|---|---|---|---|---|
| Mapas: extractor de teselas | `go-pmtiles` (`pmtiles`) | github.com/protomaps/go-pmtiles | BSD-2-Clause | — | Sí, binario | `findMapTool()` junto al binario; si falta → `available:false` sin mensaje |
| Mapas: datos base | planet PMTiles | data.source.coop/protomaps/openstreetmap | — | ODbL (OpenStreetMap) | n/a | URL por defecto (`MAP_SOURCE_URL`) |
| Mapas: geocoder | GeoNames `cities500.zip` | download.geonames.org | — | CC-BY 4.0 | n/a | URL por defecto (`MAP_GEONAMES_URL`) |
| Biblioteca: catálogo ZIM | Kiwix | library.kiwix.org | — | varía por ZIM (p. ej. CC-BY-SA en Wikipedia) | n/a | URL por defecto (`KIWIX_CATALOG_URL`) |
| Traducción | `translateLocally` + modelos NMT | Bergamot / Marian | MIT (motor); modelos varían | — | **No hay binario oficial** | ruta Windows hardcodeada en `translate-wrap` (`TRANSLATE_BIN`) |
| Import de media (si se usa) | `yt-dlp`, `ffmpeg` | proyectos oficiales | Unlicense / LGPL-GPL | — | Sí | vía `*_PATH` en `.env` |

Notas:
- El motor de descargas del Core (el que baja ZIMs al pool) ya existe y es
  reutilizable para bajar herramientas.
- El binario `pmtiles` debe colocarse **junto al ejecutable del Core**
  (`siblingDir`), no en el `PATH`. Esto hoy no está documentado en ningún sitio.

## 3. Principios de diseño

1. **No redistribuir binarios de terceros dentro del repositorio.** Se descargan
   del upstream oficial en el momento de instalar. Esto evita problemas de
   licencia y de tamaño del repositorio.
2. **Todo aprovisionamiento es verificable.** Versión fijada + checksum SHA256
   por plataforma. Una descarga que no cuadre con el checksum se rechaza.
3. **El Panel es la fuente de verdad del estado.** Cada capacidad opcional
   declara si está lista y, si no, qué falta y qué la desbloquea.
4. **Multiplataforma de primera clase.** Linux (incl. ARM64) y Windows como
   ciudadanos iguales. Nada de rutas de máquina concreta en los valores por
   defecto.
5. **Degradación limpia.** Sin una dependencia opcional, el resto del sistema
   funciona; solo se desactiva esa capacidad, y se dice por qué.

## 4. Pilar 1 — Dependency Doctor (el Panel dice qué falta)

Nuevo endpoint administrativo, p. ej. `GET /api/admin/health/deps`, que agrega el
estado que hoy ya está disperso (`maps.available`, `maps.installed`,
`geocoder.installed`, `translate/available`) en una sola respuesta:

```json
{
  "capabilities": [
    {
      "id": "maps",
      "label": "Mapas",
      "ready": false,
      "requires": [
        { "id": "pmtiles", "kind": "tool", "installed": false,
          "reason": "extractor PMTiles no instalado",
          "installable": true },
        { "id": "map-region", "kind": "data", "installed": false,
          "reason": "no hay teselas de ninguna región" }
      ]
    },
    {
      "id": "translate",
      "label": "Traducción",
      "ready": false,
      "requires": [
        { "id": "translateLocally", "kind": "tool", "installed": false,
          "reason": "motor no disponible en esta plataforma",
          "installable": false,
          "note": "sin binario oficial para linux/arm64; usar servidor remoto" }
      ]
    }
  ]
}
```

En el Panel: una sección "Estado del sistema" con una fila por capacidad
(Mapas, Traducción, Media, ...). Verde = lista. Ámbar = falta algo, con el
detalle y, si `installable: true`, un botón **Instalar**.

## 5. Pilar 2 — Manifiesto de dependencias y auto-instalación

### 5.1 Manifiesto

Fichero versionado en el repositorio: `dependencies.json` (o
`library-server/dependencies.json`). Declara cada herramienta descargable, con
URL, checksum y destino por plataforma.

```json
{
  "schema": 1,
  "tools": [
    {
      "id": "pmtiles",
      "unlocks": "maps",
      "license": "BSD-2-Clause",
      "source": "https://github.com/protomaps/go-pmtiles",
      "version": "1.31.1",
      "platforms": {
        "linux/arm64": {
          "url": "https://github.com/protomaps/go-pmtiles/releases/download/v1.31.1/go-pmtiles_1.31.1_Linux_arm64.tar.gz",
          "archive": "tar.gz",
          "extract": "pmtiles",
          "sha256_archive": "2c343014c87dae67e956f47d7cf583b5be8357ab8836722dcc42121f533818d3",
          "sha256_binary": "847cfe3307bc2a12176b775b55a7321e4abf97b6bbc56c5ce7315d3b2510caac",
          "dest": "pmtiles"
        }
      }
    }
  ]
}
```

- `dest` es relativo al directorio del binario del Core (donde `findMapTool`
  busca).
- `sha256_binary` permite verificar tras extraer, no solo el archivo.
- Los valores de `pmtiles` de arriba son reales, tomados de la instalación de
  esta sesión (v1.31.1, Linux arm64).

### 5.2 Instalador

Nuevo endpoint administrativo, p. ej. `POST /api/admin/deps/install`
`{ "id": "pmtiles" }`, que:

1. Resuelve la entrada del manifiesto para `runtime.GOOS/GOARCH`.
2. Descarga a un `.part` (reutilizando el motor de descargas del pool).
3. Verifica `sha256_archive`.
4. Extrae `extract` del archivo, verifica `sha256_binary`.
5. Coloca en `dest` con permiso de ejecución.
6. Revalida (`findMapTool()` u homólogo) y refresca el Doctor.

Sin este endpoint, la alternativa manual (documentada, ver §7) sigue siendo
válida como respaldo.

### 5.3 Fuentes de datos (no binarios)

Las descargas de datos (planet PMTiles por región, GeoNames, ZIMs) **ya tienen
URL por defecto** y flujo en el Panel. Solo hay que asegurarse de que el Doctor
refleje su estado y que las atribuciones se muestren (ver §6).

## 6. Pilar 3 — Cumplimiento legal

Al ser repositorio público, la postura es:

- **Software de terceros:** no se incluye en el repo; se descarga del upstream
  oficial con versión y checksum fijados. `docs/DEPENDENCIAS.md` (a crear) lista
  cada herramienta con nombre, versión, licencia y URL oficial.
- **Datos de terceros y atribución obligatoria:**
  - OpenStreetMap / Protomaps → **ODbL**: atribución "© OpenStreetMap
    contributors" visible en el visor de mapas. Ya que se sirven teselas,
    la atribución debe verse en la interfaz de Mapas.
  - GeoNames → **CC-BY 4.0**: atribución a GeoNames.
  - ZIMs (p. ej. Wikipedia) → **CC-BY-SA** u otras según el ZIM; la atribución
    la porta el propio contenido.
- **THIRD-PARTY-NOTICES:** el build ya regenera
  `nimos-library/public/THIRD-PARTY-NOTICES.txt`. Debe incluir también las
  herramientas aprovisionadas en runtime (pmtiles, y translateLocally/yt-dlp/
  ffmpeg cuando apliquen), no solo las dependencias compiladas.

## 7. Instalación en Linux / ARM (lo que falta hoy)

No existe equivalente no-Windows de `build.ps1` / `install-service.ps1`. Se
propone `scripts/install.sh` (y `scripts/build.sh`) que:

1. Compile las 4 piezas Go: `core`, `supervisor`, `translate-wrap` (opcional),
   y ensamble `www-panel` y `www-client` desde los builds npm.
2. Coloque `www-client`, `www-panel`, `maps-www`, `mapdata` junto al binario.
3. Aprovisione las herramientas del manifiesto para la plataforma actual.
4. Genere un `.env` a partir de `.env.example` con un `POOL_ROOT` real y un
   `NIMOS_SETUP_TOKEN` aleatorio.
5. Instale una unidad **systemd** (equivalente del servicio Windows del
   supervisor) para arranque automático y reinicio ante caída.

### Referencia manual (mientras no exista el instalador)

Compilación por piezas, desde la raíz del repositorio:

```bash
# Panel  -> core/www-panel
( cd library-server/panel && npm install && npm run build )

# Cliente PWA -> core/www-client
( cd nimos-library && npm install && npm run build )
cp -r nimos-library/dist library-server/core/www-client

# Core y Supervisor
( cd library-server/core && go build -o library-server . )
( cd library-server/supervisor && go build -o library-supervisor . )

# Herramienta de mapas (ejemplo linux/arm64, v1.31.1)
#   colocar el binario 'pmtiles' junto a library-server (mismo directorio)
```

Arranque del Core con pool y bind reales:

```bash
POOL_ROOT=/ruta/al/pool ZIM_ENGINE=native PORT=8090 BIND=0.0.0.0 \
  NIMOS_SETUP_TOKEN=<codigo-largo-aleatorio> ./library-server
```

Notas de red y firewall:
- `BIND=0.0.0.0` publica en la LAN; el primer admin desde otro equipo exige
  `NIMOS_SETUP_TOKEN`. Desde la propia máquina (loopback) no hace falta código.
- Si hay `ufw` activo, abrir el puerto solo a la subred local:
  `sudo ufw allow from <subred>/24 to any port 8090 proto tcp`.

## 8. Orden de trabajo propuesto

1. `dependencies.json` con la entrada `pmtiles` (datos ya conocidos) —
   fundación del Pilar 2.
2. Endpoint `POST /api/admin/deps/install` reutilizando el motor de descargas.
3. Endpoint `GET /api/admin/health/deps` (Pilar 1) y sección "Estado del
   sistema" en el Panel.
4. `docs/DEPENDENCIAS.md` + atribución OSM/GeoNames visible en Mapas (Pilar 3).
5. `scripts/install.sh` + unidad systemd (Linux/ARM de primera clase).

Cada paso es independiente y deja el sistema en un estado usable. El paso 1-2 es
el de mayor valor por esfuerzo: convierte "adivina qué falta" en "el Panel te lo
instala".

## 9. Fuera de alcance

- Traducción en ARM64: sigue sin binario oficial de `translateLocally`. La vía
  soportada es un motor en otra máquina x86 vía `TRANSLATE_URL`. Compilar
  Bergamot/Marian para ARM es un proyecto aparte.
- Empaquetado nativo (Wails) para Linux: la app de escritorio es cliente
  Windows; en Linux la experiencia de app es la PWA servida en `/`.
