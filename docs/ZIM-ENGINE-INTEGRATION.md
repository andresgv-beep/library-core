# Integracion de zim-engine en la separacion

## Decision

`zim-engine` pertenece a Library Server. Nimos Library cliente no lee ZIM, no abre indices Bleve y no accede al almacenamiento.

```text
Nimos Library
    -> HTTP
Library Server
    -> adaptador zim_native.go / zim_fts.go
zim-engine
    -> archivos .zim + indices .bleve del pool
```

## Limite entre Core y motor

`engines/zim-engine` conserva su identidad como modulo Go independiente:

- `zim/` interpreta el formato, abre entradas, resuelve redirects, gestiona caches y limites;
- `fts/` construye, valida y consulta los indices Bleve;
- `cmd/zimtool` sigue siendo herramienta de diagnostico e indexacion;
- no conoce usuarios, permisos, HTTP, Panel ni Nimos Library.

`library-server/core` conserva las responsabilidades de producto:

- localizar las colecciones registradas;
- aplicar permisos antes de servir bytes;
- exponer `/content/*` y las APIs de busqueda;
- aplicar CSP, MIME, Range, ETag y cabeceras HTTP;
- convertir resultados del motor a `Collection`, `Item` y `SearchResult`;
- invalidar archives e indices al registrar o retirar un ZIM.

## Enlace del modulo

El enlace anterior era absoluto y dependia del Escritorio de desarrollo:

```text
C:\Users\asus\Desktop\zim-engine
```

La copia separada usa:

```text
replace github.com/andresgv-beep/zim-engine => ../../engines/zim-engine
```

De esta forma el proyecto completo puede moverse a otra carpeta o maquina sin editar `go.mod`.

## Despliegue

El codigo del motor se compila dentro del binario de Library Server. No se instala como daemon separado y no abre otro puerto. Los `.zim` y `.bleve` continúan viviendo en el pool administrado por Library Server.

## Relacion con el problema de mismo origen

El motor propio elimina Kiwix y mantiene todo el procesamiento ZIM en el servidor, pero no elimina la restriccion del navegador: el Reader inspecciona el DOM del articulo para indice, navegacion y traduccion. Si Nimos Library se ejecuta desde otro origen, esa inspeccion queda bloqueada.

Por eso siguen siendo problemas distintos:

1. `zim-engine`: como Library Server lee y busca dentro de ZIM.
2. gateway de cliente: como Nimos Library presenta ese contenido bajo un origen compatible.

El gateway propuesto no contiene motor ZIM, indices ni almacenamiento. Solo transporta las peticiones al Library Server para preservar la experiencia del Reader.
