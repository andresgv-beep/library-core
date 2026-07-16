# Library Server

Paquete servidor de Nimos Library.

Incluye Library Core, Panel de Control, motores, almacenamiento, importadores,
permisos y streaming. Su paquete de despliegue incluye tambien la build PWA de
Nimos Library bajo `/`; sigue siendo un artefacto independiente y no comparte
logica ni codigo servidor.

El lector ZIM y el indice full-text propio viven en `../engines/zim-engine` como modulo Go independiente. `core/go.mod` lo enlaza mediante una ruta relativa para que el arbol pueda moverse o instalarse en otra maquina.

## Build completo

Desde `library-server`:

```powershell
.\build.ps1
```

El paquete queda en `release/` con el servidor, el cliente PWA, el Panel y los
recursos de mapas. `release/install-service.ps1` registra el supervisor como
servicio de Windows, activa la recuperacion automatica y crea el acceso al Panel.
Para compilar las piezas manualmente:

```powershell
cd panel
npm install
npm run build

cd ../core
go test ./...
go build -o library-server.exe .
```

La build del Panel se escribe en `core/www-panel`, desde donde la sirve Library Server bajo `/panel/`.
La build de Nimos Library se copia como `www-client`, desde donde se sirve bajo `/`.

## Ciclo de vida

`library-supervisor.exe` es el unico propietario de los procesos Core y
traduccion. Reinicia Core si cae y atiende el codigo de reinicio controlado que
puede solicitar un administrador desde el Panel. Ni el Panel ni Nimos Library
inician, detienen o manipulan procesos directamente.

Para clientes en otro origen, declare una lista exacta separada por comas:

```text
CLIENT_ORIGINS=http://localhost:5173,https://library-client.example
```

No use un comodin cuando haya credenciales.
