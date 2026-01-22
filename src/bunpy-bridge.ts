// Stub file for bunpy bridge (removed in v3.0)
// Python dependencies are no longer included by default
// To use bunpy bridge, install: npm install bunpy

import { PythonBridge, RLMConfig, RLMResult } from './bridge-interface';

export class BunpyBridge implements PythonBridge {
  constructor() {
    throw new Error(
      'bunpy bridge is not available (Python dependencies removed in v3.0). ' +
      'Please use the Go bridge (default) or install bunpy separately: npm install bunpy'
    );
  }

  async initialize(_model: string, _config: RLMConfig): Promise<void> {
    throw new Error('bunpy bridge not available');
  }

  async completion(_query: string, _context: string): Promise<RLMResult> {
    throw new Error('bunpy bridge not available');
  }

  async cleanup(): Promise<void> {
    // No-op
  }
}
