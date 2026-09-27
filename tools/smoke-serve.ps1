# Smoke test for dcsmm serve — the headless mode.
#
# It starts the manager headless, waits for the health endpoint, checks that the
# embedded UI is served (not the fallback page), then stops it. It needs no
# window, so it runs unattended.
#
# Usage: pwsh -File tools/smoke-serve.ps1 [-Exe .\dcsmm.exe] [-Port 8080]
param(
    [string]$Exe = (Join-Path $PSScriptRoot '..\dcsmm.exe'),
    [int]$Port = 8080
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path $Exe)) { throw "not found: $Exe" }

# A port of 0 would work too, but the log line is harder to read back from a
# script; an explicit, unlikely-to-be-taken port keeps this simple.
$env:DCSMM_HTTP_ADDR = "127.0.0.1:$Port"
$env:DCSMM_UDP_ADDR = "127.0.0.1:0"
$env:DCSMM_TCP_ADDR = "127.0.0.1:0"
$env:DCSMM_DB_ENABLED = 'false'

& $Exe version

$proc = Start-Process -FilePath $Exe -ArgumentList 'serve' -PassThru -WindowStyle Hidden
try {
    $up = $false
    for ($i = 0; $i -lt 40; $i++) {
        if ($proc.HasExited) { throw 'the manager exited during startup' }
        try {
            Invoke-RestMethod "http://127.0.0.1:$Port/api/health" -TimeoutSec 2 | Out-Null
            $up = $true
            break
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $up) { throw 'the health endpoint never answered' }

    $html = (Invoke-WebRequest "http://127.0.0.1:$Port/" -UseBasicParsing).Content
    if ($html -notmatch '<div id="app">') { throw 'the embedded UI is not served (fallback page returned)' }

    Write-Host 'OK: dcsmm serve is up and serves the embedded UI'
} finally {
    if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue }
}
