import * as fs from 'fs';
import * as path from 'path';
import { Bridge } from './bridge-interface';

export type BridgeType = 'go';

const DEFAULT_GO_BINARY = process.platform === 'win32' ? 'rlm-go.exe' : 'rlm-go';

function resolveDefaultGoBinary(): string {
  return path.join(__dirname, '..', 'bin', DEFAULT_GO_BINARY);
}

function isGoBinaryAvailable(): boolean {
  const envPath = process.env.RLM_GO_BINARY;
  if (envPath && fs.existsSync(envPath)) {
    return true;
  }
  return fs.existsSync(resolveDefaultGoBinary());
}

/**
 * Create the Go bridge for RLM communication.
 * Throws if the Go binary is not available.
 */
export async function createBridge(bridgeType: BridgeType = 'go'): Promise<Bridge> {
  if (!isGoBinaryAvailable()) {
    throw new Error(
      'Go RLM binary not found. Build it with: node scripts/build-go-binary.js\n' +
      'Ensure Go 1.25+ is installed: https://go.dev/dl/'
    );
  }

  const { GoBridge } = await import('./go-bridge');
  return new GoBridge();
}
