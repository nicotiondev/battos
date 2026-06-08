param(
    [string]$ApiUrl = $(if ($env:BATTOS_API_URL) { $env:BATTOS_API_URL } else { "http://127.0.0.1:8000" }),
    [string]$DatabasePath = $(if ($env:BATTOS_DATABASE_PATH) { $env:BATTOS_DATABASE_PATH } else { "" }),
    [string]$DockerImage = $(if ($env:BATTOS_EXECUTION_DOCKER_IMAGE) { $env:BATTOS_EXECUTION_DOCKER_IMAGE } else { "battos-runtime-agents:dev" }),
    [string]$ClaudeCredentialsDir = $(if ($env:BATTOS_EXECUTION_CLAUDE_CREDENTIALS_DIR) { $env:BATTOS_EXECUTION_CLAUDE_CREDENTIALS_DIR } else { Join-Path $env:USERPROFILE ".claude" }),
    [switch]$SkipImageCheck,
    [switch]$NoArtifactCheck
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

Write-Host "BattOS Claude host_session smoke"
Write-Host "API: $ApiUrl"
Write-Host "Docker image: $DockerImage"
Write-Host "Claude credentials: $ClaudeCredentialsDir"

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

Write-Step "Checking Claude host session"
if (-not (Test-Path -LiteralPath $ClaudeCredentialsDir)) {
    throw "Claude credentials directory not found. Run 'claude login' on this host or set BATTOS_EXECUTION_CLAUDE_CREDENTIALS_DIR. Path: $ClaudeCredentialsDir"
}

Write-Step "Checking Docker"
$previousErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$dockerVersion = docker info --format "{{.ServerVersion}}" 2>&1
$dockerExitCode = $LASTEXITCODE
$ErrorActionPreference = $previousErrorActionPreference
if ($dockerExitCode -ne 0) {
    throw "Docker is not available. Start Docker Desktop/daemon and retry. Details: $dockerVersion"
}
Write-Host $dockerVersion

if (-not $SkipImageCheck) {
    Write-Step "Checking runtime image"
    docker image inspect $DockerImage | Out-Null
    docker run --rm --network none $DockerImage bash -lc "claude --version" | Write-Host
}

Write-Step "Registering runtime and smoke agent"
try {
    Invoke-BattOSPost -Path "/runtime-adapters/detect" -Body @{} | Out-Null
} catch {
    Write-Host "Runtime detect skipped or failed: $($_.Exception.Message)"
}
try {
    Invoke-BattOSPost -Path "/agents" -Body @{
        slug = "claude-host-session-smoke-agent"
        name = "Claude Host Session Smoke Agent"
        role = "host session smoke test"
        runtime_id = "claude-code-host-session"
        risk_level = "high"
        status = "active"
    } | Out-Null
} catch {
    Write-Host "Smoke agent may already exist: $($_.Exception.Message)"
}

$runId = $null
try {
    Write-Step "Creating project, task and run"
    $stamp = Get-Date -Format "yyyyMMddHHmmss"
    $project = Invoke-BattOSPost -Path "/projects" -Body @{
        slug = "smoke-claude-host-session-$stamp"
        name = "Smoke Claude Host Session"
        status = "active"
    }
    $task = Invoke-BattOSPost -Path "/tasks" -Body @{
        project_id = $project.id
        title = "Run Claude host_session smoke"
        status = "ready"
    }
    $marker = "battos-claude-host-session-ok"
    $prompt = @"
This is a BattOS Claude host_session smoke test.

Do exactly this inside the current workspace:
1. Create the directory outputs.
2. Create a Markdown file at outputs/host-session-smoke.md.
3. The file must contain the marker $marker.
4. Print the marker $marker to stdout.

Keep the run short. Do not inspect host files. Do not install dependencies.
"@
    $run = Invoke-BattOSPost -Path "/runs" -Body @{
        project_id = $project.id
        task_id = $task.id
        agent_id = "claude-host-session-smoke-agent"
        runtime_adapter_id = "claude-code-host-session"
        prompt = $prompt
        requested_network = $true
    }
    $runId = $run.id
    Write-Host "Run created: $runId"

    Write-Step "Approving network and execute"
    $networkApproval = Invoke-BattOSPost -Path "/runs/$runId/approvals" -Body @{
        kind = "network"
        decision = "approved"
        reason = "claude host_session requires provider network access"
    }
    if (-not $networkApproval.run.network_enabled) {
        throw "Network approval did not enable network"
    }
    $executeApproval = Invoke-BattOSPost -Path "/runs/$runId/approvals" -Body @{
        kind = "execute"
        decision = "approved"
        reason = "claude host_session smoke"
    }
    if ($executeApproval.run.status -ne "queued") {
        throw "Run was not queued after execute approval: $($executeApproval.run.status)"
    }

    Write-Step "Processing run with worker Docker sandbox"
    if (-not [string]::IsNullOrWhiteSpace($DatabasePath)) {
        $env:BATTOS_DATABASE_PATH = (Resolve-Path -LiteralPath $DatabasePath).Path
    }
    $env:BATTOS_EXECUTION_SANDBOX_MODE = "docker"
    $env:BATTOS_EXECUTION_DOCKER_IMAGE = $DockerImage
    $env:BATTOS_EXECUTION_HOST_SESSION_ENABLED = "true"
    $env:BATTOS_EXECUTION_CLAUDE_CREDENTIALS_DIR = (Resolve-Path -LiteralPath $ClaudeCredentialsDir).Path
    $goCachePath = "data\.cache\go-build"
    New-Item -ItemType Directory -Force -Path $goCachePath | Out-Null
    $env:GOCACHE = (Resolve-Path $goCachePath).Path
    $workerBinDir = "data\.cache\dev-bin"
    New-Item -ItemType Directory -Force -Path $workerBinDir | Out-Null
    $workerBin = Join-Path (Resolve-Path $workerBinDir).Path "battos-worker-dev.exe"
    go build -o $workerBin ./apps/api/cmd/worker
    powershell -ExecutionPolicy Bypass -File .\scripts\sign-battos-dev.ps1 -ExePath $workerBin | Write-Host
    & $workerBin -once -run-id $runId | Write-Host

    Write-Step "Validating run result"
    $result = Invoke-BattOSGet -Path "/runs/$runId"
    if ($result.status -ne "succeeded") {
        throw "Run status = $($result.status), want succeeded. Error: $($result.error_message)"
    }
    $logs = Invoke-BattOSGet -Path "/runs/$runId/logs"
    $logText = ($logs | ConvertTo-Json -Depth 8)
    if ($logText -notmatch "network: enabled by approval") {
        throw "Run logs did not include expected approved network state"
    }
    if ($logText -notmatch [regex]::Escape($marker)) {
        throw "Run logs did not include expected marker $marker"
    }

    if (-not $NoArtifactCheck) {
        $artifacts = Invoke-BattOSGet -Path "/artifacts?project_id=$($project.id)"
        $smokeArtifact = @($artifacts | Where-Object { $_.run_id -eq $runId -and $_.name -eq "outputs/host-session-smoke.md" }) | Select-Object -First 1
        if ($null -eq $smokeArtifact) {
            throw "Run artifact outputs/host-session-smoke.md was not registered"
        }
        $artifactPath = Join-Path (Resolve-Path "data\artifacts").Path $smokeArtifact.managed_path
        if (-not (Test-Path -LiteralPath $artifactPath)) {
            throw "Managed artifact file was not written: $artifactPath"
        }
        $artifactContent = Get-Content -LiteralPath $artifactPath -Raw
        if ($artifactContent -notmatch [regex]::Escape($marker)) {
            throw "Managed artifact did not include expected marker $marker"
        }
    }

    Write-Host ""
    Write-Host "BattOS Claude host_session smoke passed."
    Write-Host "Run ID: $runId"
} catch {
    Write-Host ""
    Write-Host "BattOS Claude host_session smoke failed."
    if ($runId) {
        Show-RunLogs -RunId $runId
    }
    throw
}
