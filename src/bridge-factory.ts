import * as fs from 'fs';
import * as path from 'path';
import { PythonBridge } from './bridge-interface';

export type BridgeType = 'go' | 'pythonia' | 'bunpy' | 'auto';

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
 * Detect the current JavaScript runtime
 */
function detectRuntime(): 'node' | 'bun' | 'unknown' {
  // Check for Bun
  if (typeof (globalThis as any).Bun !== 'undefined') {
    return 'bun';
  }
  
  // Check for Node.js
  if (typeof process !== 'undefined' && process.versions && process.versions.node) {
    return 'node';
  }
  
  return 'unknown';
}

/**
 * Create appropriate Python bridge based on runtime or explicit choice
 */
export async function createBridge(bridgeType: BridgeType = 'auto'): Promise<PythonBridge> {
  let selectedBridge: 'go' | 'pythonia' | 'bunpy';
  
  if (bridgeType === 'auto') {
    if (isGoBinaryAvailable()) {
      selectedBridge = 'go';
    } else {
    const runtime = detectRuntime();
    
    if (runtime === 'bun') {
      selectedBridge = 'bunpy';
    } else if (runtime === 'node') {
      selectedBridge = 'pythonia';
    } else {
      throw new Error(
        'Unable to detect runtime. Please explicitly specify bridge type.\n' +
        'Supported runtimes: Go binary, Node.js (pythonia), or Bun (bunpy)'
      );
    }
    }
  } else {
    selectedBridge = bridgeType;
  }
  
  if (selectedBridge === 'go') {
    const { GoBridge } = await import('./go-bridge');
    return new GoBridge();
  }

  if (selectedBridge === 'bunpy') {
    try {
      const { BunpyBridge } = await import('./bunpy-bridge');
      return new BunpyBridge();
    } catch (error: any) {
      if (bridgeType === 'auto' && error.message?.includes('bunpy is not installed')) {
        console.warn('[recursive-llm-ts] bunpy not found, falling back to pythonia');
        selectedBridge = 'pythonia';
      } else {
        throw error;
      }
    }
  }
  
  if (selectedBridge === 'pythonia') {
    const { PythoniaBridge } = await import('./rlm-bridge');
    return new PythoniaBridge();
  }
  
  throw new Error('Failed to initialize bridge');
}
