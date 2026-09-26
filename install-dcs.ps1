# DCS Mission Manager installation on the DCS (Windows) side.
#
# Usage:
#   .\install-dcs.ps1                 # detects Saved Games and installs
#   .\install-dcs.ps1 -DryRun         # shows what would be done
#   .\install-dcs.ps1 -SavedGames "D:\Saved Games\DCS"

param(
    [string]$SavedGames = "",
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

$exe = Join-Path $root 'dcsmm.exe'
if (-not (Test-Path $exe)) {
    Write-Host "dcsmm.exe not found. Build it first: .\build.ps1" -ForegroundColor Red
    exit 1
}

$args = @('install-lua')
if ($SavedGames) { $args += @('--saved-games', $SavedGames) }
if ($DryRun) { $args += '--dry-run' }

& $exe @args
exit $LASTEXITCODE
