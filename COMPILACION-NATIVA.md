# Memoria de compilación e instalación nativa

Esta guía existe para que cualquier chat o colaborador pueda pasar un cambio del código fuente a la aplicación nativa de Windows sin dejarlo únicamente en el repositorio.

## Regla principal

Modificar un archivo **no actualiza la aplicación instalada**. El recorrido completo es siempre:

1. Conservar e inspeccionar los cambios existentes.
2. Probar el código afectado.
3. Ejecutar la compilación `all-in-one` completa.
4. Instalar el resultado con permisos reales de administrador.
5. Verificar la instalación de `C:\Program Files\Nimos Library`.
6. Reiniciar o abrir la aplicación nativa.

No se debe editar ni copiar manualmente dentro de `Program Files`. La instalación la realiza `library-desktop\install-all-in-one.ps1`.

## Rutas importantes

Partiendo de la raíz del repositorio:

| Cambio | Fuente |
|---|---|
| Maps: interfaz, estilos y recursos | `library-server/core/maps-www/` |
| Maps: búsqueda, teselas, indexación y API | `library-server/core/maps*.go` y `library-server/core/main.go` |
| Cliente principal | `nimos-library/src/` |
| Panel de Control web | `library-server/panel/src/` |
| Ventanas nativas y gateway | `library-desktop/` |
| Supervisor/servicio | `library-server/supervisor/` |

Los archivos de Maps no van incrustados mágicamente al guardar. `build.ps1` copia `library-server/core/maps-www` a `library-desktop/bin/maps-www`, y el instalador copia después ese paquete a:

```text
C:\Program Files\Nimos Library\bin\maps-www
```

## 1. Revisar y preservar los cambios

Desde la raíz del repositorio:

```powershell
git status --short
git diff --check
git diff --stat
```

El árbol puede contener cambios de otro chat o del usuario. No usar `git reset --hard`, `git checkout --` ni borrar cambios para “limpiar” el proyecto. La compilación debe incluir el estado actual autorizado, incluso si todavía no está confirmado en Git.

## 2. Ejecutar las pruebas relevantes

Para cambios del servidor o Maps:

```powershell
Set-Location .\library-server\core
$env:GOCACHE = 'C:\Users\asus\Documents\GitHub\library-core\.tmp-gocache'
go test ./...
```

Para un cambio exclusivamente visual en `maps-www/index.html`, la compilación completa sigue siendo obligatoria aunque no haya una prueba Go específica.

## 3. Compilar el todo-en-uno

Desde `library-desktop`:

```powershell
Set-Location C:\Users\asus\Documents\GitHub\library-core\library-desktop
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Mode all-in-one
```

La compilación correcta termina mostrando estos resultados:

```text
library-desktop\nimos-library-all-in-one.exe
library-desktop\library-control-panel.exe
library-desktop\bin\core.exe
library-desktop\bin\library-supervisor.exe
library-desktop\bin\maps-www\...
```

`build.ps1` ensambla ocho piezas: cliente PWA, Panel de Control, servidor, traducción, supervisor, PMTiles, aplicación nativa y Panel de Control nativo. Algunos avisos de Svelte ya conocidos no significan que la compilación haya fallado; lo decisivo es el código de salida y el mensaje final `OK`.

No usar `-Mode remote` para actualizar el todo-en-uno. Ese modo genera únicamente el cliente remoto.

## 4. Instalar con elevación real de Windows

El instalador exige administrador. Ejecutarlo en una consola normal suele terminar con `Ejecuta este instalador como administrador`. La forma fiable es abrir el aviso UAC mediante `-Verb RunAs`:

```powershell
$installer = 'C:\Users\asus\Documents\GitHub\library-core\library-desktop\install-all-in-one.ps1'
$process = Start-Process -FilePath 'powershell.exe' `
  -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $installer) `
  -Verb RunAs -Wait -PassThru
"INSTALL_EXIT=$($process.ExitCode)"
```

El usuario debe aceptar el aviso de Control de cuentas de Windows. El resultado válido es:

```text
INSTALL_EXIT=0
```

El instalador detiene el servicio de forma segura, sustituye binarios y recursos, vuelve a instalar/arrancar el supervisor y actualiza los accesos directos. No borra el pool de datos, los mapas descargados ni las bases de datos del usuario.

## 5. Verificar que la instalación contiene el cambio

### Servicio y salud

```powershell
$service = Get-Service NimosLibraryServer
"SERVICE=$($service.Status) START=$($service.StartType)"

$health = Invoke-RestMethod 'http://127.0.0.1:8090/api/health' -TimeoutSec 5
$health | ConvertTo-Json -Compress
```

Debe indicar servicio `Running`, inicio `Automatic` y salud correcta.

### Comparar el ejecutable nativo

```powershell
$built = Get-FileHash '.\nimos-library-all-in-one.exe' -Algorithm SHA256
$installed = Get-FileHash 'C:\Program Files\Nimos Library\nimos-library.exe' -Algorithm SHA256
"EXE_MATCH=$($built.Hash -eq $installed.Hash)"
```

Debe devolver `EXE_MATCH=True`.

### Comparar Maps

Desde la raíz del repositorio:

```powershell
$source = Get-FileHash '.\library-server\core\maps-www\index.html' -Algorithm SHA256
$installed = Get-FileHash 'C:\Program Files\Nimos Library\bin\maps-www\index.html' -Algorithm SHA256
"MAP_MATCH=$($source.Hash -eq $installed.Hash)"
```

Debe devolver `MAP_MATCH=True`. También se puede consultar la versión servida:

```powershell
(Invoke-WebRequest 'http://127.0.0.1:8090/maps/' -UseBasicParsing -TimeoutSec 5).Content
```

## 6. Abrir la aplicación nueva

Si había una instancia antigua abierta, debe cerrarse y abrirse de nuevo para cargar el ejecutable y los recursos actualizados.

```powershell
Start-Process 'C:\Program Files\Nimos Library\nimos-library.exe'
```

La tarea solo está terminada cuando:

- la compilación completa finaliza correctamente;
- el instalador devuelve `0`;
- los hashes de fuente/instalación coinciden;
- el servicio está activo y saludable;
- la aplicación nativa se ha vuelto a abrir;
- el cambio se comprueba en la interfaz instalada, no solamente en una vista de desarrollo.

## Errores frecuentes

- **“Lo cambié pero no aparece”**: faltó ejecutar `build.ps1` o instalar el resultado.
- **“Compilé y sigue igual”**: se compiló, pero `Program Files` conserva la instalación anterior.
- **“El instalador pide administrador”**: usar `Start-Process ... -Verb RunAs` y aceptar UAC.
- **“Maps sigue mostrando el HTML antiguo”**: comparar `library-server/core/maps-www/index.html` con `Program Files/Nimos Library/bin/maps-www/index.html`.
- **“El servicio funciona pero la ventana no cambió”**: cerrar y volver a abrir `nimos-library.exe`.
- **“Solo generé nimos-library-client.exe”**: se usó el modo `remote`; para esta instalación se necesita `all-in-one`.

