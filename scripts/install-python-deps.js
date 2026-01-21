#!/usr/bin/env node
const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');
const pyprojectPath = path.join(pythonPackagePath, 'pyproject.toml');

// Check if pyproject.toml exists
if (!fs.existsSync(pyprojectPath)) {
  console.error('Error: pyproject.toml not found at', pyprojectPath);
  process.exit(1);
}

console.log('Installing Python dependencies for recursive-llm...');
console.log('Note: Python source is vendored with patches pre-applied.');

try {
  // Install the Python package in editable mode
  execSync(`pip install -e "${pythonPackagePath}"`, {
    stdio: 'inherit',
    cwd: pythonPackagePath
  });
  console.log('✓ Python dependencies installed successfully');
} catch (error) {
  console.error('Failed to install Python dependencies.');
  console.error('Please ensure Python 3.9+ and pip are installed.');
  console.error('You can manually install by running:');
  console.error(`  cd ${pythonPackagePath} && pip install -e .`);
  process.exit(1);
}
