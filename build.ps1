# DCS Mission Manager — build script (Windows / PowerShell)
#
# Usage:
#   .\build.ps1              # frontend + backend
#   .\build.ps1 -Target run  # run from source

param(
    [ValidateSet('all', 'frontend', 'backend', 'run', 'test')]
    [string]$Target = 'all'
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

function Build-Frontend {
    Write-Host '==> Building frontend' -ForegroundColor Cyan
    Push-Location (Join-Path $root 'frontend')
    npm install
    npm run build
    Pop-Location
}

function Build-Backend {
    Write-Host '==> Building backend' -ForegroundColor Cyan
    Push-Location (Join-Path $root 'backend')
    $version = (Select-String -Path (Join-Path $root 'VERSION') -Pattern '^\s*(.+)$').Matches.Groups[1].Value.Trim()
    if (-not $version) { $version = 'dev' }
    go build -trimpath -ldflags="-s -w -X main.Version=$version" -o (Join-Path $root 'dcsmm.exe') ./cmd/dcsmm
    Pop-Location
    Write-Host "==> Built dcsmm.exe ($version)" -ForegroundColor Green
}

switch ($Target) {
    'frontend' { Build-Frontend }
    'backend'  { Build-Backend }
    'all'      { Build-Frontend; Build-Backend }
    'run'      { Push-Location (Join-Path $root 'backend'); go run ./cmd/dcsmm; Pop-Location }
    'test'     { Push-Location (Join-Path $root 'backend'); go test ./...; Pop-Location }
}
