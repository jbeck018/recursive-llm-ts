#!/usr/bin/env node
const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const corePyPath = path.join(__dirname, '..', 'recursive-llm', 'src', 'rlm', 'core.py');
const patchPath = path.join(__dirname, '..', 'patches', 'core.py.patch');

// Check if core.py exists
if (!fs.existsSync(corePyPath)) {
  console.error('Error: core.py not found at', corePyPath);
  process.exit(1);
}

// Check if patch exists
if (!fs.existsSync(patchPath)) {
  console.error('Error: patch file not found at', patchPath);
  process.exit(1);
}

console.log('Applying patches to recursive-llm Python package...');

// Check if patch is already applied by looking for the patch marker
const coreContent = fs.readFileSync(corePyPath, 'utf8');
if (coreContent.includes('# Patch for recursive-llm-ts bug where config is passed as 2nd positional arg')) {
  console.log('✓ Patch already applied to core.py');
  process.exit(0);
}

// Apply the patch
const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');

try {
  // Try to apply patch
  execSync(`patch -p1 -i "${patchPath}"`, {
    cwd: pythonPackagePath,
    stdio: 'inherit'
  });
  console.log('✓ Successfully applied core.py patch');
} catch (error) {
  console.error('Failed to apply patch using patch command.');
  console.error('Attempting manual patch application...');
  
  // Fallback: manually apply the patch by replacing the constructor
  try {
    const updatedContent = coreContent.replace(
      /(\s+)self\.model = model\n(\s+)self\.recursive_model = recursive_model or model\n(\s+)self\.api_base = api_base\n(\s+)self\.api_key = api_key\n(\s+)self\.max_depth = max_depth\n(\s+)self\.max_iterations = max_iterations\n(\s+)self\._current_depth = _current_depth\n(\s+)self\.llm_kwargs = llm_kwargs/,
      `$1# Patch for recursive-llm-ts bug where config is passed as 2nd positional arg
$1if isinstance(recursive_model, dict):
$1    config = recursive_model
$1    # Reset recursive_model default
$1    self.recursive_model = config.get('recursive_model', model)
$1    self.api_base = config.get('api_base', api_base)
$1    self.api_key = config.get('api_key', api_key)
$1    self.max_depth = int(config.get('max_depth', max_depth))
$1    self.max_iterations = int(config.get('max_iterations', max_iterations))
$1    
$1    # Extract other llm kwargs
$1    excluded = {'recursive_model', 'api_base', 'api_key', 'max_depth', 'max_iterations'}
$1    self.llm_kwargs = {k: v for k, v in config.items() if k not in excluded}
$1    # Merge with any actual kwargs passed
$1    self.llm_kwargs.update(llm_kwargs)
$1else:
$1    self.recursive_model = recursive_model or model
$1    self.api_base = api_base
$1    self.api_key = api_key
$1    self.max_depth = max_depth
$1    self.max_iterations = max_iterations
$1    self.llm_kwargs = llm_kwargs

$1self._current_depth = _current_depth
$1self.model = model`
    );
    
    if (updatedContent === coreContent) {
      console.error('Manual patch failed: pattern not found in core.py');
      console.error('The file may have been updated upstream.');
      console.error('Please manually apply the patch from patches/core.py.patch');
      process.exit(1);
    }
    
    fs.writeFileSync(corePyPath, updatedContent, 'utf8');
    console.log('✓ Successfully applied patch manually');
  } catch (manualError) {
    console.error('Manual patch application failed:', manualError.message);
    process.exit(1);
  }
}
