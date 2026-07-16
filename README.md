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

El servidor expone el Panel en `/panel/` y publica el cliente PWA en `/`. De este
modo el lector y `/content` comparten origen y el lector ZIM conserva todas sus
funciones.

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

El cliente instalado usa `library-desktop` como gateway de transporte. Puede
proxificar un Library Server remoto o conectarse al servicio local instalado
por el paquete todo-en-uno. Library Supervisor, independiente de las interfaces,
es el unico propietario de Core y sus motores. En ambos casos la SPA, la API y
el contenido se presentan bajo un solo origen.

Consulte [docs/DESPLIEGUE-CLIENTE-SERVIDOR.md](docs/DESPLIEGUE-CLIENTE-SERVIDOR.md)
para los builds y topologias soportadas.

Consulte `docs/SEPARACION-SERVER-NIMOS-LIBRARY.md` para el plan completo.
