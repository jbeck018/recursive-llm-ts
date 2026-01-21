# Patches to recursive-llm Submodule

This document tracks custom patches applied to the `recursive-llm` Python submodule.

## Custom Provider Support Patch

**File:** `recursive-llm/src/rlm/core.py`  
**Commit:** 3ae2229

### What it does
Allows the `RLM` constructor to accept a configuration dictionary as the second positional argument, instead of requiring named parameters. This is necessary because the TypeScript bridge (`pythonia`) passes the config object as a dict.

### Why it's needed
The TypeScript wrapper calls the Python RLM class like this:
```typescript
const rlmInstance = await RLMClass(model, rlmConfig);
```

Where `rlmConfig` is an object containing `api_base`, `api_key`, `temperature`, etc.

Without this patch, custom providers (like AWS Bedrock or Azure OpenAI) wouldn't work because the Python side expects these as separate named parameters.

### How it works
The patch detects if the second argument is a dict and extracts the config values:
- `recursive_model`
- `api_base` (for custom providers)
- `api_key`
- `max_depth`
- `max_iterations`
- Any other LiteLLM parameters (temperature, timeout, etc.)

### Automatic Patch Application
The patch is **automatically applied** during `npm install` via the `postinstall` script.

**How it works:**
1. `npm install` runs `postinstall` hook
2. Script calls `scripts/apply-patches.js`
3. Patch is applied from `patches/core.py.patch`
4. If already applied, script skips (idempotent)
5. Falls back to manual patching if `patch` command fails

### Maintaining the patch
When updating the upstream submodule:
1. Pull latest changes: `cd recursive-llm && git pull`
2. Test if patch still applies: `node scripts/apply-patches.js`
3. If conflicts occur, update `patches/core.py.patch` manually
4. Test with custom providers
5. Commit updated submodule reference and patch file
6. Publish new package version

### Testing custom providers
```bash
# Set your Bedrock/custom endpoint
export BEDROCK_URL="https://your-endpoint/v1"
export BEDROCK_API_KEY="your-key"
export MODEL_ID="qwen3-30b"

# Run the example
node examples/recursive-extraction-demo.js
```
