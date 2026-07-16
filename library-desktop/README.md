# Nimos Library Desktop

Shell Wails que mantiene la SPA y todo el contenido bajo el mismo origen.

## Cliente remoto

```powershell
.\build.ps1 -Mode remote
```

Genera `nimos-library-client.exe`. En el primer arranque pide la direccion de
Library Server, la guarda en la configuracion del usuario y actua como reverse
proxy. No incluye ni arranca sidecars.

La variable `NIMOS_LIBRARY_SERVER` puede fijar o sustituir el destino:

```text
NIMOS_LIBRARY_SERVER=https://library.example
```

## Todo en uno

```powershell
.\build.ps1 -Mode all-in-one
```

Genera dos interfaces nativas, `nimos-library-all-in-one.exe` y
`library-control-panel.exe`, junto a `bin/`, que contiene Core, traduccion, PWA,
Panel y `library-supervisor.exe`. Despues se instala el servicio y los dos accesos
con:

```powershell
.\install-all-in-one.ps1
```

El supervisor, no el lector, controla Core. Cerrar Nimos Library o el navegador
del Panel no detiene el servidor.

En ambos modos la SPA usa rutas relativas. El shell reescribe `Host` hacia el
destino, conserva streaming y Range, y no necesita CORS.
