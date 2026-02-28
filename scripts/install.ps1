param(
  [string]$Version = "latest",
  [string]$InstallDir = "$HOME\bin"
)

$ErrorActionPreference = "Stop"

$Repo = "gelleson/autoport"
$Binary = "autoport.exe"
$Os = "windows"
$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

if ($Version -eq "latest") {
  $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $Release.tag_name
  if (-not $Version) {
    throw "Failed to resolve latest release tag"
  }
}

$Archive = "autoport_${Version}_${Os}_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"
$TempDir = Join-Path $env:TEMP ("autoport-install-" + [Guid]::NewGuid().ToString())
$ZipPath = Join-Path $TempDir $Archive

New-Item -ItemType Directory -Path $TempDir | Out-Null
if (-not (Test-Path $InstallDir)) {
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Write-Host "Downloading $Url"
Invoke-WebRequest -Uri $Url -OutFile $ZipPath
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
Copy-Item -Path (Join-Path $TempDir $Binary) -Destination (Join-Path $InstallDir $Binary) -Force

Write-Host "Installed $Binary $Version to $InstallDir"
Write-Host "Add '$InstallDir' to PATH if needed."
