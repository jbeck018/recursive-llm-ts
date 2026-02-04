import { RLMConfig, RLMResult, TraceEvent } from './bridge-interface';
import { createBridge, BridgeType } from './bridge-factory';
import { PythonBridge } from './bridge-interface';
import { z } from 'zod';
import { StructuredRLMResult } from './structured-types';

export class RLM {
  private bridge: PythonBridge | null = null;
  private model: string;
  private rlmConfig: RLMConfig;
  private bridgeType: BridgeType;
  private lastTraceEvents: TraceEvent[] = [];

  constructor(model: string, rlmConfig: RLMConfig = {}, bridgeType: BridgeType = 'auto') {
    this.model = model;
    this.rlmConfig = this.normalizeConfig(rlmConfig);
    this.bridgeType = bridgeType;
  }

  private normalizeConfig(config: RLMConfig): RLMConfig {
    // Normalize debug shorthand into observability config
    if (config.debug && !config.observability) {
      config.observability = { debug: true };
    } else if (config.debug && config.observability) {
      config.observability.debug = true;
    }
    return config;
  }

  private async ensureBridge(): Promise<PythonBridge> {
    if (!this.bridge) {
      this.bridge = await createBridge(this.bridgeType);
    }
    return this.bridge;
  }

  public async completion(query: string, context: string): Promise<RLMResult> {
    const bridge = await this.ensureBridge();
    const result = await bridge.completion(this.model, query, context, this.rlmConfig);
    if (result.trace_events) {
      this.lastTraceEvents = result.trace_events;
    }
    return result;
  }

  public async structuredCompletion<T>(
    query: string,
    context: string,
    schema: z.ZodSchema<T>,
    options: { maxRetries?: number; parallelExecution?: boolean } = {}
  ): Promise<StructuredRLMResult<T>> {
    const bridge = await this.ensureBridge();
    const jsonSchema = this.zodToJsonSchema(schema);

    const structuredConfig = {
      schema: jsonSchema,
      parallelExecution: options.parallelExecution ?? true,
      maxRetries: options.maxRetries ?? 3
    };

    const result = await bridge.completion(
      this.model,
      query,
      context,
      { ...this.rlmConfig, structured: structuredConfig }
    );

    if (result.trace_events) {
      this.lastTraceEvents = result.trace_events;
    }

    // Validate result against Zod schema for type safety
    const validated = schema.parse(result.result);

    return {
      result: validated,
      stats: result.stats,
      trace_events: result.trace_events
    };
  }

  /**
   * Returns trace events from the last operation.
   * Only populated when observability is enabled in the config.
   */
  public getTraceEvents(): TraceEvent[] {
    return this.lastTraceEvents;
  }

  private zodToJsonSchema(schema: z.ZodSchema<any>): any {
    const def = (schema as any)._def;
    const defType = def.type;
    
    // Handle wrapped types (optional, nullable, default, catch)
    if (defType === 'optional' || defType === 'nullable' || defType === 'default' || defType === 'catch') {
      const inner = this.zodToJsonSchema(def.innerType);
      if (defType === 'nullable') {
        return { ...inner, nullable: true };
      }
      return inner; // Optional/Default/Catch don't change the schema, just validation
    }
    
    // Handle effects (refine, transform, preprocess) - unwrap to inner type
    if (defType === 'effects') {
      return this.zodToJsonSchema(def.schema);
    }
    
    // Handle pipeline (pipe) - use the output schema
    if (defType === 'pipeline') {
      return this.zodToJsonSchema(def.out);
    }
    
    // Handle lazy schemas - unwrap the getter
    if (defType === 'lazy') {
      // For lazy schemas, we need to call the getter to get the actual schema
      try {
        const actualSchema = def.getter();
        return this.zodToJsonSchema(actualSchema);
      } catch (e) {
        // If lazy getter fails, fall back to generic object
        return { type: 'object' };
      }
    }
    
    // Handle branded types - unwrap to base type
    if (defType === 'branded') {
      return this.zodToJsonSchema(def.type);
    }
    
    // Handle readonly - pass through
    if (defType === 'readonly') {
      return this.zodToJsonSchema(def.innerType);
    }
    
    // Handle literals
    if (defType === 'literal') {
      // Literals in this Zod version use 'values' array
      if (def.values && def.values.length > 0) {
        const value = def.values[0];
        const valueType = typeof value;
        return {
          type: valueType === 'object' ? 'string' : valueType,
          enum: [value]
        };
      }
      // Fallback for other literal formats
      const value = def.value;
      if (value !== undefined) {
        const valueType = typeof value;
        return {
          type: valueType === 'object' ? 'string' : valueType,
          enum: [value]
        };
      }
    }
    
    
    // Handle unions
    if (defType === 'union' || defType === 'discriminatedUnion') {
      const options = def.options || Array.from(def.optionsMap?.values() || []);
      if (options.length > 0) {
        return {
          anyOf: options.map((opt: any) => this.zodToJsonSchema(opt))
        };
      }
    }
    
    // Handle intersections
    if (defType === 'intersection') {
      return {
        allOf: [
          this.zodToJsonSchema(def.left),
          this.zodToJsonSchema(def.right)
        ]
      };
    }
    
    // Handle object type
    if (def.shape || defType === 'object') {
      const shape = def.shape || {};
      const properties: any = {};
      const required: string[] = [];
      
      for (const [key, value] of Object.entries(shape)) {
        properties[key] = this.zodToJsonSchema(value as z.ZodSchema<any>);
        // A field is required if it's not optional and doesn't have a default
        const valueDef = (value as any)._def;
        const isOptional = (value as any).isOptional?.() ?? false;
        const hasDefault = valueDef?.type === 'default';
        if (!isOptional && !hasDefault) {
          required.push(key);
        }
      }
      
      const result: any = {
        type: 'object',
        properties
      };
      
      if (required.length > 0) {
        result.required = required;
      }
      
      // Handle unknown keys via catchall
      if (def.catchall) {
        const catchallType = (def.catchall as any)._def?.type;
        if (catchallType === 'unknown') {
          result.additionalProperties = true;
        } else if (catchallType === 'never') {
          result.additionalProperties = false;
        }
      }
      
      // Also check legacy unknownKeys
      if (def.unknownKeys === 'passthrough') {
        result.additionalProperties = true;
      } else if (def.unknownKeys === 'strict') {
        result.additionalProperties = false;
      }
      
      return result;
    }
    
    // Handle array type
    if (defType === 'array') {
      const itemSchema = def.element;
      const result: any = {
        type: 'array',
        items: this.zodToJsonSchema(itemSchema)
      };
      
      // Handle array length constraints from checks
      if (def.checks && Array.isArray(def.checks)) {
        for (const check of def.checks) {
          const checkDef = check._zod?.def || check.def || check;
          
          switch (checkDef.check) {
            case 'min_length':
              result.minItems = checkDef.minimum || checkDef.value;
              break;
            case 'max_length':
              result.maxItems = checkDef.maximum || checkDef.value;
              break;
            case 'exact_length':
              result.minItems = checkDef.value;
              result.maxItems = checkDef.value;
              break;
          }
        }
      }
      
      // Also check direct properties (legacy)
      if (def.minLength) result.minItems = def.minLength.value || def.minLength;
      if (def.maxLength) result.maxItems = def.maxLength.value || def.maxLength;
      if (def.exactLength) {
        const exact = def.exactLength.value || def.exactLength;
        result.minItems = exact;
        result.maxItems = exact;
      }
      
      return result;
    }
    
    // Handle tuple
    if (defType === 'tuple') {
      const items = def.items?.map((item: any) => this.zodToJsonSchema(item)) || [];
      const result: any = {
        type: 'array',
        prefixItems: items,
        minItems: items.length,
        maxItems: def.rest ? undefined : items.length
      };
      
      // Handle rest element
      if (def.rest) {
        result.items = this.zodToJsonSchema(def.rest);
      } else {
        result.items = false; // No additional items allowed
      }
      
      return result;
    }
    
    // Handle set - convert to array
    if (defType === 'set') {
      return {
        type: 'array',
        uniqueItems: true,
        items: def.valueType ? this.zodToJsonSchema(def.valueType) : {}
      };
    }
    
    // Handle map - convert to object
    if (defType === 'map') {
      return {
        type: 'object',
        additionalProperties: def.valueType ? this.zodToJsonSchema(def.valueType) : true
      };
    }
    
    // Handle record
    if (defType === 'record') {
      return {
        type: 'object',
        additionalProperties: def.valueType ? this.zodToJsonSchema(def.valueType) : true
      };
    }
    
    // Handle enum
    if (defType === 'enum' || defType === 'nativeEnum') {
      if (def.values && Array.isArray(def.values)) {
        return { type: 'string', enum: def.values };
      }
      if (def.entries) {
        return { type: 'string', enum: Object.keys(def.entries) };
      }
    }
    
    // Handle string with constraints
    if (defType === 'string') {
      const result: any = { type: 'string' };
      
      if (def.checks && Array.isArray(def.checks)) {
        for (const check of def.checks) {
          // Access the actual check data via _zod.def
          const checkDef = check._zod?.def || check.def || check;
          
          switch (checkDef.check) {
            case 'min_length':
              result.minLength = checkDef.minimum || checkDef.value;
              break;
            case 'max_length':
              result.maxLength = checkDef.maximum || checkDef.value;
              break;
            case 'length_equals':
              result.minLength = checkDef.length;
              result.maxLength = checkDef.length;
              break;
            case 'string_format':
              switch (checkDef.format) {
                case 'email':
                  result.format = 'email';
                  break;
                case 'url':
                  result.format = 'uri';
                  break;
                case 'uuid':
                  result.format = 'uuid';
                  break;
                case 'regex':
                  if (checkDef.pattern) {
                    result.pattern = checkDef.pattern.source || checkDef.pattern;
                  }
                  break;
              }
              break;
            case 'regex':
              result.pattern = checkDef.pattern?.source || checkDef.pattern;
              break;
          }
          
          // Also check nested def for formats
          if (check.def && check.def.format) {
            switch (check.def.format) {
              case 'email':
                result.format = 'email';
                break;
              case 'url':
                result.format = 'uri';
                break;
              case 'uuid':
                result.format = 'uuid';
                break;
            }
          }
        }
      }
      
      return result;
    }
    
    // Handle number/bigint with constraints
    if (defType === 'number' || defType === 'bigint') {
      const result: any = { type: defType === 'bigint' ? 'integer' : 'number' };
      
      if (def.checks && Array.isArray(def.checks)) {
        for (const check of def.checks) {
          // Access the actual check data via _zod.def
          const checkDef = check._zod?.def || check.def || check;
          
          switch (checkDef.check) {
            case 'number_format':
              if (checkDef.format === 'safeint') {
                result.type = 'integer';
              }
              break;
            case 'greater_than':
              result.minimum = checkDef.value;
              if (!checkDef.inclusive) {
                result.exclusiveMinimum = true;
              }
              break;
            case 'less_than':
              result.maximum = checkDef.value;
              if (!checkDef.inclusive) {
                result.exclusiveMaximum = true;
              }
              break;
            case 'multiple_of':
              result.multipleOf = checkDef.value;
              break;
          }
          
          // Also check direct properties (legacy support)
          if (check.isInt === true) {
            result.type = 'integer';
          }
        }
      }
      
      return result;
    }
    
    // Handle boolean
    if (defType === 'boolean') {
      return { type: 'boolean' };
    }
    
    // Handle date
    if (defType === 'date') {
      return { type: 'string', format: 'date-time' };
    }
    
    // Handle null
    if (defType === 'null') {
      return { type: 'null' };
    }
    
    // Handle undefined (not really JSON-serializable, but treat as null)
    if (defType === 'undefined') {
      return { type: 'null' };
    }
    
    // Handle void (same as undefined)
    if (defType === 'void') {
      return { type: 'null' };
    }
    
    // Handle any/unknown - no constraints
    if (defType === 'any' || defType === 'unknown') {
      return {}; // Empty schema accepts anything
    }
    
    // Handle never - impossible to satisfy
    if (defType === 'never') {
      return { not: {} }; // Schema that matches nothing
    }
    
    // Handle promise - unwrap to inner type
    if (defType === 'promise') {
      return this.zodToJsonSchema(def.innerType || def.type);
    }
    
    // Handle function - not JSON-serializable
    if (defType === 'function') {
      return { type: 'string', description: 'Function (not serializable)' };
    }
    
    // Default fallback
    console.warn(`Unknown Zod type: ${defType}, falling back to string`);
    return { type: 'string' };
  }
  
  private escapeRegex(str: string): string {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  public async cleanup(): Promise<void> {
    if (this.bridge) {
      await this.bridge.cleanup();
      this.bridge = null;
    }
  }
}
