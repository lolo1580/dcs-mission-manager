param([string]$Installer)
$ErrorActionPreference = 'Stop'
$taskProject = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskVersion = (Get-Content -LiteralPath (Join-Path $taskProject 'VERSION') -Raw).Trim()
$taskTestDirectory = Join-Path $taskProject 'dist\test'
if (!$Installer) { $Installer = Join-Path $taskTestDirectory "DCSManager-Setup-$taskVersion.exe" }
$Installer = [IO.Path]::GetFullPath($Installer)
if (![string]::Equals([IO.Path]::GetDirectoryName($Installer), $taskTestDirectory, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Utiliser uniquement le setup isolé produit avec -TestBuild dans dist/test.'
}
$taskSmoke = Join-Path $taskProject ('dist\smoke-' + [Guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $taskSmoke
$taskApp = Join-Path $taskSmoke 'app'
$taskData = Join-Path $taskSmoke 'userdata'
function Invoke-TestSetup([string]$LogName) {
    $taskProcess = Start-Process -FilePath $Installer -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',('/DIR="' + $taskApp + '"'),('/LOG="' + (Join-Path $taskSmoke $LogName) + '"')) -WindowStyle Hidden -Wait -PassThru
    if ($taskProcess.ExitCode -ne 0) { throw "Installation: $($taskProcess.ExitCode), voir $taskSmoke" }
}
Invoke-TestSetup 'install.log'
foreach ($taskFile in @('dcsmanager.exe','installed-mode','unins000.exe','profiles\panels\FA-18C_hornet.json')) {
    if (!(Test-Path -LiteralPath (Join-Path $taskApp $taskFile))) { throw "Fichier absent: $taskFile" }
}
if (!(Test-Path -LiteralPath (Join-Path $taskData 'settings.json'))) { throw 'Préparation utilisateur absente' }
$taskSettings = Get-Content -LiteralPath (Join-Path $taskData 'settings.json') -Raw
$null = New-Item -ItemType Directory -Path (Join-Path $taskData 'data') -Force
Set-Content -LiteralPath (Join-Path $taskData 'data\retention-test.txt') -Value 'preserve-this-data'
Invoke-TestSetup 'upgrade.log'
if ((Get-Content -LiteralPath (Join-Path $taskData 'settings.json') -Raw) -ne $taskSettings) { throw 'Configuration changée pendant mise à jour' }
if ((Get-Content -LiteralPath (Join-Path $taskData 'data\retention-test.txt') -Raw).Trim() -ne 'preserve-this-data') { throw 'Données changées pendant mise à jour' }
# The uninstaller below removes only this generated test application. User data
# stays in the sibling userdata folder; no production install or DCS is touched.
$taskUninstall = Start-Process -FilePath (Join-Path $taskApp 'unins000.exe') -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',('/LOG="' + (Join-Path $taskSmoke 'uninstall.log') + '"')) -WindowStyle Hidden -Wait -PassThru
if ($taskUninstall.ExitCode -ne 0) { throw "Désinstallation: $($taskUninstall.ExitCode)" }
if (Test-Path -LiteralPath (Join-Path $taskApp 'dcsmanager.exe')) { throw 'Programme encore présent après désinstallation' }
if (!(Test-Path -LiteralPath (Join-Path $taskData 'data\retention-test.txt'))) { throw 'Données supprimées par désinstallation' }
Write-Host "Installation, mise à jour et désinstallation réussies ; données conservées. Journaux : $taskSmoke"
