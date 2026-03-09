# 🦎 Gecko installer for Windows
# Usage: irm https://raw.githubusercontent.com/gecko-iac/gecko/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo    = "gecko-iac/gecko"
$Binary  = "gecko.exe"
$InstallDir = if ($env:GECKO_INSTALL_DIR) { $env:GECKO_INSTALL_DIR } else { "$env:LOCALAPPDATA\gecko\bin" }

function Info($msg)    { Write-Host "  -> $msg" -ForegroundColor Cyan }
function Ok($msg)      { Write-Host "  v $msg"  -ForegroundColor Green }
function Fail($msg)    { Write-Host "  x $msg"  -ForegroundColor Red; exit 1 }

Write-Host ""
Write-Host "  🦎 gecko installer" -ForegroundColor Green
Write-Host "  ──────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

# Detect architecture
$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

# Fetch latest version
$Version = if ($env:GECKO_VERSION) {
    $env:GECKO_VERSION
} else {
    Info "Fetching latest version..."
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $release.tag_name -replace "^v", ""
}

Info "Installing gecko v$Version (windows/$Arch)"

$Filename     = "gecko_${Version}_windows_${Arch}.zip"
$DownloadUrl  = "https://github.com/$Repo/releases/download/v$Version/$Filename"
$ChecksumUrl  = "https://github.com/$Repo/releases/download/v$Version/gecko_${Version}_checksums.txt"
$TmpDir       = Join-Path $env:TEMP "gecko-install-$([System.Guid]::NewGuid().ToString('N'))"

New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    # Download archive
    Info "Downloading $Filename..."
    $ZipPath = Join-Path $TmpDir $Filename
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing

    # Verify checksum
    Info "Verifying checksum..."
    try {
        $ChecksumFile = Join-Path $TmpDir "checksums.txt"
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumFile -UseBasicParsing
        $Expected = (Select-String -Path $ChecksumFile -Pattern $Filename).Line.Split(" ")[0]
        $Actual   = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash.ToLower()
        if ($Expected -ne $Actual) { Fail "Checksum mismatch — aborting" }
        Ok "Checksum verified"
    } catch {
        Write-Host "  ! Could not verify checksum — continuing" -ForegroundColor Yellow
    }

    # Extract
    Info "Extracting..."
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force

    # Install
    $BinSrc = Join-Path $TmpDir $Binary
    if (-not (Test-Path $BinSrc)) { Fail "Binary not found in archive" }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $BinSrc -Destination (Join-Path $InstallDir $Binary) -Force

    # Add to PATH if not already there
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Info "Adding $InstallDir to PATH..."
        [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
        $env:PATH += ";$InstallDir"
    }

    Write-Host ""
    Ok "gecko v$Version installed to $InstallDir\$Binary"
    Write-Host ""
    Write-Host "  Run " -NoNewline
    Write-Host "gecko --version" -ForegroundColor Cyan -NoNewline
    Write-Host " to confirm."
    Write-Host "  Run " -NoNewline
    Write-Host "gecko help" -ForegroundColor Cyan -NoNewline
    Write-Host " to get started."
    Write-Host ""

} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
