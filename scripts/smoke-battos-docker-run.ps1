param(
    [string]$ApiUrl = $(if ($env:BATTOS_API_URL) { $env:BATTOS_API_URL } else { "http://127.0.0.1:8000" }),
    [string]$DockerImage = "alpine:3.20",
    [switch]$RequireMemoryContext
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

Write-Host "BattOS Docker run smoke"
Write-Host "API: $ApiUrl"
Write-Host "Require memory context: $RequireMemoryContext"

Write-Step "Checking API status"
$status = Invoke-RestMethod -UseBasicParsing -Uri "$ApiUrl/status" -TimeoutSec 5
if ($status.overall -ne "ok") {
    throw "BattOS API is not OK: $($status.overall)"
}
$db = $status.subsystems | Where-Object { $_.name -eq "database" } | Select-Object -First 1
if ($null -eq $db -or $db.status -ne "ok") {
    throw "Database subsystem is not OK"
}
Write-Host "API and DB are OK"

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
$previousErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$dockerProbe = docker run --rm --network none $DockerImage sh -c "echo battos-docker-sandbox-ok" 2>&1
$dockerProbeExitCode = $LASTEXITCODE
$ErrorActionPreference = $previousErrorActionPreference
if ($dockerProbeExitCode -ne 0) {
    throw "Docker sandbox probe failed for image ${DockerImage}: $dockerProbe"
}
Write-Host $dockerProbe
if (Test-Path "infra\.env") {
    $runningComposeServices = @(docker compose -f infra/docker-compose.yml --env-file infra/.env ps --status running --services 2>$null)
    if ($LASTEXITCODE -eq 0 -and $runningComposeServices -contains "battos-worker") {
        throw "battos-worker Compose is running and may claim the smoke run first. Stop it or run it in DockerSandbox mode before this smoke: docker compose -f infra/docker-compose.yml --env-file infra/.env stop battos-worker"
    }
}

Write-Step "Registering sandbox smoke runtime and agent"
$runtimeId = $(if ($RequireMemoryContext) { "sandbox-memory-smoke" } else { "sandbox-smoke" })
$agentId = $(if ($RequireMemoryContext) { "sandbox-memory-smoke-agent" } else { "sandbox-smoke-agent" })
$agentName = $(if ($RequireMemoryContext) { "Sandbox Memory Smoke Agent" } else { "Sandbox Smoke Agent" })
$agentBody = @{
    slug = $agentId
    name = $agentName
    role = "worker smoke test"
    runtime_id = $runtimeId
    risk_level = "low"
    status = "active"
} | ConvertTo-Json
try {
    Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl/agents" -ContentType "application/json" -Body $agentBody | Out-Null
} catch {
    Write-Host "Smoke agent may already exist: $($_.Exception.Message)"
}

Write-Step "Creating project, task, run and execute approval"
$stamp = Get-Date -Format "yyyyMMddHHmmss"
$projectId = "smoke-docker-$stamp"
$project = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl/projects" -ContentType "application/json" -Body (@{
    slug = $projectId
    name = "Smoke Docker Sandbox"
    status = "active"
} | ConvertTo-Json)
$task = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl/tasks" -ContentType "application/json" -Body (@{
    project_id = $project.id
    title = "Run Docker sandbox smoke"
    status = "ready"
} | ConvertTo-Json)
if ($RequireMemoryContext) {
    Write-Step "Saving project memory for context injection"
    $memory = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl/memory/save" -ContentType "application/json" -Body (@{
        project_id = $project.id
        scope = "project"
        type = "decision"
        topic_key = "$($project.id)/memory-bridge-smoke"
        title = "Memory Bridge smoke"
        content = "memory bridge smoke marker for $($project.id)"
    } | ConvertTo-Json)
    Write-Host "Memory saved: $($memory.id)"
}
$run = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl/runs" -ContentType "application/json" -Body (@{
    project_id = $project.id
    task_id = $task.id
    agent_id = $agentId
    runtime_adapter_id = $runtimeId
    prompt = "Validate BattOS DockerSandbox with no network"
    requested_network = $false
} | ConvertTo-Json)
$approval = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ApiUrl/runs/$($run.id)/approvals" -ContentType "application/json" -Body (@{
    kind = "execute"
    decision = "approved"
    reason = "smoke docker sandbox"
} | ConvertTo-Json)
if ($approval.run.status -ne "queued") {
    throw "Run was not queued after approval: $($approval.run.status)"
}
Write-Host "Run queued: $($run.id)"

Write-Step "Processing run with worker Docker sandbox"
$env:BATTOS_EXECUTION_SANDBOX_MODE = "docker"
$env:BATTOS_EXECUTION_DOCKER_IMAGE = $DockerImage
$env:GOCACHE = (Resolve-Path "data\.cache\go-build").Path
$workerBinDir = "data\.cache\dev-bin"
New-Item -ItemType Directory -Force -Path $workerBinDir | Out-Null
$workerBin = Join-Path (Resolve-Path $workerBinDir).Path "battos-worker-dev.exe"
go build -o $workerBin ./apps/api/cmd/worker
powershell -ExecutionPolicy Bypass -File .\scripts\sign-battos-dev.ps1 -ExePath $workerBin | Write-Host
for ($i = 0; $i -lt 10; $i++) {
    & $workerBin -once -run-id $run.id | Write-Host
    $current = Invoke-RestMethod -UseBasicParsing -Uri "$ApiUrl/runs/$($run.id)" -TimeoutSec 5
    if (@("succeeded", "failed", "cancelled") -contains $current.status) {
        break
    }
    Start-Sleep -Milliseconds 500
}

Write-Step "Validating run result and logs"
$result = Invoke-RestMethod -UseBasicParsing -Uri "$ApiUrl/runs/$($run.id)" -TimeoutSec 5
if ($result.status -ne "succeeded") {
    throw "Run status = $($result.status), want succeeded. Error: $($result.error_message)"
}
$logs = Invoke-RestMethod -UseBasicParsing -Uri "$ApiUrl/runs/$($run.id)/logs" -TimeoutSec 5
$logText = ($logs | ConvertTo-Json -Depth 8)
if ($logText -notmatch "network: disabled") {
    throw "Run logs did not include expected network state"
}
if ($RequireMemoryContext) {
    if ($logText -notmatch "memory context injected" -or $logText -notmatch "battos-memory-context-ok") {
        throw "Run logs did not include expected memory context injection output"
    }
} elseif ($logText -notmatch "battos-worker-docker-ok") {
    throw "Run logs did not include expected Docker smoke output"
}
$artifacts = Invoke-RestMethod -UseBasicParsing -Uri "$ApiUrl/artifacts?project_id=$($project.id)" -TimeoutSec 5
$smokeArtifact = @($artifacts | Where-Object { $_.run_id -eq $run.id -and $_.name -eq "outputs/smoke.md" }) | Select-Object -First 1
if ($null -eq $smokeArtifact) {
    throw "Run artifact outputs/smoke.md was not registered"
}
if ([string]::IsNullOrWhiteSpace($smokeArtifact.managed_path)) {
    throw "Run artifact did not include a managed_path"
}
$artifactPath = Join-Path (Resolve-Path "data\artifacts").Path $smokeArtifact.managed_path
if (-not (Test-Path -LiteralPath $artifactPath)) {
    throw "Managed artifact file was not written: $artifactPath"
}
$workspaceItems = @(Get-ChildItem "data\runs\workspaces" -Force -ErrorAction SilentlyContinue)
if ($workspaceItems.Count -ne 0) {
    throw "Workspace cleanup failed; data\runs\workspaces is not empty"
}

Write-Host ""
Write-Host "BattOS Docker run smoke passed."
Write-Host "Run ID: $($run.id)"
