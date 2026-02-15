#!/bin/bash
# build-npm-packages.sh
# Builds npm packages using the optionalDependencies pattern (like esbuild/opencode)
# This creates platform-specific packages with embedded binaries instead of postinstall downloads
#
# Usage: ./scripts/npm-optional-deps/build-npm-packages.sh <version> <dist-dir>
# Example: ./scripts/npm-optional-deps/build-npm-packages.sh 0.43.0 ./dist

set -euo pipefail

VERSION="${1:-}"
DIST_DIR="${2:-./dist}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NPM_DIR="${DIST_DIR}/npm"

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [dist-dir]"
    echo "Example: $0 0.43.0 ./dist"
    exit 1
fi

echo "Building npm packages for Crush v${VERSION}"
echo "Distribution directory: ${DIST_DIR}"

# Clean and create npm directory
rm -rf "${NPM_DIR}"
mkdir -p "${NPM_DIR}"

# Platform mappings: goreleaser archive name -> npm platform-arch
declare -A PLATFORMS=(
    ["Linux_x86_64"]="linux-x64"
    ["Linux_arm64"]="linux-arm64"
    ["Linux_i386"]="linux-ia32"
    ["Linux_armv7"]="linux-arm"
    ["Darwin_x86_64"]="darwin-x64"
    ["Darwin_arm64"]="darwin-arm64"
    ["Windows_x86_64"]="win32-x64"
    ["Windows_arm64"]="win32-arm64"
    ["Windows_i386"]="win32-ia32"
)

# OS mappings for package.json
declare -A OS_MAP=(
    ["linux-x64"]="linux"
    ["linux-arm64"]="linux"
    ["linux-ia32"]="linux"
    ["linux-arm"]="linux"
    ["darwin-x64"]="darwin"
    ["darwin-arm64"]="darwin"
    ["win32-x64"]="win32"
    ["win32-arm64"]="win32"
    ["win32-ia32"]="win32"
)

# CPU mappings for package.json
declare -A CPU_MAP=(
    ["linux-x64"]="x64"
    ["linux-arm64"]="arm64"
    ["linux-ia32"]="ia32"
    ["linux-arm"]="arm"
    ["darwin-x64"]="x64"
    ["darwin-arm64"]="arm64"
    ["win32-x64"]="x64"
    ["win32-arm64"]="arm64"
    ["win32-ia32"]="ia32"
)

OPTIONAL_DEPS=""
BUILT_PLATFORMS=""

# Build platform-specific packages
for archive_suffix in "${!PLATFORMS[@]}"; do
    npm_platform="${PLATFORMS[$archive_suffix]}"
    os="${OS_MAP[$npm_platform]}"
    cpu="${CPU_MAP[$npm_platform]}"
    
    # Find the archive
    if [ "$os" = "win32" ]; then
        archive="${DIST_DIR}/crush_${VERSION}_${archive_suffix}.zip"
        binary_name="crush.exe"
    else
        archive="${DIST_DIR}/crush_${VERSION}_${archive_suffix}.tar.gz"
        binary_name="crush"
    fi
    
    if [ ! -f "$archive" ]; then
        echo "  Skipping ${npm_platform}: archive not found (${archive})"
        continue
    fi
    
    echo "  Building @charmland/crush-${npm_platform}..."
    
    pkg_dir="${NPM_DIR}/crush-${npm_platform}"
    mkdir -p "${pkg_dir}/bin"
    
    # Extract binary
    if [ "$os" = "win32" ]; then
        unzip -q -j "$archive" "*/${binary_name}" -d "${pkg_dir}/bin/" 2>/dev/null || \
        unzip -q -j "$archive" "${binary_name}" -d "${pkg_dir}/bin/" 2>/dev/null || \
        unzip -q "$archive" -d "${pkg_dir}/temp" && mv "${pkg_dir}/temp"/*/"${binary_name}" "${pkg_dir}/bin/" && rm -rf "${pkg_dir}/temp"
    else
        tar -xzf "$archive" --strip-components=1 -C "${pkg_dir}/bin" --wildcards "*/${binary_name}" 2>/dev/null || \
        tar -xzf "$archive" -C "${pkg_dir}/bin" "${binary_name}" 2>/dev/null || \
        (tar -xzf "$archive" -C "${pkg_dir}" && mv "${pkg_dir}"/crush_*/"${binary_name}" "${pkg_dir}/bin/" && rm -rf "${pkg_dir}"/crush_*)
    fi
    
    chmod +x "${pkg_dir}/bin/${binary_name}" 2>/dev/null || true
    
    # Create package.json
    cat > "${pkg_dir}/package.json" << EOF
{
  "name": "@charmland/crush-${npm_platform}",
  "version": "${VERSION}",
  "description": "Crush binary for ${npm_platform}",
  "license": "FSL-1.1-MIT",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/charmbracelet/crush.git"
  },
  "homepage": "https://charm.sh/crush",
  "bugs": {
    "url": "https://github.com/charmbracelet/crush/issues"
  },
  "os": ["${os}"],
  "cpu": ["${cpu}"],
  "files": ["bin/"],
  "preferUnplugged": true
}
EOF
    
    # Create README
    cat > "${pkg_dir}/README.md" << EOF
# @charmland/crush-${npm_platform}

Platform-specific binary package for Crush on ${npm_platform}.

This package is automatically installed as a dependency of \`@charmland/crush\`.
You should not need to install this package directly.

## About Crush

Crush is a glamorous agentic coding assistant for your terminal.
Learn more at https://charm.sh/crush
EOF
    
    # Pack the package
    (cd "${pkg_dir}" && npm pack --pack-destination "${NPM_DIR}")
    
    # Add to optional dependencies
    if [ -n "$OPTIONAL_DEPS" ]; then
        OPTIONAL_DEPS="${OPTIONAL_DEPS},"
    fi
    OPTIONAL_DEPS="${OPTIONAL_DEPS}
    \"@charmland/crush-${npm_platform}\": \"${VERSION}\""
    
    BUILT_PLATFORMS="${BUILT_PLATFORMS} ${npm_platform}"
done

echo ""
echo "Building main @charmland/crush package..."

# Create main wrapper package
main_pkg="${NPM_DIR}/crush"
mkdir -p "${main_pkg}/bin"

# Copy wrapper script
cp "${SCRIPT_DIR}/crush-wrapper.js" "${main_pkg}/bin/crush.js"
chmod +x "${main_pkg}/bin/crush.js"

# Copy README and LICENSE from repo root
cp "${DIST_DIR}/../README.md" "${main_pkg}/" 2>/dev/null || echo "# Crush" > "${main_pkg}/README.md"
cp "${DIST_DIR}/../LICENSE.md" "${main_pkg}/" 2>/dev/null || true

# Create package.json
cat > "${main_pkg}/package.json" << EOF
{
  "name": "@charmland/crush",
  "version": "${VERSION}",
  "description": "Glamourous agentic coding for all - A powerful terminal-based AI assistant for developers",
  "license": "FSL-1.1-MIT",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/charmbracelet/crush.git"
  },
  "homepage": "https://charm.sh/crush",
  "bugs": {
    "url": "https://github.com/charmbracelet/crush/issues"
  },
  "keywords": [
    "ai",
    "cli",
    "coding",
    "assistant",
    "terminal",
    "llm",
    "agent",
    "charm"
  ],
  "bin": {
    "crush": "./bin/crush.js"
  },
  "files": ["bin/", "README.md", "LICENSE.md"],
  "engines": {
    "node": ">=16"
  },
  "optionalDependencies": {${OPTIONAL_DEPS}
  }
}
EOF

# Pack main package
(cd "${main_pkg}" && npm pack --pack-destination "${NPM_DIR}")

echo ""
echo "=========================================="
echo "npm packages built successfully!"
echo "=========================================="
echo ""
echo "Output directory: ${NPM_DIR}"
echo "Platforms built:${BUILT_PLATFORMS}"
echo ""
echo "Packages created:"
ls -la "${NPM_DIR}"/*.tgz
echo ""
echo "To publish:"
echo "  cd ${NPM_DIR}"
echo "  npm publish charmland-crush-*.tgz --access public"
echo "  npm publish charmland-crush-${VERSION}.tgz --access public"
