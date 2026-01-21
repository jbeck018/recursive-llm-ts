#!/usr/bin/env node
const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');
const pyprojectPath = path.join(pythonPackagePath, 'pyproject.toml');

// Check if pyproject.toml exists
if (!fs.existsSync(pyprojectPath)) {
  console.warn('[recursive-llm-ts] Warning: pyproject.toml not found at', pyprojectPath);
  console.warn('[recursive-llm-ts] Python dependencies will need to be installed manually');
  process.exit(0); // Don't fail the install
}

console.log('[recursive-llm-ts] Installing Python dependencies...');

try {
  // Check if pip is available
  try {
    execSync('pip --version', { stdio: 'pipe' });
  } catch {
    // Try pip3
    execSync('pip3 --version', { stdio: 'pipe' });
  }

  // Install the Python package in editable mode
  const pipCommand = process.platform === 'win32' 
    ? `pip install -e "${pythonPackagePath}"`
    : `pip install -e "${pythonPackagePath}" || pip3 install -e "${pythonPackagePath}"`;
  
  execSync(pipCommand, {
    stdio: 'inherit',
    cwd: pythonPackagePath
  });
  console.log('[recursive-llm-ts] ✓ Python dependencies installed successfully');
} catch (error) {
  console.warn('[recursive-llm-ts] Warning: Failed to auto-install Python dependencies');
  console.warn('[recursive-llm-ts] This is not critical - you can install them manually:');
  console.warn(`[recursive-llm-ts]   cd node_modules/recursive-llm-ts/recursive-llm && pip install -e .`);
  console.warn('[recursive-llm-ts] Or ensure Python 3.9+ and pip are in your PATH');
  // Don't fail the npm install - exit with 0
  process.exit(0);
}
