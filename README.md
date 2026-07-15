# Nimos Library separado

Esta carpeta es una copia de trabajo nueva. El proyecto original `nimos-library-v3` no se modifica.

```text
library-server/
  core/             Library Core, API, motores y streaming
  panel/            Panel de Control, incluido con el servidor
  translate-wrap/   motor opcional de traduccion

engines/
  zim-engine/       lector ZIM y full-text Bleve, modulo Go independiente

nimos-library/      cliente lector independiente
docs/               decisiones y plan de separacion
```

## Regla de producto

Library Server posee, organiza, protege y sirve todo el contenido. Nimos Library solo navega y visualiza lo publicado por el servidor.

## Desarrollo

1. Compile el Panel desde `library-server/panel` con `npm run build`.
2. Inicie el Core desde `library-server/core` con `go run .` y las variables necesarias.
3. Inicie el cliente desde `nimos-library` con `npm run dev`.
4. Configure la direccion de Library Server desde Ajustes en Nimos Library.

El servidor expone el Panel en `/panel/`. Su ruta `/` redirige al Panel y ya no entrega el lector.

## Estado de esta primera separacion

- arbol de servidor y cliente independiente;
- build independiente del cliente en `nimos-library/dist`;
- direccion de servidor configurable;
- cliente HTTP centralizado;
- rutas administrativas retiradas del cliente;
- CORS configurable mediante `CLIENT_ORIGINS`;
- inventario de almacenamiento protegido como administracion;
- sesion de cliente preparada mediante Bearer token para llamadas API.
- motor ZIM propio incluido como modulo del servidor, sin rutas absolutas.

Antes de empaquetar el cliente instalado hay que cerrar la estrategia de mismo origen para articulos ZIM. El lector actual inspecciona el DOM del iframe para navegacion, indice y traduccion; un iframe remoto puede visualizar el articulo, pero el navegador impide esa inspeccion entre origenes. La opcion recomendada para una app instalada es un gateway local de transporte que sirva la interfaz y proxifique Library Server, sin almacenar ni gestionar contenido.

Consulte `docs/SEPARACION-SERVER-NIMOS-LIBRARY.md` para el plan completo.
