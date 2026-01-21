import { python } from 'pythonia';
import * as path from 'path';

export interface RLMStats {
  llm_calls: number;
  iterations: number;
  depth: number;
}

export interface RLMResult {
  result: string;
  stats: RLMStats;
}

export interface RLMError {
  error: string;
}

export interface RLMConfig {
  recursive_model?: string;
  api_base?: string;
  api_key?: string;
  max_depth?: number;
  max_iterations?: number;
  pythonia_timeout?: number;  // Timeout in milliseconds for pythonia calls (default: 100000ms)
  [key: string]: any;
}

export class RLMBridge {
  private rlmModule: any = null;

  private async ensureRLMModule(): Promise<void> {
    if (this.rlmModule) return;

    // Get path to recursive-llm Python module
    const rlmPath = path.join(__dirname, '..', 'recursive-llm', 'src', 'rlm');
    
    // Import sys module to add path
    const sys = await python('sys');
    const pathList = await sys.path;
    await pathList.insert(0, path.join(__dirname, '..', 'recursive-llm', 'src'));
    
    // Import the rlm module
    this.rlmModule = await python('rlm');
  }

  public async completion(
    model: string,
    query: string,
    context: string,
    rlmConfig: RLMConfig = {}
  ): Promise<RLMResult> {
    await this.ensureRLMModule();

    try {
      // Extract pythonia timeout (default: 100000ms)
      const pythoniaTimeout = rlmConfig.pythonia_timeout || 100000;
      
      // Remove pythonia_timeout from config passed to Python
      const { pythonia_timeout, ...pythonConfig } = rlmConfig;
      
      // Create RLM instance with config, passing timeout to pythonia
      const RLMClass = await this.rlmModule.RLM;
      const rlmInstance = await RLMClass(model, { ...pythonConfig, $timeout: pythoniaTimeout });

      // Call completion method with timeout
      const result = await rlmInstance.completion(query, context, { $timeout: pythoniaTimeout });
      const stats = await rlmInstance.stats;

      // Convert Python stats dict to JS object
      const statsObj: RLMStats = {
        llm_calls: await stats.llm_calls,
        iterations: await stats.iterations,
        depth: await stats.depth
      };

      return {
        result: String(result),
        stats: statsObj
      };
    } catch (error: any) {
      console.error('[RLM_DEBUG] Error details:', error);
      throw new Error(`RLM completion failed: ${error.message || error}`);
    }
  }

  public async cleanup(): Promise<void> {
    if (this.rlmModule) {
      python.exit();
      this.rlmModule = null;
    }
  }
}
