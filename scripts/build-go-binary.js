#!/usr/bin/env node
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const repoRoot = path.join(__dirname, '..');
const goRoot = path.join(repoRoot, 'go');
const binDir = path.join(repoRoot, 'bin');
const binaryName = process.platform === 'win32' ? 'rlm-go.exe' : 'rlm-go';
const binaryPath = path.join(binDir, binaryName);

function goAvailable() {
  try {
    execFileSync('go', ['version'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

if (!fs.existsSync(goRoot)) {
  console.warn('[recursive-llm-ts] Go source directory not found; skipping Go build');
  process.exit(0);
}

if (!goAvailable()) {
  console.warn('[recursive-llm-ts] Go is not installed; skipping Go binary build');
  console.warn('[recursive-llm-ts] Install Go 1.21+ and rerun: node scripts/build-go-binary.js');
  process.exit(0);
}

try {
  fs.mkdirSync(binDir, { recursive: true });
  // Build with optimization: -s removes symbol table, -w removes DWARF debug info
  execFileSync('go', ['build', '-ldflags=-s -w', '-o', binaryPath, './cmd/rlm'], { stdio: 'inherit', cwd: goRoot });
  console.log(`[recursive-llm-ts] ✓ Go binary built at ${binaryPath}`);
} catch (error) {
  console.warn('[recursive-llm-ts] Warning: Failed to build Go binary');
  console.warn(error.message || error);
  process.exit(0);
}
