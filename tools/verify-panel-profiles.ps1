param(
    [string]$ProfilesPath = (Join-Path $PSScriptRoot '..\data\mappings.json'),
    [string]$MetadataDir = (Join-Path $env:USERPROFILE 'Saved Games\DCS\Scripts\DCS-BIOS\doc\json')
)

# Read-only verification: never starts DCS or sends panel commands.
$ErrorActionPreference = 'Stop'
$profiles = (Get-Content -LiteralPath $ProfilesPath -Raw | ConvertFrom-Json).profiles
$summary = @()
$findings = @()
foreach ($profile in $profiles) {
    $aircraft = $profile.aircraft
    $modulePath = Join-Path $MetadataDir ($aircraft + '.json')
    if (!(Test-Path -LiteralPath $modulePath)) {
        $findings += "ERROR $aircraft : catalogue absent"
        continue
    }
    $module = Get-Content -LiteralPath $modulePath -Raw | ConvertFrom-Json
    $catalog = @{}
    foreach ($category in $module.PSObject.Properties) {
        foreach ($control in $category.Value.PSObject.Properties) { $catalog[$control.Name] = $control.Value }
    }
    $keys = @{}
    if ($aircraft -eq 'FA-18C_hornet') {
        $catalog['DCSM_PITCH_TRIM'] = [pscustomobject]@{ inputs = @([pscustomobject]@{ interface='variable_step'; suggested_step=1 }) }
    }
    foreach ($binding in $profile.bindings) {
        $key = "$($binding.model)/$($binding.control)/$($binding.mode)".ToUpperInvariant()
        if ($keys.ContainsKey($key)) { $findings += "ERROR $aircraft : attribution dupliquee $key" }
        $keys[$key] = $true
        $control = $catalog[$binding.command]
        if (!$control) { $findings += "ERROR $aircraft : commande absente $($binding.command)"; continue }
        $interfaces = @($control.inputs | Where-Object { $_.interface -eq $binding.interface })
        if (!$interfaces.Count) { $findings += "ERROR $aircraft : interface incompatible $($binding.command)/$($binding.interface)"; continue }
        foreach ($property in @('state_on','state_off','pulse_reset')) {
            if ($null -ne $binding.$property -and ($binding.interface -ne 'set_state' -or $binding.$property -lt 0 -or $binding.$property -gt $interfaces[0].max_value)) {
                $findings += "ERROR $aircraft/$($binding.control) : position personnalisee invalide $property"
            }
        }
        if ($binding.interface -eq 'set_state' -and $interfaces[0].max_value -gt 1) {
            $maximum = $interfaces[0].max_value
            $on = if ($null -ne $binding.state_on) { $binding.state_on } else { $maximum }
            $off = if ($null -ne $binding.state_off) { $binding.state_off } else { 0 }
            if ($binding.invert) { $swap = $on; $on = $off; $off = $swap }
            $positions = @($control.positions) -join ', '
            $prefix = if ($null -ne $binding.state_on -or $null -ne $binding.state_off) { 'INFO' } else { 'WARN' }
            $findings += "$prefix $aircraft/$($binding.control) -> $($binding.command) : activation=$on, desactivation=$off (relachement ignore pour volets/selecteurs); positions=[$positions]"
        }
    }
    foreach ($output in $profile.outputs) {
        $control = $catalog[$output.command]
        if (!$control -or !@($control.outputs).Count) { $findings += "ERROR $aircraft : source LED absente $($output.command)" }
        foreach ($rule in $output.rules) {
            $source = $catalog[$rule.command]
            if (!$source -or $rule.export -lt 0 -or $rule.export -ge @($source.outputs).Count) { $findings += "ERROR $aircraft : source de regle LED invalide $($rule.command)" }
        }
    }
    foreach ($display in $profile.displays) {
        $control = $catalog[$display.command]
        if (!$control -or $display.export -lt 0 -or $display.export -ge @($control.outputs).Count) {
            $findings += "ERROR $aircraft : source LCD invalide $($display.command)"; continue
        }
        if ($control.outputs[$display.export].type -ne 'integer') { $findings += "ERROR $aircraft : source LCD non numerique $($display.command)" }
    }
    $summary += [pscustomobject]@{Aircraft=$aircraft; Inputs=@($profile.bindings).Count; LEDs=@($profile.outputs).Count; LCD=@($profile.displays).Count}
}
$summary | Format-Table -AutoSize
$findings | ForEach-Object { Write-Output $_ }
Write-Output ('Profile SHA256: ' + (Get-FileHash -LiteralPath $ProfilesPath -Algorithm SHA256).Hash)
if (@($findings | Where-Object { $_ -like 'ERROR *' }).Count) { exit 1 }
