import * as path from 'path';
import { PythonBridge, RLMConfig, RLMResult, RLMStats } from './bridge-interface';

export class BunpyBridge implements PythonBridge {
  private rlmModule: any = null;
  private python: any = null;

  private async ensureRLMModule(): Promise<void> {
    if (this.rlmModule) return;

    // Lazy load bunpy to avoid errors in Node.js environments
    try {
      // Dynamic import to avoid TypeScript errors when bunpy is not installed
      const bunpy = await (new Function('return import("bunpy")')() as Promise<any>);
      this.python = bunpy.python;
    } catch (error) {
      throw new Error(
        'bunpy is not installed. Install it with: bun add bunpy\n' +
        'Note: bunpy only works with Bun runtime, not Node.js'
      );
    }

    // Import sys module to add path
    const sys = this.python.import('sys');
    const pythonExecutable = (() => {
      try {
        const sysExecutable = (sys as any).executable;
        if (sysExecutable && typeof sysExecutable.valueOf === 'function') {
          const value = sysExecutable.valueOf();
          if (typeof value === 'string' && value.trim()) {
            return value;
          }
        }
        if (typeof sysExecutable === 'string' && sysExecutable.trim()) {
          return sysExecutable;
        }
      } catch {
        // Fall back if bunpy can't coerce sys.executable
      }
      return 'python';
    })();
    const pythonCmd = pythonExecutable.includes(' ') ? `"${pythonExecutable}"` : pythonExecutable;
    const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');
    const pythonSrcPath = path.join(pythonPackagePath, 'src');
    const pipCmd = `${pythonCmd} -m pip install -e "${pythonPackagePath}"`;
    sys.path.insert(0, pythonSrcPath);

    // Try to import rlm, install deps if import fails
    try {
      this.rlmModule = this.python.import('rlm');
    } catch (error: any) {
      // If import fails, try installing dependencies
      if (error.message?.includes('No module named')) {
        console.log('[recursive-llm-ts] Installing Python dependencies (first time only)...');
        try {
          const { execSync } = await import('child_process');
          execSync(pipCmd, { stdio: 'inherit' });
          console.log('[recursive-llm-ts] ✓ Python dependencies installed');
          
          // Try import again
          this.rlmModule = this.python.import('rlm');
        } catch (installError: any) {
          throw new Error(
            'Failed to import rlm module after installing dependencies.\n' +
            `Manual installation: ${pipCmd}\n` +
            `Error: ${installError.message || installError}`
          );
        }
      } else {
        throw new Error(
          'Failed to import rlm module.\n' +
          `Run: ${pipCmd}\n` +
          `Original error: ${error.message || error}`
        );
      }
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
      // Remove pythonia_timeout from config (not applicable to bunpy)
      const { pythonia_timeout, ...pythonConfig } = rlmConfig;

      // Create RLM instance with config
      const RLMClass = this.rlmModule.RLM;
      const rlmInstance = RLMClass(model, pythonConfig);

      // Call completion method
      const result = rlmInstance.completion(query, context);
      const stats = rlmInstance.stats;

      // Convert Python stats dict to JS object
      const statsObj: RLMStats = {
        llm_calls: stats.llm_calls,
        iterations: stats.iterations,
        depth: stats.depth
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
    // Bunpy doesn't need explicit cleanup like pythonia
    this.rlmModule = null;
    this.python = null;
  }
}
