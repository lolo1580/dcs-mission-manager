param(
    [string]$ProfilesPath = (Join-Path $PSScriptRoot '..\data\mappings.json'),
    [string]$LibraryPath = (Join-Path $PSScriptRoot '..\profiles\panels'),
    [switch]$Preview
)
$ErrorActionPreference = 'Stop'
$file = Get-Content -LiteralPath $ProfilesPath -Raw | ConvertFrom-Json
$library = Get-ChildItem -LiteralPath $LibraryPath -Filter '*.json' | ForEach-Object { Get-Content -LiteralPath $_.FullName -Raw | ConvertFrom-Json }
$repairedKeys = @{
    'FA-18C_hornet' = @('MASTER_BAT','FLAPS_UP','FLAPS_DOWN')
    'F-16C_50' = @('MASTER_BAT')
    'F-5E-3' = @('FLAPS_UP','FLAPS_DOWN')
}
foreach ($default in $library) {
    $current = $file.profiles | Where-Object { $_.aircraft -eq $default.aircraft } | Select-Object -First 1
    if (!$current) {
        $file.profiles = @($file.profiles) + $default
        Write-Output "Added $($default.aircraft)"
        continue
    }
    foreach ($binding in $default.bindings) {
        $existing = $current.bindings | Where-Object { $_.model -eq $binding.model -and $_.control -eq $binding.control -and $_.mode -eq $binding.mode } | Select-Object -First 1
        if (!$existing) { $current.bindings = @($current.bindings) + $binding; continue }
        if ($binding.control -in $repairedKeys[$default.aircraft]) {
            # Only repair the old known mapping or the already repaired command.
            # Preserve a command the operator chose independently.
            $expected = $binding.command
            if ($default.aircraft -eq 'F-16C_50' -and $binding.control -eq 'MASTER_BAT') { $expected = 'FUEL_MASTER_SW' }
            if ($existing.command -eq $expected -or $existing.command -eq $binding.command) {
                $current.bindings = @($current.bindings | Where-Object { $_ -ne $existing }) + $binding
                Write-Output "Repaired $($default.aircraft)/$($binding.control)"
            } else { Write-Output "Preserved custom $($default.aircraft)/$($binding.control)" }
        }
    }
}
if ($Preview) { $file | ConvertTo-Json -Depth 20; return }
if (Get-Process dcsmanager* -ErrorAction SilentlyContinue) { throw 'Close DCS Manager before updating profiles.' }
$backup = Join-Path (Split-Path -Parent $ProfilesPath) ('mappings.before-profile-fixes-' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '.json')
Copy-Item -LiteralPath $ProfilesPath -Destination $backup
$temporary = $ProfilesPath + '.updating'
try {
    $file | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $temporary -Encoding utf8
    $null = Get-Content -LiteralPath $temporary -Raw | ConvertFrom-Json
    Move-Item -LiteralPath $temporary -Destination $ProfilesPath -Force
} finally { if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary } }
Write-Output "Backup: $backup"
