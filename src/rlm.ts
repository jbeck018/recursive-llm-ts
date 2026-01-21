import { RLMConfig, RLMResult } from './bridge-interface';
import { createBridge, BridgeType } from './bridge-factory';
import { PythonBridge } from './bridge-interface';

export class RLM {
  private bridge: PythonBridge | null = null;
  private model: string;
  private rlmConfig: RLMConfig;
  private bridgeType: BridgeType;

  constructor(model: string, rlmConfig: RLMConfig = {}, bridgeType: BridgeType = 'auto') {
    this.model = model;
    this.rlmConfig = rlmConfig;
    this.bridgeType = bridgeType;
  }

  private async ensureBridge(): Promise<PythonBridge> {
    if (!this.bridge) {
      this.bridge = await createBridge(this.bridgeType);
    }
    return this.bridge;
  }

  public async completion(query: string, context: string): Promise<RLMResult> {
    const bridge = await this.ensureBridge();
    return bridge.completion(this.model, query, context, this.rlmConfig);
  }

  public async cleanup(): Promise<void> {
    if (this.bridge) {
      await this.bridge.cleanup();
      this.bridge = null;
    }
  }
}
