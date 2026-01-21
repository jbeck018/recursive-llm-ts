#!/usr/bin/env node
const { execSync, execFileSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');
const pyprojectPath = path.join(pythonPackagePath, 'pyproject.toml');
const pythonDepsPath = path.join(pythonPackagePath, '.pydeps');

// Check if pyproject.toml exists
if (!fs.existsSync(pyprojectPath)) {
  console.warn('[recursive-llm-ts] Warning: pyproject.toml not found at', pyprojectPath);
  console.warn('[recursive-llm-ts] Python dependencies will need to be installed manually');
  process.exit(0); // Don't fail the install
}

console.log('[recursive-llm-ts] Installing Python dependencies locally...');

const dependencySpecifiers = (() => {
  try {
    const pyproject = fs.readFileSync(pyprojectPath, 'utf8');
    const depsBlock = pyproject.match(/dependencies\s*=\s*\[[\s\S]*?\]/);
    if (!depsBlock) return [];
    return Array.from(depsBlock[0].matchAll(/"([^"]+)"/g), (match) => match[1]);
  } catch {
    return [];
  }
})();

const installArgs = dependencySpecifiers.length > 0
  ? ['-m', 'pip', 'install', '--upgrade', '--target', pythonDepsPath, ...dependencySpecifiers]
  : ['-m', 'pip', 'install', '-e', pythonPackagePath];
const dependencyArgs = dependencySpecifiers.map((dep) => `"${dep}"`).join(' ');

try {

  const pythonCandidates = [];
  if (process.env.PYTHON) {
    pythonCandidates.push({ command: process.env.PYTHON, args: [] });
  }
  if (process.platform === 'win32') {
    pythonCandidates.push({ command: 'py', args: ['-3'] });
    pythonCandidates.push({ command: 'python', args: [] });
    pythonCandidates.push({ command: 'python3', args: [] });
  } else {
    pythonCandidates.push({ command: 'python3', args: [] });
    pythonCandidates.push({ command: 'python', args: [] });
  }

  let pythonCmd = null;
  for (const candidate of pythonCandidates) {
    try {
      execFileSync(candidate.command, [...candidate.args, '--version'], { stdio: 'pipe' });
      pythonCmd = candidate;
      break;
    } catch {
      // Try next candidate
    }
  }

  if (pythonCmd) {
    fs.mkdirSync(pythonDepsPath, { recursive: true });
    execFileSync(
      pythonCmd.command,
      [...pythonCmd.args, ...installArgs],
      { stdio: 'inherit', cwd: pythonPackagePath }
    );
  } else {
    // Fall back to pip/pip3 if a Python executable wasn't found
    try {
      execSync('pip --version', { stdio: 'pipe' });
    } catch {
      execSync('pip3 --version', { stdio: 'pipe' });
    }
    const pipCommand = dependencySpecifiers.length > 0
      ? `pip install --upgrade --target "${pythonDepsPath}" ${dependencySpecifiers.map((dep) => `"${dep}"`).join(' ')}`
      : process.platform === 'win32'
        ? `pip install -e "${pythonPackagePath}"`
        : `pip install -e "${pythonPackagePath}" || pip3 install -e "${pythonPackagePath}"`;
    fs.mkdirSync(pythonDepsPath, { recursive: true });
    execSync(pipCommand, {
      stdio: 'inherit',
      cwd: pythonPackagePath
    });
  }
  console.log('[recursive-llm-ts] ✓ Python dependencies installed successfully');
} catch (error) {
  console.warn('[recursive-llm-ts] Warning: Failed to auto-install Python dependencies');
  console.warn('[recursive-llm-ts] This is not critical - you can install them manually:');
  if (dependencySpecifiers.length > 0) {
    console.warn(
      `[recursive-llm-ts]   cd node_modules/recursive-llm-ts/recursive-llm && ` +
      `python -m pip install --upgrade --target .pydeps ${dependencyArgs}`
    );
  } else {
    console.warn(`[recursive-llm-ts]   cd node_modules/recursive-llm-ts/recursive-llm && python -m pip install -e .`);
  }
  console.warn('[recursive-llm-ts] Or ensure Python 3.9+ and pip are in your PATH');
  // Don't fail the npm install - exit with 0
  process.exit(0);
}
