$ErrorActionPreference = 'Stop'
$Server  = $PSScriptRoot
$Root    = Split-Path -Parent $Server
$Release = Join-Path $Server 'release'
$env:GOCACHE = Join-Path $env:TEMP 'nimos-go-build-cache'
$env:GOTELEMETRY = 'off'

function Assert-NativeSuccess([string]$Step) {
  if ($LASTEXITCODE -ne 0) { throw "$Step fallo con codigo $LASTEXITCODE" }
}

if (Test-Path -LiteralPath $Release) {
  $resolvedServer = (Resolve-Path -LiteralPath $Server).Path.TrimEnd('\')
  $resolvedRelease = (Resolve-Path -LiteralPath $Release).Path
  if (-not $resolvedRelease.StartsWith($resolvedServer + '\')) { throw 'Ruta de release fuera de library-server' }
  Remove-Item -LiteralPath $resolvedRelease -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $Release | Out-Null

Write-Host '[1/6] Cliente PWA hospedado...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'nimos-library')
try { npm.cmd run build; Assert-NativeSuccess 'Cliente PWA' } finally { Pop-Location }
Copy-Item -Recurse (Join-Path $Root 'nimos-library\dist') (Join-Path $Release 'www-client')

Write-Host '[2/6] Panel de Control...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'panel')
try { npm.cmd run build; Assert-NativeSuccess 'Panel de Control' } finally { Pop-Location }
Copy-Item -Recurse (Join-Path $Server 'core\www-panel') (Join-Path $Release 'www-panel')

Write-Host '[3/6] Library Server...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'core')
try { go build -o (Join-Path $Release 'library-server.exe') .; Assert-NativeSuccess 'Library Server' } finally { Pop-Location }

Write-Host '[4/6] Motor opcional y recursos...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'translate-wrap')
try { go build -o (Join-Path $Release 'translate-wrap.exe') .; Assert-NativeSuccess 'Motor de traduccion' } finally { Pop-Location }
Copy-Item -Recurse (Join-Path $Server 'core\maps-www') (Join-Path $Release 'maps-www')
if (Test-Path -LiteralPath (Join-Path $Server 'core\mapdata')) {
  Copy-Item -Recurse (Join-Path $Server 'core\mapdata') (Join-Path $Release 'mapdata')
}

Write-Host '[5/6] Supervisor independiente...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'supervisor')
try { go build -o (Join-Path $Release 'library-supervisor.exe') .; Assert-NativeSuccess 'Supervisor' } finally { Pop-Location }
Copy-Item -LiteralPath (Join-Path $Server 'install-service.ps1') (Join-Path $Release 'install-service.ps1')

Write-Host '[6/6] Extractor oficial PMTiles...' -ForegroundColor Cyan
$DesktopPMTiles = Join-Path $Root 'library-desktop\bin\pmtiles.exe'
if (Test-Path -LiteralPath $DesktopPMTiles) {
  Copy-Item -LiteralPath $DesktopPMTiles (Join-Path $Release 'pmtiles.exe')
} else {
  $env:GOBIN = $Release
  go install github.com/protomaps/go-pmtiles@v1.30.2
  Assert-NativeSuccess 'Extractor PMTiles'
  $GoPMTiles = Join-Path $Release 'go-pmtiles.exe'
  if (Test-Path -LiteralPath $GoPMTiles) { Move-Item -LiteralPath $GoPMTiles -Destination (Join-Path $Release 'pmtiles.exe') -Force }
}

Write-Host "OK -> $Release" -ForegroundColor Green
