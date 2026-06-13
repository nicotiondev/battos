# build-web.ps1 - compila el dashboard Next.js y lo embebe en el binario Go
#
# Uso:
#   .\scripts\build-web.ps1
#
# Resultado: apps/api/internal/static/out/ actualizado con el build de produccion.
# Luego compilar el binario: go build -o battos-api.exe .\apps\api\cmd\api

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

Write-Host "1. Building Next.js dashboard..." -ForegroundColor Cyan
Push-Location (Join-Path $Root "apps\web")
try {
    npm run build
} finally {
    Pop-Location
}

Write-Host "2. Copying to Go embed directory..." -ForegroundColor Cyan
$Src = Join-Path $Root "apps\web\out"
$Dst = Join-Path $Root "apps\api\internal\static\out"
Remove-Item -Recurse -Force $Dst -ErrorAction SilentlyContinue
Copy-Item -Recurse -Force $Src $Dst
$Count = (Get-ChildItem $Dst -Recurse -File).Count
Write-Host "   $Count archivos copiados a $Dst"

Write-Host "3. Rebuilding battos-api..." -ForegroundColor Cyan
Push-Location $Root
try {
    go build -o (Join-Path $Root "battos-api.exe") .\apps\api\cmd\api
} finally {
    Pop-Location
}

Write-Host ""
Write-Host "Listo. Dashboard embebido en battos-api.exe" -ForegroundColor Green
Write-Host "Reinicia battos serve para ver los cambios."
