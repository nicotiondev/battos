param(
    [string]$DatabaseUrl = $env:DATABASE_URL,
    [int]$Port = 8000,
    [switch]$StopExisting,
    [switch]$Background,
    [switch]$Wait,
    [switch]$NoDatabase,
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
if ($NoDatabase) {
    $DatabaseUrl = ""
} elseif ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
    $DatabaseUrl = "postgresql://battos:change-me@127.0.0.1:5432/battos?sslmode=disable"
}

if ($StopExisting) {
    $listeners = netstat -ano | Select-String ":$Port\s+.*LISTENING"
    foreach ($listener in $listeners) {
        $parts = ($listener.Line -split "\s+") | Where-Object { $_ -ne "" }
        $pidText = $parts[-1]
        if ($pidText -match "^\d+$") {
            $pidValue = [int]$pidText
            Write-Host "Stopping process on port $Port (PID $pidValue)"
            Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
        }
    }

    $releaseDeadline = (Get-Date).AddSeconds(15)
    do {
        $stillListening = netstat -ano | Select-String ":$Port\s+.*LISTENING"
        if (-not $stillListening) {
            break
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $releaseDeadline)
}

if ($Background) {
	$cacheDir = Join-Path $repoRoot "data\.cache\go-build"
	$logsDir = Join-Path $repoRoot "data\logs"
	New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
	New-Item -ItemType Directory -Force -Path $logsDir | Out-Null
	$goPath = (Get-Command go).Source
	$repoRootEscaped = $repoRoot.Replace("'", "''")
	$goPathEscaped = $goPath.Replace("'", "''")
	$cacheDirEscaped = $cacheDir.Replace("'", "''")
	$databaseUrlEscaped = $DatabaseUrl.Replace("'", "''")
	$script = @(
		"`$ErrorActionPreference = 'Stop'",
		"Set-Location '$repoRootEscaped'",
		"`$env:BATTOS_API_PORT = '$Port'",
		"`$env:GOCACHE = '$cacheDirEscaped'"
	)
	if ($NoDatabase) {
		$script += "Remove-Item Env:DATABASE_URL -ErrorAction SilentlyContinue"
	} else {
		$script += "`$env:DATABASE_URL = '$databaseUrlEscaped'"
	}
	$script += "& '$goPathEscaped' run ./apps/api/cmd/api"
	$launcherPath = Join-Path $cacheDir "start-api-$Port.ps1"
	$stdoutPath = Join-Path $logsDir "start-api-$Port.out.log"
	$stderrPath = Join-Path $logsDir "start-api-$Port.err.log"
	Set-Content -LiteralPath $launcherPath -Value ($script -join "`r`n") -Encoding UTF8
	Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "`"$launcherPath`"" -WorkingDirectory $repoRoot -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
	Write-Host "BattOS API starting in background on port $Port"
	if ($NoDatabase) {
		Write-Host "DATABASE_URL=<unset>"
	} else {
		Write-Host "DATABASE_URL=$DatabaseUrl"
	}
	Write-Host "GOCACHE=$cacheDir"
	Write-Host "STDOUT=$stdoutPath"
	Write-Host "STDERR=$stderrPath"

    if ($Wait) {
        $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
        $statusUrl = "http://127.0.0.1:$Port/status"
        do {
            try {
                $status = Invoke-RestMethod -UseBasicParsing -Uri $statusUrl -TimeoutSec 2
                Write-Host "BattOS API ready: $($status.overall)"
                exit 0
            } catch {
                Start-Sleep -Milliseconds 500
            }
        } while ((Get-Date) -lt $deadline)

		throw "BattOS API did not become ready at $statusUrl within $TimeoutSeconds seconds"
	}

    exit 0
}

if ($NoDatabase) {
    Remove-Item Env:DATABASE_URL -ErrorAction SilentlyContinue
} else {
    $env:DATABASE_URL = $DatabaseUrl
}
$env:BATTOS_API_PORT = "$Port"
Set-Location $repoRoot
Write-Host "BattOS API running in foreground on port $Port"
if ($NoDatabase) {
    Write-Host "DATABASE_URL=<unset>"
} else {
    Write-Host "DATABASE_URL=$DatabaseUrl"
}
go run ./apps/api/cmd/api
