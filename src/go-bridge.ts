import * as fs from 'fs';
import * as path from 'path';
import { spawn } from 'child_process';
import { PythonBridge, RLMConfig, RLMResult } from './bridge-interface';

const DEFAULT_BINARY_NAME = process.platform === 'win32' ? 'rlm.exe' : 'rlm';

function resolveBinaryPath(rlmConfig: RLMConfig): string {
  const configuredPath = rlmConfig.go_binary_path || process.env.RLM_GO_BINARY;
  if (configuredPath) {
    return configuredPath;
  }

  // Try multiple locations
  const possiblePaths = [
    path.join(__dirname, '..', 'go', DEFAULT_BINARY_NAME),  // Development
    path.join(__dirname, '..', 'bin', DEFAULT_BINARY_NAME),  // NPM package
  ];

  for (const p of possiblePaths) {
    if (fs.existsSync(p)) {
      return p;
    }
  }

  return possiblePaths[0]; // Return first path, error will be caught later
}

function assertBinaryExists(binaryPath: string): void {
  if (!fs.existsSync(binaryPath)) {
    throw new Error(
      `Go RLM binary not found at ${binaryPath}.\n` +
      'Build it with: node scripts/build-go-binary.js'
    );
  }
}

function sanitizeConfig(config: RLMConfig): Record<string, unknown> {
  const { pythonia_timeout, go_binary_path, ...sanitized } = config;
  return sanitized;
}

export class GoBridge implements PythonBridge {
  public async completion(
    model: string,
    query: string,
    context: string,
    rlmConfig: RLMConfig = {}
  ): Promise<RLMResult> {
    const binaryPath = resolveBinaryPath(rlmConfig);
    assertBinaryExists(binaryPath);

    const payload = JSON.stringify({
      model,
      query,
      context,
      config: sanitizeConfig(rlmConfig)
    });

    return new Promise<RLMResult>((resolve, reject) => {
      const child = spawn(binaryPath, [], { stdio: ['pipe', 'pipe', 'pipe'] });
      let stdout = '';
      let stderr = '';

      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });

      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      child.on('error', (error) => {
        reject(new Error(`Failed to start Go binary: ${error.message}`));
      });

      child.on('close', (code) => {
        if (code !== 0) {
          reject(new Error(stderr || `Go binary exited with code ${code}`));
          return;
        }

        try {
          const parsed = JSON.parse(stdout) as RLMResult;
          resolve(parsed);
        } catch (error: any) {
          reject(new Error(`Failed to parse Go response: ${error.message || error}`));
        }
      });

      child.stdin.write(payload);
      child.stdin.end();
    });
  }

  public async cleanup(): Promise<void> {
    // No persistent processes to clean up.
  }
}
