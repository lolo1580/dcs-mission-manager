param([string]$Executable = (Join-Path $PSScriptRoot '..\dist\staging\dcsmanager.exe'))
$ErrorActionPreference = 'Stop'
$taskAssetDir = Join-Path $PSScriptRoot '..\installer\assets'
$null = New-Item -ItemType Directory -Path $taskAssetDir -Force
Add-Type -AssemblyName System.Drawing
$taskLogo = [Drawing.Image]::FromFile((Join-Path $PSScriptRoot '..\frontend\src\assets\logo.png'))
try {
    foreach ($taskSize in @(@{Name='wizard.bmp';Width=164;Height=314;Logo=128},@{Name='header.bmp';Width=55;Height=55;Logo=48})) {
        $taskBitmap = [Drawing.Bitmap]::new($taskSize.Width,$taskSize.Height)
        $taskGraphics = [Drawing.Graphics]::FromImage($taskBitmap)
        try {
            $taskGraphics.Clear([Drawing.Color]::FromArgb(17,25,39))
            $taskGraphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
            $taskX = [int](($taskSize.Width-$taskSize.Logo)/2)
            $taskY = if ($taskSize.Height -gt 100) { 45 } else { [int](($taskSize.Height-$taskSize.Logo)/2) }
            $taskGraphics.DrawImage($taskLogo,$taskX,$taskY,$taskSize.Logo,$taskSize.Logo)
            if ($taskSize.Height -gt 100) {
                $taskFont=[Drawing.Font]::new('Segoe UI',15,[Drawing.FontStyle]::Bold)
                $taskBrush=[Drawing.SolidBrush]::new([Drawing.Color]::FromArgb(75,195,230))
                $taskFormat=[Drawing.StringFormat]::new();$taskFormat.Alignment=[Drawing.StringAlignment]::Center
                try { $taskGraphics.DrawString("DCS`nMANAGER",$taskFont,$taskBrush,[Drawing.RectangleF]::new(0,205,164,80),$taskFormat) }
                finally {$taskFont.Dispose();$taskBrush.Dispose();$taskFormat.Dispose()}
            }
            $taskBitmap.Save((Join-Path $taskAssetDir $taskSize.Name),[Drawing.Imaging.ImageFormat]::Bmp)
        } finally {$taskGraphics.Dispose();$taskBitmap.Dispose()}
    }
} finally {$taskLogo.Dispose()}
$taskIcon=[Drawing.Icon]::ExtractAssociatedIcon([IO.Path]::GetFullPath($Executable))
$taskStream=[IO.File]::Create((Join-Path $taskAssetDir 'setup.ico'))
try {$taskIcon.Save($taskStream)} finally {$taskStream.Dispose();$taskIcon.Dispose()}
