param([string]$ISCC,[switch]$SkipBuild,[switch]$TestBuild)
$ErrorActionPreference = 'Stop'
$taskProject = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskStage = Join-Path $taskProject 'dist\staging'
$null = New-Item -ItemType Directory -Path $taskStage -Force
$taskVersion = (Get-Content -LiteralPath (Join-Path $taskProject 'VERSION') -Raw).Trim()
function Assert-Native { if ($LASTEXITCODE -ne 0) { throw "Compilation interrompue: code $LASTEXITCODE" } }
if (!$ISCC) {
    $taskCandidates = @((Join-Path $taskProject '.tools\inno\ISCC.exe'),"${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe","${env:ProgramFiles}\Inno Setup 7\ISCC.exe")
    $ISCC = $taskCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if (!$ISCC) { $taskCommand = Get-Command ISCC.exe -ErrorAction SilentlyContinue; if ($taskCommand) { $ISCC = $taskCommand.Source } }
}
if (!$ISCC) { throw 'Inno Setup 6.7+ requis. Installer depuis https://jrsoftware.org/isdl.php ou fournir -ISCC.' }
if (!$SkipBuild) {
    Push-Location (Join-Path $taskProject 'frontend')
    try {
        if (!(Test-Path -LiteralPath 'node_modules')) { npm.cmd ci; Assert-Native }
        npm.cmd run build; Assert-Native
    } finally { Pop-Location }
    Push-Location $taskProject
    try { node tools/gen-lua-embed.mjs; Assert-Native } finally { Pop-Location }
    Push-Location (Join-Path $taskProject 'backend')
    try { go build -trimpath -ldflags="-s -w -X main.Version=$taskVersion" -o (Join-Path $taskStage 'dcsmanager.exe') ./cmd/dcsmanager; Assert-Native }
    finally { Pop-Location }
}
if (!(Test-Path -LiteralPath (Join-Path $taskStage 'dcsmanager.exe'))) { throw 'Exécutable préparé absent.' }
& (Join-Path $PSScriptRoot 'build-installer-assets.ps1') -Executable (Join-Path $taskStage 'dcsmanager.exe')
$taskOutput = Join-Path $taskProject 'dist'
if ($TestBuild) { $taskOutput = Join-Path $taskOutput 'test'; $null=New-Item -ItemType Directory -Path $taskOutput -Force }
$taskArguments = @("/DAppVersion=$taskVersion","/DStageDir=$taskStage","/DOutputPath=$taskOutput")
if ($TestBuild) { $taskArguments += '/DTestBuild=1' }
& $ISCC @taskArguments (Join-Path $taskProject 'installer\DCSManager.iss'); Assert-Native
Get-Item -LiteralPath (Join-Path $taskOutput "DCSManager-Setup-$taskVersion.exe") | Select-Object FullName,Length
