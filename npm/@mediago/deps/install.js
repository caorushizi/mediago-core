#!/usr/bin/env node

/**
 * Post-install script for @mediago/deps
 * Copies platform-specific helper binaries into the local bin directory.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');

const PACKAGE_SCOPE = '@mediago';
const DEPS_PACKAGE = 'deps';
const DEPS_PLATFORM_PREFIX = 'deps-';
const PACKAGE_NAME = `${PACKAGE_SCOPE}/${DEPS_PACKAGE}`;
const BIN_DIR = 'bin';

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

function resolvePackageDir(packageName) {
  try {
    const pkgPath = require.resolve(`${packageName}/package.json`);
    return path.dirname(pkgPath);
  } catch (err) {
    console.error(`Error: Could not find ${packageName}.`);
    console.error(`Please reinstall ${PACKAGE_NAME} to ensure all dependencies are installed.`);
    process.exit(1);
  }
}

function copyDirectoryContents(srcDir, destDir) {
  if (!fs.existsSync(srcDir)) return;

  fs.mkdirSync(destDir, { recursive: true });
  for (const item of fs.readdirSync(srcDir)) {
    const srcPath = path.join(srcDir, item);
    const destPath = path.join(destDir, item);
    fs.cpSync(srcPath, destPath, { recursive: true });
  }
}

function ensureExecutablePermissions(dir) {
  if (os.platform() === 'win32') return;
  if (!fs.existsSync(dir)) return;
  for (const item of fs.readdirSync(dir)) {
    const filePath = path.join(dir, item);
    try {
      const stat = fs.statSync(filePath);
      if (stat.isFile()) {
        fs.chmodSync(filePath, 0o755);
      } else if (stat.isDirectory()) {
        ensureExecutablePermissions(filePath);
      }
    } catch {
      // ignore permission errors
    }
  }
}

function setupDeps() {
  const platform = detectPlatform();
  const rootDir = __dirname;
  const targetBinDir = path.join(rootDir, BIN_DIR);
  const depsPackageName = `${PACKAGE_SCOPE}/${DEPS_PLATFORM_PREFIX}${platform}`;
  const depsDir = resolvePackageDir(depsPackageName);
  const sourceBinDir = path.join(depsDir, BIN_DIR);

  console.log(`Setting up helper binaries for ${platform}...`);

  if (fs.existsSync(targetBinDir)) {
    fs.rmSync(targetBinDir, { recursive: true, force: true });
  }
  fs.mkdirSync(targetBinDir, { recursive: true });

  copyDirectoryContents(sourceBinDir, targetBinDir);
  ensureExecutablePermissions(targetBinDir);

  console.log('Helper binaries are ready.');
}

try {
  setupDeps();
} catch (err) {
  console.error('Installation failed:', err.message);
  process.exit(1);
}

