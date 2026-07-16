param([switch]$Uninstall)

$ErrorActionPreference = 'Stop'
$SourceRoot = $PSScriptRoot
$SourceBin = Join-Path $SourceRoot 'bin'
$InstallRoot = Join-Path $env:ProgramFiles 'Nimos Library'
$InstallBin = Join-Path $InstallRoot 'bin'
$Supervisor = Join-Path $InstallBin 'library-supervisor.exe'
$Client = Join-Path $InstallRoot 'nimos-library.exe'
$Panel = Join-Path $InstallRoot 'library-control-panel.exe'
$StartMenu = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\Nimos Library'

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw 'Ejecuta este instalador como administrador.'
}

if ($Uninstall) {
  if (Test-Path -LiteralPath $Supervisor) { & $Supervisor uninstall }
  if (Test-Path -LiteralPath $StartMenu) { Remove-Item -LiteralPath $StartMenu -Recurse -Force }
  if (Test-Path -LiteralPath $InstallRoot) {
    $programFilesPath = (Resolve-Path -LiteralPath $env:ProgramFiles).Path.TrimEnd('\')
    $resolvedInstall = (Resolve-Path -LiteralPath $InstallRoot).Path
    if (-not $resolvedInstall.StartsWith($programFilesPath + '\')) { throw 'Ruta de instalacion fuera de Program Files' }
    Remove-Item -LiteralPath $resolvedInstall -Recurse -Force
  }
  Write-Host 'Aplicaciones y servicio retirados. Los datos de ProgramData se conservan.' -ForegroundColor Green
  exit 0
}

$SourceSupervisor = Join-Path $SourceBin 'library-supervisor.exe'
$SourceClient = Join-Path $SourceRoot 'nimos-library-all-in-one.exe'
$SourcePanel = Join-Path $SourceRoot 'library-control-panel.exe'
foreach ($required in @($SourceSupervisor, $SourceClient, $SourcePanel)) {
  if (-not (Test-Path -LiteralPath $required)) { throw "Falta $required. Ejecuta build.ps1 -Mode all-in-one." }
}

# Actualizacion segura: detener el servicio instalado antes de sustituir binarios.
if (Test-Path -LiteralPath $Supervisor) { & $Supervisor stop }
New-Item -ItemType Directory -Force -Path $InstallBin | Out-Null
Copy-Item -Path (Join-Path $SourceBin '*') -Destination $InstallBin -Recurse -Force
Copy-Item -LiteralPath $SourceClient -Destination $Client -Force
Copy-Item -LiteralPath $SourcePanel -Destination $Panel -Force

& $Supervisor install
if ($LASTEXITCODE -ne 0) { throw 'No se pudo instalar el servicio de Library Server.' }
& $Supervisor start
if ($LASTEXITCODE -ne 0) { throw 'No se pudo arrancar el servicio de Library Server.' }

New-Item -ItemType Directory -Force -Path $StartMenu | Out-Null
$shell = New-Object -ComObject WScript.Shell
$clientShortcut = $shell.CreateShortcut((Join-Path $StartMenu 'Nimos Library.lnk'))
$clientShortcut.TargetPath = $Client
$clientShortcut.WorkingDirectory = $InstallRoot
$clientShortcut.Save()

$panelShortcut = $shell.CreateShortcut((Join-Path $StartMenu 'Library Control Panel.lnk'))
$panelShortcut.TargetPath = $Panel
$panelShortcut.WorkingDirectory = $InstallRoot
$panelShortcut.Save()

Write-Host "Todo-en-uno instalado en $InstallRoot" -ForegroundColor Green
Write-Host 'Interfaces: Nimos Library y Library Control Panel.' -ForegroundColor Cyan
