#!/bin/bash
# VeniceCode Multi-Platform Build Script

set -e

VERSION="${1:-1.0.0}"
BUILD_DIR="dist"
BINARY_NAME="venicecode"

echo "Building VeniceCode v${VERSION} for multiple platforms..."

# Clean previous builds
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Build flags
LDFLAGS="-X main.version=${VERSION} -X main.name=VeniceCode -s -w"

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$PLATFORM"
    OUTPUT_NAME="${BINARY_NAME}-${GOOS}-${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "${BUILD_DIR}/${OUTPUT_NAME}" \
        .
    
    # Create tarball (except for Windows)
    if [ "$GOOS" != "windows" ]; then
        tar -czf "${BUILD_DIR}/${BINARY_NAME}-${GOOS}-${GOARCH}.tar.gz" \
            -C "$BUILD_DIR" "${OUTPUT_NAME}"
        rm "${BUILD_DIR}/${OUTPUT_NAME}"
    else
        # Create zip for Windows
        (cd "$BUILD_DIR" && zip "${BINARY_NAME}-${GOOS}-${GOARCH}.zip" "${OUTPUT_NAME}")
        rm "${BUILD_DIR}/${OUTPUT_NAME}"
    fi
    
    echo "✓ Built ${OUTPUT_NAME}"
done

# Generate checksums
echo "Generating checksums..."
(cd "$BUILD_DIR" && shasum -a 256 * > SHA256SUMS)

echo ""
echo "Build complete! Artifacts in ${BUILD_DIR}/"
ls -lh "$BUILD_DIR"

echo ""
echo "Upload these files to GitHub Releases:"
echo "  https://github.com/georgeglarson/venicecode/releases/new"
