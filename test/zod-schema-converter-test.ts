import { z } from 'zod';
import { RLM } from '../src/rlm';

// Helper to get JSON schema from Zod schema
function getJsonSchema(zodSchema: z.ZodSchema<any>): any {
  const rlm = new RLM('gpt-4o-mini', {});
  return (rlm as any).zodToJsonSchema(zodSchema);
}

// Test suite
async function runTests() {
  console.log('🧪 Testing Zod to JSON Schema Conversion\n');
  console.log('='.repeat(80));
  
  const tests: Array<{ name: string; schema: z.ZodSchema<any>; validate: (json: any) => void }> = [
    // Primitives
    {
      name: 'String',
      schema: z.string(),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
      }
    },
    {
      name: 'Number',
      schema: z.number(),
      validate: (json) => {
        if (json.type !== 'number') throw new Error('Expected type: number');
      }
    },
    {
      name: 'Integer',
      schema: z.number().int(),
      validate: (json) => {
        if (json.type !== 'integer') throw new Error('Expected type: integer');
      }
    },
    {
      name: 'Boolean',
      schema: z.boolean(),
      validate: (json) => {
        if (json.type !== 'boolean') throw new Error('Expected type: boolean');
      }
    },
    {
      name: 'Date',
      schema: z.date(),
      validate: (json) => {
        if (json.type !== 'string' || json.format !== 'date-time') {
          throw new Error('Expected string with date-time format');
        }
      }
    },
    
    // Number constraints
    {
      name: 'Number with min/max',
      schema: z.number().min(1).max(10),
      validate: (json) => {
        if (json.minimum !== 1 || json.maximum !== 10) {
          throw new Error('Expected minimum: 1, maximum: 10');
        }
      }
    },
    {
      name: 'Integer with constraints',
      schema: z.number().int().min(1).max(5),
      validate: (json) => {
        if (json.type !== 'integer' || json.minimum !== 1 || json.maximum !== 5) {
          throw new Error('Expected integer with min: 1, max: 5');
        }
      }
    },
    {
      name: 'Number with multipleOf',
      schema: z.number().multipleOf(5),
      validate: (json) => {
        if (json.multipleOf !== 5) throw new Error('Expected multipleOf: 5');
      }
    },
    
    // String constraints
    {
      name: 'String with min/max length',
      schema: z.string().min(5).max(100),
      validate: (json) => {
        if (json.minLength !== 5 || json.maxLength !== 100) {
          throw new Error('Expected minLength: 5, maxLength: 100');
        }
      }
    },
    {
      name: 'Email',
      schema: z.string().email(),
      validate: (json) => {
        if (json.format !== 'email') throw new Error('Expected format: email');
      }
    },
    {
      name: 'URL',
      schema: z.string().url(),
      validate: (json) => {
        if (json.format !== 'uri') throw new Error('Expected format: uri');
      }
    },
    {
      name: 'UUID',
      schema: z.string().uuid(),
      validate: (json) => {
        if (json.format !== 'uuid') throw new Error('Expected format: uuid');
      }
    },
    {
      name: 'String with regex',
      schema: z.string().regex(/^[A-Z]+$/),
      validate: (json) => {
        if (json.pattern !== '^[A-Z]+$') throw new Error('Expected pattern: ^[A-Z]+$');
      }
    },
    
    // Arrays
    {
      name: 'Array of strings',
      schema: z.array(z.string()),
      validate: (json) => {
        if (json.type !== 'array' || json.items.type !== 'string') {
          throw new Error('Expected array of strings');
        }
      }
    },
    {
      name: 'Array with length constraints',
      schema: z.array(z.number()).min(1).max(10),
      validate: (json) => {
        if (json.minItems !== 1 || json.maxItems !== 10) {
          throw new Error('Expected minItems: 1, maxItems: 10');
        }
      }
    },
    {
      name: 'Array of objects',
      schema: z.array(z.object({
        id: z.string(),
        value: z.number()
      })),
      validate: (json) => {
        if (json.type !== 'array' || json.items.type !== 'object') {
          throw new Error('Expected array of objects');
        }
        if (!json.items.properties.id || !json.items.properties.value) {
          throw new Error('Expected id and value properties');
        }
      }
    },
    
    // Objects
    {
      name: 'Simple object',
      schema: z.object({
        name: z.string(),
        age: z.number()
      }),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        if (!json.properties.name || !json.properties.age) {
          throw new Error('Expected name and age properties');
        }
        if (!json.required || !json.required.includes('name') || !json.required.includes('age')) {
          throw new Error('Expected both fields to be required');
        }
      }
    },
    {
      name: 'Object with optional fields',
      schema: z.object({
        required: z.string(),
        optional: z.string().optional()
      }),
      validate: (json) => {
        if (!json.required.includes('required')) {
          throw new Error('Expected required field');
        }
        if (json.required.includes('optional')) {
          throw new Error('Optional field should not be required');
        }
      }
    },
    {
      name: 'Object with default values',
      schema: z.object({
        name: z.string(),
        tags: z.array(z.string()).default([])
      }),
      validate: (json) => {
        if (!json.required.includes('name')) {
          throw new Error('Expected name to be required');
        }
        if (json.required.includes('tags')) {
          throw new Error('Field with default should not be required');
        }
      }
    },
    
    // Enums
    {
      name: 'Enum',
      schema: z.enum(['red', 'green', 'blue']),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
        if (!json.enum || json.enum.length !== 3) {
          throw new Error('Expected 3 enum values');
        }
      }
    },
    
    // Literals
    {
      name: 'Literal string',
      schema: z.literal('hello'),
      validate: (json) => {
        if (!json.enum || json.enum[0] !== 'hello') {
          throw new Error('Expected literal value: hello');
        }
      }
    },
    {
      name: 'Literal number',
      schema: z.literal(42),
      validate: (json) => {
        if (!json.enum || json.enum[0] !== 42) {
          throw new Error('Expected literal value: 42');
        }
      }
    },
    
    // Unions
    {
      name: 'Union',
      schema: z.union([z.string(), z.number()]),
      validate: (json) => {
        if (!json.anyOf || json.anyOf.length !== 2) {
          throw new Error('Expected anyOf with 2 options');
        }
      }
    },
    
    // Intersections
    {
      name: 'Intersection',
      schema: z.intersection(
        z.object({ name: z.string() }),
        z.object({ age: z.number() })
      ),
      validate: (json) => {
        if (!json.allOf || json.allOf.length !== 2) {
          throw new Error('Expected allOf with 2 schemas');
        }
      }
    },
    
    // Tuples
    {
      name: 'Tuple',
      schema: z.tuple([z.string(), z.number(), z.boolean()]),
      validate: (json) => {
        if (json.type !== 'array') throw new Error('Expected type: array');
        if (!json.prefixItems || json.prefixItems.length !== 3) {
          throw new Error('Expected 3 prefix items');
        }
      }
    },
    
    // Record
    {
      name: 'Record',
      schema: z.record(z.string(), z.number()),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        if (!json.additionalProperties || json.additionalProperties.type !== 'number') {
          throw new Error('Expected additionalProperties with number type');
        }
      }
    },
    
    // Nullable/Optional
    {
      name: 'Nullable',
      schema: z.string().nullable(),
      validate: (json) => {
        if (json.type !== 'string' || !json.nullable) {
          throw new Error('Expected nullable string');
        }
      }
    },
    
    // Effects (refine, transform)
    {
      name: 'Refined string',
      schema: z.string().refine(s => s.length > 5),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
      }
    },
    {
      name: 'Transformed string',
      schema: z.string().transform(s => s.toUpperCase()),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
      }
    },
    
    // Complex nested schema (like conversation insights)
    {
      name: 'Complex nested schema',
      schema: z.object({
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
        signals: z.array(z.object({
          type: z.string(),
          phrase: z.string(),
          confidence: z.number().optional()
        })).default([])
      }),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        
        // Check sentiment object
        const sentiment = json.properties.sentiment;
        if (!sentiment || sentiment.type !== 'object') {
          throw new Error('Expected sentiment to be object');
        }
        if (sentiment.properties.score.type !== 'integer') {
          throw new Error('Expected score to be integer');
        }
        
        // Check phrases array
        const phrases = json.properties.phrases;
        if (!phrases || phrases.type !== 'array') {
          throw new Error('Expected phrases to be array');
        }
        if (phrases.items.type !== 'object') {
          throw new Error('Expected phrase items to be objects');
        }
        
        // Check required fields
        if (!json.required.includes('sentiment')) {
          throw new Error('Expected sentiment to be required');
        }
        if (json.required.includes('phrases')) {
          throw new Error('Phrases with default should not be required');
        }
      }
    },
    
    // Set
    {
      name: 'Set',
      schema: z.set(z.string()),
      validate: (json) => {
        if (json.type !== 'array' || !json.uniqueItems) {
          throw new Error('Expected array with uniqueItems: true');
        }
      }
    },
    
    // Map
    {
      name: 'Map',
      schema: z.map(z.string(), z.number()),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
      }
    },
    
    // BigInt
    {
      name: 'BigInt',
      schema: z.bigint(),
      validate: (json) => {
        if (json.type !== 'integer') throw new Error('Expected type: integer');
      }
    },
    
    // Any/Unknown
    {
      name: 'Any',
      schema: z.any(),
      validate: (json) => {
        if (Object.keys(json).length !== 0) {
          throw new Error('Expected empty schema for any');
        }
      }
    },
    {
      name: 'Unknown',
      schema: z.unknown(),
      validate: (json) => {
        if (Object.keys(json).length !== 0) {
          throw new Error('Expected empty schema for unknown');
        }
      }
    },
    
    // Additional string validations
    {
      name: 'String with length',
      schema: z.string().length(10),
      validate: (json) => {
        if (json.minLength !== 10 || json.maxLength !== 10) {
          throw new Error('Expected minLength and maxLength: 10');
        }
      }
    },
    
    // Lazy schemas
    {
      name: 'Lazy schema',
      schema: z.lazy(() => z.object({ name: z.string() })),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        if (!json.properties.name) throw new Error('Expected name property');
      }
    },
    
    // Branded types
    {
      name: 'Branded type',
      schema: z.string().brand<'UserId'>(),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
      }
    },
    
    // Readonly
    {
      name: 'Readonly',
      schema: z.object({ name: z.string() }).readonly(),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        if (!json.properties.name) throw new Error('Expected name property');
      }
    },
    
    // Catch
    {
      name: 'Catch',
      schema: z.string().catch('default'),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
      }
    },
    
    // Null types
    {
      name: 'Null',
      schema: z.null(),
      validate: (json) => {
        if (json.type !== 'null') throw new Error('Expected type: null');
      }
    },
    {
      name: 'Undefined',
      schema: z.undefined(),
      validate: (json) => {
        if (json.type !== 'null') throw new Error('Expected type: null');
      }
    },
    {
      name: 'Void',
      schema: z.void(),
      validate: (json) => {
        if (json.type !== 'null') throw new Error('Expected type: null');
      }
    },
    
    // Never
    {
      name: 'Never',
      schema: z.never(),
      validate: (json) => {
        if (!json.not) throw new Error('Expected not constraint');
      }
    },
    
    // Promise (unwrapped)
    {
      name: 'Promise',
      schema: z.promise(z.string()),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected unwrapped string type');
      }
    },
    
    // Discriminated union
    {
      name: 'Discriminated union',
      schema: z.discriminatedUnion('type', [
        z.object({ type: z.literal('a'), value: z.string() }),
        z.object({ type: z.literal('b'), value: z.number() })
      ]),
      validate: (json) => {
        if (!json.anyOf || json.anyOf.length !== 2) {
          throw new Error('Expected anyOf with 2 options');
        }
      }
    },
    
    // Tuple with rest
    {
      name: 'Tuple with rest',
      schema: z.tuple([z.string()]).rest(z.number()),
      validate: (json) => {
        if (json.type !== 'array') throw new Error('Expected type: array');
        if (!json.prefixItems || json.prefixItems.length !== 1) {
          throw new Error('Expected 1 prefix item');
        }
        if (!json.items || json.items.type !== 'number') {
          throw new Error('Expected rest items to be numbers');
        }
      }
    },
    
    // Array nonempty
    {
      name: 'Array nonempty',
      schema: z.array(z.string()).nonempty(),
      validate: (json) => {
        if (json.minItems !== 1) {
          throw new Error('Expected minItems: 1');
        }
      }
    },
    
    // Preprocess
    {
      name: 'Preprocess',
      schema: z.preprocess((val) => String(val), z.string()),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
      }
    },
    
    // Object with passthrough
    {
      name: 'Object passthrough',
      schema: z.object({ name: z.string() }).passthrough(),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        if (json.additionalProperties !== true) {
          throw new Error('Expected additionalProperties: true');
        }
      }
    },
    
    // Object with strict
    {
      name: 'Object strict',
      schema: z.object({ name: z.string() }).strict(),
      validate: (json) => {
        if (json.type !== 'object') throw new Error('Expected type: object');
        if (json.additionalProperties !== false) {
          throw new Error('Expected additionalProperties: false');
        }
      }
    },
    
    // Native enum
    {
      name: 'Native enum',
      schema: z.nativeEnum({ RED: 'red', GREEN: 'green', BLUE: 'blue' }),
      validate: (json) => {
        if (json.type !== 'string') throw new Error('Expected type: string');
        if (!json.enum || json.enum.length !== 3) {
          throw new Error('Expected 3 enum values');
        }
      }
    },
    
    // Number positive/negative
    {
      name: 'Number positive',
      schema: z.number().positive(),
      validate: (json) => {
        if (json.type !== 'number') throw new Error('Expected type: number');
        // positive() is greater_than 0 (exclusive)
        if (json.minimum !== 0 || !json.exclusiveMinimum) {
          throw new Error('Expected minimum: 0 with exclusiveMinimum: true');
        }
      }
    },
    {
      name: 'Number negative',
      schema: z.number().negative(),
      validate: (json) => {
        if (json.type !== 'number') throw new Error('Expected type: number');
        // negative() is less_than 0 (exclusive)
        if (json.maximum !== 0 || !json.exclusiveMaximum) {
          throw new Error('Expected maximum: 0 with exclusiveMaximum: true');
        }
      }
    }
  ];
  
  let passed = 0;
  let failed = 0;
  
  for (const test of tests) {
    try {
      const jsonSchema = getJsonSchema(test.schema);
      test.validate(jsonSchema);
      console.log(`✓ ${test.name}`);
      passed++;
    } catch (error) {
      console.error(`✗ ${test.name}:`, (error as Error).message);
      failed++;
    }
  }
  
  console.log('\n' + '='.repeat(80));
  console.log(`\nResults: ${passed} passed, ${failed} failed out of ${tests.length} tests`);
  
  if (failed > 0) {
    process.exit(1);
  }
  
  console.log('\n✅ All tests passed!');
}

runTests();
