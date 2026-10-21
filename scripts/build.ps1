# Cross-platform build script for Windows PowerShell
$ErrorActionPreference = "Stop"

$BUILD_DIR = "bin"
if (!(Test-Path $BUILD_DIR)) {
    New-Item -ItemType Directory -Path $BUILD_DIR | Out-Null
}

$TARGETS = @(
    @{ OS = "windows"; ARCH = "amd64"; EXT = ".exe" },
    @{ OS = "darwin";  ARCH = "arm64"; EXT = "" },
    @{ OS = "darwin";  ARCH = "amd64"; EXT = "" },
    @{ OS = "linux";   ARCH = "amd64"; EXT = "" },
    @{ OS = "linux";   ARCH = "arm64"; EXT = "" }
)

Write-Host "Starting cross-platform builds..." -ForegroundColor Cyan

foreach ($t in $TARGETS) {
    $os = $t.OS
    $arch = $t.ARCH
    $ext = $t.EXT
    $OUTPUT = "$BUILD_DIR/csv-diff-$os-$arch$ext"
    Write-Host "Building for $os/$arch -> $OUTPUT"
    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o $OUTPUT ./cmd/csv-diff
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed for $os/$arch"
    }
}

# Reset env vars
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

Write-Host "All cross-platform builds completed successfully!" -ForegroundColor Green
Get-ChildItem -Path $BUILD_DIR | Select-Object Name, Length
