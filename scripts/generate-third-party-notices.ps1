param(
  [string]$OutputPath = '',
  [string[]]$BinaryPaths = @()
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
if (-not $OutputPath) {
  $OutputPath = Join-Path $Root 'nimos-library\public\THIRD-PARTY-NOTICES.txt'
}

$components = @{}
$missingLicenses = [System.Collections.Generic.List[string]]::new()

function Get-LicenseFiles([string]$Dir) {
  if (-not $Dir -or -not (Test-Path -LiteralPath $Dir -PathType Container)) { return @() }
  return @(Get-ChildItem -LiteralPath $Dir -File | Where-Object {
    $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)(\.|$|[-_])'
  } | Sort-Object Name)
}

function Add-Component(
  [string]$Kind,
  [string]$Name,
  [string]$Version,
  [string]$Source,
  [string]$Dir
) {
  if (-not $Name -or $Name -eq 'translate-wrap' -or $Name -like 'github.com/andresgv-beep/*') { return }
  if (-not $Version) { $Version = 'version not recorded' }
  $key = "$Name@$Version"
  if ($components.ContainsKey($key)) { return }
  $licenses = @(Get-LicenseFiles $Dir)
  $components[$key] = [pscustomobject]@{
    Kind = $Kind
    Name = $Name
    Version = $Version
    Source = $Source
    Licenses = $licenses
  }
  if ($licenses.Count -eq 0) { $missingLicenses.Add($key) }
}

function ConvertTo-GoCacheName([string]$Value) {
  $builder = [System.Text.StringBuilder]::new()
  foreach ($char in $Value.ToCharArray()) {
    if ([char]::IsUpper($char)) {
      [void]$builder.Append('!')
      [void]$builder.Append([char]::ToLowerInvariant($char))
    } else {
      [void]$builder.Append($char)
    }
  }
  return $builder.ToString()
}

$goRoot = (& go env GOROOT).Trim()
if ($LASTEXITCODE -ne 0) { throw 'No se pudo localizar la instalacion de Go' }
Add-Component 'Go toolchain' 'Go standard library and runtime' (& go version).Replace('go version ', '') 'https://go.dev/' $goRoot

$goModuleCache = (& go env GOMODCACHE).Trim()
foreach ($relativeBinary in $BinaryPaths) {
  $binary = if ([IO.Path]::IsPathRooted($relativeBinary)) { $relativeBinary } else { Join-Path $Root $relativeBinary }
  if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw "No existe el binario para auditar: $binary" }
  $moduleLines = @(& go version -m $binary)
  if ($LASTEXITCODE -ne 0) { throw "No se pudieron leer los modulos de $binary" }
  foreach ($line in $moduleLines) {
    if ($line -notmatch '^\s*(mod|dep)\s+([^\s]+)\s+([^\s]+)') { continue }
    $moduleName = $Matches[2]
    $moduleVersion = $Matches[3]
    $cacheDir = Join-Path $goModuleCache ((ConvertTo-GoCacheName $moduleName) + '@' + (ConvertTo-GoCacheName $moduleVersion))
    Add-Component 'Go binary module' $moduleName $moduleVersion $moduleName $cacheDir
  }
}

$npmRoot = Join-Path $Root 'nimos-library'
foreach ($packageName in @('svelte', 'pdfjs-dist')) {
  $packageJson = Join-Path $npmRoot "node_modules\$packageName\package.json"
  if (-not (Test-Path -LiteralPath $packageJson -PathType Leaf)) { throw "No esta instalado el paquete npm $packageName" }
  $packageVersion = (& node -p "require(process.argv[1]).version" $packageJson).Trim()
  if ($LASTEXITCODE -ne 0 -or -not $packageVersion) { throw "No se pudo leer la version npm de $packageName" }
  Add-Component 'Bundled npm package' $packageName $packageVersion "https://www.npmjs.com/package/$packageName" (Join-Path $npmRoot "node_modules\$packageName")
}

$mapLibreFile = Join-Path $Root 'library-server\core\maps-www\vendor\maplibre-gl.js'
$mapLibreHeader = Get-Content -LiteralPath $mapLibreFile -TotalCount 8 | Out-String
$mapLibreVersion = if ($mapLibreHeader -match '/v([^/]+)/LICENSE') { $Matches[1] } else { 'vendored version' }
Add-Component 'Vendored browser library' 'MapLibre GL JS' $mapLibreVersion 'https://github.com/maplibre/maplibre-gl-js' (Join-Path $Root 'library-server\core\maps-www\vendor\licenses\maplibre')
Add-Component 'Vendored browser library' 'PMTiles for JavaScript' 'vendored version' 'https://github.com/protomaps/PMTiles' (Join-Path $Root 'library-server\core\maps-www\vendor\licenses\pmtiles')

$builder = [System.Text.StringBuilder]::new()
[void]$builder.AppendLine('NIMOS LIBRARY - THIRD-PARTY NOTICES')
[void]$builder.AppendLine('===================================')
[void]$builder.AppendLine()
[void]$builder.AppendLine('This file is generated from the packaged browser libraries and compiled Go binaries.')
[void]$builder.AppendLine('Do not edit it by hand. Run scripts/generate-third-party-notices.ps1 instead.')
[void]$builder.AppendLine()
[void]$builder.AppendLine('OpenStreetMap data used by downloaded maps is available under the ODbL 1.0:')
[void]$builder.AppendLine('https://www.openstreetmap.org/copyright')
[void]$builder.AppendLine()

$ordered = @($components.Values | Sort-Object Kind, Name, Version)
[void]$builder.AppendLine("Components recorded: $($ordered.Count)")
[void]$builder.AppendLine()

foreach ($component in $ordered) {
  [void]$builder.AppendLine(('=' * 78))
  [void]$builder.AppendLine("$($component.Name) $($component.Version)")
  [void]$builder.AppendLine("Type: $($component.Kind)")
  [void]$builder.AppendLine("Source/module: $($component.Source)")
  if ($component.Licenses.Count -eq 0) {
    [void]$builder.AppendLine('License text: REVIEW REQUIRED - no local license file was found.')
    [void]$builder.AppendLine()
    continue
  }
  foreach ($license in $component.Licenses) {
    [void]$builder.AppendLine()
    [void]$builder.AppendLine("--- $($license.Name) ---")
    [void]$builder.AppendLine()
    [void]$builder.AppendLine((Get-Content -Raw -LiteralPath $license.FullName).Trim())
    [void]$builder.AppendLine()
  }
}

if ($missingLicenses.Count -gt 0) {
  [void]$builder.AppendLine(('=' * 78))
  [void]$builder.AppendLine('REVIEW QUEUE: COMPONENTS WITHOUT A LOCAL LICENSE FILE')
  [void]$builder.AppendLine()
  foreach ($item in @($missingLicenses | Sort-Object)) { [void]$builder.AppendLine("- $item") }
  [void]$builder.AppendLine()
}

$outputDir = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
[IO.File]::WriteAllText($OutputPath, $builder.ToString(), [Text.UTF8Encoding]::new($false))
Write-Host "Third-party notices: $($ordered.Count) components -> $OutputPath" -ForegroundColor Green
if ($missingLicenses.Count -gt 0) {
  Write-Warning "$($missingLicenses.Count) components need a manual license review; see the end of the generated file."
}
