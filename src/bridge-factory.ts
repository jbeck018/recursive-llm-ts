import { PythonBridge } from './bridge-interface';

export type BridgeType = 'pythonia' | 'bunpy' | 'auto';

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
  let selectedBridge: 'pythonia' | 'bunpy';
  
  if (bridgeType === 'auto') {
    const runtime = detectRuntime();
    
    if (runtime === 'bun') {
      selectedBridge = 'bunpy';
    } else if (runtime === 'node') {
      selectedBridge = 'pythonia';
    } else {
      throw new Error(
        'Unable to detect runtime. Please explicitly specify bridge type.\n' +
        'Supported runtimes: Node.js (pythonia) or Bun (bunpy)'
      );
    }
  } else {
    selectedBridge = bridgeType;
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
