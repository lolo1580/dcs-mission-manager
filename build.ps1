# DCS Manager — build script (Windows / PowerShell)
#
# Usage:
#   .\build.ps1              # frontend + backend
#   .\build.ps1 -Target run  # run from source

param(
    [ValidateSet('all', 'frontend', 'backend', 'run', 'test', 'winres')]
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

# Regenerate the Windows resource object (icon + version metadata) embedded into
# the executable. The generated rsrc_windows_amd64.syso is committed, so a normal
# build needs no extra tool; this target is only for when the icon or the winres
# config changes. Requires go-winres: go install github.com/tc-hib/go-winres@latest
function Build-Winres {
    Write-Host '==> Generating Windows resources (icon, version)' -ForegroundColor Cyan
    $tool = Get-Command go-winres.exe -ErrorAction SilentlyContinue
    if (-not $tool) {
        $candidate = Join-Path (go env GOPATH) 'bin\go-winres.exe'
        if (Test-Path $candidate) { $tool = Get-Item $candidate }
    }
    if (-not $tool) {
        Write-Host 'go-winres not found. Install it with:' -ForegroundColor Yellow
        Write-Host '  go install github.com/tc-hib/go-winres@latest' -ForegroundColor Yellow
        exit 1
    }
    $version = (Select-String -Path (Join-Path $root 'VERSION') -Pattern '^\s*(.+)$').Matches.Groups[1].Value.Trim()
    if (-not $version) { $version = 'dev' }
    Push-Location (Join-Path $root 'backend\cmd\dcsmanager')
    & $tool.Source make --arch amd64 --file-version $version --product-version $version
    Pop-Location
    Write-Host "==> Windows resources generated" -ForegroundColor Green
}

function Build-Backend {
    Write-Host '==> Building backend' -ForegroundColor Cyan
    Push-Location (Join-Path $root 'backend')
    $version = (Select-String -Path (Join-Path $root 'VERSION') -Pattern '^\s*(.+)$').Matches.Groups[1].Value.Trim()
    if (-not $version) { $version = 'dev' }
    go build -trimpath -ldflags="-s -w -X main.Version=$version" -o (Join-Path $root 'dcsmanager.exe') ./cmd/dcsmanager
    Pop-Location
    Write-Host "==> Built dcsmanager.exe ($version)" -ForegroundColor Green
}

switch ($Target) {
    'frontend' { Build-Frontend }
    'backend'  { Build-Backend }
    'all'      { Build-Frontend; Build-Backend }
    'winres'   { Build-Winres }
    'run'      { Push-Location (Join-Path $root 'backend'); go run ./cmd/dcsmanager; Pop-Location }
    'test'     { Push-Location (Join-Path $root 'backend'); go test ./...; Pop-Location }
}
