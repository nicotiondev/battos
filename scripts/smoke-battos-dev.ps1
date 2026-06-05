param(
    [string]$ApiUrl = $(if ($env:BATTOS_API_URL) { $env:BATTOS_API_URL } else { "http://localhost:8000" }),
    [switch]$RequireDatabase,
    [switch]$UseGoRun
)

$ErrorActionPreference = "Stop"

function Invoke-SmokeCommand {
    param(
        [string]$Name,
        [string[]]$CommandArgs
    )

    Write-Host ""
    Write-Host "==> $Name"
    $env:BATTOS_NO_BANNER = "1"
    $battosArgs = @("--api", $ApiUrl) + $CommandArgs
    if ($UseGoRun) {
        $env:GOCACHE = (Resolve-Path "data\.cache\go-build").Path
        & go run ./apps/cli/cmd/battos @battosArgs
    } else {
        & battos @battosArgs
    }
    if ($LASTEXITCODE -ne 0) {
        throw "Smoke command failed: battos $($battosArgs -join ' ')"
    }
}

Write-Host "BattOS dev smoke"
Write-Host "API: $ApiUrl"

$status = Invoke-RestMethod -UseBasicParsing -Uri "$ApiUrl/status" -TimeoutSec 5
Write-Host "Status overall: $($status.overall)"

if ($RequireDatabase) {
    $db = $status.subsystems | Where-Object { $_.name -eq "database" } | Select-Object -First 1
    if ($null -eq $db -or $db.status -ne "ok") {
        throw "Database subsystem is not OK"
    }
    Write-Host "Database: OK"
}

Invoke-SmokeCommand -Name "status" -CommandArgs @("status")
Invoke-SmokeCommand -Name "project list" -CommandArgs @("project", "list")
Invoke-SmokeCommand -Name "goal list" -CommandArgs @("goal", "list")
Invoke-SmokeCommand -Name "task list" -CommandArgs @("task", "list")
Invoke-SmokeCommand -Name "knowledge workspace list" -CommandArgs @("knowledge", "workspace", "list")
Invoke-SmokeCommand -Name "memory stats" -CommandArgs @("memory", "stats")

Write-Host ""
Write-Host "BattOS dev smoke passed."
