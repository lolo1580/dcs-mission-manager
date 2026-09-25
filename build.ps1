# DCS Mission Manager — build script (Windows / PowerShell)
#
# Usage:
#   .\build.ps1              # frontend + backend
#   .\build.ps1 -Target run  # run from source
#   .\build.ps1 -Target docker

param(
    [ValidateSet('all', 'frontend', 'backend', 'run', 'test', 'docker')]
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
    go build -o (Join-Path $root 'dcsmm.exe') ./cmd/dcsmm
    Pop-Location
    Write-Host '==> Built dcsmm.exe' -ForegroundColor Green
}

switch ($Target) {
    'frontend' { Build-Frontend }
    'backend'  { Build-Backend }
    'all'      { Build-Frontend; Build-Backend }
    'run'      { Push-Location (Join-Path $root 'backend'); go run ./cmd/dcsmm; Pop-Location }
    'test'     { Push-Location (Join-Path $root 'backend'); go test ./...; Pop-Location }
    'docker'   { docker build -f (Join-Path $root 'deploy/Dockerfile') -t dcsmm:latest $root }
}
