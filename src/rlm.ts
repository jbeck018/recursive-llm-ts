import { RLMBridge, RLMConfig, RLMResult } from './rlm-bridge';

export class RLM {
  private bridge: RLMBridge;
  private model: string;
  private rlmConfig: RLMConfig;

  constructor(model: string, rlmConfig: RLMConfig = {}) {
    this.bridge = new RLMBridge();
    this.model = model;
    this.rlmConfig = rlmConfig;
  }

  public completion(query: string, context: string): Promise<RLMResult> {
    return this.bridge.completion(this.model, query, context, this.rlmConfig);
  }

  public cleanup(): Promise<void> {
    return this.bridge.cleanup();
  }
}
