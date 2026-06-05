param(
    [ValidateSet("codex", "claude-code", "all")]
    [string]$Adapter = "codex",
    [string]$ApiUrl = $(if ($env:BATTOS_API_URL) { $env:BATTOS_API_URL } else { "http://127.0.0.1:8000" }),
    [string]$DatabaseUrl = $(if ($env:DATABASE_URL) { $env:DATABASE_URL } else { "postgresql://battos:change-me@127.0.0.1:5432/battos?sslmode=disable" }),
    [string]$DockerImage = $(if ($env:BATTOS_EXECUTION_DOCKER_IMAGE) { $env:BATTOS_EXECUTION_DOCKER_IMAGE } else { "battos-runtime-agents:dev" }),
    [switch]$SkipImageCheck,
    [switch]$NoArtifactCheck,
    [switch]$KeepGoing
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

function Get-BattOSHeaders {
    $headers = @{}
    if ($env:BATTOS_API_TOKEN) {
        $headers["Authorization"] = "Bearer $($env:BATTOS_API_TOKEN)"
    }
    return $headers
}

function Invoke-BattOSGet {
    param([string]$Path)
    Invoke-RestMethod -UseBasicParsing -Method Get -Uri "$ApiUrl$Path" -Headers (Get-BattOSHeaders) -TimeoutSec 10
}

function Invoke-BattOSPost {
    param([string]$Path, [hashtable]$Body)
    Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl$Path" -Headers (Get-BattOSHeaders) -ContentType "application/json" -Body ($Body | ConvertTo-Json -Depth 8) -TimeoutSec 15
}

function Show-RunLogs {
    param([string]$RunId)
    try {
        $logs = Invoke-BattOSGet -Path "/runs/$RunId/logs"
        Write-Host ""
        Write-Host "---- run logs: $RunId ----"
        foreach ($entry in $logs) {
            Write-Host ("[{0}] {1}" -f $entry.stream, $entry.message)
        }
        Write-Host "---- end logs ----"
    } catch {
        Write-Host "Could not fetch run logs: $($_.Exception.Message)"
    }
}

function Get-AdapterSpec {
    param([string]$Name)
    switch ($Name) {
        "codex" {
            return @{
                Adapter = "codex"
                AgentId = "codex-real-smoke-agent"
                AgentName = "Codex Real Smoke Agent"
                ProviderEnv = "OPENAI_API_KEY"
                VersionCommand = "codex --version"
                Marker = "battos-real-codex-ok"
            }
        }
        "claude-code" {
            return @{
                Adapter = "claude-code"
                AgentId = "claude-code-real-smoke-agent"
                AgentName = "Claude Code Real Smoke Agent"
                ProviderEnv = "ANTHROPIC_API_KEY"
                VersionCommand = "claude --version"
                Marker = "battos-real-claude-ok"
            }
        }
        default {
            throw "Unknown adapter: $Name"
        }
    }
}

function Assert-ProviderKey {
    param([hashtable]$Spec)
    $keyName = $Spec.ProviderEnv
    $keyValue = [Environment]::GetEnvironmentVariable($keyName)
    if ([string]::IsNullOrWhiteSpace($keyValue)) {
        throw "$keyName is required to smoke adapter $($Spec.Adapter)"
    }
}

function Register-SmokeAgent {
    param([hashtable]$Spec)
    $runtimeId = $Spec.Adapter
    $agentId = $Spec.AgentId
    $agentName = $Spec.AgentName.Replace("'", "''")
    $sql1 = "UPDATE agent_runtimes SET status = 'configured' WHERE id = '$runtimeId';"
    $sql2 = "INSERT INTO agents (id, slug, name, role, runtime_id, risk_level, status) VALUES ('$agentId', '$agentId', '$agentName', 'real adapter smoke test', '$runtimeId', 'high', 'active') ON CONFLICT (id) DO UPDATE SET runtime_id = EXCLUDED.runtime_id, status = EXCLUDED.status;"
    docker exec battos-db psql -U battos -d battos -v ON_ERROR_STOP=1 -c $sql1 | Write-Host
    docker exec battos-db psql -U battos -d battos -v ON_ERROR_STOP=1 -c $sql2 | Write-Host
}

function Invoke-RealAdapterSmoke {
    param([hashtable]$Spec)

    $runId = $null
    try {
        Write-Step "Checking provider key for $($Spec.Adapter)"
        Assert-ProviderKey -Spec $Spec

        if (-not $SkipImageCheck) {
            Write-Step "Checking runtime image for $($Spec.Adapter)"
            docker image inspect $DockerImage | Out-Null
            docker run --rm --network none $DockerImage bash -lc $Spec.VersionCommand | Write-Host
        }

        Write-Step "Registering runtime and smoke agent for $($Spec.Adapter)"
        Register-SmokeAgent -Spec $Spec

        Write-Step "Creating project, task and run for $($Spec.Adapter)"
        $stamp = Get-Date -Format "yyyyMMddHHmmss"
        $projectId = "smoke-$($Spec.Adapter)-$stamp"
        $project = Invoke-BattOSPost -Path "/projects" -Body @{
            slug = $projectId
            name = "Smoke $($Spec.Adapter) Adapter"
            status = "active"
        }
        $task = Invoke-BattOSPost -Path "/tasks" -Body @{
            project_id = $project.id
            title = "Run $($Spec.Adapter) real adapter smoke"
            status = "ready"
        }
        $prompt = @"
This is a BattOS real adapter smoke test.

Do exactly this inside the current workspace:
1. Create the directory outputs.
2. Create a Markdown file at outputs/adapter-smoke.md.
3. The file must contain the marker $($Spec.Marker).
4. Print the marker $($Spec.Marker) to stdout.

Keep the run short. Do not inspect host files. Do not install dependencies.
"@
        $run = Invoke-BattOSPost -Path "/runs" -Body @{
            project_id = $project.id
            task_id = $task.id
            agent_id = $Spec.AgentId
            runtime_adapter_id = $Spec.Adapter
            prompt = $prompt
            requested_network = $true
        }
        $runId = $run.id
        Write-Host "Run created: $runId"

        Write-Step "Approving network and execute for $($Spec.Adapter)"
        $networkApproval = Invoke-BattOSPost -Path "/runs/$runId/approvals" -Body @{
            kind = "network"
            decision = "approved"
            reason = "real adapter smoke requires provider API access"
        }
        if (-not $networkApproval.run.network_enabled) {
            throw "Network approval did not enable network"
        }
        $executeApproval = Invoke-BattOSPost -Path "/runs/$runId/approvals" -Body @{
            kind = "execute"
            decision = "approved"
            reason = "real adapter smoke"
        }
        if ($executeApproval.run.status -ne "queued") {
            throw "Run was not queued after execute approval: $($executeApproval.run.status)"
        }

        Write-Step "Processing run with worker Docker sandbox"
        $env:DATABASE_URL = $DatabaseUrl
        $env:BATTOS_EXECUTION_SANDBOX_MODE = "docker"
        $env:BATTOS_EXECUTION_DOCKER_IMAGE = $DockerImage
        $goCachePath = "data\.cache\go-build"
        New-Item -ItemType Directory -Force -Path $goCachePath | Out-Null
        $env:GOCACHE = (Resolve-Path $goCachePath).Path
        $workerBinDir = "data\.cache\dev-bin"
        New-Item -ItemType Directory -Force -Path $workerBinDir | Out-Null
        $workerBin = Join-Path (Resolve-Path $workerBinDir).Path "battos-worker-dev.exe"
        go build -o $workerBin ./apps/api/cmd/worker
        powershell -ExecutionPolicy Bypass -File .\scripts\sign-battos-dev.ps1 -ExePath $workerBin | Write-Host
        & $workerBin -once -run-id $run.id | Write-Host

        Write-Step "Validating run result for $($Spec.Adapter)"
        $result = Invoke-BattOSGet -Path "/runs/$runId"
        if ($result.status -ne "succeeded") {
            throw "Run status = $($result.status), want succeeded. Error: $($result.error_message)"
        }
        $logs = Invoke-BattOSGet -Path "/runs/$runId/logs"
        $logText = ($logs | ConvertTo-Json -Depth 8)
        if ($logText -notmatch "network: enabled by approval") {
            throw "Run logs did not include expected approved network state"
        }
        if ($logText -notmatch [regex]::Escape($Spec.Marker)) {
            throw "Run logs did not include expected marker $($Spec.Marker)"
        }

        if (-not $NoArtifactCheck) {
            $artifacts = Invoke-BattOSGet -Path "/artifacts?project_id=$($project.id)"
            $smokeArtifact = @($artifacts | Where-Object { $_.run_id -eq $runId -and $_.name -eq "outputs/adapter-smoke.md" }) | Select-Object -First 1
            if ($null -eq $smokeArtifact) {
                throw "Run artifact outputs/adapter-smoke.md was not registered"
            }
            if ([string]::IsNullOrWhiteSpace($smokeArtifact.managed_path)) {
                throw "Run artifact did not include a managed_path"
            }
            $artifactPath = Join-Path (Resolve-Path "data\artifacts").Path $smokeArtifact.managed_path
            if (-not (Test-Path -LiteralPath $artifactPath)) {
                throw "Managed artifact file was not written: $artifactPath"
            }
            $artifactContent = Get-Content -LiteralPath $artifactPath -Raw
            if ($artifactContent -notmatch [regex]::Escape($Spec.Marker)) {
                throw "Managed artifact did not include expected marker $($Spec.Marker)"
            }
        }

        Write-Host ""
        Write-Host "BattOS real adapter smoke passed: $($Spec.Adapter)"
        Write-Host "Run ID: $runId"
    } catch {
        Write-Host ""
        Write-Host "BattOS real adapter smoke failed: $($Spec.Adapter)"
        if ($runId) {
            Show-RunLogs -RunId $runId
        }
        if ($KeepGoing) {
            Write-Host "Continuing because -KeepGoing was provided."
            return
        }
        throw
    }
}

Write-Host "BattOS real adapter smoke"
Write-Host "API: $ApiUrl"
Write-Host "DB: $DatabaseUrl"
Write-Host "Docker image: $DockerImage"
Write-Host "Adapter: $Adapter"

Write-Step "Checking API status"
$status = Invoke-BattOSGet -Path "/status"
if ($status.overall -ne "ok") {
    throw "BattOS API is not OK: $($status.overall)"
}
$db = $status.subsystems | Where-Object { $_.name -eq "database" } | Select-Object -First 1
if ($null -eq $db -or $db.status -ne "ok") {
    throw "Database subsystem is not OK"
}
Write-Host "API and DB are OK"

Write-Step "Checking migrations"
goose -dir apps/api/migrations postgres $DatabaseUrl up | Write-Host

Write-Step "Checking Docker"
docker info --format "{{.ServerVersion}}" | Write-Host
if (Test-Path "infra\.env") {
    $runningComposeServices = @(docker compose -f infra/docker-compose.yml --env-file infra/.env ps --status running --services 2>$null)
    if ($LASTEXITCODE -eq 0 -and $runningComposeServices -contains "battos-worker") {
        throw "battos-worker Compose is running and may claim the adapter smoke run first. Stop it or run it in DockerSandbox mode before this smoke: docker compose -f infra/docker-compose.yml --env-file infra/.env stop battos-worker"
    }
}

$adapters = @()
if ($Adapter -eq "all") {
    $adapters = @("codex", "claude-code")
} else {
    $adapters = @($Adapter)
}

foreach ($adapterName in $adapters) {
    Invoke-RealAdapterSmoke -Spec (Get-AdapterSpec -Name $adapterName)
}

Write-Host ""
Write-Host "BattOS real adapter smoke finished."
