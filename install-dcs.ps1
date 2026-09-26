# Installation de DCS Mission Manager côté DCS (Windows).
#
# Usage :
#   .\install-dcs.ps1                 # détecte Saved Games et installe
#   .\install-dcs.ps1 -DryRun         # montre ce qui serait fait
#   .\install-dcs.ps1 -SavedGames "D:\Saved Games\DCS"

param(
    [string]$SavedGames = "",
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

$exe = Join-Path $root 'dcsmm.exe'
if (-not (Test-Path $exe)) {
    Write-Host "dcsmm.exe introuvable. Compile d'abord : .\build.ps1" -ForegroundColor Red
    exit 1
}

$args = @('install-lua')
if ($SavedGames) { $args += @('--saved-games', $SavedGames) }
if ($DryRun) { $args += '--dry-run' }

& $exe @args
exit $LASTEXITCODE
