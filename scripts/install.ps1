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

$VersionNum = $Version.TrimStart("v")
$TempDir = Join-Path $env:TEMP ("autoport-install-" + [Guid]::NewGuid().ToString())

New-Item -ItemType Directory -Path $TempDir | Out-Null
if (-not (Test-Path $InstallDir)) {
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Candidates = @(
  "autoport_${Version}_${Os}_${Arch}.zip",
  "autoport_${VersionNum}_${Os}_${Arch}.zip",
  "autoport_${Version}_${Os}_${Arch}.exe",
  "autoport_${VersionNum}_${Os}_${Arch}.exe"
)

$Asset = $null
foreach ($Candidate in $Candidates) {
  $Url = "https://github.com/$Repo/releases/download/$Version/$Candidate"
  $OutPath = Join-Path $TempDir $Candidate
  Write-Host "Downloading $Url"
  try {
    Invoke-WebRequest -Uri $Url -OutFile $OutPath
    $Asset = $Candidate
    break
  } catch {
  }
}

if (-not $Asset) {
  throw "No matching release asset found for $Os/$Arch under tag $Version"
}

$AssetPath = Join-Path $TempDir $Asset
if ($Asset.EndsWith(".zip")) {
  Expand-Archive -Path $AssetPath -DestinationPath $TempDir -Force
  Copy-Item -Path (Join-Path $TempDir $Binary) -Destination (Join-Path $InstallDir $Binary) -Force
} else {
  Copy-Item -Path $AssetPath -Destination (Join-Path $InstallDir $Binary) -Force
}

Write-Host "Installed $Binary $Version to $InstallDir"
Write-Host "Add '$InstallDir' to PATH if needed."
