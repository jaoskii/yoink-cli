# Install yoink from this checkout (Windows):
# checks Go, downloads dependencies, builds, installs, and offers to fix PATH.
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

function Show-GoHelp {
    Write-Host 'Install Go 1.21 or newer, then re-run this script:'
    Write-Host '  winget install GoLang.Go     (or download from https://go.dev/dl/)'
    exit 1
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host 'Go is not installed.'
    Show-GoHelp
}

# go.mod asks for a newer toolchain; Go 1.21+ downloads it automatically.
$ver = (go env GOVERSION) -replace '^go', ''
$parts = $ver.Split('.')
if ([int]$parts[0] -lt 1 -or ([int]$parts[0] -eq 1 -and [int]$parts[1] -lt 21)) {
    Write-Host "Go $ver is too old."
    Show-GoHelp
}

Write-Host '-> Downloading dependencies'
go mod download
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host '-> Building and installing yoink'
$version = 'dev'
if (Get-Command git -ErrorAction SilentlyContinue) {
    $described = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -eq 0 -and $described) { $version = $described }
}
$env:CGO_ENABLED = '0'
go install -trimpath -ldflags "-s -w -X main.version=$version" ./cmd/yoink
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$bindir = go env GOBIN
if (-not $bindir) { $bindir = Join-Path (go env GOPATH) 'bin' }
Write-Host "OK Installed $bindir\yoink.exe ($version)"

if (($env:Path -split ';') -contains $bindir) {
    Write-Host ''
    Write-Host 'Run: yoink --help'
    exit 0
}

Write-Host ''
Write-Host "$bindir is not on your PATH yet."
$answer = Read-Host 'Add it to your user PATH now? [y/N]'
if ($answer -match '^[yY]') {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$bindir", 'User')
    Write-Host 'Added. Open a new terminal, then run: yoink --help'
} else {
    Write-Host "Run it directly: $bindir\yoink.exe --help"
}
