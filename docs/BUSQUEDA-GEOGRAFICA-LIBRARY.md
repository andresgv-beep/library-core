# Especificación técnica: búsqueda geográfica integrada en Library

## 1. Objetivo

Cuando el usuario busca en la pantalla principal de Library una calle, ciudad, código postal o lugar reconocible por el mapa offline, la vista de resultados debe combinar dos carriles independientes:

1. un resultado geográfico principal con mapa, ubicación y puntos de interés cercanos;
2. los resultados documentales actuales procedentes de ZIM y medios locales.

El resultado geográfico no es una tarjeta ni una ventana de Maps incrustada. Es una superficie sin marco que se funde con `--ground`, mantiene la jerarquía visual del mockup y aparece antes de «De tus colecciones».

La característica es completamente offline y opcional: si no existe un mapa activo, no está instalado el geocodificador o la consulta no es geográfica, la búsqueda documental debe comportarse exactamente como ahora.

## 2. Alcance de la primera versión

### Incluido

- Búsqueda simultánea de documentos y ubicaciones desde `Home.svelte`.
- Reconocimiento de ciudades, calles, códigos postales y lugares admitidos por el geocodificador actual.
- Un resultado geográfico principal por consulta.
- Mapa de contexto no interactivo, sin marco y con fundido hacia el fondo de Library.
- Marcador de la ubicación buscada y marcadores secundarios de POI.
- POI con radio regulable entre 0 y 5 km, en pasos de 500 m.
- Lista visible de hasta 6 POI y respuesta del servidor de hasta 18.
- Tema claro y oscuro.
- Funcionamiento offline con el PMTiles activo.
- Carga progresiva: los resultados documentales no esperan al mapa y viceversa.
- Accesibilidad por teclado y textos traducibles.

### Fuera de alcance inicial

- Navegación paso a paso o cálculo de rutas.
- Indicaciones de tráfico, horarios, reseñas o datos de Internet.
- Múltiples mapas geográficos simultáneos en una misma búsqueda.
- Edición o creación de POI.
- Un nuevo modo o pestaña «Lugares» en el selector `Todo / Imágenes`.
- Mostrar resultados geográficos en el modo `Imágenes`.
- Sustituir la vista completa de Maps.

## 3. Experiencia y jerarquía visual

### Orden de la vista

En modo `Todo`, después del buscador compacto:

1. resultado geográfico, si existe;
2. «Lugares de interés cercanos»;
3. «De tus colecciones» con los resultados actuales.

La cifra general de resultados documentales no debe sumar la ubicación ni los POI. Cada bloque comunica su propio recuento para evitar mezclar entidades incompatibles.

### Resultado geográfico

La composición de escritorio usa dos zonas dentro de una única superficie sin contenedor visible:

- izquierda: tipo, nombre normalizado, contexto y selector de radio;
- derecha: mapa que ocupa el fondo y se desvanece hacia Library.

Reglas visuales:

- sin borde, sombra, fondo de tarjeta ni esquinas que delimiten una caja;
- el fundido se crea desde el contenedor de Library, no dentro de Maps;
- el lado del texto conserva una zona opaca suficiente para garantizar contraste;
- el fundido lateral debe terminar cerca del borde y no lavar el contenido central;
- el mapa mantiene la cartografía clara existente, atenuada en tema oscuro mediante una capa de `--ground`, no cambiando a otro estilo cartográfico;
- el marcador principal usa `--accent`; los POI usan una identidad secundaria estable;
- la atribución de OpenStreetMap/Protomaps permanece visible en formato compacto;
- no se muestran controles de zoom, brújula ni buscador dentro de la previsualización.

En anchos menores de 720 px, el texto ocupa la parte superior y el mapa pasa debajo. El fundido se vuelve vertical. Los POI se apilan en una columna y no se permite desplazamiento horizontal interno.

### Radio

- Control: deslizador continuo de **0 a 5 km**.
- Paso: **500 m**.
- Valor inicial: **2,5 km**.
- A 0 km se mantiene la ubicación y no se consultan ni muestran POI.
- La preferencia vive en el estado de la búsqueda de esa pestaña, no como ajuste global.
- Mientras se arrastra sólo cambian el valor visible y el círculo de alcance del mapa; la petición se lanza al soltar el control o tras 180 ms sin movimiento.
- Cambiar el radio conserva ubicación y resultados documentales; sólo actualiza los POI.
- Durante la actualización se mantienen los POI anteriores con un estado de carga discreto para evitar saltos de composición.

### Selección de POI

- Cada fila muestra nombre, categoría traducida y distancia.
- Se ordenan por distancia ascendente.
- Al seleccionar un POI se destaca su marcador y aparece una línea breve de contexto.
- La primera versión no abandona Library al seleccionar un POI.
- Una acción posterior «Abrir en Maps» puede añadirse en una segunda fase, reutilizando la vista completa existente.

## 4. Detección de una consulta geográfica

No se aplican expresiones regulares en el cliente para decidir si un texto «parece» una dirección. Library consulta el motor geográfico y el servidor decide si existe una coincidencia suficientemente fuerte.

Una coincidencia puede mostrarse cuando:

- el nombre normalizado coincide exactamente con la consulta; o
- nombre más contexto cubren los términos significativos de la consulta; o
- es una calle y la consulta incluye el nombre de vía, con o sin número de portal; o
- es un código postal exacto.

No se muestra el módulo cuando el resultado es únicamente una coincidencia FTS débil. Ejemplos:

- `Madrid` puede mostrar la ciudad;
- `Calle de Alcalá 42 Madrid` puede mostrar la calle con posición aproximada;
- `historia de Madrid` no debe apropiarse del resultado geográfico si el conjunto de términos no identifica una ubicación;
- una consulta sin resultado geográfico continúa sólo con ZIM y medios.

El servidor devuelve `matchQuality` con uno de estos valores:

- `exact`: coincidencia directa del nombre o código postal;
- `strong`: nombre y contexto cubren la consulta;
- `weak`: coincidencia FTS insuficiente para el módulo integrado.

Library sólo representa `exact` y `strong`. `weak` puede conservarse para diagnóstico y futuras sugerencias, pero no llega a la interfaz principal.

## 5. Contrato HTTP propuesto

### Endpoint agregado

```text
GET /api/maps/search?q=<consulta>&radius=2500
```

`radius` acepta enteros entre `0` y `5000`, ambos incluidos, en pasos de `500`. Si falta, se usa `2500`. Valores fuera de rango o que no respeten el paso devuelven `400`.

Respuesta con resultado:

```json
{
  "available": true,
  "reason": "",
  "query": "Calle de Alcalá, Madrid",
  "radius": 2500,
  "location": {
    "name": "Calle de Alcalá",
    "kind": "street",
    "lat": 40.4196,
    "lon": -3.6920,
    "context": "Madrid",
    "houseNumber": "",
    "approximate": false,
    "matchQuality": "strong"
  },
  "alternatives": [],
  "pois": [
    {
      "name": "Museo del Prado",
      "kind": "museum",
      "category": "Cultura y ocio",
      "categoryCode": "culture",
      "lat": 40.4138,
      "lon": -3.6921,
      "distance": 1200
    }
  ],
  "map": {
    "name": "España",
    "file": "spain.pmtiles",
    "bbox": [-10.2, 35.5, 4.8, 44.3],
    "maxZoom": 14,
    "style": "/maps/style-light.json",
    "tiles": "/api/maps/tiles/spain.pmtiles/{z}/{x}/{y}.mvt"
  }
}
```

Respuesta sin módulo representable:

```json
{
  "available": false,
  "reason": "no_match",
  "query": "historia de Madrid",
  "radius": 2500,
  "location": null,
  "alternatives": [],
  "pois": [],
  "map": null
}
```

`reason` admite:

- `no_map`: no hay PMTiles activo;
- `no_geocoder`: no existe índice geográfico;
- `outside_map`: la coincidencia cae fuera del mapa activo;
- `no_match`: no existe coincidencia fuerte;
- `map_incompatible`: el mapa no contiene teselas vectoriales compatibles.

Estas situaciones normales devuelven `200`. Los errores de parámetros devuelven `400`; fallos internos inesperados, `500`. El cliente no enseña mensajes de error geográfico en la vista general: degrada silenciosamente a resultados documentales y registra el fallo en consola para diagnóstico.

### Compatibilidad

Se conservan sin cambios:

- `/api/maps/config`;
- `/api/maps/geocode`;
- `/api/maps/nearby`;
- `/api/maps/tiles/...`;
- `/maps/`.

El endpoint agregado reutiliza la lógica interna de los dos primeros servicios, pero no hace llamadas HTTP contra el propio servidor.

`nearbyHit` añade `categoryCode` y mantiene temporalmente `category` para no romper la interfaz actual de Maps. El código estable se traduce en el cliente; la etiqueta española existente queda como compatibilidad hasta migrar Maps a i18n.

## 6. Cambios en Core

### Refactor de geocodificación

Extraer de `handleGeocode` una función reutilizable sin dependencias HTTP:

```go
func (s *Server) searchGeo(query, mapFile string, bbox *[4]float64) []GeoHit
```

El handler actual valida parámetros, llama a esta función y mantiene su respuesta actual. La función agregada calcula además `matchQuality` para elegir el primer resultado representable.

### Refactor de POI

Extraer de `handleNearby` una operación cancelable:

```go
func (m *mapManager) nearby(ctx context.Context, lat, lon float64, mapFile string, radius int) ([]nearbyHit, error)
```

Requisitos:

- validar un radio entre 0 y 5 km, en pasos de 500 m;
- devolver inmediatamente una lista vacía cuando el radio sea 0;
- calcular la ventana de teselas a partir del radio y la latitud;
- filtrar finalmente con distancia Haversine, no sólo por tesela;
- comprobar `ctx.Done()` entre lecturas de tesela;
- deduplicar como ahora;
- ordenar por distancia;
- limitar la salida a 18 POI;
- impedir que una consulta explore una cantidad no acotada de teselas.

La estrategia inicial debe seleccionar el zoom más alto que mantenga la ventana bajo un máximo de 121 teselas. Si una región no expone suficientes POI a ese zoom, se documentará el resultado antes de introducir un índice espacial adicional. No se construirá un nuevo índice SQLite de POI en la primera iteración sin medir primero esta solución.

### Caché

Core incorpora una caché LRU corta para POI con clave:

```text
mapFile | lat redondeada a 4 decimales | lon redondeada a 4 decimales | radius
```

- TTL recomendado: 10 minutos.
- Capacidad inicial: 128 entradas.
- Se invalida naturalmente al cambiar `mapFile`; no necesita persistencia.
- No se comparten resultados entre mapas distintos.

La geocodificación puede reutilizar la caché de búsqueda existente o una caché pequeña específica, siempre incluyendo mapa y `bbox` en la clave.

### Registro de ruta

En `main.go`, después de crear `mapAdmin`:

```go
mux.HandleFunc("/api/maps/search", s.handleMapSearch(mapAdmin))
```

La ruta es pública como las demás consultas de Maps y no modifica estado.

## 7. Cambios en el cliente Svelte

### Estado por pestaña

`emptySearch()` incorpora:

```js
location: {
  status: 'idle',
  result: null,
  radius: 2500,
  selectedPoi: null
}
```

Estados admitidos:

- `idle`: caja vacía o modo Imágenes;
- `loading`: consulta geográfica en curso;
- `ready`: hay ubicación representable;
- `empty`: no hay coincidencia;
- `unavailable`: falta mapa o geocodificador;
- `error`: fallo inesperado, no visible como error principal.

El resultado completo vive en `result`; no se reparten `location`, `pois` y `map` como propiedades paralelas del tab.

### Cliente API

En `libraryApi.js`:

```js
export async function mapSearch(q, radius = 2500, { signal } = {})
```

La función devuelve el objeto completo del endpoint. Un `available:false` no lanza excepción.

### Orquestación de la búsqueda

`Home.svelte` mantiene controladores separados para documentos y ubicación. En modo `all`, cada consulta:

1. cancela ambos trabajos anteriores;
2. limpia inmediatamente el resultado geográfico de la consulta anterior;
3. inicia `itemSearch` y `mapSearch` en paralelo;
4. publica cada resultado tan pronto como termina;
5. descarta respuestas cuyo término, modo o identificador de petición ya no coincidan;
6. conserva el historial reciente actual;
7. no lanza búsqueda geográfica en modo `images`.

No se debe envolver ambos trabajos en un único `Promise.all`, porque un fallo geográfico no puede descartar los documentos y una búsqueda ZIM lenta no debe retrasar el mapa.

La navegación mediante barra de direcciones (`navigateAddress`) debe invocar el mismo orquestador que `Home`, en lugar de mantener una segunda implementación que sólo llame a `itemSearch`.

### Componentes nuevos

```text
nimos-library/src/lib/LocationSearchResult.svelte
nimos-library/src/lib/MiniMap.svelte
nimos-library/src/lib/maplibreLoader.js
```

Responsabilidades:

- `LocationSearchResult`: composición, radio, lista de POI, selección y estados accesibles;
- `MiniMap`: ciclo de vida de MapLibre, fuente vectorial, cámara y marcadores;
- `maplibreLoader`: promesa singleton que carga una sola vez los recursos vendorizados.

No se añade inicialmente `maplibre-gl` al paquete npm ni se duplica dentro del bundle de Library. `maplibreLoader.js` reutiliza:

```text
/maps/vendor/maplibre-gl.js
/maps/vendor/maplibre-gl.css
```

La previsualización consume las teselas HTTP de Core y no necesita `pmtiles.js`, porque `/api/maps/tiles/...` ya entrega MVT.

### Ciclo de vida de MiniMap

- Crear el mapa en `onMount` cuando existen contenedor y configuración.
- Desactivar interacción, teclado, zoom, rotación y controles.
- Usar `style-light.json`, glifos y sprites locales.
- Ajustar cámara a la ubicación y al radio seleccionado, respetando el zoom máximo disponible.
- Representar ubicación y POI como fuentes/capas GeoJSON para evitar múltiples nodos DOM.
- Al cambiar ubicación, radio o POI seleccionado, actualizar fuentes y cámara sin recrear MapLibre.
- Ejecutar `map.remove()` al desmontar el componente.
- Si WebGL falla, ocultar sólo el lienzo: nombre, contexto y lista de POI continúan visibles.

## 8. Rendimiento y límites

Objetivos medidos en el equipo de referencia con datos locales calientes:

- el resultado documental conserva su tiempo actual;
- geocodificación: objetivo menor de 150 ms;
- POI a 2,5 km: objetivo menor de 250 ms;
- POI a 5 km: objetivo menor de 450 ms;
- primera previsualización del mapa: menor de 1 s después de recibir la respuesta;
- cambio de radio con caché: menor de 150 ms;
- ninguna búsqueda lee más de 121 teselas;
- máximo 18 POI en JSON y 6 visibles inicialmente.

Si el objetivo de 5 km no se cumple con lectura de teselas, la siguiente decisión será crear un índice lateral de POI durante la indexación del mapa. No se amplía silenciosamente el límite de teselas.

## 9. Accesibilidad e i18n

- El mapa tiene un nombre accesible que resume ubicación y número de POI; no se intenta describir cada calle del lienzo.
- La información esencial existe fuera del mapa.
- El radio usa un `input type="range"` con etiqueta visible, valor actualizado y salida accesible en kilómetros.
- Los POI son botones con nombre, categoría y distancia accesibles.
- La selección no depende sólo del color.
- Se respeta `prefers-reduced-motion`; la cámara salta sin animación en ese caso.
- Todos los textos se añaden a `messages.js` en español e inglés.
- `categoryCode` se traduce en cliente; no se usa `kind` crudo como etiqueta de usuario.

## 10. Seguridad y privacidad

- No se usa geolocalización del dispositivo.
- No se realizan peticiones externas.
- La consulta sólo llega al servidor local ya usado por Library.
- `mapFile` siempre se obtiene del estado activo del servidor o se valida con `filepath.Base`.
- Radio, coordenadas y límites se validan antes de abrir el PMTiles.
- La función cancelable evita continuar leyendo el mapa después de que el usuario cambie la consulta.

## 11. Pruebas

### Core

- `map search` devuelve ciudad exacta con configuración de mapa.
- Una calle con número conserva `approximate` y `houseNumber`.
- Una consulta temática como `historia de Madrid` no supera el umbral.
- Sin mapa, sin geocodificador y fuera de `bbox` devuelven cada `reason` correcto.
- Radios fuera de 0–5 km o que no sean múltiplos de 500 m devuelven `400`.
- Radio 0 devuelve ubicación y ningún POI sin leer teselas.
- El filtro Haversine excluye POI fuera del radio.
- La salida está ordenada, deduplicada y limitada.
- La cancelación detiene el recorrido de teselas.
- `categoryCode` es estable y `category` sigue presente.
- Los endpoints antiguos conservan sus contratos.

### Cliente

- Documentos y ubicación pueden terminar en cualquier orden.
- Un error de Maps no elimina resultados documentales.
- Cambiar rápido de consulta no deja un mapa anterior.
- Limpiar la caja aborta ambas solicitudes y vuelve a Inicio.
- Imágenes no dispara `mapSearch`.
- Cambiar radio no vuelve a buscar ZIM.
- El estado sobrevive al historial de la pestaña como el resto de la búsqueda.
- WebGL no disponible conserva texto y POI.
- Diseño verificado a 320, 720 y 1280 px en ambos temas.

### Integración nativa

La validación final sigue `COMPILACION-NATIVA.md`:

1. pruebas Go y build del cliente;
2. compilación completa `all-in-one`;
3. instalación mediante `install-all-in-one.ps1` con UAC;
4. hashes coincidentes entre fuente e instalación;
5. servicio saludable;
6. aplicación reabierta;
7. comprobación del resultado en la aplicación instalada.

## 12. Fases de implementación

### Fase A — Core reutilizable

- Extraer geocodificación y POI de sus handlers.
- Añadir radio, límite de teselas, cancelación y caché.
- Crear `/api/maps/search` y sus pruebas.

### Fase B — Estado y carga paralela

- Añadir `mapSearch` al cliente API.
- Extender el estado por pestaña.
- Unificar búsqueda de Home y navegación por dirección.
- Verificar que la búsqueda existente no regresa.

### Fase C — Interfaz integrada

- Implementar `LocationSearchResult` y `MiniMap`.
- Reproducir el mockup sin card, con fundido y comportamiento responsive.
- Añadir i18n, accesibilidad y estados de degradación.

### Fase D — Entrega nativa

- Ejecutar pruebas y revisión visual.
- Compilar, instalar y verificar siguiendo la memoria de compilación nativa.

## 13. Criterios de aceptación

La funcionalidad está terminada cuando:

- `Madrid` muestra ubicación, mapa y POI junto a resultados documentales;
- una calle con contexto suficiente centra correctamente la previsualización;
- 2,5 km es el valor inicial y el rango 0–5 km actualiza sólo los POI;
- el mapa se funde con Library sin borde, sombra ni apariencia de iframe;
- el contenido documental aparece aunque Maps falle o no esté instalado;
- no aparecen módulos geográficos por coincidencias débiles;
- no se realizan conexiones a Internet;
- tiempos, límites y pruebas de esta especificación se cumplen;
- la aplicación instalada contiene y muestra el cambio.
