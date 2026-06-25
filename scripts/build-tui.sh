#!/usr/bin/env bash
# Build pre-compiled labproxy-tui binaries for multiple architectures
# and upload them to GitHub Releases.
#
# Usage:
#   VERSION=v1.0.0 bash scripts/build-tui.sh    # tag + release
#   VERSION=dev bash scripts/build-tui.sh        # dry-run, skip release

set -e

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT" || exit 1

BUILD_DIR="${PROJECT_ROOT}/build"
VERSION="${VERSION:-dev}"

mkdir -p "$BUILD_DIR"

echo "Building labproxy-tui ${VERSION}..."

TARGETS=(
    "linux/amd64"
    "linux/386"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

ARCHIVES=()

for target in "${TARGETS[@]}"; do
    GOOS="${target%/*}"
    GOARCH="${target#*/}"

    echo "Building for ${GOOS}/${GOARCH}..."

    BIN_NAME="labproxy-tui-${GOOS}-${GOARCH}"
    ARCHIVE_NAME="labproxy-tui-${GOOS}-${GOARCH}.tar.gz"

    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o "$BUILD_DIR/$BIN_NAME" \
        ./cmd/labproxy-tui

    (cd "$BUILD_DIR" && tar -czf "$ARCHIVE_NAME" "$BIN_NAME")
    ARCHIVES+=("$BUILD_DIR/$ARCHIVE_NAME")

    echo "  -> $BUILD_DIR/$ARCHIVE_NAME"
done

echo ""
echo "Build complete!"

# ── GitHub Release ──────────────────────────────────────────────────
if [ "$VERSION" = "dev" ]; then
    echo "VERSION=dev — skipping GitHub Release (dry-run mode)"
    echo "Binaries are in $BUILD_DIR/"
    ls -lh "$BUILD_DIR"/labproxy-tui-*
    exit 0
fi

if ! command -v gh &>/dev/null; then
    echo "gh CLI not found — skipping GitHub Release"
    echo "Install: https://cli.github.com/"
    exit 0
fi

if ! gh auth status &>/dev/null; then
    echo "gh not authenticated — skipping GitHub Release"
    echo "Run: gh auth login"
    exit 0
fi

# Create git tag if it doesn't exist
if ! git rev-parse "$VERSION" &>/dev/null; then
    echo "Creating tag $VERSION..."
    git tag "$VERSION"
    git push origin "$VERSION"
fi

# Create or reuse GitHub Release
if gh release view "$VERSION" &>/dev/null; then
    echo "Release $VERSION already exists, uploading assets..."
    gh release upload "$VERSION" "${ARCHIVES[@]}" --clobber
else
    echo "Creating release $VERSION..."
    gh release create "$VERSION" "${ARCHIVES[@]}" \
        --title "labproxy-tui ${VERSION}" \
        --notes "Pre-compiled labproxy-tui binaries for ${VERSION}.

Built from $(git rev-parse --short HEAD)."
fi

echo ""
echo "Release: https://github.com/Azhi-ss/labproxy/releases/tag/${VERSION}"
