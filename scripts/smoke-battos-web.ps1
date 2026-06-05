param(
    [string]$ApiUrl = $(if ($env:BATTOS_API_URL) { $env:BATTOS_API_URL } else { "http://localhost:8000" }),
    [string]$WebUrl = $(if ($env:BATTOS_WEB_URL) { $env:BATTOS_WEB_URL } else { "http://localhost:3000" }),
    [string]$ApiToken = $env:BATTOS_API_TOKEN,
    [switch]$RequireDatabase,
    [switch]$CheckWeb,
    [switch]$CheckSSE,
    [switch]$CheckRunSSE,
    [int]$TimeoutSec = 10
)

$ErrorActionPreference = "Stop"

function New-AuthHeaders {
    $headers = @{}
    if (-not [string]::IsNullOrWhiteSpace($ApiToken)) {
        $headers["Authorization"] = "Bearer $ApiToken"
    }
    return $headers
}

function Invoke-JsonEndpoint {
    param(
        [string]$Name,
        [string]$Path,
        [int[]]$AllowedStatus = @(200)
    )

    $uri = "$ApiUrl$Path"
    Write-Host "==> $Name $Path"
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $uri -Headers (New-AuthHeaders) -TimeoutSec $TimeoutSec
        if ($AllowedStatus -notcontains [int]$response.StatusCode) {
            throw "Unexpected HTTP $($response.StatusCode)"
        }
        if ($response.Content) {
            $null = $response.Content | ConvertFrom-Json
        }
        Write-Host "    OK HTTP $($response.StatusCode)"
        return $response.Content
    } catch {
        $resp = $_.Exception.Response
        if ($resp -and $AllowedStatus -contains [int]$resp.StatusCode) {
            Write-Host "    OK HTTP $([int]$resp.StatusCode)"
            return ""
        }
        throw "Dashboard API smoke failed for $Name ($Path): $($_.Exception.Message)"
    }
}

function Assert-ArrayEndpoint {
    param(
        [string]$Name,
        [string]$Path
    )
    $json = Invoke-JsonEndpoint -Name $Name -Path $Path
    $value = $json | ConvertFrom-Json
    if ($null -eq $value) {
        throw "$Name returned empty response"
    }
    if ($value -isnot [array] -and $value.GetType().Name -ne "Object[]") {
        throw "$Name expected an array JSON response"
    }
    return @($value)
}

function Assert-SSEEvents {
    param(
        [string]$Name,
        [string]$Path,
        [string[]]$ExpectedEvents,
        [int]$MaxEvents = 8
    )

    Write-Host "==> $Name $Path"
    Add-Type -AssemblyName System.Net.Http
    $reader = $null
    $stream = $null
    $response = $null
    $client = [System.Net.Http.HttpClient]::new()
    $seen = New-Object System.Collections.Generic.List[string]
    try {
        foreach ($key in (New-AuthHeaders).Keys) {
            $client.DefaultRequestHeaders.Add($key, (New-AuthHeaders)[$key])
        }
        $client.DefaultRequestHeaders.Add("Accept", "text/event-stream")
        $cts = [System.Threading.CancellationTokenSource]::new()
        $cts.CancelAfter([TimeSpan]::FromSeconds($TimeoutSec))
        $response = $client.GetAsync("$ApiUrl$Path", [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead, $cts.Token).GetAwaiter().GetResult()
        if (-not $response.IsSuccessStatusCode) {
            throw "Unexpected HTTP $([int]$response.StatusCode)"
        }
        $stream = $response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
        $reader = [System.IO.StreamReader]::new($stream)

        while (-not $reader.EndOfStream -and $seen.Count -lt $MaxEvents) {
            $line = $reader.ReadLineAsync().GetAwaiter().GetResult()
            if ($line -like "event: *") {
                $eventName = $line.Substring("event: ".Length).Trim()
                if (-not [string]::IsNullOrWhiteSpace($eventName)) {
                    $seen.Add($eventName)
                }
                foreach ($expected in $ExpectedEvents) {
                    if ($seen -contains $expected) {
                        Write-Host "    OK SSE event: $expected"
                        return
                    }
                }
            }
        }
        throw "SSE stream did not emit expected events: $($ExpectedEvents -join ', '). Seen: $($seen -join ', ')"
    } finally {
        if ($reader) { $reader.Dispose() }
        if ($stream) { $stream.Dispose() }
        if ($response) { $response.Dispose() }
        $client.Dispose()
    }
}

Write-Host "BattOS web/API smoke"
Write-Host "API: $ApiUrl"
Write-Host "WEB: $WebUrl"

$statusJson = Invoke-JsonEndpoint -Name "Command Center status" -Path "/status"
$status = $statusJson | ConvertFrom-Json
Write-Host "Overall: $($status.overall)"

$db = @($status.subsystems) | Where-Object { $_.name -eq "database" } | Select-Object -First 1
$databaseReady = $null -ne $db -and $db.status -eq "ok"

if ($RequireDatabase) {
    if (-not $databaseReady) {
        throw "Database subsystem is not OK"
    }
    Write-Host "Database: OK"
}

$null = Invoke-JsonEndpoint -Name "Memory recent" -Path "/memory/recent"
$null = Invoke-JsonEndpoint -Name "Memory stats" -Path "/memory/stats"

if (-not $databaseReady) {
    Write-Host "    SKIP database-backed dashboard endpoints: database subsystem is $($db.status)"
} else {
    $projects = Assert-ArrayEndpoint -Name "Work Board projects" -Path "/projects"
    $null = Assert-ArrayEndpoint -Name "Work Board goals" -Path "/goals"
    $null = Assert-ArrayEndpoint -Name "Work Board tasks" -Path "/tasks"
    $null = Assert-ArrayEndpoint -Name "Agents" -Path "/agents"
    $null = Assert-ArrayEndpoint -Name "Skills" -Path "/skills"
    $null = Assert-ArrayEndpoint -Name "Runtime adapters" -Path "/runtime-adapters"
    $null = Assert-ArrayEndpoint -Name "Providers" -Path "/providers"
    $null = Assert-ArrayEndpoint -Name "Control Room runs" -Path "/runs"
    $null = Assert-ArrayEndpoint -Name "Repositories" -Path "/repositories"
    $null = Assert-ArrayEndpoint -Name "Knowledge workspaces" -Path "/knowledge/workspaces"
    $null = Assert-ArrayEndpoint -Name "NovaCore conversations" -Path "/novacore/conversations"
    $null = Assert-ArrayEndpoint -Name "Usage overview" -Path "/usage/overview"

    if ($projects.Count -gt 0) {
        $projectID = $projects[0].id
        if ([string]::IsNullOrWhiteSpace($projectID)) {
            $projectID = $projects[0].slug
        }
        if (-not [string]::IsNullOrWhiteSpace($projectID)) {
            $encodedProjectID = [uri]::EscapeDataString($projectID)
            $null = Assert-ArrayEndpoint -Name "Knowledge journals by project" -Path "/journals?project_id=$encodedProjectID"
            $null = Assert-ArrayEndpoint -Name "Knowledge artifacts by project" -Path "/artifacts?project_id=$encodedProjectID"
        }
    } else {
        Write-Host "    SKIP journals/artifacts by project: no projects available"
    }
}

if ($CheckSSE) {
    Assert-SSEEvents -Name "System metrics SSE" -Path "/events/system-metrics" -ExpectedEvents @("system.metrics")
}

if ($CheckRunSSE) {
    if (-not $databaseReady) {
        if ($RequireDatabase) {
            throw "Cannot check run SSE because database is not OK"
        }
        Write-Host "    SKIP run SSE: database subsystem is $($db.status)"
    } else {
        $runs = Assert-ArrayEndpoint -Name "Control Room runs for SSE" -Path "/runs"
        if ($runs.Count -eq 0) {
            Write-Host "    SKIP run SSE: no runs available"
        } else {
            $runID = $runs[0].id
            if ([string]::IsNullOrWhiteSpace($runID)) {
                throw "Latest run did not include id"
            }
            $encodedRunID = [uri]::EscapeDataString($runID)
            Assert-SSEEvents -Name "Control Room run SSE" -Path "/events/runs/$encodedRunID" -ExpectedEvents @("run.snapshot", "run.log", "run.done")
        }
    }
}

if ($CheckWeb) {
    Write-Host "==> Web shell $WebUrl"
    $web = Invoke-WebRequest -UseBasicParsing -Uri $WebUrl -TimeoutSec $TimeoutSec
    if ($web.StatusCode -ne 200) {
        throw "Unexpected web HTTP $($web.StatusCode)"
    }
    if (-not ($web.Content -match "BattOS")) {
        throw "Web response does not contain BattOS marker"
    }
    Write-Host "    OK HTTP $($web.StatusCode)"
}

Write-Host ""
Write-Host "BattOS web/API smoke passed."
