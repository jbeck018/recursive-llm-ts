import { z } from 'zod';
import { RLMResult, TraceEvent } from './bridge-interface';

export interface StructuredRLMResult<T> {
  result: T;
  stats: {
    llm_calls: number;
    iterations: number;
    depth: number;
    parsing_retries?: number;
  };
  trace_events?: TraceEvent[];
}

export interface SubTask {
  id: string;
  query: string;
  schema: z.ZodSchema<any>;
  dependencies: string[];
  path: string[];
}

export interface CoordinatorConfig {
  parallelExecution?: boolean; // Default: true
  maxRetries?: number; // Default: 3
  progressiveValidation?: boolean; // Default: true
}

export interface SchemaDecomposition {
  subTasks: SubTask[];
  dependencyGraph: Map<string, string[]>;
}
