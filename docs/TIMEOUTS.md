# Timeout Configuration Guide

## Understanding Timeouts in recursive-llm-ts

There are two separate timeout settings that work together:

### 1. `pythonia_timeout` (Python Bridge Timeout)
- **What it controls:** How long to wait for Python method calls to complete
- **Default:** 100000ms (100 seconds)
- **Units:** Milliseconds
- **When to increase:** 
  - Very large documents (1M+ tokens)
  - Many recursive iterations (>30)
  - Slow LLM APIs
  - Queue-based processing
  
### 2. `timeout` (LLM API Timeout)
- **What it controls:** How long each individual LLM API request can take
- **Default:** Varies by LiteLLM provider
- **Units:** Seconds
- **When to increase:**
  - Slow custom endpoints
  - High token limits
  - Cold-start delays

## Examples

### Default (Fast Processing)
```typescript
const rlm = new RLM('gpt-4o-mini');
// pythonia_timeout: 100s
// timeout: default
```

### Medium Documents (Up to 100k tokens)
```typescript
const rlm = new RLM('gpt-4o-mini', {
  pythonia_timeout: 300000,  // 5 minutes for Python bridge
  timeout: 120               // 2 minutes per LLM call
});
```

### Large Documents / Queue Processing (500k+ tokens)
```typescript
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 50,
  pythonia_timeout: 600000,  // 10 minutes for Python bridge
  timeout: 300               // 5 minutes per LLM call
});
```

### Very Long Running / Batch Jobs
```typescript
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 100,
  pythonia_timeout: 1800000, // 30 minutes for Python bridge
  timeout: 600               // 10 minutes per LLM call
});
```

## Troubleshooting

### Error: "Python didn't respond in time"
**Cause:** `pythonia_timeout` exceeded  
**Solution:** Increase `pythonia_timeout`

```typescript
const rlm = new RLM('gpt-4o-mini', {
  pythonia_timeout: 600000  // Increase to 10 minutes
});
```

### Error: LLM timeout / connection timeout
**Cause:** LLM API `timeout` exceeded  
**Solution:** Increase `timeout` parameter

```typescript
const rlm = new RLM('gpt-4o-mini', {
  timeout: 300  // Increase to 5 minutes
});
```

### Processing hangs indefinitely
**Cause:** Both timeouts may be too high, or LLM is stuck in a loop  
**Solution:** 
1. Check `max_iterations` isn't too high
2. Review query/context for issues
3. Set reasonable timeout bounds

```typescript
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 30,        // Reasonable upper bound
  pythonia_timeout: 300000,  // 5 min max
  timeout: 120               // 2 min per call
});
```

## Best Practices

1. **Start conservative:** Use defaults first, then increase only if needed
2. **Monitor stats:** Check `result.stats.iterations` to see if you're hitting limits
3. **Queue processing:** Set timeouts based on worst-case document size
4. **Testing:** Test with representative large documents before production
5. **Logging:** Log timeout errors to tune values based on actual usage

## Formula for Estimating Timeouts

```
pythonia_timeout ≥ (max_iterations × timeout × 1000) + buffer
```

Example:
- `max_iterations`: 50
- `timeout`: 120s per LLM call
- Buffer: 60s for overhead

```
pythonia_timeout = (50 × 120 × 1000) + 60000 = 6,060,000ms ≈ 10 minutes
```
