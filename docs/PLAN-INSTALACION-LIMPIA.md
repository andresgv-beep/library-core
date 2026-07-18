# Plan de instalación limpia y aprovisionamiento de dependencias

Estado: propuesta de diseño (v2, reescrita 2026-07-18). No implementado todavía.

Objetivo: que **instalar sea trivial para un usuario normal** (descargar, doble
clic, funciona) y que las piezas externas opcionales (mapas, traducción) se
instalen **bajo demanda con un botón**, descargándolas del autor oficial y
verificándolas — sin que el usuario tenga que saber qué falta ni de dónde sacarlo.

## 0. La filosofía (lo que lo hace viable)

Dos planos, y ninguno carga complejidad sobre el usuario:

1. **Instalación base = doble clic, defaults, cero preguntas.** Ruta y pool
   predeterminados; nada que elegir salvo en un "Avanzado" escondido. La app
   arranca y **funciona** sin ningún motor opcional. Nada bloquea.

2. **Motores opcionales = un botón en el Panel.** Mapas y traducción **no** van
   dentro del instalador (por licencia y tamaño). Cuando el Panel detecta que
   falta un motor, muestra una tarjeta con su origen, licencia y versión, y un
   botón **Instalar** que:
   1. descarga el binario del **GitHub/upstream oficial** del proyecto,
   2. verifica el **SHA256**,
   3. lo coloca donde el Core lo busca,
   4. lo marca como disponible.

**Esto no es una invención ni complejidad de ingeniero: es el patrón estándar.**
Lo hacen Winget, Scoop, Chocolatey, Homebrew, VSCode (extensiones), Steam
(runtimes), Docker Desktop (componentes), Kiwix, Lutris y Heroic. Ninguno
redistribuye el binario del tercero: **automatizan la descarga desde el autor**.
Es limpio legalmente y es **un clic** para el usuario.

```
✗ PMTiles no instalado

Fuente:     github.com/protomaps/go-pmtiles
Licencia:   BSD-2-Clause
Versión:    1.31.1

[ Instalar ]
```

**Degradación limpia:** sin un motor opcional, el resto del sistema funciona; solo
se desactiva esa capacidad, y la tarjeta dice exactamente qué falta y cómo
desbloquearla. Nunca un `available: false` mudo.

## 1. Estado real hoy (verificado contra el código)

Lo que de verdad está roto o falta para una instalación limpia:

- **No hay instalación fuera de Windows.** Todo es PowerShell (`build.ps1`,
  `install-*.ps1`). No existe `scripts/install.sh` ni unidad systemd. En Linux/ARM
  instalar es 100% manual y exige conocer el código.
- **Una ruta de máquina hardcodeada** (la única): `library-server/translate-wrap/
  main.go:415` usa por defecto `C:\Users\asus\...\translateLocally.exe`. Sin
  `TRANSLATE_BIN` en el entorno, la traducción local apunta a una ruta que solo
  existe en la máquina donde se compiló. **Bug prerequisito** (ver §7).
- **`pmtiles` debe ir junto al binario del Core** (`siblingDir`), no en el `PATH`,
  y eso no está documentado en ningún sitio.
- **No hay una superficie única que diga "qué falta y cómo arreglarlo".** El estado
  está disperso (`maps.available`, `translate/available`) sin agregación.

Corrección respecto a versiones anteriores de este doc: **NO** hay "varios valores
por defecto que sean rutas de la máquina Windows". Es **solo uno** (el de arriba).
El pool usa `POOL_ROOT` con default `./data` (`.env.example`) y `resolvePoolPath`
con fallbacks sanos (`storage.go`); las URLs de mapas/geo/kiwix ya tienen default y
flujo en el Panel. El código está en mejor forma de lo que se decía.

## 2. Inventario de dependencias externas

| Capacidad | Pieza externa | Fuente oficial | Licencia (software) | Licencia (datos) | ARM64 | Cómo se instala |
|---|---|---|---|---|---|---|
| Mapas: extractor de teselas | `go-pmtiles` (`pmtiles`) | github.com/protomaps/go-pmtiles | BSD-2-Clause | — | Sí, binario oficial | Botón [Instalar] (descarga + SHA256) |
| Mapas: datos base | planet PMTiles por región | data.source.coop/protomaps/openstreetmap | — | ODbL (OpenStreetMap) | n/a | Descarga en el Panel (ya existe) |
| Mapas: geocoder | GeoNames `cities500.zip` | download.geonames.org | — | CC-BY 4.0 | n/a | Descarga en el Panel (ya existe) |
| Biblioteca: catálogo ZIM | Kiwix | library.kiwix.org | — | varía por ZIM | n/a | Descarga en el Panel (ya existe) |
| Traducción | `translateLocally` + modelos NMT | Bergamot / Marian | MIT (motor); modelos varían | — | **Sin binario oficial linux/arm64** (ver §6) | Botón [Instalar] en x86; remoto en ARM |
| Import de media (opcional) | `yt-dlp`, `ffmpeg` | proyectos oficiales | Unlicense / LGPL-GPL | — | Sí | Botón [Instalar] (futuro) o `*_PATH` |

Notas:
- El **motor de descargas del Core ya existe** (resume, `.part` + rename atómico,
  SQLite, semáforo) y es reutilizable para bajar herramientas. No hay que escribir
  transporte nuevo.
- El binario `pmtiles` se coloca **junto al ejecutable del Core** (`siblingDir`),
  donde `findMapTool()` lo busca.

## 3. Principios

1. **No redistribuir binarios de terceros en el repositorio.** Se descargan del
   upstream oficial al pulsar Instalar, con versión y checksum fijados. Igual que
   Winget/Scoop/VSCode.
2. **Todo aprovisionamiento es verificable.** Versión fijada + SHA256 por
   plataforma. Una descarga que no cuadre se rechaza.
3. **El Panel es la fuente de verdad del estado.** Cada capacidad opcional declara
   si está lista y, si no, qué falta, de dónde sale, su licencia y qué la desbloquea.
4. **Instalación base sin fricción.** Defaults sensatos, sin preguntas. Los motores
   nunca son un requisito de arranque.
5. **Degradación limpia.** Sin una dependencia opcional, el resto funciona.

## 4. Pilar 1 — Dependency Doctor (el Panel dice qué falta)

Endpoint administrativo `GET /api/admin/health/deps` que agrega el estado hoy
disperso (`maps.available`, `maps.installed`, `geocoder.installed`,
`translate/available`) en una sola respuesta, con lo que necesita la tarjeta:

```json
{
  "capabilities": [
    {
      "id": "maps",
      "label": "Mapas",
      "ready": false,
      "requires": [
        { "id": "pmtiles", "kind": "tool", "installed": false,
          "installable": true,
          "source": "github.com/protomaps/go-pmtiles",
          "license": "BSD-2-Clause",
          "version": "1.31.1",
          "reason": "extractor PMTiles no instalado" },
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
          "installable": false,
          "reason": "sin binario oficial para esta plataforma (linux/arm64)",
          "note": "usar un motor remoto (TRANSLATE_URL)" }
      ]
    }
  ]
}
```

En el Panel: sección "Estado del sistema" con una fila/tarjeta por capacidad.
Verde = lista. Ámbar = falta algo, con el detalle y, si `installable: true`, el
botón **Instalar** (mockup en §0).

## 5. Pilar 2 — Manifiesto y auto-instalación

### 5.1 Manifiesto

Fichero versionado en el repo: `dependencies.json`. Declara cada herramienta
descargable, con URL, checksum y destino por plataforma.

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
- `sha256_binary` verifica tras extraer, no solo el archivo.
- Los valores de `pmtiles` son reales (v1.31.1, Linux arm64). Al añadir Windows y
  Linux x86 se completan sus entradas con sus checksums.

### 5.2 Instalador

Endpoint administrativo `POST /api/admin/deps/install` `{ "id": "pmtiles" }`:

1. Resuelve la entrada del manifiesto para `runtime.GOOS/GOARCH`.
2. Descarga a un `.part` (**reutilizando el motor de descargas del pool**).
3. Verifica `sha256_archive`.
4. Extrae `extract`, verifica `sha256_binary`.
5. Coloca en `dest` con permiso de ejecución.
6. Revalida (`findMapTool()` u homólogo) y refresca el Doctor.

La alternativa manual (§6) sigue siendo respaldo válido.

### 5.3 Fuentes de datos (no binarios)

Las descargas de datos (planet PMTiles por región, GeoNames, ZIMs) **ya tienen URL
por defecto y flujo en el Panel**. Solo hay que asegurar que el Doctor refleje su
estado y que las atribuciones se muestren (§7).

## 6. Instalación base por sistema (fina, sin fricción)

El instalador base **solo** hace: copiar binarios, colocar `www-client`,
`www-panel`, `maps-www`; registrar el servicio; arrancar. **No** instala motores
(eso es el botón). Defaults sin preguntas.

- **Windows:** el `install-all-in-one.ps1` actual ya cubre el arranque (servicio +
  interfaces) con defaults. Un `.msi` (WiX/Inno) más pulido es mejora posterior, no
  requisito.
- **Linux (lo que falta hoy):** `scripts/install.sh` + `scripts/build.sh` que
  compilen las piezas Go, ensamblen `www-*`, y registren una unidad **systemd**
  (equivalente del servicio Windows). Ruta fija FHS (`/opt/nimos-library`), pool
  configurable (`/var/lib/nimos-library` por env del systemd). Un `.deb` es la
  evolución natural; el `install.sh` es el mínimo viable.

### Caso especial honesto: traducción en ARM

`translateLocally` publica binarios oficiales para Windows, macOS (incl. Apple
Silicon) y Linux x86_64, pero **no para Linux-ARM64** (la Pi). El motor por debajo
(Bergamot/Marian) **sí corre en ARM** — Firefox traduce en móviles ARM con esa
tecnología — pero eso es **compilarlo**, no descargarlo. Por tanto:

- En Windows y Linux x86: el botón [Instalar] funciona.
- En Linux-ARM64: el Doctor marca `installable: false` con la nota "usar motor
  remoto (`TRANSLATE_URL`)". Compilar Bergamot/Marian para ARM64 y empaquetarlo es
  un proyecto aparte (§9, fuera de alcance de este plan).

### Referencia manual (mientras no exista `install.sh`)

```bash
# Panel  -> core/www-panel ; Cliente PWA -> core/www-client
( cd library-server/panel && npm install && npm run build )
( cd nimos-library && npm install && npm run build ) && cp -r nimos-library/dist library-server/core/www-client

# Core y Supervisor
( cd library-server/core && go build -o library-server . )
( cd library-server/supervisor && go build -o library-supervisor . )

# 'pmtiles' junto a library-server (mismo directorio) — o instalarlo desde el Panel
```

Arranque con pool y bind reales:

```bash
POOL_ROOT=/ruta/al/pool ZIM_ENGINE=native PORT=8090 BIND=0.0.0.0 \
  NIMOS_SETUP_TOKEN=<codigo-largo-aleatorio> ./library-server
```

- `BIND=0.0.0.0` publica en la LAN; el primer admin desde otro equipo exige
  `NIMOS_SETUP_TOKEN`. Desde loopback no hace falta.
- Con `ufw`: `sudo ufw allow from <subred>/24 to any port 8090 proto tcp`.

## 7. Cumplimiento legal (repo público)

- **Software de terceros:** no se incluye en el repo; se descarga del upstream
  oficial con versión y checksum fijados (patrón Winget/Scoop). `docs/DEPENDENCIAS.md`
  (a crear) lista cada herramienta con nombre, versión, licencia y URL oficial.
- **Datos de terceros y atribución obligatoria:**
  - OpenStreetMap / Protomaps → **ODbL**: "© OpenStreetMap contributors" visible en
    el visor de Mapas.
  - GeoNames → **CC-BY 4.0**: atribución a GeoNames.
  - ZIMs → **CC-BY-SA** u otras según el ZIM; la porta el propio contenido.
- **THIRD-PARTY-NOTICES:** el build ya regenera `nimos-library/public/
  THIRD-PARTY-NOTICES.txt`; debe incluir también las herramientas aprovisionadas en
  runtime (pmtiles, y translateLocally/yt-dlp/ffmpeg cuando apliquen).

## 8. Prerequisito: arreglar el `TRANSLATE_BIN` hardcodeado

Antes de nada, `library-server/translate-wrap/main.go:415` no debe usar por defecto
una ruta de máquina concreta. Default a búsqueda **junto al ejecutable**
(`siblingDir`, como `pmtiles`) o vacío, para que el botón [Instalar] coloque el
binario ahí y el motor lo encuentre en cualquier equipo.

## 9. Orden de trabajo

```
1. Arreglar TRANSLATE_BIN (1 línea)                     — prerequisito, cero riesgo
2. dependencies.json (entrada pmtiles, datos reales)    — fundación del Pilar 2
3. POST /api/admin/deps/install (reusa el download mgr) — el botón funciona
4. GET /api/admin/health/deps + tarjeta en el Panel     — el Panel dice qué falta
5. docs/DEPENDENCIAS.md + atribución OSM/GeoNames en Mapas
6. scripts/install.sh + unidad systemd                  — Linux de primera clase
```

Cada paso deja el sistema usable. **Los pasos 2-3-4 son el mayor valor por
esfuerzo**: convierten "adivina qué falta y búscalo" en "un botón te lo instala",
que es exactamente lo que hacen los gestores maduros.

## 10. Fuera de alcance

- **Compilar `translateLocally`/Bergamot para ARM64** (la Pi): proyecto aparte. La
  vía soportada en ARM es motor remoto vía `TRANSLATE_URL`.
- **`.msi`/`.deb` pulidos con asistente gráfico y elección de ruta:** mejora
  posterior. El `install-all-in-one.ps1` (Windows) y `install.sh`+systemd (Linux)
  cubren el arranque sin fricción; el botón del Panel cubre los motores.
- **Empaquetado nativo (Wails) para Linux:** en Linux la experiencia de app es la
  PWA servida en `/`.
```
