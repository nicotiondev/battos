# smoke-battos-egress.ps1 — valida el proxy de egress de BattOS (ADR-0022 Step C).
#
# REQUISITOS:
#   - Docker Desktop / daemon corriendo y accesible en PATH.
#   - El repo compilado (o disponible para compilar la imagen con docker build).
#
# Qué hace:
#   1. Construye la imagen battos-egress-proxy desde infra/Dockerfile.egress-proxy.
#   2. Crea la red Docker interna battos-egress (si no existe).
#   3. Arranca el proxy en modo enforce con la allowlist de test.
#   4. Lanza un contenedor de test en la red battos-egress con HTTPS_PROXY seteado.
#   5. Verifica que una petición a un dominio PERMITIDO tiene éxito.
#   6. Verifica que una petición a un dominio BLOQUEADO es rechazada (403 o connection refused).
#   7. Limpia contenedores y red.
#
# Uso desde la raíz del repo:
#   .\scripts\smoke-battos-egress.ps1
#
# Variables de entorno opcionales:
#   BATTOS_EGRESS_PROXY_IMAGE  nombre de imagen a buildear (default: battos-egress-proxy:smoke)
#   BATTOS_EGRESS_ALLOWED_HOST dominio permitido para el test (default: httpbin.org)
#   BATTOS_EGRESS_BLOCKED_HOST dominio bloqueado para el test (default: example.com)

param(
    [string]$ProxyImage   = $(if ($env:BATTOS_EGRESS_PROXY_IMAGE)  { $env:BATTOS_EGRESS_PROXY_IMAGE }  else { "battos-egress-proxy:smoke" }),
    [string]$AllowedHost  = $(if ($env:BATTOS_EGRESS_ALLOWED_HOST) { $env:BATTOS_EGRESS_ALLOWED_HOST } else { "httpbin.org" }),
    [string]$BlockedHost  = $(if ($env:BATTOS_EGRESS_BLOCKED_HOST) { $env:BATTOS_EGRESS_BLOCKED_HOST } else { "example.com" })
)

$ErrorActionPreference = "Stop"

$NetworkName     = "battos-egress-smoke"
$ProxyContainer  = "battos-egress-proxy-smoke"
$RunnerContainer = "battos-egress-runner-smoke"
$ProxyPort       = "18888"  # puerto del host para acceder al proxy desde fuera (solo para build check)

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

function Cleanup {
    Write-Host "Limpiando contenedores y red del smoke..."
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    docker rm -f $ProxyContainer  2>$null | Out-Null
    docker rm -f $RunnerContainer 2>$null | Out-Null
    docker network rm $NetworkName 2>$null | Out-Null
    $ErrorActionPreference = $prev
}

# Registrar limpieza al finalizar (éxito o error).
trap {
    Cleanup
    Write-Host ""
    Write-Host "Smoke FALLIDO: $_"
    exit 1
}

Write-Host "BattOS egress proxy smoke (ADR-0022)"
Write-Host "Proxy image  : $ProxyImage"
Write-Host "Allowed host : $AllowedHost"
Write-Host "Blocked host : $BlockedHost"

# --- 1. Verificar Docker ---
Write-Step "Verificando Docker"
$prevEAP = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$dockerInfo = docker info --format "{{.ServerVersion}}" 2>&1
$dockerExit = $LASTEXITCODE
$ErrorActionPreference = $prevEAP
if ($dockerExit -ne 0) {
    throw "Docker no disponible. Iniciar Docker Desktop/daemon y reintentar. Detalle: $dockerInfo"
}
Write-Host "Docker server: $dockerInfo"

# --- 2. Build de la imagen del proxy ---
Write-Step "Construyendo imagen del proxy ($ProxyImage)"
docker build -f infra/Dockerfile.egress-proxy -t $ProxyImage .
if ($LASTEXITCODE -ne 0) { throw "Build de $ProxyImage falló" }
Write-Host "Imagen $ProxyImage lista."

# --- 3. Limpiar restos de runs previos ---
Cleanup

# --- 4. Crear red interna de smoke ---
Write-Step "Creando red interna $NetworkName"
docker network create --internal $NetworkName
if ($LASTEXITCODE -ne 0) { throw "No se pudo crear la red $NetworkName" }

# --- 5. Arrancar el proxy ---
# El proxy necesita salida a internet: adjuntamos el bridge por defecto además de la red interna.
# En compose esto se logra con la red "default"; en el smoke lo hacemos con --network bridge
# seguido de --network connect.
Write-Step "Arrancando proxy ($ProxyContainer) en modo enforce"
docker run -d --name $ProxyContainer `
    --network bridge `
    -e "BATTOS_EGRESS_MODE=enforce" `
    -e "BATTOS_EGRESS_ALLOWLIST=$AllowedHost" `
    -e "BATTOS_EGRESS_ADDR=0.0.0.0:8888" `
    $ProxyImage
if ($LASTEXITCODE -ne 0) { throw "No se pudo arrancar $ProxyContainer" }

# Conectar también a la red interna del smoke para que los runners lo alcancen.
docker network connect $NetworkName $ProxyContainer
if ($LASTEXITCODE -ne 0) { throw "No se pudo conectar $ProxyContainer a $NetworkName" }

# Esperar a que el proxy esté listo.
Write-Host "Esperando al proxy..."
Start-Sleep -Seconds 2

# Obtener la IP del proxy en la red interna.
$ProxyIP = docker inspect -f "{{``.NetworkSettings.Networks.$NetworkName.IPAddress``}}" $ProxyContainer
if ([string]::IsNullOrWhiteSpace($ProxyIP)) {
    throw "No se pudo obtener la IP del proxy en la red $NetworkName"
}
Write-Host "Proxy IP en $NetworkName : $ProxyIP"
$ProxyAddr = "${ProxyIP}:8888"

# --- 6. Test: dominio PERMITIDO debe alcanzarse ---
Write-Step "Test ALLOW: $AllowedHost (debe conectar via proxy)"
$prevEAP = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$allowOut = docker run --rm --name $RunnerContainer `
    --network $NetworkName `
    -e "HTTPS_PROXY=http://$ProxyAddr" `
    -e "HTTP_PROXY=http://$ProxyAddr" `
    -e "NO_PROXY=localhost,127.0.0.1" `
    alpine:3.20 `
    sh -c "apk add --no-cache curl -q 2>/dev/null; curl -sf --max-time 10 http://$AllowedHost/ -o /dev/null && echo ALLOW_OK || echo ALLOW_FAIL" 2>&1
$allowExit = $LASTEXITCODE
$ErrorActionPreference = $prevEAP
Write-Host "Salida ALLOW: $allowOut (exit: $allowExit)"
if ($allowOut -notmatch "ALLOW_OK") {
    throw "Test ALLOW falló: el dominio $AllowedHost debería ser accesible via proxy."
}
Write-Host "ALLOW OK: $AllowedHost alcanzable via proxy."

# --- 7. Test: dominio BLOQUEADO debe ser rechazado ---
Write-Step "Test BLOCK: $BlockedHost (debe ser rechazado por proxy)"
$prevEAP = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$blockOut = docker run --rm --name $RunnerContainer `
    --network $NetworkName `
    -e "HTTPS_PROXY=http://$ProxyAddr" `
    -e "HTTP_PROXY=http://$ProxyAddr" `
    -e "NO_PROXY=localhost,127.0.0.1" `
    alpine:3.20 `
    sh -c "apk add --no-cache curl -q 2>/dev/null; curl -sf --max-time 10 http://$BlockedHost/ -o /dev/null && echo BLOCK_FAIL || echo BLOCK_OK" 2>&1
$blockExit = $LASTEXITCODE
$ErrorActionPreference = $prevEAP
Write-Host "Salida BLOCK: $blockOut (exit: $blockExit)"
if ($blockOut -notmatch "BLOCK_OK") {
    throw "Test BLOCK falló: el dominio $BlockedHost debería ser rechazado por el proxy."
}
Write-Host "BLOCK OK: $BlockedHost correctamente rechazado."

# --- 8. Test: conexión directa sin proxy falla (la red es internal) ---
Write-Step "Test RED INTERNA: conexión directa a internet debe fallar (sin proxy)"
$prevEAP = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$directOut = docker run --rm --name $RunnerContainer `
    --network $NetworkName `
    alpine:3.20 `
    sh -c "apk add --no-cache curl -q 2>/dev/null; curl -sf --max-time 5 http://$AllowedHost/ -o /dev/null && echo DIRECT_FAIL || echo DIRECT_OK" 2>&1
$directExit = $LASTEXITCODE
$ErrorActionPreference = $prevEAP
Write-Host "Salida DIRECTO: $directOut (exit: $directExit)"
if ($directOut -notmatch "DIRECT_OK") {
    throw "Test RED INTERNA falló: la conexión directa debería fallar en una red internal."
}
Write-Host "RED INTERNA OK: conexión directa rechazada (sin ruta a internet)."

# --- Limpieza final ---
Cleanup

Write-Host ""
Write-Host "BattOS egress proxy smoke pasó."
Write-Host "  ALLOW  : $AllowedHost accesible via proxy en modo enforce."
Write-Host "  BLOCK  : $BlockedHost bloqueado por proxy."
Write-Host "  INTERNO: conexión directa sin proxy falla por falta de ruta."
