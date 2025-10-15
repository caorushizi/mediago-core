#!/usr/bin/env node

/**
 * Post-install script for @mediago/core
 * Detects the current platform and symlinks/copies the binary from the appropriate optional dependency
 */

const fs = require('fs');
const path = require('path');
const os = require('os');

const PACKAGE_NAME = '@mediago/core';

// Platform detection
function detectPlatform() {
  const platform = os.platform();
  const arch = os.arch();

  const platformMap = {
    darwin: { x64: 'darwin-x64', arm64: 'darwin-arm64' },
    linux: { x64: 'linux-x64', arm64: 'linux-arm64' },
    win32: { x64: 'win32-x64', arm64: 'win32-arm64' },
  };

  const archMap = platformMap[platform];
  if (!archMap) {
    throw new Error(`Unsupported platform: ${platform}`);
  }

  const target = archMap[arch];
  if (!target) {
    throw new Error(`Unsupported architecture: ${arch} on ${platform}`);
  }

  return target;
}

// Find the platform package path
function findPlatformPackage(platformName) {
  const packageName = `${PACKAGE_NAME}-${platformName}`;

  // Try to resolve the package from node_modules
  try {
    const pkgPath = require.resolve(`${packageName}/package.json`);
    return path.dirname(pkgPath);
  } catch (err) {
    console.error(
      `Error: Could not find ${packageName}. This package is required for ${platformName}.`
    );
    console.error(
      `Please run 'npm install' again to ensure all dependencies are installed.`
    );
    process.exit(1);
  }
}

// Setup the binary
function setupBinary() {
  const platform = detectPlatform();
  const isWindows = platform.startsWith('win32');
  const binaryName = isWindows ? 'mediago-core.exe' : 'mediago-core';

  console.log(`Setting up MediaGo Player for ${platform}...`);

  // Find the platform-specific package
  const platformPkgDir = findPlatformPackage(platform);
  const sourceBinary = path.join(platformPkgDir, 'bin', binaryName);

  // Check if source binary exists
  if (!fs.existsSync(sourceBinary)) {
    console.error(`Error: Binary not found at ${sourceBinary}`);
    process.exit(1);
  }

  // Create bin directory in the root package
  const rootDir = __dirname;
  const binDir = path.join(rootDir, 'bin');
  const targetBinary = path.join(binDir, isWindows ? 'mediago-core.exe' : 'mediago-core');

  // Create bin directory if it doesn't exist
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  // Remove existing binary if it exists
  if (fs.existsSync(targetBinary)) {
    fs.unlinkSync(targetBinary);
  }

  // On Windows, copy the binary; on Unix, create a symlink
  if (isWindows) {
    fs.copyFileSync(sourceBinary, targetBinary);
    console.log(`Copied binary from ${sourceBinary}`);
  } else {
    // Create relative symlink
    const relativePath = path.relative(binDir, sourceBinary);
    fs.symlinkSync(relativePath, targetBinary);
    console.log(`Created symlink from ${sourceBinary}`);
  }

  console.log(`MediaGo Player is ready! Run 'npx ${PACKAGE_NAME}' to start.`);
}

// Main
try {
  setupBinary();
} catch (err) {
  console.error('Installation failed:', err.message);
  process.exit(1);
}
