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

# PowerShell does not turn a non-zero exit of a native command into a terminating
# error, even with $ErrorActionPreference='Stop'. Without this check `npm run
# build` or `go build` could fail and the script would still print a success
# message, leaving an old executable to be mistaken for a fresh build.
# Call it right after any native command (npm, go, go-winres).
function Assert-Ok {
    if ($LASTEXITCODE -ne 0) {
        throw "previous command exited with code $LASTEXITCODE"
    }
}

function Build-Frontend {
    Write-Host '==> Building frontend' -ForegroundColor Cyan
    Push-Location (Join-Path $root 'frontend')
    try {
        npm install; Assert-Ok
        npm run build; Assert-Ok
    }
    finally {
        Pop-Location
    }
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
    try {
        & $tool.Source make --arch amd64 --file-version $version --product-version $version; Assert-Ok
    }
    finally {
        Pop-Location
    }
    Write-Host "==> Windows resources generated" -ForegroundColor Green
}

function Build-Backend {
    Write-Host '==> Building backend' -ForegroundColor Cyan
    Push-Location (Join-Path $root 'backend')
    try {
        $version = (Select-String -Path (Join-Path $root 'VERSION') -Pattern '^\s*(.+)$').Matches.Groups[1].Value.Trim()
        if (-not $version) { $version = 'dev' }
        go build -trimpath -ldflags="-s -w -X main.Version=$version" -o (Join-Path $root 'dcsmanager.exe') ./cmd/dcsmanager; Assert-Ok
    }
    finally {
        Pop-Location
    }
    Write-Host "==> Built dcsmanager.exe ($version)" -ForegroundColor Green
}

switch ($Target) {
    'frontend' { Build-Frontend }
    'backend'  { Build-Backend }
    'all'      { Build-Frontend; Build-Backend }
    'winres'   { Build-Winres }
    'run'      { Push-Location (Join-Path $root 'backend'); try { go run ./cmd/dcsmanager; Assert-Ok } finally { Pop-Location } }
    'test'     { Push-Location (Join-Path $root 'backend'); try { go test ./...; Assert-Ok } finally { Pop-Location } }
}
