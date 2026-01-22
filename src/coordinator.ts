import { z } from 'zod';
import { RLM } from './rlm';
import { RLMConfig } from './bridge-interface';
import { BridgeType } from './bridge-factory';
import { 
  StructuredRLMResult, 
  SubTask, 
  CoordinatorConfig, 
  SchemaDecomposition 
} from './structured-types';

export class RLMAgentCoordinator {
  private rlm: RLM;
  private config: CoordinatorConfig;

  constructor(
    model: string,
    rlmConfig: RLMConfig = {},
    bridgeType: BridgeType = 'auto',
    coordinatorConfig: CoordinatorConfig = {}
  ) {
    this.rlm = new RLM(model, rlmConfig, bridgeType);
    this.config = {
      parallelExecution: coordinatorConfig.parallelExecution ?? true,
      maxRetries: coordinatorConfig.maxRetries ?? 3,
      progressiveValidation: coordinatorConfig.progressiveValidation ?? true
    };
  }

  /**
   * Process a complex query with structured output using schema decomposition
   */
  async processComplex<T>(
    query: string,
    context: string,
    schema: z.ZodSchema<T>
  ): Promise<StructuredRLMResult<T>> {
    // Delegate to RLM which now handles everything in Go
    return this.rlm.structuredCompletion(query, context, schema, {
      maxRetries: this.config.maxRetries,
      parallelExecution: this.config.parallelExecution
    });
  }

  /**
   * Clean up resources
   */
  async cleanup(): Promise<void> {
    await this.rlm.cleanup();
  }
}
