# Nimos Library

Cliente lector independiente para Library Server. No administra almacenamiento, descargas, proveedores, usuarios ni colecciones.

La direccion del servidor se configura desde Ajustes o mediante:

```text
VITE_LIBRARY_SERVER=http://127.0.0.1:8090
```

## Desarrollo

```powershell
npm install
npm run dev
```

## Build

```powershell
npm run build
```

La salida queda en `dist/`; ya no se copia dentro de Library Server.
