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

if ($Mode -eq 'remote') {
  Write-Host '[1/1] Cliente de escritorio remoto...' -ForegroundColor Cyan
  Push-Location $Desktop
  try {
    go build -tags 'desktop production' -ldflags '-H windowsgui -X main.distributionMode=remote' -o 'nimos-library-client.exe' .
  } finally { Pop-Location }
  Write-Host "OK -> $Desktop\nimos-library-client.exe" -ForegroundColor Green
  exit 0
}

New-Item -ItemType Directory -Force -Path $Bin | Out-Null

Write-Host '[1/7] Cliente PWA...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'nimos-library')
try { npm.cmd run build } finally { Pop-Location }
$ClientOut = Join-Path $Bin 'www-client'
if (Test-Path -LiteralPath $ClientOut) { Remove-Item -LiteralPath $ClientOut -Recurse -Force }
Copy-Item -Recurse (Join-Path $Root 'nimos-library\dist') $ClientOut

Write-Host '[2/7] Panel de Control...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\panel')
try { npm.cmd run build } finally { Pop-Location }
$PanelOut = Join-Path $Bin 'www-panel'
if (Test-Path -LiteralPath $PanelOut) { Remove-Item -LiteralPath $PanelOut -Recurse -Force }
Copy-Item -Recurse (Join-Path $Root 'library-server\core\www-panel') $PanelOut

Write-Host '[3/7] Library Server...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\core')
try { go build -o (Join-Path $Bin 'core.exe') . } finally { Pop-Location }

Write-Host '[4/7] Motor de traduccion...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\translate-wrap')
try { go build -o (Join-Path $Bin 'translate-wrap.exe') . } finally { Pop-Location }

Write-Host '[5/7] Supervisor independiente...' -ForegroundColor Cyan
Push-Location (Join-Path $Root 'library-server\supervisor')
try { go build -o (Join-Path $Bin 'library-supervisor.exe') . } finally { Pop-Location }

Write-Host '[6/7] Nimos Library...' -ForegroundColor Cyan
Push-Location $Desktop
try {
  go build -tags 'desktop production' -ldflags '-H windowsgui' -o 'nimos-library-all-in-one.exe' .
} finally { Pop-Location }

Write-Host '[7/7] Library Control Panel nativo...' -ForegroundColor Cyan
Push-Location $Desktop
try {
  go build -tags 'desktop production' -ldflags '-H windowsgui -X main.interfaceMode=panel' -o 'library-control-panel.exe' .
} finally { Pop-Location }

Write-Host "OK -> $Desktop\nimos-library-all-in-one.exe" -ForegroundColor Green
Write-Host "OK -> $Desktop\library-control-panel.exe" -ForegroundColor Green
Write-Host "Instalar servicio e interfaces -> $Desktop\install-all-in-one.ps1" -ForegroundColor Green
