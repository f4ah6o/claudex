# Claudex launcher for Windows PowerShell.
# The server binary and this script should be placed in the same directory.

$ErrorActionPreference = "Stop"

function Resolve-ClaudexPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ($Path -eq "~") {
        return $HOME
    }
    if ($Path.StartsWith("~/") -or $Path.StartsWith("~\")) {
        $Path = Join-Path $HOME ($Path.Substring(2))
    }
    if (-not [System.IO.Path]::IsPathRooted($Path)) {
        $Path = Join-Path (Get-Location).Path $Path
    }
    return [System.IO.Path]::GetFullPath($Path)
}

function Test-ClaudexReady {
    param([Parameter(Mandatory = $true)][string]$Url)

    $request = $null
    $response = $null
    try {
        $request = [System.Net.WebRequest]::Create($Url)
        $request.Method = "GET"
        $request.Timeout = 2000
        $response = $request.GetResponse()
        return $true
    } catch [System.Net.WebException] {
        # Any HTTP response, including 404, proves that the listener is alive.
        return ($null -ne $_.Exception.Response)
    } catch {
        return $false
    } finally {
        if ($null -ne $response) {
            $response.Close()
        }
    }
}

function Get-ClaudexLocalKey {
    param([Parameter(Mandatory = $true)][string]$Path)

    $contents = Get-Content -LiteralPath $Path -Raw
    $pattern = '(?ms)^\s*api-keys:\s*\r?\n\s*-\s*["'']?(?<key>[^"''\s#]+)["'']?'
    $match = [regex]::Match($contents, $pattern)
    if (-not $match.Success) {
        throw "could not find the first api-keys entry in $Path"
    }

    $key = $match.Groups["key"].Value.Trim()
    if ([string]::IsNullOrWhiteSpace($key) -or $key -match '^(?i:replace-|your-api-key)') {
        throw "claudex.yaml still contains a placeholder api key"
    }
    return $key
}

$configSetting = $env:CLAUDEX_CONFIG
if ([string]::IsNullOrWhiteSpace($configSetting)) {
    $configSetting = Join-Path $HOME ".config\claudex\claudex.yaml"
}
$configPath = Resolve-ClaudexPath $configSetting
if (-not (Test-Path -LiteralPath $configPath -PathType Leaf)) {
    throw "Claudex configuration not found: $configPath"
}

$serverSetting = $env:CLAUDEX_SERVER_PATH
if ([string]::IsNullOrWhiteSpace($serverSetting)) {
    $serverSetting = Join-Path $PSScriptRoot "claudex-server.exe"
}
$serverPath = Resolve-ClaudexPath $serverSetting
if (-not (Test-Path -LiteralPath $serverPath -PathType Leaf)) {
    throw "Claudex server binary not found: $serverPath"
}

$baseUrl = $env:CLAUDEX_BASE_URL
if ([string]::IsNullOrWhiteSpace($baseUrl)) {
    $baseUrl = "http://127.0.0.1:8317"
}
$baseUrl = $baseUrl -replace '/+$', ''
$localKey = Get-ClaudexLocalKey $configPath

if (-not (Test-ClaudexReady "$baseUrl/")) {
    if ($env:CLAUDEX_NO_START -eq "1") {
        throw "Claudex server is not running at $baseUrl"
    }

    $logDirSetting = $env:CLAUDEX_LOG_DIR
    if ([string]::IsNullOrWhiteSpace($logDirSetting)) {
        $logDirSetting = Join-Path $HOME ".claudex"
    }
    $logDir = Resolve-ClaudexPath $logDirSetting
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $stdoutLog = Join-Path $logDir "serve.stdout.log"
    $stderrLog = Join-Path $logDir "serve.stderr.log"
    $configArgument = '"{0}"' -f $configPath.Replace('"', '\"')
    $serverProcess = Start-Process -FilePath $serverPath `
        -ArgumentList @("serve", "--config", $configArgument) `
        -WorkingDirectory (Split-Path -Parent $configPath) `
        -RedirectStandardOutput $stdoutLog `
        -RedirectStandardError $stderrLog `
        -WindowStyle Hidden `
        -PassThru

    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        if (Test-ClaudexReady "$baseUrl/") {
            $ready = $true
            break
        }
        if ($serverProcess.HasExited) {
            break
        }
        Start-Sleep -Milliseconds 250
    }
    if (-not $ready) {
        $details = Get-Content -LiteralPath $stderrLog -Tail 20 -ErrorAction SilentlyContinue
        throw "Claudex server did not become ready at $baseUrl`n$($details -join [Environment]::NewLine)"
    }
}

$claudeCommand = $env:CLAUDEX_CLAUDE_COMMAND
if ([string]::IsNullOrWhiteSpace($claudeCommand)) {
    $claudeCommand = "claude"
}
$command = Get-Command $claudeCommand -ErrorAction SilentlyContinue
if ($null -eq $command) {
    throw "Claude Code command not found: $claudeCommand"
}
$commandPath = if ($command.CommandType -eq "Application") { $command.Source } else { $command.Name }

# These variables are inherited only by this Claude Code process.
$env:ANTHROPIC_BASE_URL = $baseUrl
$env:ANTHROPIC_AUTH_TOKEN = $localKey
$env:ANTHROPIC_MODEL = "gpt-5.6-sol"
$env:ANTHROPIC_DEFAULT_OPUS_MODEL = "gpt-5.6-sol"
$env:ANTHROPIC_DEFAULT_SONNET_MODEL = "gpt-5.6-terra"
$env:ANTHROPIC_DEFAULT_HAIKU_MODEL = "gpt-5.6-luna"
$env:CLAUDE_CODE_EFFORT_LEVEL = "xhigh"
$env:CLAUDE_CODE_ALWAYS_ENABLE_EFFORT = "1"

& $commandPath --model gpt-5.6-sol --effort xhigh @args
$exitCode = $LASTEXITCODE
if ($null -eq $exitCode) {
    $exitCode = 0
}
exit $exitCode
