param(
  [ValidateSet('all-in-one', 'remote')]
  [string]$Mode = 'all-in-one'
)

# Ensambla una de las dos distribuciones de escritorio:
#   .\build.ps1 -Mode remote      -> nimos-library-client.exe (solo gateway)
#   .\build.ps1 -Mode all-in-one -> nimos-library-all-in-one.exe + bin\
$ErrorActionPreference = 'Stop'
$Root    = Split-Path -Parent $PSScriptRoot
$Desktop = $PSScriptRoot
$Bin     = Join-Path $Desktop 'bin'
$env:PATH = "$env:PATH;$env:USERPROFILE\go\bin"
$env:GOCACHE = Join-Path $env:TEMP 'nimos-go-build-cache'
$env:GOTELEMETRY = 'off'

function Assert-NativeSuccess([string]$Step) {
  if ($LASTEXITCODE -ne 0) { throw "$Step fallo con codigo $LASTEXITCODE" }
}

if ($Mode -eq 'remote') {
  Write-Host '[1/1] Cliente de escritorio remoto...' -ForegroundColor Cyan
  Push-Location $Desktop
  try {
    go build -tags 'desktop production' -ldflags '-H windowsgui -X main.distributionMode=remote' -o 'nimos-library-client.exe' .
    Assert-NativeSuccess 'Cliente remoto'
  } finally { Pop-Location }
  Write-Host "OK -> $Desktop\nimos-library-client.exe" -ForegroundColor Green
  exit 0
}

New-Item -ItemType Directory -Force -Path $Bin | Out-Null

Write-Host '[1/8] Cliente PWA...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'nimos-library')
try { npm.cmd run build; Assert-NativeSuccess 'Cliente PWA' } finally { Pop-Location }
$ClientOut = Join-Path $Bin 'www-client'
if (Test-Path -LiteralPath $ClientOut) { Remove-Item -LiteralPath $ClientOut -Recurse -Force }
Copy-Item -Recurse (Join-Path $Root 'nimos-library\dist') $ClientOut

Write-Host '[2/8] Panel de Control...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\panel')
try { npm.cmd run build; Assert-NativeSuccess 'Panel de Control' } finally { Pop-Location }
$PanelOut = Join-Path $Bin 'www-panel'
if (Test-Path -LiteralPath $PanelOut) { Remove-Item -LiteralPath $PanelOut -Recurse -Force }
Copy-Item -Recurse (Join-Path $Root 'library-server\core\www-panel') $PanelOut
$MapsOut = Join-Path $Bin 'maps-www'
if (Test-Path -LiteralPath $MapsOut) { Remove-Item -LiteralPath $MapsOut -Recurse -Force }
Copy-Item -Recurse (Join-Path $Root 'library-server\core\maps-www') $MapsOut

Write-Host '[3/8] Library Server...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\core')
try { go build -o (Join-Path $Bin 'core.exe') .; Assert-NativeSuccess 'Library Server' } finally { Pop-Location }

Write-Host '[4/8] Motor de traduccion...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\translate-wrap')
try { go build -o (Join-Path $Bin 'translate-wrap.exe') .; Assert-NativeSuccess 'Motor de traduccion' } finally { Pop-Location }

Write-Host '[5/8] Supervisor independiente...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\supervisor')
try { go build -o (Join-Path $Bin 'library-supervisor.exe') .; Assert-NativeSuccess 'Supervisor' } finally { Pop-Location }

Write-Host '[6/8] Extractor oficial PMTiles...' -ForegroundColor Cyan
$PMTiles = Join-Path $Bin 'pmtiles.exe'
if (-not (Test-Path -LiteralPath $PMTiles)) {
  $env:GOBIN = $Bin
  go install github.com/protomaps/go-pmtiles@v1.30.2
  Assert-NativeSuccess 'Extractor PMTiles'
  $GoPMTiles = Join-Path $Bin 'go-pmtiles.exe'
  if (Test-Path -LiteralPath $GoPMTiles) { Move-Item -LiteralPath $GoPMTiles -Destination $PMTiles -Force }
}
if (-not (Test-Path -LiteralPath $PMTiles)) { throw 'No se genero pmtiles.exe' }

Write-Host '[7/8] Nimos Library...' -ForegroundColor Cyan
Push-Location $Desktop
try {
  go build -tags 'desktop production' -ldflags '-H windowsgui' -o 'nimos-library-all-in-one.exe' .
  Assert-NativeSuccess 'Nimos Library'
} finally { Pop-Location }

Write-Host '[8/8] Library Control Panel nativo...' -ForegroundColor Cyan
Push-Location $Desktop
try {
  go build -tags 'desktop production' -ldflags '-H windowsgui -X main.interfaceMode=panel' -o 'library-control-panel.exe' .
  Assert-NativeSuccess 'Library Control Panel'
} finally { Pop-Location }

Write-Host "OK -> $Desktop\nimos-library-all-in-one.exe" -ForegroundColor Green
Write-Host "OK -> $Desktop\library-control-panel.exe" -ForegroundColor Green
Write-Host "Instalar servicio e interfaces -> $Desktop\install-all-in-one.ps1" -ForegroundColor Green
