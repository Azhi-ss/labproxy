#!/usr/bin/env bash
# Build pre-compiled clash-tui binaries for multiple architectures

set -e

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT" || exit 1

# Output directory for built binaries
BUILD_DIR="${PROJECT_ROOT}/build"
ZIP_DIR="${PROJECT_ROOT}/resources/zip"

mkdir -p "$BUILD_DIR" "$ZIP_DIR"

# Version information (can be passed via env var)
VERSION="${VERSION:-dev}"

echo "Building clash-tui ${VERSION}..."

# Build targets
TARGETS=(
    "linux/amd64"
    "linux/386"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

for target in "${TARGETS[@]}"; do
    GOOS="${target%/*}"
    GOARCH="${target#*/}"

    echo "Building for ${GOOS}/${GOARCH}..."

    BIN_NAME="clash-tui-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        BIN_NAME="${BIN_NAME}.exe"
    fi

    # Build
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o "$BUILD_DIR/$BIN_NAME" \
        ./cmd/clash-tui

    # Package as tar.gz (or zip for Windows)
    ARCHIVE_NAME="clash-tui-${GOOS}-${GOARCH}.tar.gz"
    if [ "$GOOS" = "windows" ]; then
        ARCHIVE_NAME="clash-tui-${GOOS}-${GOARCH}.zip"
        (cd "$BUILD_DIR" && zip "$ZIP_DIR/$ARCHIVE_NAME" "$BIN_NAME")
    else
        (cd "$BUILD_DIR" && tar -czf "$ZIP_DIR/$ARCHIVE_NAME" "$BIN_NAME")
    fi

    echo "  -> $ZIP_DIR/$ARCHIVE_NAME"
done

echo ""
echo "Build complete! Binaries in $ZIP_DIR/"
ls -lh "$ZIP_DIR/clash-tui-"*
