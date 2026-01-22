export interface RLMStats {
  llm_calls: number;
  iterations: number;
  depth: number;
}

export interface RLMResult {
  result: string;
  stats: RLMStats;
}

export interface RLMConfig {
  recursive_model?: string;
  api_base?: string;
  api_key?: string;
  max_depth?: number;
  max_iterations?: number;
  pythonia_timeout?: number;  // Timeout in milliseconds for pythonia calls (default: 100000ms)
  go_binary_path?: string; // Optional override path for Go binary
  [key: string]: any;
}

export interface PythonBridge {
  completion(
    model: string,
    query: string,
    context: string,
    rlmConfig: RLMConfig
  ): Promise<RLMResult>;
  
  cleanup(): Promise<void>;
}
