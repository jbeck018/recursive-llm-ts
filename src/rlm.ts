import { RLMConfig, RLMResult } from './bridge-interface';
import { createBridge, BridgeType } from './bridge-factory';
import { PythonBridge } from './bridge-interface';
import { z } from 'zod';
import { StructuredRLMResult } from './structured-types';

export class RLM {
  private bridge: PythonBridge | null = null;
  private model: string;
  private rlmConfig: RLMConfig;
  private bridgeType: BridgeType;

  constructor(model: string, rlmConfig: RLMConfig = {}, bridgeType: BridgeType = 'auto') {
    this.model = model;
    this.rlmConfig = rlmConfig;
    this.bridgeType = bridgeType;
  }

  private async ensureBridge(): Promise<PythonBridge> {
    if (!this.bridge) {
      this.bridge = await createBridge(this.bridgeType);
    }
    return this.bridge;
  }

  public async completion(query: string, context: string): Promise<RLMResult> {
    const bridge = await this.ensureBridge();
    return bridge.completion(this.model, query, context, this.rlmConfig);
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
    
    // Validate result against Zod schema for type safety
    const validated = schema.parse(result.result);
    
    return {
      result: validated,
      stats: result.stats
    };
  }

  private zodToJsonSchema(schema: z.ZodSchema<any>): any {
    const def = (schema as any)._def;
    
    // Check for object type by presence of shape
    if (def.shape) {
        const shape = def.shape;
        const properties: any = {};
        const required: string[] = [];
        
        for (const [key, value] of Object.entries(shape)) {
          properties[key] = this.zodToJsonSchema(value as z.ZodSchema<any>);
          if (!(value as any).isOptional()) {
            required.push(key);
          }
        }
        
        return {
          type: 'object',
          properties,
          required: required.length > 0 ? required : undefined
        };
    }
    
    // Check for array type - Zod arrays have an 'element' property (or 'type' in older versions)
    if (def.type === 'array' && (def.element || def.type)) {
      const itemSchema = def.element || def.type;
      return {
        type: 'array',
        items: this.zodToJsonSchema(itemSchema)
      };
    }
    
    // Check for enum - Zod enums have a 'type' of 'enum' and 'entries' object
    if (def.type === 'enum' && def.entries) {
      return {
        type: 'string',
        enum: Object.keys(def.entries)
      };
    }
    
    // Check for legacy enum with values array
    if (def.values && Array.isArray(def.values)) {
      return {
        type: 'string',
        enum: def.values
      };
    }
    
    // Check for optional/nullable
    if (def.innerType) {
      const inner = this.zodToJsonSchema(def.innerType);
      return def.typeName === 'ZodNullable' ? { ...inner, nullable: true } : inner;
    }
    
    // Detect primitive types
    const defType = def.type;
    if (defType === 'string') return { type: 'string' };
    if (defType === 'number') return { type: 'number' };
    if (defType === 'boolean') return { type: 'boolean' };
    
    // Default fallback
    return { type: 'string' };
  }

  public async cleanup(): Promise<void> {
    if (this.bridge) {
      await this.bridge.cleanup();
      this.bridge = null;
    }
  }
}
