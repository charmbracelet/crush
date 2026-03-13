#!/usr/bin/env node
/**
 * Crush CLI wrapper script
 * 
 * This script finds and executes the platform-specific Crush binary
 * from the optionalDependencies packages.
 * 
 * Pattern based on esbuild, turbo, and opencode distribution.
 */

const { execFileSync, spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

const PACKAGE_NAME = '@charmland/crush';

/**
 * Get the platform-specific package name
 */
function getPlatformPackage() {
  const platform = process.platform;
  const arch = process.arch;
  return `${PACKAGE_NAME}-${platform}-${arch}`;
}

/**
 * Find the binary in various possible locations
 */
function findBinary() {
  const platform = process.platform;
  const arch = process.arch;
  const platformPkg = getPlatformPackage();
  const binaryName = platform === 'win32' ? 'crush.exe' : 'crush';
  
  // Possible locations to check
  const locations = [
    // Standard node_modules location (most common)
    path.join(__dirname, '..', '..', platformPkg, 'bin', binaryName),
    
    // Hoisted to root node_modules
    path.join(__dirname, '..', '..', '..', platformPkg, 'bin', binaryName),
    
    // pnpm/yarn PnP style
    (() => {
      try {
        const resolved = require.resolve(`${platformPkg}/bin/${binaryName}`);
        return resolved;
      } catch {
        return null;
      }
    })(),
    
    // Fallback: try to resolve the package and find bin inside
    (() => {
      try {
        const pkgPath = require.resolve(`${platformPkg}/package.json`);
        return path.join(path.dirname(pkgPath), 'bin', binaryName);
      } catch {
        return null;
      }
    })(),
  ].filter(Boolean);
  
  for (const loc of locations) {
    if (fs.existsSync(loc)) {
      return loc;
    }
  }
  
  return null;
}

/**
 * Print helpful error message when binary is not found
 */
function printNotFoundError() {
  const platform = process.platform;
  const arch = process.arch;
  const platformPkg = getPlatformPackage();
  
  console.error(`
Crush binary not found for ${platform}-${arch}

The platform-specific package "${platformPkg}" was not installed.

This can happen if:
1. Your platform (${platform}-${arch}) is not supported
2. npm failed to install the optional dependency
3. You're using a package manager that doesn't support optionalDependencies

To fix this, try:
  npm install ${platformPkg}

Or install Crush directly:
  # macOS
  brew install charmbracelet/tap/crush
  
  # Windows
  winget install charmbracelet.crush
  
  # Linux (Debian/Ubuntu)
  sudo apt install crush
  
  # Or download from:
  https://github.com/charmbracelet/crush/releases

Supported platforms:
  - linux-x64, linux-arm64, linux-ia32, linux-arm
  - darwin-x64, darwin-arm64
  - win32-x64, win32-arm64, win32-ia32
`);
}

/**
 * Main entry point
 */
function main() {
  const binaryPath = findBinary();
  
  if (!binaryPath) {
    printNotFoundError();
    process.exit(1);
  }
  
  // Forward all arguments to the binary
  const args = process.argv.slice(2);
  
  // Use spawn for better signal handling and TTY support
  const child = spawn(binaryPath, args, {
    stdio: 'inherit',
    windowsHide: false,
  });
  
  child.on('error', (err) => {
    console.error(`Failed to execute Crush: ${err.message}`);
    process.exit(1);
  });
  
  child.on('exit', (code, signal) => {
    if (signal) {
      // Re-raise the signal
      process.kill(process.pid, signal);
    } else {
      process.exit(code ?? 1);
    }
  });
  
  // Forward signals to child process
  ['SIGINT', 'SIGTERM', 'SIGHUP'].forEach((signal) => {
    process.on(signal, () => {
      if (!child.killed) {
        child.kill(signal);
      }
    });
  });
}

main();
