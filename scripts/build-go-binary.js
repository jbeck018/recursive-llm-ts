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
  // No Go source — this is fine for pre-built binary installs
  process.exit(0);
}

if (fs.existsSync(binaryPath)) {
  console.log('[recursive-llm-ts] ✓ Go binary already exists at ' + binaryPath);
  process.exit(0);
}

if (!goAvailable()) {
  console.warn('');
  console.warn('╔══════════════════════════════════════════════════════════════════╗');
  console.warn('║  recursive-llm-ts: Go binary could not be built                 ║');
  console.warn('║                                                                  ║');
  console.warn('║  Go 1.25+ is required to compile the backend binary.             ║');
  console.warn('║  Without it, all RLM operations will fail at runtime.            ║');
  console.warn('║                                                                  ║');
  console.warn('║  Install Go: https://go.dev/dl/                                  ║');
  console.warn('║  Then run:   node scripts/build-go-binary.js                     ║');
  console.warn('╚══════════════════════════════════════════════════════════════════╝');
  console.warn('');
  process.exit(0);
}

try {
  fs.mkdirSync(binDir, { recursive: true });
  execFileSync('go', ['build', '-ldflags=-s -w', '-o', binaryPath, './cmd/rlm'], { stdio: 'inherit', cwd: goRoot });
  console.log('[recursive-llm-ts] ✓ Go binary built at ' + binaryPath);
} catch (error) {
  console.warn('[recursive-llm-ts] Warning: Failed to build Go binary');
  console.warn(error.message || error);
  process.exit(0);
}
