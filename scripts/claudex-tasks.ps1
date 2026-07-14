param(
    [Parameter(Mandatory = $true, Position = 0)]
    [ValidateSet("setup", "build", "login", "serve", "run", "verify", "version")]
    [string]$Task
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$configDir = Join-Path $HOME ".config\claudex"
$configPath = Join-Path $configDir "claudex.yaml"
$installDir = Join-Path $HOME "bin"
$serverPath = Join-Path $installDir "claudex-server.exe"
$launcherPath = Join-Path $installDir "claudex.ps1"
$templatePath = Join-Path $repoRoot "claudex.example.yaml"

function New-LocalApiKey {
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    return [Convert]::ToBase64String($bytes).TrimEnd("=").Replace("+", "-").Replace("/", "_")
}

function Write-Utf8File {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Contents
    )

    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Contents, $utf8NoBom)
}

function Ensure-Config {
    New-Item -ItemType Directory -Force -Path $configDir | Out-Null

    if (-not (Test-Path -LiteralPath $configPath -PathType Leaf)) {
        if (-not (Test-Path -LiteralPath $templatePath -PathType Leaf)) {
            throw "Claudex configuration template not found: $templatePath"
        }

        Copy-Item -LiteralPath $templatePath -Destination $configPath
        $contents = Get-Content -LiteralPath $configPath -Raw
        $contents = $contents.Replace("replace-with-a-local-random-key", (New-LocalApiKey))
        Write-Utf8File -Path $configPath -Contents $contents
        Write-Output "Created Claudex configuration: $configPath"
        return
    }

    $contents = Get-Content -LiteralPath $configPath -Raw
    if ($contents.Contains("replace-with-a-local-random-key")) {
        $contents = $contents.Replace("replace-with-a-local-random-key", (New-LocalApiKey))
        Write-Utf8File -Path $configPath -Contents $contents
        Write-Output "Replaced the placeholder local API key in: $configPath"
    }
}

function Ensure-Install {
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null

    $oldCgo = $env:CGO_ENABLED
    $oldGoos = $env:GOOS
    $oldGoarch = $env:GOARCH
    try {
        $env:CGO_ENABLED = "0"
        $env:GOOS = "windows"
        $env:GOARCH = "arm64"
        & go build -o $serverPath ./cmd/claudex
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
    } finally {
        $env:CGO_ENABLED = $oldCgo
        $env:GOOS = $oldGoos
        $env:GOARCH = $oldGoarch
    }

    Copy-Item -LiteralPath (Join-Path $repoRoot "claudex.ps1") -Destination $launcherPath -Force
    Copy-Item -LiteralPath (Join-Path $repoRoot "claudex.cmd") -Destination (Join-Path $installDir "claudex.cmd") -Force
    Write-Output "Installed Claudex launcher and server in: $installDir"
}

function Ensure-UserPath {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($userPath -split ";" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $alreadyPresent = $entries | Where-Object { $_.TrimEnd("\") -ieq $installDir.TrimEnd("\") }
    if ($null -eq $alreadyPresent) {
        $newUserPath = (($entries + $installDir) -join ";")
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        Write-Output "Added $installDir to the user PATH. Open a new terminal to use claudex directly."
    }
}

function Invoke-Server {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    if (-not (Test-Path -LiteralPath $serverPath -PathType Leaf)) {
        Ensure-Install
    }
    & $serverPath @Arguments --config $configPath
    exit $LASTEXITCODE
}

switch ($Task) {
    "setup" {
        Ensure-Config
        Ensure-Install
        Ensure-UserPath
    }
    "build" {
        Ensure-Install
    }
    "login" {
        Ensure-Config
        Invoke-Server -Arguments @("login")
    }
    "serve" {
        Ensure-Config
        Invoke-Server -Arguments @("serve")
    }
    "run" {
        Ensure-Config
        if (-not (Test-Path -LiteralPath $launcherPath -PathType Leaf)) {
            Ensure-Install
        }
        & $launcherPath
        exit $LASTEXITCODE
    }
    "verify" {
        & go test ./internal/claudex ./cmd/claudex
        if ($LASTEXITCODE -ne 0) {
            throw "Targeted tests failed with exit code $LASTEXITCODE"
        }
        Ensure-Install
        Write-Output "Claudex verification completed."
    }
    "version" {
        if (-not (Test-Path -LiteralPath $serverPath -PathType Leaf)) {
            Ensure-Install
        }
        & $serverPath version
        exit $LASTEXITCODE
    }
}
