# install.ps1 - instalador de BattOS para Windows
#
# Uso (desde PowerShell):
#   iwr -useb https://raw.githubusercontent.com/nicotiondev/battos/master/scripts/install.ps1 | iex
#
# Variables opcionales:
#   $env:BATTOS_VERSION     version especifica, ej. "v1.0.0"  (default: ultima release)
#   $env:BATTOS_INSTALL_DIR directorio destino                 (default: $HOME\.local\bin)

$ErrorActionPreference = "Stop"

$Repo = "nicotiondev/battos"
$InstallDir = if ($env:BATTOS_INSTALL_DIR) { $env:BATTOS_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }

# Detectar version
$Version = $env:BATTOS_VERSION
if (-not $Version) {
    Write-Host "Obteniendo ultima version..." -NoNewline
    try {
        $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $rel.tag_name
    } catch {
        Write-Error "No se pudo obtener la version. Seteá BATTOS_VERSION=vX.Y.Z y reintenta."
    }
    Write-Host " $Version"
}

# Construir URL - GoReleaser usa 1.0.0 (sin v) en el nombre del archivo
$FileVersion = $Version -replace "^v", ""
$Arch = if ([System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE") -eq "ARM64") {
    "arm64"
} elseif ([System.Environment]::Is64BitOperatingSystem) {
    "amd64"
} else {
    "386"
}
$FileName = "battos_${FileVersion}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$FileName"

Write-Host "Instalando BattOS $Version para windows/$Arch..."
Write-Host "  Fuente: $Url"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$TmpDir = Join-Path $env:TEMP ("battos-install-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Force -Path $TmpDir | Out-Null

try {
    $ZipPath = Join-Path $TmpDir $FileName
    Write-Host "  Descargando..." -NoNewline
    Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
    Write-Host " OK"

    Write-Host "  Extrayendo..." -NoNewline
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force
    Write-Host " OK"

    foreach ($bin in @("battos.exe", "battos-api.exe")) {
        $src = Join-Path $TmpDir $bin
        if (Test-Path $src) {
            $dst = Join-Path $InstallDir $bin
            Copy-Item -Path $src -Destination $dst -Force
            Write-Host "  + $bin -> $dst"
        }
    }

    $ConfigDir = Join-Path $env:APPDATA "battos"
    $ConfigDest = Join-Path $ConfigDir "battos.yaml"
    $ConfigSrc = Join-Path $TmpDir "config\battos.yaml"
    if ((-not (Test-Path $ConfigDest)) -and (Test-Path $ConfigSrc)) {
        New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
        Copy-Item -Path $ConfigSrc -Destination $ConfigDest -Force
        Write-Host "  + config -> $ConfigDest"
    }

} finally {
    Remove-Item -Recurse -Force -Path $TmpDir -ErrorAction SilentlyContinue
}

# Agregar al PATH de usuario si no esta
$UserPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [System.Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    Write-Host ""
    Write-Host "  PATH actualizado. Reabri la terminal para que tome efecto."
} else {
    Write-Host ""
    Write-Host "  $InstallDir ya esta en PATH."
}

# Agregar Git bin si existe (necesario para sh en los agentes)
$GitBin = "C:\Program Files\Git\bin"
if ((Test-Path $GitBin) -and ($UserPath -notlike "*$GitBin*")) {
    $cur = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    [System.Environment]::SetEnvironmentVariable("PATH", "$cur;$GitBin", "User")
    Write-Host "  Git bin agregado al PATH."
}

Write-Host ""
Write-Host "BattOS $Version instalado." -ForegroundColor Green
Write-Host ""
Write-Host "Para arrancar:"
Write-Host "  battos serve" -ForegroundColor Cyan
Write-Host ""
Write-Host "Agregar API key:"
Write-Host "  battos credentials set openrouter --kind api_key --value sk-..."
Write-Host "  battos credentials set anthropic  --kind api_key --value sk-ant-..."
