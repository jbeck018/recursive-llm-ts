import { RLM } from '../src/rlm';
import { RLMConfig, MetaAgentConfig, ObservabilityConfig, TraceEvent } from '../src/bridge-interface';

// Test suite for new features: meta-agent, observability, and config
async function runTests() {
  console.log('🧪 Testing New Features (Meta-Agent, Observability, Config)\n');
  console.log('='.repeat(80));

  const tests: Array<{ name: string; test: () => void }> = [
    // Config interface tests
    {
      name: 'MetaAgentConfig interface',
      test: () => {
        const config: MetaAgentConfig = {
          enabled: true,
          model: 'gpt-4o',
          max_optimize_len: 5000
        };
        if (!config.enabled) throw new Error('Expected enabled to be true');
        if (config.model !== 'gpt-4o') throw new Error('Expected model gpt-4o');
        if (config.max_optimize_len !== 5000) throw new Error('Expected max_optimize_len 5000');
      }
    },
    {
      name: 'MetaAgentConfig minimal',
      test: () => {
        const config: MetaAgentConfig = { enabled: true };
        if (!config.enabled) throw new Error('Expected enabled to be true');
        if (config.model !== undefined) throw new Error('Expected model to be undefined');
      }
    },
    {
      name: 'ObservabilityConfig interface',
      test: () => {
        const config: ObservabilityConfig = {
          debug: true,
          trace_enabled: true,
          trace_endpoint: 'localhost:4317',
          service_name: 'my-rlm',
          log_output: 'stderr',
          langfuse_enabled: false
        };
        if (!config.debug) throw new Error('Expected debug to be true');
        if (!config.trace_enabled) throw new Error('Expected trace_enabled to be true');
        if (config.trace_endpoint !== 'localhost:4317') throw new Error('Expected correct endpoint');
        if (config.service_name !== 'my-rlm') throw new Error('Expected correct service_name');
      }
    },
    {
      name: 'ObservabilityConfig with Langfuse',
      test: () => {
        const config: ObservabilityConfig = {
          langfuse_enabled: true,
          langfuse_public_key: 'pk-test',
          langfuse_secret_key: 'sk-test',
          langfuse_host: 'https://custom.langfuse.com'
        };
        if (!config.langfuse_enabled) throw new Error('Expected langfuse_enabled');
        if (config.langfuse_public_key !== 'pk-test') throw new Error('Expected correct public key');
        if (config.langfuse_host !== 'https://custom.langfuse.com') throw new Error('Expected correct host');
      }
    },
    {
      name: 'TraceEvent interface',
      test: () => {
        const event: TraceEvent = {
          timestamp: '2024-01-01T00:00:00Z',
          type: 'llm_call',
          name: 'completion',
          attributes: { model: 'gpt-4o', message_count: '3' },
          duration: 1500,
          trace_id: 'abc123',
          span_id: 'def456'
        };
        if (event.type !== 'llm_call') throw new Error('Expected type llm_call');
        if (event.attributes.model !== 'gpt-4o') throw new Error('Expected model attribute');
      }
    },
    {
      name: 'RLMConfig with meta_agent',
      test: () => {
        const config: RLMConfig = {
          api_key: 'test-key',
          meta_agent: {
            enabled: true,
            model: 'gpt-4o'
          }
        };
        if (!config.meta_agent?.enabled) throw new Error('Expected meta_agent enabled');
        if (config.meta_agent?.model !== 'gpt-4o') throw new Error('Expected meta_agent model');
      }
    },
    {
      name: 'RLMConfig with observability',
      test: () => {
        const config: RLMConfig = {
          api_key: 'test-key',
          observability: {
            debug: true,
            trace_enabled: true,
            service_name: 'my-service'
          }
        };
        if (!config.observability?.debug) throw new Error('Expected observability debug');
        if (!config.observability?.trace_enabled) throw new Error('Expected trace_enabled');
      }
    },
    {
      name: 'RLMConfig debug shorthand',
      test: () => {
        const config: RLMConfig = {
          api_key: 'test-key',
          debug: true
        };
        if (!config.debug) throw new Error('Expected debug to be true');
      }
    },
    {
      name: 'RLMConfig full featured',
      test: () => {
        const config: RLMConfig = {
          api_key: 'test-key',
          api_base: 'https://api.example.com',
          max_depth: 5,
          max_iterations: 30,
          meta_agent: {
            enabled: true,
            model: 'gpt-4o',
            max_optimize_len: 10000
          },
          observability: {
            debug: true,
            trace_enabled: true,
            trace_endpoint: 'localhost:4317',
            service_name: 'rlm-prod',
            langfuse_enabled: true,
            langfuse_public_key: 'pk-123',
            langfuse_secret_key: 'sk-456'
          }
        };
        if (!config.meta_agent?.enabled) throw new Error('Expected meta_agent');
        if (!config.observability?.langfuse_enabled) throw new Error('Expected langfuse');
      }
    },
    {
      name: 'RLM constructor with meta-agent config',
      test: () => {
        const rlm = new RLM('gpt-4o-mini', {
          api_key: 'test-key',
          meta_agent: { enabled: true }
        });
        if (!rlm) throw new Error('Expected RLM instance');
      }
    },
    {
      name: 'RLM constructor with observability config',
      test: () => {
        const rlm = new RLM('gpt-4o-mini', {
          api_key: 'test-key',
          observability: { debug: true }
        });
        if (!rlm) throw new Error('Expected RLM instance');
      }
    },
    {
      name: 'RLM constructor with debug shorthand',
      test: () => {
        const rlm = new RLM('gpt-4o-mini', {
          api_key: 'test-key',
          debug: true
        });
        if (!rlm) throw new Error('Expected RLM instance');
      }
    },
    {
      name: 'RLM getTraceEvents returns empty initially',
      test: () => {
        const rlm = new RLM('gpt-4o-mini', { api_key: 'test-key' });
        const events = rlm.getTraceEvents();
        if (!Array.isArray(events)) throw new Error('Expected array');
        if (events.length !== 0) throw new Error('Expected empty array');
      }
    },
    {
      name: 'Config backward compatibility',
      test: () => {
        // Old-style configs should still work
        const config: RLMConfig = {
          api_key: 'test-key',
          max_iterations: 15,
          recursive_model: 'gpt-4o-mini',
          go_binary_path: '/custom/path'
        };
        const rlm = new RLM('gpt-4o-mini', config);
        if (!rlm) throw new Error('Expected RLM instance');
      }
    },
    {
      name: 'Config with all observability providers',
      test: () => {
        const config: RLMConfig = {
          api_key: 'test-key',
          observability: {
            debug: true,
            trace_enabled: true,
            trace_endpoint: 'localhost:4317',
            service_name: 'rlm',
            log_output: '/tmp/rlm.log',
            langfuse_enabled: true,
            langfuse_public_key: 'pk-test',
            langfuse_secret_key: 'sk-test',
            langfuse_host: 'https://cloud.langfuse.com'
          }
        };
        if (!config.observability) throw new Error('Expected observability');
        if (config.observability.log_output !== '/tmp/rlm.log') {
          throw new Error('Expected log_output');
        }
      }
    }
  ];

  let passed = 0;
  let failed = 0;

  for (const test of tests) {
    try {
      test.test();
      console.log(`✓ ${test.name}`);
      passed++;
    } catch (error) {
      console.error(`✗ ${test.name}:`, (error as Error).message);
      failed++;
    }
  }

  console.log('\n' + '='.repeat(80));
  console.log(`\nResults: ${passed} passed, ${failed} failed out of ${tests.length} tests`);

  if (failed > 0) {
    process.exit(1);
  }

  console.log('\n✅ All new feature tests passed!');
}

runTests();
