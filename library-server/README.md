# Library Server

Paquete servidor de Nimos Library.

Incluye Library Core, Panel de Control, motores, almacenamiento, importadores, permisos y streaming. No incluye la interfaz lectora Nimos Library.

El lector ZIM y el indice full-text propio viven en `../engines/zim-engine` como modulo Go independiente. `core/go.mod` lo enlaza mediante una ruta relativa para que el arbol pueda moverse o instalarse en otra maquina.

## Build

```powershell
cd panel
npm install
npm run build

cd ../core
go test ./...
go build -o library-server.exe .
```

La build del Panel se escribe en `core/www-panel`, desde donde la sirve Library Server bajo `/panel/`.

Para clientes en otro origen, declare una lista exacta separada por comas:

```text
CLIENT_ORIGINS=http://localhost:5173,https://library-client.example
```

No use un comodin cuando haya credenciales.
