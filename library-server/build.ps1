$ErrorActionPreference = 'Stop'
$Server  = $PSScriptRoot
$Root    = Split-Path -Parent $Server
$Release = Join-Path $Server 'release'

if (Test-Path -LiteralPath $Release) {
  $resolvedServer = (Resolve-Path -LiteralPath $Server).Path.TrimEnd('\')
  $resolvedRelease = (Resolve-Path -LiteralPath $Release).Path
  if (-not $resolvedRelease.StartsWith($resolvedServer + '\')) { throw 'Ruta de release fuera de library-server' }
  Remove-Item -LiteralPath $resolvedRelease -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $Release | Out-Null

Write-Host '[1/5] Cliente PWA hospedado...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'nimos-library')
try { npm.cmd run build } finally { Pop-Location }
Copy-Item -Recurse (Join-Path $Root 'nimos-library\dist') (Join-Path $Release 'www-client')

Write-Host '[2/5] Panel de Control...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'panel')
try { npm.cmd run build } finally { Pop-Location }
Copy-Item -Recurse (Join-Path $Server 'core\www-panel') (Join-Path $Release 'www-panel')

Write-Host '[3/5] Library Server...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'core')
try { go build -o (Join-Path $Release 'library-server.exe') . } finally { Pop-Location }

Write-Host '[4/5] Motor opcional y recursos...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'translate-wrap')
try { go build -o (Join-Path $Release 'translate-wrap.exe') . } finally { Pop-Location }
Copy-Item -Recurse (Join-Path $Server 'core\maps-www') (Join-Path $Release 'maps-www')
if (Test-Path -LiteralPath (Join-Path $Server 'core\mapdata')) {
  Copy-Item -Recurse (Join-Path $Server 'core\mapdata') (Join-Path $Release 'mapdata')
}

Write-Host '[5/5] Supervisor independiente...' -ForegroundColor Cyan
Push-Location (Join-Path $Server 'supervisor')
try { go build -o (Join-Path $Release 'library-supervisor.exe') . } finally { Pop-Location }
Copy-Item -LiteralPath (Join-Path $Server 'install-service.ps1') (Join-Path $Release 'install-service.ps1')

Write-Host "OK -> $Release" -ForegroundColor Green
