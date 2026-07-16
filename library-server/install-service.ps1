param([switch]$Uninstall)

$ErrorActionPreference = 'Stop'
$Release = if ((Split-Path -Leaf $PSScriptRoot) -eq 'release') { $PSScriptRoot } else { Join-Path $PSScriptRoot 'release' }
$Supervisor = Join-Path $Release 'library-supervisor.exe'
$StartMenu = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\Nimos Library'

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw 'Ejecuta este instalador como administrador.'
}
if (-not (Test-Path -LiteralPath $Supervisor)) { throw "No existe $Supervisor. Ejecuta build.ps1 primero." }

if ($Uninstall) {
  & $Supervisor uninstall
  if (Test-Path -LiteralPath $StartMenu) { Remove-Item -LiteralPath $StartMenu -Recurse -Force }
  Write-Host 'Servicio retirado. Los datos de Library Server se conservan.' -ForegroundColor Green
  exit 0
}

& $Supervisor install
if ($LASTEXITCODE -ne 0) { throw 'No se pudo instalar el servicio de Library Server.' }
& $Supervisor start
if ($LASTEXITCODE -ne 0) { throw 'No se pudo arrancar el servicio de Library Server.' }
New-Item -ItemType Directory -Force -Path $StartMenu | Out-Null
$panelLink = "[InternetShortcut]`r`nURL=http://127.0.0.1:8090/panel/`r`n"
[IO.File]::WriteAllText((Join-Path $StartMenu 'Library Control Panel.url'), $panelLink)
Write-Host 'Library Server instalado y supervisado.' -ForegroundColor Green
Write-Host 'Panel: http://127.0.0.1:8090/panel/' -ForegroundColor Cyan
