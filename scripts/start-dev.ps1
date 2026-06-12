# start-dev.ps1 — arranca BattOS en modo desarrollo (Windows)
#
# Levanta dos procesos:
#   1. api.exe       — HTTP server en :8000
#   2. worker2.exe   — worker daemon en modo polling (-once=false)
#
# Requisitos:
#   - Git for Windows instalado (para que `sh` sea visible en PATH)
#   - api.exe y worker2.exe ya buildeados (go build ./apps/api/cmd/api y ./apps/api/cmd/worker)
#   - Si usas Nova: setear OPENROUTER_API_KEY antes de correr este script
#
# Uso:
#   .\scripts\start-dev.ps1
#   .\scripts\start-dev.ps1 -OpenBrowser   # abre el frontend en :3000 tambien

param(
    [switch]$OpenBrowser
)

$Root = Split-Path -Parent $PSScriptRoot

# Agregar Git bin al PATH para que sh sea visible por el worker (necesario en Windows)
$GitBin = "C:\Program Files\Git\bin"
if (Test-Path $GitBin) {
    $env:PATH = "$GitBin;$env:PATH"
    Write-Host "  Git sh: $GitBin/sh.exe agregado al PATH"
} else {
    Write-Warning "Git for Windows no encontrado en $GitBin — los adapters sh pueden fallar."
}

# Verificar binarios
$ApiExe    = Join-Path $Root "api.exe"
$WorkerExe = Join-Path $Root "worker2.exe"

if (-not (Test-Path $ApiExe)) {
    Write-Error "api.exe no encontrado. Buildear con: go build -o api.exe .\apps\api\cmd\api"
    exit 1
}
if (-not (Test-Path $WorkerExe)) {
    Write-Error "worker2.exe no encontrado. Buildear con: go build -o worker2.exe .\apps\api\cmd\worker"
    exit 1
}

# Crear directorio de logs si no existe
$LogDir = Join-Path $Root "data\logs"
New-Item -ItemType Directory -Force $LogDir | Out-Null

Write-Host ""
Write-Host "BattOS dev — arrancando..." -ForegroundColor Cyan

# Matar instancias anteriores si existen
Get-Process -Name "api"     -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "worker2" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500

# Arrancar API
$ApiProc = Start-Process -FilePath $ApiExe `
    -RedirectStandardOutput "$LogDir\api.log" `
    -RedirectStandardError  "$LogDir\api-err.log" `
    -PassThru
Write-Host "  API      PID $($ApiProc.Id) — http://localhost:8000" -ForegroundColor Green

# Arrancar worker en modo daemon
$WorkerProc = Start-Process -FilePath $WorkerExe `
    -ArgumentList @("-once=false") `
    -RedirectStandardOutput "$LogDir\worker.log" `
    -RedirectStandardError  "$LogDir\worker-err.log" `
    -PassThru
Write-Host "  Worker   PID $($WorkerProc.Id) — polling cada 2s" -ForegroundColor Green

# Esperar a que la API responda
Write-Host ""
Write-Host "  Esperando API..." -NoNewline
$ready = $false
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Milliseconds 500
    try {
        $resp = Invoke-RestMethod -Uri "http://localhost:8000/health" -ErrorAction Stop
        if ($resp.status -eq "ok") { $ready = $true; break }
    } catch {}
    Write-Host "." -NoNewline
}

if ($ready) {
    Write-Host " OK" -ForegroundColor Green
} else {
    Write-Warning "API no respondio en 7.5s — revisar data\logs\api-err.log"
}

Write-Host ""
Write-Host "  Logs:    data\logs\api.log  |  data\logs\worker.log"
Write-Host "  API:     http://localhost:8000/health"
Write-Host ""
Write-Host "Para el frontend (desarrollo):" -ForegroundColor Yellow
Write-Host "  cd apps\web; npm run dev    -> http://localhost:3000"
Write-Host ""
Write-Host "Presiona Ctrl+C para terminar los procesos." -ForegroundColor DarkGray

if ($OpenBrowser) {
    Start-Process "http://localhost:3000"
}

# Mantener el script vivo hasta Ctrl+C, y al salir matar los procesos hijos
try {
    while ($true) { Start-Sleep -Seconds 5 }
} finally {
    Write-Host ""
    Write-Host "Deteniendo BattOS..." -ForegroundColor Yellow
    $ApiProc    | Stop-Process -Force -ErrorAction SilentlyContinue
    $WorkerProc | Stop-Process -Force -ErrorAction SilentlyContinue
    Write-Host "Listo." -ForegroundColor Green
}
