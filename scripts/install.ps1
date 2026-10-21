# Automated Global Installation Script for Windows
$ErrorActionPreference = "Stop"

$BIN_NAME = "csv-diff.exe"
$GO_BIN = "$([Environment]::GetFolderPath('UserProfile'))\go\bin"
$SCOOP_SHIMS = "$([Environment]::GetFolderPath('UserProfile'))\scoop\shims"

Write-Host "Installing CSV Diff globally..." -ForegroundColor Cyan

# 1. Build and install via go install
go install ./cmd/csv-diff

$INSTALLED_EXE = Join-Path $GO_BIN $BIN_NAME

if (!(Test-Path $INSTALLED_EXE)) {
    Write-Error "Failed to locate installed binary at $INSTALLED_EXE"
}

Write-Host "Compiled binary located at: $INSTALLED_EXE" -ForegroundColor Green

# 2. Check if Scoop shims exists (common in Windows dev setups)
if (Test-Path $SCOOP_SHIMS) {
    Copy-Item -Path $INSTALLED_EXE -Destination (Join-Path $SCOOP_SHIMS $BIN_NAME) -Force
    Write-Host "Copied $BIN_NAME to Scoop shims directory ($SCOOP_SHIMS)." -ForegroundColor Green
}

# 3. Ensure ~/go/bin is in User PATH
$USER_PATH = [Environment]::GetEnvironmentVariable("Path", "User")
if ($USER_PATH -notlike "*$GO_BIN*") {
    $NEW_PATH = "$USER_PATH;$GO_BIN"
    [Environment]::SetEnvironmentVariable("Path", $NEW_PATH, "User")
    $env:Path = "$env:Path;$GO_BIN"
    Write-Host "Added $GO_BIN to your User PATH environment variable." -ForegroundColor Green
} else {
    Write-Host "$GO_BIN is already in your User PATH." -ForegroundColor DarkGray
}

Write-Host "`nInstallation complete! You can now run 'csv-diff' from any terminal." -ForegroundColor Cyan
