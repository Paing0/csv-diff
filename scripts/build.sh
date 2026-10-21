#!/usr/bin/env bash
set -euo pipefail

BUILD_DIR="bin"
mkdir -p "$BUILD_DIR"

targets=(
    "windows/amd64/.exe"
    "darwin/arm64/"
    "darwin/amd64/"
    "linux/amd64/"
    "linux/arm64/"
)

echo "Starting cross-platform builds..."

for target in "${targets[@]}"; do
    IFS="/" read -r GOOS GOARCH EXT <<< "$target"
    OUTPUT="$BUILD_DIR/csv-diff-${GOOS}-${GOARCH}${EXT}"
    echo "Building for ${GOOS}/${GOARCH} -> ${OUTPUT}..."
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags="-s -w" -o "$OUTPUT" ./cmd/csv-diff
done

echo "All targets built successfully into $BUILD_DIR/"
ls -lh "$BUILD_DIR"
