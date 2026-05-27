$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot

Push-Location (Join-Path $root "apps/api")
try {
    sqlc generate
} finally {
    Pop-Location
}

Write-Host "Generated sqlc store sources."
