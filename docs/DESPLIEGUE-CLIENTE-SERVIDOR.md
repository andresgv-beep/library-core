# Despliegue de Library Client y Library Server

## Regla de arquitectura

Library Server posee y sirve catalogo, usuarios, estado personal, ZIM,
multimedia, mapas y API. Nimos Library consume esos servicios. El lector y el
contenido siempre deben verse desde el mismo origen.

## Topologias soportadas

### Servidor central y PWA

Library Server sirve Nimos Library en `/`, el Panel en `/panel/` y el contenido
en `/content` y `/media`. Es la opcion principal para movil, tableta y cualquier
equipo con navegador moderno.

### Cliente de escritorio remoto

`nimos-library-client.exe` pide la direccion del servidor en el primer arranque.
El shell Wails hace health-check y reverse proxy; no instala ni ejecuta Core.
La direccion puede cambiarse despues desde Ajustes.

### Todo en uno

El instalador todo-en-uno registra Library Supervisor como servicio e instala
dos ventanas nativas separadas: Nimos Library y Library Control Panel. El supervisor arranca Core y el
motor opcional desde `bin/`. Cerrar cualquiera de las interfaces no afecta al
servidor. El mismo equipo puede usar la biblioteca y tambien publicarla en red
si se configura el servidor para escuchar fuera de loopback.

### Supervisor

Library Supervisor es independiente de las interfaces:

- inicia Core y traduccion;
- reinicia automaticamente cualquier proceso que termine;
- aplica espera exponencial para evitar bucles de caida agresivos;
- conserva los datos fuera del directorio de aplicacion;
- en Windows se registra como servicio con recuperacion del propio supervisor.

El Panel dispone de un boton de reinicio solo para administradores. La peticion
llega a la API administrativa; Core sale con un codigo reservado y el supervisor
lo levanta de nuevo. El Panel nunca ejecuta procesos.

## Builds

```powershell
# Paquete de Library Server con PWA y Panel
cd library-server
.\build.ps1

# Cliente de escritorio contra un servidor remoto
cd ..\library-desktop
.\build.ps1 -Mode remote

# Aplicacion autocontenida
.\build.ps1 -Mode all-in-one
.\install-all-in-one.ps1
```

## PWA y cache

El service worker guarda solamente el HTML de entrada y recursos propios de la
interfaz (`/assets`, `/pdfjs`, iconos y manifest). Nunca intercepta ni almacena:

- `/api`;
- `/content`;
- `/media`;
- `/maps` y `/mapdata`;
- `/catalog`.

La PWA instalada sigue necesitando conexion con Library Server. Su cache no es
un modo offline de contenidos.

## Seguridad y transporte

- El gateway solo acepta destinos `http` o `https` sin credenciales ni rutas.
- En HTTPS, Go deriva SNI del host de destino.
- El proxy reescribe `Host`, necesario para servidores virtuales como Caddy.
- Bearer, cookies, query de sesion, cabeceras Range y streaming atraviesan el
  proxy sin que el shell gestione autenticacion.
- El HTML de `/content` no se modifica ni se cachea como interfaz.
