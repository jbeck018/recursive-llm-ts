import * as path from 'path';
import { PythonBridge, RLMConfig, RLMResult, RLMStats } from './bridge-interface';

export class PythoniaBridge implements PythonBridge {
  private rlmModule: any = null;
  private python: any = null;

  private async ensureRLMModule(): Promise<void> {
    if (this.rlmModule) return;

    // Lazy load pythonia to avoid errors in Bun environments
    if (!this.python) {
      try {
        const pythonia = await import('pythonia');
        this.python = pythonia.python;
      } catch (error) {
        throw new Error(
          'pythonia is not installed. Install it with: npm install pythonia\n' +
          'Note: pythonia only works with Node.js runtime, not Bun'
        );
      }
    }

    const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');
    
    // Import sys module to add path
    const sys = await this.python('sys');
    const pathList = await sys.path;
    await pathList.insert(0, path.join(pythonPackagePath, 'src'));
    
    // Try to import rlm, install deps if needed
    try {
      // First check if litellm is available
      try {
        await this.python('litellm');
      } catch {
        // litellm not found, install dependencies
        console.log('[recursive-llm-ts] Installing Python dependencies (first time only)...');
        const { execSync } = await import('child_process');
        const pipCmd = `pip install -e "${pythonPackagePath}" || pip3 install -e "${pythonPackagePath}"`;
        execSync(pipCmd, { stdio: 'inherit' });
        console.log('[recursive-llm-ts] ✓ Python dependencies installed');
      }
      
      this.rlmModule = await this.python('rlm');
    } catch (error: any) {
      throw new Error(
        'Failed to import rlm module. Python dependencies may not be installed.\n' +
        `Run: pip install -e ${pythonPackagePath}\n` +
        `Original error: ${error.message || error}`
      );
    }
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
    if (this.python && this.rlmModule) {
      this.python.exit();
      this.rlmModule = null;
      this.python = null;
    }
  }
}
