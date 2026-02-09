export interface RLMStats {
  llm_calls: number;
  iterations: number;
  depth: number;
  parsing_retries?: number;
}

export interface RLMResult {
  result: string | any; // Can be string for normal completions or object for structured
  stats: RLMStats;
  structured_result?: boolean; // Flag indicating if result is structured
  trace_events?: TraceEvent[]; // Observability trace events when enabled
}

export interface MetaAgentConfig {
  enabled: boolean;
  model?: string; // Model to use for meta-agent (defaults to main model)
  max_optimize_len?: number; // Max context length before optimization (0 = always optimize)
}

export interface ObservabilityConfig {
  debug?: boolean; // Enable verbose debug logging of all internal operations
  trace_enabled?: boolean; // Enable OpenTelemetry tracing
  trace_endpoint?: string; // OTLP endpoint for trace export (e.g., "localhost:4317")
  service_name?: string; // Service name for traces (default: "rlm")
  log_output?: string; // Where to write debug logs ("stderr", "stdout", or a file path)
  langfuse_enabled?: boolean; // Enable Langfuse-compatible trace output
  langfuse_public_key?: string; // Langfuse public key
  langfuse_secret_key?: string; // Langfuse secret key
  langfuse_host?: string; // Langfuse API host (default: "https://cloud.langfuse.com")
}

export interface TraceEvent {
  timestamp: string;
  type: string; // "span_start", "span_end", "llm_call", "log", "error", "event"
  name: string;
  attributes: Record<string, string>;
  duration?: number;
  trace_id?: string;
  span_id?: string;
  parent_id?: string;
}

export interface RLMConfig {
  recursive_model?: string;
  api_base?: string;
  api_key?: string;
  max_depth?: number;
  max_iterations?: number;
  pythonia_timeout?: number;  // Timeout in milliseconds for pythonia calls (default: 100000ms)
  go_binary_path?: string; // Optional override path for Go binary

  // Meta-agent configuration
  meta_agent?: MetaAgentConfig;

  // Observability configuration
  observability?: ObservabilityConfig;

  // Shorthand for observability.debug
  debug?: boolean;

  [key: string]: any;
}

export interface FileStorageConfig {
  /** Storage type: 'local' or 's3' */
  type: 'local' | 's3';
  /** For local: root directory path. For S3: bucket name */
  path: string;
  /** For S3: the prefix (folder path) within the bucket */
  prefix?: string;
  /** For S3: AWS region */
  region?: string;
  /** For S3: explicit credentials */
  credentials?: { accessKeyId: string; secretAccessKey: string; sessionToken?: string };
  /** For S3: custom endpoint URL (e.g. for MinIO, LocalStack) */
  endpoint?: string;
  /** Glob patterns to include (e.g. ['*.ts', '*.md']) */
  includePatterns?: string[];
  /** Glob patterns to exclude (e.g. ['node_modules/**']) */
  excludePatterns?: string[];
  /** Maximum file size in bytes to include (default: 1MB) */
  maxFileSize?: number;
  /** Maximum total context size in bytes (default: 10MB) */
  maxTotalSize?: number;
  /** Maximum number of files to include (default: 1000) */
  maxFiles?: number;
  /** File extensions to include (e.g. ['.ts', '.md', '.txt']) */
  extensions?: string[];
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
