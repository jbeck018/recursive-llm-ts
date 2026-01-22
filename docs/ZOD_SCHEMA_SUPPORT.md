# Zod Schema Support

The `recursive-llm-ts` library includes comprehensive Zod to JSON Schema conversion that supports virtually all Zod constructs.

## Supported Zod Types

### ✅ Primitives
- `z.string()` - String type
- `z.number()` - Number type  
- `z.boolean()` - Boolean type
- `z.bigint()` - BigInt (converted to integer)
- `z.date()` - Date (converted to string with date-time format)
- `z.null()`, `z.undefined()`, `z.void()` - Null types

### ✅ Complex Types
- `z.object()` - Objects with properties and required fields
- `z.array()` - Arrays with item schemas
- `z.tuple()` - Fixed-length arrays with type per position
- `z.enum()` - Enumerated string values
- `z.literal()` - Specific literal values
- `z.union()` - One of multiple types (anyOf)
- `z.intersection()` - Combination of types (allOf)
- `z.record()` - Objects with dynamic keys
- `z.map()` - Map types (converted to objects)
- `z.set()` - Set types (converted to arrays with uniqueItems)

### ✅ Modifiers & Wrappers
- `.optional()` - Optional fields
- `.nullable()` - Nullable types
- `.default()` - Fields with default values
- `.catch()` - Error catching wrapper

### ✅ Constraints

#### String Constraints
- `.min()`, `.max()` - Min/max length
- `.length()` - Exact length
- `.email()` - Email format
- `.url()` - URL format
- `.uuid()` - UUID format
- `.regex()` - Custom regex pattern

#### Number Constraints
- `.min()`, `.max()` - Min/max values
- `.int()` - Integer type
- `.multipleOf()` - Multiple of specific value
- `.positive()`, `.negative()` - Sign constraints
- `.finite()` - Finite number constraint

#### Array Constraints
- `.min()`, `.max()` - Min/max items
- `.length()` - Exact length
- `.nonempty()` - At least one item

### ✅ Advanced Features
- `z.refine()` - Custom validation (unwrapped to base type)
- `z.transform()` - Transformations (uses output type)
- `z.pipe()` - Piped schemas (uses output schema)
- `z.lazy()` - Lazy/recursive schemas
- `z.branded()` - Branded types (unwrapped to base)
- `z.readonly()` - Readonly wrapper (pass-through)
- `z.any()`, `z.unknown()` - Unrestricted types
- `z.never()` - Impossible type
- `z.promise()` - Promise wrapper (unwrapped)

### ⚠️ Limitations
- **Functions**: `z.function()` is converted to string (functions aren't JSON-serializable)
- **Custom validations**: `.refine()` and `.superRefine()` logic isn't captured in JSON Schema
- **Transforms**: Only the output type is used, not the transformation logic

## Usage Examples

### Basic Types
```typescript
const schema = z.object({
  name: z.string().min(2).max(100),
  age: z.number().int().min(0).max(120),
  email: z.string().email(),
  verified: z.boolean()
});

const result = await rlm.structuredCompletion(query, context, schema);
```

### Complex Nested Schema
```typescript
const schema = z.object({
  sentiment: z.object({
    score: z.number().int().min(1).max(5),
    confidence: z.number().min(0).max(1)
  }),
  phrases: z.array(z.object({
    phrase: z.string(),
    sentiment: z.number().int().min(1).max(5),
    confidence: z.number().min(0).max(1),
    type: z.string().optional()
  })).default([]),
  tags: z.array(z.string()).optional()
});
```

### Unions and Literals
```typescript
const schema = z.object({
  status: z.enum(['active', 'inactive', 'pending']),
  type: z.union([
    z.literal('user'),
    z.literal('admin'),
    z.literal('guest')
  ]),
  config: z.record(z.string(), z.any())
});
```

## Testing

The library includes comprehensive tests covering 35+ different Zod constructs:

```bash
bun run test/zod-schema-converter-test.ts
```

All tests should pass, validating support for:
- Primitives and their constraints
- Complex nested structures
- Arrays and objects with defaults
- Unions, intersections, and tuples
- Enums and literals
- Sets, maps, and records
- Effect wrappers (refine, transform, etc.)

## Implementation Details

### Zod Version Compatibility

The converter is designed to work with modern Zod versions that use the `_zod.def` internal structure for constraint definitions. The implementation:

1. Checks `def.type` for the schema type
2. Accesses constraints via `check._zod?.def` for nested check definitions
3. Falls back to legacy property access for older Zod versions

### Dynamic Query Generation

When using parallel execution mode, the library automatically generates field-specific extraction queries based on the schema structure:

```typescript
// For a phrases array with required fields
"Extract the phrases from the conversation. Return a JSON array where each item 
is an object with REQUIRED fields: 'phrase' (string), 'sentiment' (number), 
'confidence' (number). Optional fields: 'type' (string), 'explanation' (string)."
```

This ensures LLMs produce correctly structured outputs matching your schema requirements.

## Related Documentation

- [Main README](../README.md)
- [Structured Output Guide](./STRUCTURED_OUTPUT.md)
- [Parallel Execution](./PARALLEL_EXECUTION.md)
