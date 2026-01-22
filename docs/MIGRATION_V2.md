# Migration Guide: v1.x → v2.0.0

## Breaking Changes

### Constructor Signature Change
The `RLM` constructor now accepts an optional third parameter for bridge type selection:

**v1.x:**
```typescript
const rlm = new RLM(model, config);
```

**v2.0.0:**
```typescript
const rlm = new RLM(model, config, bridgeType); // bridgeType is optional
```

## What's New in v2.0.0

### 🎉 Bun Support
You can now use `recursive-llm-ts` with Bun! The package automatically detects your runtime and uses the appropriate Python bridge.

### Runtime Detection
- **Node.js** → Uses `pythonia` (no changes needed)
- **Bun** → Uses `bunpy` (requires `bun add bunpy`)

### For Existing Node.js Users
**No action required!** Your existing code will continue to work exactly as before:

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY
});

// Everything works the same
const result = await rlm.completion(query, context);
```

### For Bun Users

1. **Install bunpy:**
   ```bash
   bun add bunpy
   ```

2. **Use the same API:**
   ```typescript
   import { RLM } from 'recursive-llm-ts';

   // Automatic detection - works in Bun!
   const rlm = new RLM('gpt-4o-mini', {
     api_key: process.env.OPENAI_API_KEY
   });

   const result = await rlm.completion(query, context);
   ```

3. **No more `bunx tsx` required!** 🎉
   ```bash
   # Before (v1.x)
   bunx tsx script.ts

   # After (v2.0.0)
   bun run script.ts
   ```

## Explicit Bridge Selection

If you need to explicitly specify which bridge to use (rare):

```typescript
// Force bunpy (for Bun)
const rlm = new RLM('gpt-4o-mini', config, 'bunpy');

// Force pythonia (for Node.js)
const rlm = new RLM('gpt-4o-mini', config, 'pythonia');

// Auto-detect (default - recommended)
const rlm = new RLM('gpt-4o-mini', config, 'auto');
```

## Type Imports

If you were importing types directly from `rlm-bridge`, update your imports:

**v1.x:**
```typescript
import { RLMConfig, RLMResult, RLMStats } from 'recursive-llm-ts/rlm-bridge';
```

**v2.0.0:**
```typescript
import { RLMConfig, RLMResult, RLMStats } from 'recursive-llm-ts';
```

## Troubleshooting

### "bunpy is not installed" error in Bun
```bash
bun add bunpy
```

### "Unable to detect runtime" error
Pass explicit bridge type:
```typescript
const rlm = new RLM('gpt-4o-mini', config, 'bunpy'); // or 'pythonia'
```

### Still getting pythonia errors in Bun
Make sure you're using v2.0.0+:
```bash
bun remove recursive-llm-ts
bun add recursive-llm-ts@latest
bun add bunpy
```

## Summary

- ✅ **Node.js users:** No changes required
- ✅ **Bun users:** Install `bunpy` and enjoy native support
- ✅ **Automatic detection:** Works out of the box
- ✅ **Backward compatible:** Existing code still works
