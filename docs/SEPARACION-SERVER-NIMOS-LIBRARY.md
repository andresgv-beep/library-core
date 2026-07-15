# Separación de Library Server y Nimos Library

**Estado:** propuesta para estudio, sin implementación  
**Fecha:** 2026-07-14  
**Objetivo:** separar la instalación del servidor y el panel de control de la aplicación cliente Nimos Library.

## 1. Decisión fundamental

Library Server es el propietario del contenido y de toda la lógica operativa.

Nimos Library es únicamente un navegador cliente. No administra archivos, no accede al sistema de archivos del equipo cliente y no ejecuta un motor ZIM local.

```text
Library Server gestiona y sirve.
Nimos Library consulta y visualiza.
Panel de Control administra Library Server.
```

Cuando este documento habla de contenido local, significa contenido almacenado localmente en la máquina de Library Server, no en el equipo donde se ejecuta Nimos Library.

## 2. Arquitectura objetivo

```text
MÁQUINA SERVIDOR
Library Server
├── Library Core y API
├── Panel de Control
├── usuarios y permisos
├── almacenamiento y colecciones
├── ZIM
├── vídeos, audio, PDF, EPUB y otros archivos
├── indexación y búsqueda
├── traducción y mapas offline
├── importadores y proveedores
├── descargas administrativas
└── streaming de contenido

RED LOCAL O CONEXIÓN REMOTA
             │
             ▼

EQUIPO CLIENTE
Nimos Library
├── interfaz de navegación
├── búsqueda y descubrimiento
├── lector de artículos ZIM
├── visor de PDF, EPUB, imágenes y documentos
├── reproductor de vídeo y audio
├── favoritos, notas, historial y etiquetas personales
└── conexión a Library Server
```

Nimos Library nunca debe:

- elegir carpetas de almacenamiento del servidor;
- registrar o eliminar ZIM;
- importar contenido al pool (catálogo ZIM o carga manual del admin);
- administrar usuarios, permisos o colecciones;
- controlar la cola de descargas del servidor;
- acceder a rutas físicas del servidor;
- abrir o indexar carpetas del equipo cliente;
- ejecutar un segundo Library Core local.

## 3. Responsabilidades

### Library Server

Library Server contiene:

- Library Core;
- API de consumo;
- API administrativa;
- autenticación y autorización;
- motor ZIM;
- búsqueda e índices;
- almacenamiento de contenido;
- servicio de archivos y streaming con soporte de Range;
- persistencia de estado personal;
- traducción, mapas y capacidades offline;
- proveedores, importadores y descargas;
- Panel de Control como interfaz administrativa.

El servidor decide qué contenido existe, quién puede verlo y cómo debe abrirse.

### Panel de Control

El Panel de Control forma parte del paquete de Library Server.

Sus funciones son:

- configurar almacenamiento;
- instalar, registrar y retirar colecciones ZIM;
- importar contenido;
- administrar proveedores;
- gestionar descargas;
- crear usuarios y asignar permisos;
- instalar modelos de traducción;
- consultar salud, capacidad y estado del servidor.

El Panel no debe convertirse en lector de uso cotidiano.

### Nimos Library

Nimos Library presenta exclusivamente operaciones de consumo:

- conectarse a un Library Server;
- autenticarse como usuario;
- listar colecciones publicadas;
- buscar contenido;
- abrir Items;
- leer artículos ZIM;
- visualizar documentos;
- reproducir vídeo y audio;
- navegar mapas publicados;
- traducir contenido mediante el servidor;
- guardar favoritos, notas, historial y etiquetas personales;
- descargar al equipo cliente un archivo que el servidor ya haya publicado, cuando el Item lo permita.

Las preferencias personales no son administración del servidor. Forman parte de la experiencia del lector y pueden persistirse mediante la API del usuario.

## 4. Estado actual del proyecto

El árbol actual ya contiene tres superficies diferenciadas:

```text
nimos-library/
├── shim/     Library Core actual
├── panel/    Panel de Control Svelte
└── ui/       Lector Nimos Library Svelte
```

La separación conceptual está avanzada:

- `panel/` y `ui/` son proyectos frontend distintos;
- el Panel tiene su propia build;
- las rutas administrativas se registran en `adminMux`;
- `requireAdmin` protege la administración;
- el lector consume contratos comunes como `Collection`, `Item`, `SearchResult`, `Preview` y `OpenTarget`;
- la navegación principal del lector ya no enlaza herramientas de importación ni la cola administrativa.

La separación física aún no está completada:

- `shim/main.go` sirve el Panel bajo `/panel/`;
- el mismo proceso sirve Nimos Library bajo `/`;
- `ui/vite.config.js` compila el lector dentro de `shim/www`;
- las llamadas del lector utilizan rutas relativas como `/api`, `/content`, `/media`, `/maps` y `/mapdata`;
- el arranque de desarrollo presenta `library-shim` como Core, UI y Panel en una sola aplicación.

Por tanto, actualmente las interfaces están separadas, pero el lector sigue empaquetado y desplegado junto al servidor.

## 5. Modelo de distribución objetivo

### Paquete Library Server

Debe incluir:

```text
library-server/
├── library-server.exe
├── panel/
├── motores y herramientas requeridas
├── configuración
└── datos administrados por el servidor
```

Debe exponer:

```text
/panel/*        Panel de Control
/api/*          API de consumo y administración
/content/*      contenido ZIM
/media/*        archivos publicados
/maps/*         aplicación de mapas
/mapdata/*      datos de mapas
```

No necesita incluir la build de Nimos Library.

### Paquete Nimos Library

Debe incluir:

```text
nimos-library/
├── aplicación cliente
├── configuración de conexión
└── recursos visuales del lector
```

Debe conocer como mínimo:

```text
serverBaseUrl = https://library.ejemplo.local
```

Todas las peticiones y URLs de contenido deben resolverse a partir de esa dirección.

Nimos Library puede distribuirse como aplicación instalada o como cliente web independiente. Esa decisión de empaquetado no cambia el contrato: en ambos casos seguirá siendo un cliente del servidor y no llevará un motor de contenido propio.

## 6. Cambios técnicos necesarios

### 6.1 Dirección configurable del servidor

El lector utiliza actualmente rutas relativas. Debe existir una única capa responsable de construir URLs.

Ejemplo conceptual:

```text
apiUrl("/api/collections")
contentUrl("/content/wikipedia_es/A/Saturno")
mediaUrl("/media/videos/documental.mp4")
```

La dirección del servidor no debe concatenarse manualmente en cada componente.

### 6.2 Cliente HTTP común

Conviene centralizar:

- URL base;
- cookies o credenciales;
- cabeceras de autenticación;
- tratamiento de errores;
- servidor no disponible;
- expiración de sesión;
- cancelación de peticiones;
- normalización de URLs devueltas por el Core.

Los módulos `libraryApi.js`, `readerStateApi.js`, `auth.svelte.js` y los componentes que construyen rutas de contenido deben usar esa capa.

### 6.3 URLs devueltas por el servidor

Los `OpenTarget`, previews y archivos pueden seguir conteniendo rutas del servidor, pero el cliente debe resolverlas contra `serverBaseUrl`.

El lector no debe asumir que su propio origen es el origen del contenido.

Esto afecta especialmente a:

- iframes ZIM;
- vídeo y audio;
- PDF;
- imágenes y miniaturas;
- mapas y datos de mapas;
- descargas de archivos publicadas por el servidor.

### 6.4 Autenticación

El modelo actual funciona cómodamente cuando Panel, lector y API comparten origen. Al separar Nimos Library hay que definir un contrato explícito para clientes.

Debe decidirse entre:

- sesión mediante cookie compatible con el origen del cliente;
- token de sesión enviado por el cliente;
- un proxy local o esquema equivalente que mantenga el mismo origen.

El token interno `X-Nimos-Token` debe continuar reservado al carril máquina/administración y no debe entregarse a Nimos Library.

**Transporte de auth en ELEMENTOS NATIVOS (arreglo 2026-07-14).** Problema real
observado: el listado (`/api/media`, con Bearer) funciona, pero abrir el fichero
daba **403** — un `<video>`/`<embed>`/`<img>`/`<iframe>` NO puede mandar la
cabecera `Authorization`, y con el cliente en otro origen (`:5173` vs `:8090`, o
`localhost` vs `127.0.0.1`) la cookie `SameSite=Lax` tampoco viaja fiable.
Solución pragmática implementada: el cliente añade el token en la query (`?st=`)
solo a las URLs de `/media` y `/content` (`connection.js` → `withMediaToken`), y
`requestSessionToken` (server) lo lee como fallback tras Bearer y cookie. **Es un
apaño LOCAL**: el token en la URL puede acabar en logs; para producción, el
contrato correcto es cookie de sesión same-site (servir el cliente desde el mismo
origen que el server) o URLs firmadas de vida corta. Además: los sub-recursos de
un artículo ZIM en iframe (imágenes/css relativos) se piden same-origin al server
→ dependen de que la COOKIE exista; si el cliente y el server no comparten host
(`127.0.0.1` en ambos), pueden fallar. Servir el cliente same-origin lo resuelve
todo de golpe.

### 6.5 CORS y orígenes permitidos

El middleware actual permite CORS de desarrollo para orígenes `localhost`. El servidor deberá aceptar únicamente los orígenes válidos de Nimos Library.

No debe utilizarse `Access-Control-Allow-Origin: *` junto con sesiones o credenciales.

La lista de orígenes debe ser configurable y restrictiva.

### 6.6 Seguridad del contenido

La separación visual no basta. El servidor debe aplicar permisos en las rutas reales de consumo:

- colecciones;
- Items;
- búsqueda;
- `/content/*`;
- `/media/*`;
- previews;
- mapas restringidos, si existen.

Ocultar una colección en Nimos Library no es una medida de seguridad. La autorización debe ocurrir en Library Server.

**Hallazgo (2026-07-14, probando los permisos en vivo):** la `searchCache` del
servidor cachea por consulta normalizada SIN distinguir usuario. Efecto real
observado: una búsqueda anónima (0 colecciones visibles) cachea el resultado
vacío y se lo sirve luego al usuario autenticado; a la inversa, resultados de un
usuario con acceso amplio podrían servirse a otro con menos permisos. Al
endurecer los permisos, la clave de la caché debe incluir la identidad o el
conjunto de colecciones visibles (p. ej. hash de la lista de libs), o saltarse
la caché para anónimos.

### 6.7 Información administrativa

Rutas como inventario de almacenamiento y detalles de rutas físicas pertenecen al Panel y deben quedar detrás de autorización administrativa.

Nimos Library solo necesita metadatos de consumo: título, tipo, tamaño publicado, capacidades y URL de apertura.

## 7. Limpieza de límites en la UI

Aunque ya no aparecen en la navegación principal, dentro de `ui/` quedan componentes y clientes administrativos heredados:

- componentes administrativos heredados;
- `DownloadQueue.svelte`;
- `adminProvidersApi.js`;
- `adminDownloadsApi.js`;
- exportaciones administrativas desde `api.js`.

Antes de distribuir Nimos Library como producto independiente debe comprobarse que el Panel contiene todas las funciones equivalentes y después retirar esos restos del paquete lector.

Esta limpieza no debe confundirse con eliminar la vista de contenido publicado. `PublishedLibrary.svelte` es una superficie de consumo y sí pertenece a Nimos Library.

## 8. Contrato entre cliente y servidor

La separación será estable si Nimos Library depende de contratos de producto y no de detalles físicos.

El cliente debe recibir:

- `Collection` para agrupar contenido;
- `Item` para representar cualquier unidad consumible;
- `SearchResult` para búsqueda;
- `Preview` para representaciones ligeras;
- `OpenTarget` para saber cómo abrir un Item;
- `capabilities` para saber qué acciones están permitidas.

Nimos Library no debe necesitar saber:

- la ruta física de un archivo;
- en qué disco está almacenado;
- qué proveedor lo importó;
- qué herramienta lo descargó;
- si internamente se sirve mediante Kiwix o el motor ZIM nativo;
- cómo está organizada la base de datos.

Ejemplo:

```json
{
  "itemId": "media:documentales/planeta.mp4",
  "mode": "video",
  "url": "/media/documentales/planeta.mp4",
  "mimeType": "video/mp4"
}
```

Nimos Library resuelve `url` contra el servidor configurado y lo reproduce. No inspecciona el almacenamiento ni administra el archivo.

## 9. Estado personal del usuario

Debe definirse explícitamente dónde viven:

- favoritos;
- notas;
- historial;
- etiquetas;
- progreso de vídeo;
- preferencias visuales.

Propuesta:

- favoritos, notas, historial, etiquetas y progreso sincronizable: Library Server por usuario;
- tema, idioma de interfaz y preferencias puramente visuales: almacenamiento local de Nimos Library;
- ninguna preferencia local debe otorgar acceso a contenido no autorizado.

Si más adelante se soportan varios servidores, el estado debe quedar identificado por servidor y usuario para evitar mezclar Items con IDs coincidentes.

## 10. Funcionamiento sin conexión

En esta arquitectura, offline-first significa que Library Server puede funcionar sin Internet para servir contenido ya almacenado.

No significa que Nimos Library pueda funcionar desconectado de Library Server.

```text
Internet caído + red con Library Server disponible = Library funciona.
Nimos Library sin conexión a Library Server = no hay catálogo ni contenido.
```

Una caché o descarga para uso desconectado en el cliente sería otra funcionalidad y no forma parte de esta separación.

## 11. Secuencia de migración recomendada

### Fase 1: introducir la frontera de red sin cambiar la distribución

- crear `serverBaseUrl` y un cliente HTTP común;
- resolver todas las rutas mediante esa capa;
- mantener temporalmente el valor vacío o el origen actual;
- comprobar lector ZIM, vídeo, PDF, imágenes, mapas y estado personal.

Resultado: el comportamiento visible no cambia, pero el lector deja de depender técnicamente del mismo origen.

### Fase 2: probar el lector desde otro origen

- ejecutar Nimos Library en un puerto u origen distinto;
- configurar CORS;
- validar autenticación;
- validar iframes, cookies, Range, descargas y cabeceras de seguridad;
- probar pérdida y recuperación de conexión.

Resultado: separación de red real durante desarrollo.

### Fase 3: limpiar Nimos Library

- retirar componentes administrativos heredados;
- eliminar imports y exportaciones administrativas;
- confirmar que no existen llamadas a rutas de administración;
- añadir pantalla de conexión y estado del servidor;
- diferenciar errores de red, autenticación y permisos.

Resultado: cliente de consumo puro.

### Fase 4: separar las builds

- impedir que `ui/vite.config.js` escriba en `shim/www`;
- producir un artefacto propio de Nimos Library;
- mantener la build del Panel en el paquete de Library Server;
- dejar de servir el lector desde la ruta raíz del Core, o conservar solo una redirección/página informativa durante la transición.

Resultado: dos productos distribuibles.

### Fase 5: endurecer Library Server

- proteger datos administrativos;
- verificar permisos en contenido y streaming;
- configurar TLS o despliegue detrás de proxy seguro;
- definir orígenes permitidos;
- registrar auditoría básica de administración;
- probar sesiones y múltiples usuarios.

Resultado: servidor preparado para clientes separados.

### Fase 6: empaquetado e instalación

- instalador de Library Server con Panel;
- instalador o paquete independiente de Nimos Library;
- descubrimiento o configuración manual del servidor;
- actualización independiente de servidor y cliente;
- documentación de compatibilidad de API.

## 12. Compatibilidad y versionado

Una vez que cliente y servidor se actualicen por separado, la API pasa a ser un contrato versionado.

Debe existir una forma de conocer:

- versión del servidor;
- versión o nivel de la API;
- capacidades disponibles;
- compatibilidad mínima del cliente.

La respuesta de salud podría evolucionar para incluir información de compatibilidad sin exponer detalles sensibles.

Nimos Library debe mostrar un mensaje claro si el servidor es demasiado antiguo o demasiado nuevo para el cliente.

## 13. Criterios de aceptación

La separación estará terminada cuando:

1. Library Server pueda instalarse y funcionar con su Panel sin incluir Nimos Library.
2. Nimos Library pueda instalarse por separado y pedir la dirección del servidor.
3. El cliente pueda listar, buscar, abrir y reproducir contenido remoto.
4. Los artículos ZIM funcionen dentro del lector desde otro origen.
5. Vídeos, audio y PDF funcionen con streaming y Range.
6. Favoritos, notas, historial y etiquetas pertenezcan al usuario correcto.
7. Nimos Library no contenga importadores, gestión de almacenamiento, usuarios ni colas administrativas.
8. El Panel continúe administrando todo el contenido del servidor.
9. Ninguna API entregue rutas físicas del servidor al lector.
10. Los permisos se apliquen en el servidor aunque se invoque la API directamente.
11. Cliente y servidor puedan actualizarse de forma independiente dentro de una política de compatibilidad.

## 14. Decisiones que conviene tomar antes de implementar

1. ¿Nimos Library será una aplicación instalada, una web independiente o ambas?
2. ¿Se conectará a un único servidor o podrá guardar varios?
3. ¿Cómo se descubrirá el servidor en la red: dirección manual, nombre local o descubrimiento automático?
4. ¿Qué mecanismo de sesión utilizarán los clientes separados?
5. ¿Se permitirá acceso solamente en LAN o también mediante Internet/VPN?
6. ¿Qué estado personal debe sincronizarse y qué preferencias quedan en el dispositivo?
7. ¿Qué política de compatibilidad tendrá la API?
8. ¿La ruta raíz de Library Server desaparecerá, redirigirá al Panel o mostrará una página de estado?

## 15. Veredicto

La separación es viable sin duplicar Library Core ni crear un motor local para Nimos Library.

La base actual es favorable porque Panel y lector ya son proyectos frontend distintos y el backend ya diferencia rutas administrativas. El trabajo principal consiste en convertir la dependencia implícita de mismo origen en un contrato de red explícito, limpiar los restos administrativos del lector y distribuir cada producto por separado.

El destino final debe preservar esta regla:

```text
El servidor posee, organiza, protege y sirve el conocimiento.
Nimos Library lo navega y lo presenta al usuario.
```
