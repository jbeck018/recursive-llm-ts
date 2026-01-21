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
      // Create RLM instance with config
      const RLMClass = await this.rlmModule.RLM;
      const rlmInstance = await RLMClass(model, rlmConfig);

      // Call completion method
      const result = await rlmInstance.completion(query, context);
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
