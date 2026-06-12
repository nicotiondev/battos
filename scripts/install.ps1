# install.ps1 — instalador de BattOS para Windows
#
# Uso (desde PowerShell elevado o normal):
#   iwr -useb https://raw.githubusercontent.com/nicotiondev/battos/master/scripts/install.ps1 | iex
#
# Variables opcionales (setear antes de correr):
#   $env:BATTOS_VERSION    versión específica, ej. "v1.0.0"  (default: última release)
#   $env:BATTOS_INSTALL_DIR  directorio destino               (default: $HOME\.local\bin)

$ErrorActionPreference = "Stop"

$Repo       = "nicotiondev/battos"
$InstallDir = if ($env:BATTOS_INSTALL_DIR) { $env:BATTOS_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }

# --- Detectar versión ---
$Version = $env:BATTOS_VERSION
if (-not $Version) {
    Write-Host "Obteniendo última versión..." -NoNewline
    try {
        $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $rel.tag_name
    } catch {
        Write-Error "No se pudo obtener la versión. Especificá `$env:BATTOS_VERSION = 'vX.Y.Z'` y reintentá."
    }
    Write-Host " $Version"
}

# --- Construir URL ---
# GoReleaser genera: battos_v1.0.0_windows_amd64.zip
$Arch     = if ([System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE") -eq "ARM64") { "arm64" } `
            elseif ([System.Environment]::Is64BitOperatingSystem) { "amd64" } `
            else { "386" }
$FileVersion = $Version -replace '^v', ''
$FileName = "battos_${FileVersion}_windows_${Arch}.zip"
$Url      = "https://github.com/$Repo/releases/download/$Version/$FileName"

Write-Host "Instalando BattOS $Version para windows/$Arch..."
Write-Host "  Fuente: $Url"

# --- Crear directorio destino ---
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# --- Descargar y extraer ---
$TmpDir = Join-Path $env:TEMP "battos-install-$([System.IO.Path]::GetRandomFileName())"
New-Item -ItemType Directory -Force -Path $TmpDir | Out-Null

try {
    $ZipPath = Join-Path $TmpDir $FileName
    Write-Host "  Descargando..." -NoNewline
    Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
    Write-Host " OK"

    Write-Host "  Extrayendo..." -NoNewline
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force
    Write-Host " OK"

    # --- Instalar binarios ---
    foreach ($bin in @("battos.exe", "battos-api.exe")) {
        $src = Join-Path $TmpDir $bin
        if (Test-Path $src) {
            $dst = Join-Path $InstallDir $bin
            Copy-Item -Path $src -Destination $dst -Force
            Write-Host "  + $bin -> $dst"
        }
    }

    # --- Instalar config de ejemplo si no existe ---
    $ConfigDir  = Join-Path $env:APPDATA "battos"
    $ConfigDest = Join-Path $ConfigDir "battos.yaml"
    $ConfigSrc  = Join-Path $TmpDir "config\battos.yaml"
    if (-not (Test-Path $ConfigDest) -and (Test-Path $ConfigSrc)) {
        New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
        Copy-Item -Path $ConfigSrc -Destination $ConfigDest -Force
        Write-Host "  + config -> $ConfigDest"
    }

} finally {
    Remove-Item -Recurse -Force -Path $TmpDir -ErrorAction SilentlyContinue
}

# --- Agregar al PATH de usuario si no está ---
$UserPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = "$UserPath;$InstallDir"
    [System.Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    Write-Host ""
    Write-Host "  PATH de usuario actualizado. Reabrí la terminal para que tome efecto."
    Write-Host "  O en esta sesión: `$env:PATH += `";$InstallDir`""
} else {
    Write-Host ""
    Write-Host "  $InstallDir ya está en PATH."
}

# Agregar Git bin al PATH de usuario si existe (necesario para que sh esté disponible)
$GitBin = "C:\Program Files\Git\bin"
if ((Test-Path $GitBin) -and ($UserPath -notlike "*$GitBin*")) {
    $Updated = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    [System.Environment]::SetEnvironmentVariable("PATH", "$Updated;$GitBin", "User")
    Write-Host "  Git bin agregado al PATH ($GitBin) — necesario para correr agentes."
}

Write-Host ""
Write-Host "BattOS $Version instalado correctamente." -ForegroundColor Green
Write-Host ""
Write-Host "Para arrancar:"
Write-Host "  battos serve" -ForegroundColor Cyan
Write-Host ""
Write-Host "Agregá tu API key antes del primer uso:"
Write-Host "  battos credentials set openrouter --kind api_key --value sk-..." -ForegroundColor DarkGray
Write-Host "  battos credentials set anthropic  --kind api_key --value sk-ant-..." -ForegroundColor DarkGray
Write-Host ""
Write-Host "Dashboard disponible en http://localhost:8000 al correr battos serve."
