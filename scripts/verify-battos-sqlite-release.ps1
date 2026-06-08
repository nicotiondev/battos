param(
    [int]$Port = 8030,
    [switch]$SkipWebBuild,
    [switch]$CheckDocker,
    [switch]$CheckMemoryDocker,
    [switch]$CheckRealAdapters,
    [switch]$CheckHostSessionAdapters,
    [ValidateSet("codex", "claude-code", "all")]
    [string]$RealAdapter = "all",
    [string]$CodexCredentialsDir = $(if ($env:BATTOS_EXECUTION_CODEX_CREDENTIALS_DIR) { $env:BATTOS_EXECUTION_CODEX_CREDENTIALS_DIR } else { Join-Path $env:USERPROFILE ".codex" }),
    [string]$ClaudeCredentialsDir = $(if ($env:BATTOS_EXECUTION_CLAUDE_CREDENTIALS_DIR) { $env:BATTOS_EXECUTION_CLAUDE_CREDENTIALS_DIR } else { Join-Path $env:USERPROFILE ".claude" }),
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

function Stop-PortProcess {
    param([int]$PortNumber)
    $listeners = netstat -ano | Select-String ":$PortNumber\s+.*LISTENING"
    foreach ($listener in $listeners) {
        $parts = ($listener.Line -split "\s+") | Where-Object { $_ -ne "" }
        $pidText = $parts[-1]
        if ($pidText -match "^\d+$") {
            Stop-Process -Id ([int]$pidText) -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-Required {
    param(
        [string]$Name,
        [scriptblock]$Block
    )
    Write-Step $Name
    & $Block
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$stamp = Get-Date -Format "yyyyMMddHHmmss"
$dbRelPath = "data\.cache\release-verify\$stamp\battos.db"
$dbPath = Join-Path $repoRoot $dbRelPath
$apiUrl = "http://127.0.0.1:$Port"

Write-Host "BattOS SQLite release verifier"
Write-Host "API: $apiUrl"
Write-Host "DB:  $dbPath"

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dbPath) | Out-Null
New-Item -ItemType Directory -Force -Path "data\.cache\go-build" | Out-Null

try {
    Invoke-Required -Name "Go tests" -Block {
        go test ./apps/api/... ./apps/cli/... ./packages/core/...
    }

    Invoke-Required -Name "Go build API" -Block {
        go build ./apps/api/cmd/api
    }

    Invoke-Required -Name "Go build worker" -Block {
        go build ./apps/api/cmd/worker
    }

    Invoke-Required -Name "Go build CLI" -Block {
        go build ./apps/cli/cmd/battos
    }

    Invoke-Required -Name "Web lint" -Block {
        Push-Location apps/web
        try { npm run lint } finally { Pop-Location }
    }

    Invoke-Required -Name "Web API types" -Block {
        Push-Location apps/web
        try { npm run check:api-types } finally { Pop-Location }
    }

    if (-not $SkipWebBuild) {
        Invoke-Required -Name "Web build" -Block {
            Push-Location apps/web
            try { npm run build } finally { Pop-Location }
        }
    } else {
        Write-Host ""
        Write-Host "==> Web build"
        Write-Host "    SKIP requested"
    }

    Invoke-Required -Name "Docker Compose config" -Block {
        docker compose -f infra/docker-compose.yml config | Out-Null
        docker compose -f infra/docker-compose.yml --profile worker config | Out-Null
    }

    Write-Step "Start API on fresh SQLite"
    powershell -ExecutionPolicy Bypass -File .\scripts\start-battos-api-dev.ps1 `
        -Port $Port `
        -DatabasePath $dbRelPath `
        -StopExisting `
        -Background `
        -Wait `
        -TimeoutSeconds $TimeoutSeconds

    Invoke-Required -Name "Dev smoke" -Block {
        powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-dev.ps1 `
            -ApiUrl $apiUrl `
            -RequireDatabase `
            -UseGoRun
    }

    Invoke-Required -Name "Web/API smoke" -Block {
        powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-web.ps1 `
            -ApiUrl $apiUrl `
            -RequireDatabase `
            -CheckSSE
    }

    Write-Step "Dry-run lifecycle smoke"
    $lifecycleStamp = Get-Date -Format "yyyyMMddHHmmss"
    $agentId = "dryrun-smoke-agent-$lifecycleStamp"
    $agent = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$apiUrl/agents" -ContentType "application/json" -Body (@{
        slug = $agentId
        name = "DryRun Smoke Agent"
        role = "worker dry run smoke"
        runtime_id = "sandbox-smoke"
        risk_level = "low"
        status = "active"
    } | ConvertTo-Json)
    $project = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$apiUrl/projects" -ContentType "application/json" -Body (@{
        slug = "dryrun-smoke-$lifecycleStamp"
        name = "DryRun Smoke"
        status = "active"
    } | ConvertTo-Json)
    $task = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$apiUrl/tasks" -ContentType "application/json" -Body (@{
        project_id = $project.id
        title = "Run dry-run lifecycle smoke"
        status = "ready"
    } | ConvertTo-Json)
    $run = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$apiUrl/runs" -ContentType "application/json" -Body (@{
        project_id = $project.id
        task_id = $task.id
        agent_id = $agent.id
        runtime_adapter_id = "sandbox-smoke"
        prompt = "Validate BattOS dry-run lifecycle on SQLite"
        requested_network = $false
    } | ConvertTo-Json)
    $approval = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$apiUrl/runs/$($run.id)/approvals" -ContentType "application/json" -Body (@{
        kind = "execute"
        decision = "approved"
        reason = "sqlite release verifier"
    } | ConvertTo-Json)
    if ($approval.run.status -ne "queued") {
        throw "Run was not queued after approval: $($approval.run.status)"
    }

    $env:BATTOS_DATABASE_PATH = $dbPath
    $env:BATTOS_EXECUTION_SANDBOX_MODE = "dry_run"
    $env:GOCACHE = (Resolve-Path "data\.cache\go-build").Path
    go run ./apps/api/cmd/worker -once -run-id $run.id | Write-Host
    if ($LASTEXITCODE -ne 0) {
        throw "dry-run worker failed with exit code $LASTEXITCODE"
    }

    $current = Invoke-RestMethod -UseBasicParsing -Uri "$apiUrl/runs/$($run.id)" -TimeoutSec 5
    if ($current.status -ne "succeeded") {
        throw "Run status = $($current.status), want succeeded"
    }
    $logs = Invoke-RestMethod -UseBasicParsing -Uri "$apiUrl/runs/$($run.id)/logs" -TimeoutSec 5
    $logText = $logs | ConvertTo-Json -Depth 8
    if ($logText -notmatch "sandbox dry-run" -or $logText -notmatch "network: disabled") {
        throw "Run logs did not include dry-run/network markers"
    }
    Write-Host "Dry-run lifecycle passed. Run ID: $($run.id)"

    if ($CheckDocker -or $CheckMemoryDocker) {
        Invoke-Required -Name "DockerSandbox smoke" -Block {
            powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-docker-run.ps1 -ApiUrl $apiUrl
        }
    }

    if ($CheckMemoryDocker) {
        Invoke-Required -Name "Memory Bridge DockerSandbox smoke" -Block {
            powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-docker-run.ps1 -ApiUrl $apiUrl -RequireMemoryContext
        }
    }

    if ($CheckRealAdapters) {
        Invoke-Required -Name "Real adapter smoke ($RealAdapter)" -Block {
            powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-real-adapter-run.ps1 `
                -ApiUrl $apiUrl `
                -Adapter $RealAdapter
        }
    }

    if ($CheckHostSessionAdapters) {
        Invoke-Required -Name "Codex host_session smoke" -Block {
            powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-codex-host-session-run.ps1 `
                -ApiUrl $apiUrl `
                -DatabasePath $dbPath `
                -CodexCredentialsDir $CodexCredentialsDir
        }
        Invoke-Required -Name "Claude host_session smoke" -Block {
            powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-claude-host-session-run.ps1 `
                -ApiUrl $apiUrl `
                -DatabasePath $dbPath `
                -ClaudeCredentialsDir $ClaudeCredentialsDir
        }
    }

    Write-Host ""
    Write-Host "BattOS SQLite release verifier passed."
    Write-Host "DB: $dbPath"
} finally {
    Stop-PortProcess -PortNumber $Port
}
