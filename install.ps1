# Lumina Desktop installer for Windows.
#   irm https://raw.githubusercontent.com/shivarchit/Lumina/master/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$Repo = 'shivarchit/Lumina'
$Asset = 'Lumina-Desktop-windows-amd64-installer.exe'
$Url = "https://github.com/$Repo/releases/latest/download/$Asset"
$Out = Join-Path $env:TEMP $Asset

Write-Host "Downloading $Asset ..."
Invoke-WebRequest -Uri $Url -OutFile $Out

Write-Host 'Running installer (silent) ...'
# NSIS silent install; requests elevation (UAC) for Program Files.
Start-Process -FilePath $Out -ArgumentList '/S' -Wait

Remove-Item $Out -ErrorAction SilentlyContinue
Write-Host 'Lumina Desktop installed — find it in the Start menu.'
