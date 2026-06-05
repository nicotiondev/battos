param(
    [string]$Image = $(if ($env:BATTOS_EXECUTION_DOCKER_IMAGE) { $env:BATTOS_EXECUTION_DOCKER_IMAGE } else { "battos-runtime-agents:dev" }),
    [string]$CodexVersion = $(if ($env:BATTOS_CODEX_NPM_VERSION) { $env:BATTOS_CODEX_NPM_VERSION } else { "latest" }),
    [string]$ClaudeVersion = $(if ($env:BATTOS_CLAUDE_NPM_VERSION) { $env:BATTOS_CLAUDE_NPM_VERSION } else { "latest" }),
    [switch]$NoSmoke
)

$ErrorActionPreference = "Stop"

Write-Host "BattOS runtime agents image"
Write-Host "Image: $Image"
Write-Host "Codex npm version: $CodexVersion"
Write-Host "Claude npm version: $ClaudeVersion"

docker build `
    -f infra/Dockerfile.runtime-agents `
    --build-arg CODEX_NPM_VERSION=$CodexVersion `
    --build-arg CLAUDE_NPM_VERSION=$ClaudeVersion `
    -t $Image `
    .

if (-not $NoSmoke) {
    Write-Host ""
    Write-Host "==> Verifying CLIs inside runtime image"
    docker run --rm --network none $Image bash -lc "codex --version && claude --version"
}

Write-Host ""
Write-Host "BattOS runtime image ready: $Image"
