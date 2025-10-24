#!/usr/bin/env node

/**
 * Post-install script for {{npmScope}}/{{corePackageName}}
 * Detects the current platform and assembles files from split core/deps packages.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const PACKAGE_SCOPE = '{{npmScope}}';
const CORE_PACKAGE = '{{corePackageName}}';
const CORE_PLATFORM_PREFIX = '{{corePlatformPrefix}}';
const DEPS_PLATFORM_PREFIX = '{{depsPlatformPrefix}}';
const TARGET_FILES_DIR = '{{filesDir}}';
const PACKAGE_NAME = `${PACKAGE_SCOPE}/${CORE_PACKAGE}`;
const CORE_CONFIG_FILE = '{{downloadSchemaFile}}';
const DEPS_DIR = '{{packageBinDir}}';

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

function resolvePackageDir(packageName, { optional = false } = {}) {
  try {
    const pkgPath = require.resolve(`${packageName}/package.json`);
    return path.dirname(pkgPath);
  } catch (err) {
    if (optional) {
      return null;
    }
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

function writeCliShim(rootDir, isWindows) {
  const binDir = path.join(rootDir, 'bin');
  if (fs.existsSync(binDir)) {
    fs.rmSync(binDir, { recursive: true, force: true });
  }
  fs.mkdirSync(binDir, { recursive: true });

  const shimPath = path.join(binDir, '{{appName}}');
  const shimContent = `#!/usr/bin/env node

const { spawnSync } = require('child_process');
const path = require('path');

const binaryName = process.platform === 'win32' ? '{{appName}}.exe' : '{{appName}}';
const binaryPath = path.join(__dirname, '..', '${TARGET_FILES_DIR}', binaryName);
const result = spawnSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' });

if (result.error) {
  throw result.error;
}

process.exit(result.status ?? 0);
`;

  fs.writeFileSync(shimPath, shimContent, 'utf-8');
  if (!isWindows) {
    fs.chmodSync(shimPath, 0o755);
  }
}

function copyCorePackage(coreDir, targetDir, isWindows) {
  const binaryName = `{{appName}}${isWindows ? '.exe' : ''}`;
  const sourceBinary = path.join(coreDir, binaryName);
  if (!fs.existsSync(sourceBinary)) {
    console.error(`Error: Core binary not found at ${sourceBinary}`);
    process.exit(1);
  }

  const targetBinary = path.join(targetDir, binaryName);
  fs.copyFileSync(sourceBinary, targetBinary);

  const sourceConfig = path.join(coreDir, CORE_CONFIG_FILE);
  if (fs.existsSync(sourceConfig)) {
    fs.copyFileSync(sourceConfig, path.join(targetDir, CORE_CONFIG_FILE));
  }

  if (!isWindows) {
    fs.chmodSync(targetBinary, 0o755);
  }
}

function copyDepsPackage(depsDir, targetDir) {
  if (!depsDir) return;
  const depsBinDir = path.join(depsDir, DEPS_DIR);
  copyDirectoryContents(depsBinDir, path.join(targetDir, DEPS_DIR));
}

function setupBinary() {
  const platform = detectPlatform();
  const isWindows = platform.startsWith('win32');
  const rootDir = __dirname;
  const targetDir = path.join(rootDir, TARGET_FILES_DIR);
  const corePackageName = `${PACKAGE_SCOPE}/${CORE_PLATFORM_PREFIX}${platform}`;
  const depsPackageName = `${PACKAGE_SCOPE}/${DEPS_PLATFORM_PREFIX}${platform}`;
  const coreDir = resolvePackageDir(corePackageName);
  const depsDir = resolvePackageDir(depsPackageName, { optional: true });

  console.log(`Setting up {{appName}} for ${platform}...`);

  if (fs.existsSync(targetDir)) {
    fs.rmSync(targetDir, { recursive: true, force: true });
  }
  fs.mkdirSync(targetDir, { recursive: true });

  copyCorePackage(coreDir, targetDir, isWindows);
  copyDepsPackage(depsDir, targetDir);

  writeCliShim(rootDir, isWindows);

  console.log(`{{appName}} is ready to use.`);
}

try {
  setupBinary();
} catch (err) {
  console.error('Installation failed:', err.message);
  process.exit(1);
}
