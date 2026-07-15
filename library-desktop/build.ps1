# build.ps1 — ensambla la app de escritorio Nimos Library.
#   powershell -ExecutionPolicy Bypass -File library-desktop\build.ps1
#
# Produce library-desktop\library-desktop.exe + library-desktop\bin\ con:
#   core.exe · translate-wrap.exe · www-client\ · www-panel\
# Los sidecars y sus assets viven en bin\ (siblingDir del core resuelve ahí).
$ErrorActionPreference = 'Stop'
$Root    = Split-Path -Parent $PSScriptRoot          # raíz del monorepo
$Desktop = $PSScriptRoot
$Bin     = Join-Path $Desktop 'bin'
$env:PATH = "$env:PATH;$env:USERPROFILE\go\bin"

New-Item -ItemType Directory -Force -Path $Bin | Out-Null

Write-Host '[1/5] Cliente (Vite build)...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'nimos-library')
npm run build
Pop-Location
Remove-Item -Recurse -Force (Join-Path $Bin 'www-client') -ErrorAction SilentlyContinue
Copy-Item -Recurse (Join-Path $Root 'nimos-library\dist') (Join-Path $Bin 'www-client')

Write-Host '[2/5] Panel (Vite build)...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\panel')
npm run build   # sale a ..\core\www-panel
Pop-Location
Remove-Item -Recurse -Force (Join-Path $Bin 'www-panel') -ErrorAction SilentlyContinue
Copy-Item -Recurse (Join-Path $Root 'library-server\core\www-panel') (Join-Path $Bin 'www-panel')

Write-Host '[3/5] core.exe...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\core')
go build -o (Join-Path $Bin 'core.exe') .
Pop-Location

Write-Host '[4/5] translate-wrap.exe...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\translate-wrap')
go build -o (Join-Path $Bin 'translate-wrap.exe') .
Pop-Location

Write-Host '[5/5] library-desktop.exe (tags: desktop production)...' -ForegroundColor Cyan
Push-Location $Desktop
go build -tags 'desktop production' -ldflags '-H windowsgui' -o 'library-desktop.exe' .
Pop-Location

Write-Host "OK -> $Desktop\library-desktop.exe" -ForegroundColor Green
