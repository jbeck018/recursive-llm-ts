import * as path from 'path';
import * as fs from 'fs';
import { PythonBridge, RLMConfig, RLMResult, RLMStats } from './bridge-interface';

export class PythoniaBridge implements PythonBridge {
  private rlmModule: any = null;
  private python: any = null;

  private async ensureRLMModule(): Promise<void> {
    if (this.rlmModule) return;

    // Lazy load pythonia to avoid errors in Bun environments
    if (!this.python) {
      try {
        // @ts-ignore - Optional dependency, may not be installed
        const pythonia = await import('pythonia');
        this.python = pythonia.python;
      } catch (error) {
        throw new Error(
          'pythonia is not available (Python dependencies removed in v3.0). ' +
          'Please use the Go bridge (default) or install pythonia separately: npm install pythonia\n' +
          'Note: pythonia only works with Node.js runtime, not Bun'
        );
      }
    }

    const pythonPackagePath = path.join(__dirname, '..', 'recursive-llm');
    
    // Import sys module to add path
    const sys = await this.python('sys');
    const pythonExecutable = String((await sys.executable) || 'python');
    const pythonCmd = pythonExecutable.includes(' ') ? `"${pythonExecutable}"` : pythonExecutable;
    const pipCmd = `${pythonCmd} -m pip install -e "${pythonPackagePath}"`;
    const pythonDepsPath = path.join(pythonPackagePath, '.pydeps');
    const pyprojectPath = path.join(pythonPackagePath, 'pyproject.toml');
    const dependencySpecifiers = (() => {
      try {
        const pyproject = fs.readFileSync(pyprojectPath, 'utf8');
        const depsBlock = pyproject.match(/dependencies\s*=\s*\[[\s\S]*?\]/);
        if (!depsBlock) return [];
        return Array.from(depsBlock[0].matchAll(/"([^"]+)"/g), (match) => match[1]);
      } catch {
        return [];
      }
    })();
    const depsInstallCmd = dependencySpecifiers.length > 0
      ? `${pythonCmd} -m pip install --upgrade --target "${pythonDepsPath}" ${dependencySpecifiers.map((dep) => `"${dep}"`).join(' ')}`
      : '';
    const installCmd = depsInstallCmd || pipCmd;
    const pathList = await sys.path;
    fs.mkdirSync(pythonDepsPath, { recursive: true });
    await pathList.insert(0, pythonDepsPath);
    await pathList.insert(0, path.join(pythonPackagePath, 'src'));
    
    // Try to import rlm, install deps if import fails
    try {
      this.rlmModule = await this.python('rlm');
    } catch (error: any) {
      // If import fails, try installing dependencies
      if (error.message?.includes('No module named')) {
        console.log('[recursive-llm-ts] Installing Python dependencies locally (first time only)...');
        try {
          const { execSync } = await import('child_process');
          execSync(installCmd, { stdio: 'inherit' });
          console.log('[recursive-llm-ts] ✓ Python dependencies installed');
          
          // Try import again
          this.rlmModule = await this.python('rlm');
        } catch (installError: any) {
          throw new Error(
            'Failed to import rlm module after installing dependencies.\n' +
            `Manual installation: ${installCmd}\n` +
            `Error: ${installError.message || installError}`
          );
        }
      } else {
        throw new Error(
          'Failed to import rlm module.\n' +
          `Run: ${installCmd}\n` +
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
