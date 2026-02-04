package rlm

import (
	"strings"
	"testing"
)

func TestMetaAgentCreation(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
	}
	engine := New("gpt-4o-mini", config)

	maConfig := MetaAgentConfig{
		Enabled: true,
		Model:   "gpt-4o",
	}

	obs := NewNoopObserver()
	ma := NewMetaAgent(engine, maConfig, obs)

	if ma == nil {
		t.Fatal("expected non-nil meta agent")
	}
	if ma.config.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", ma.config.Model)
	}
}

func TestMetaAgentDefaultModel(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
	}
	engine := New("gpt-4o-mini", config)

	maConfig := MetaAgentConfig{
		Enabled: true,
		// No model specified - should default to engine model
	}

	obs := NewNoopObserver()
	ma := NewMetaAgent(engine, maConfig, obs)

	if ma.config.Model != "gpt-4o-mini" {
		t.Errorf("expected default model 'gpt-4o-mini', got '%s'", ma.config.Model)
	}
}

func TestMetaAgentNeedsOptimization(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
	}
	engine := New("gpt-4o-mini", config)

	tests := []struct {
		name        string
		maConfig    MetaAgentConfig
		query       string
		context     string
		shouldNeed  bool
	}{
		{
			name: "short vague query should need optimization",
			maConfig: MetaAgentConfig{
				Enabled: true,
			},
			query:      "what?",
			context:    "some context",
			shouldNeed: true,
		},
		{
			name: "specific query with length limit should not need optimization",
			maConfig: MetaAgentConfig{
				Enabled:        true,
				MaxOptimizeLen: 10000, // Non-zero enables specificity check
			},
			query:      "Extract all the email addresses from the document and list them",
			context:    "some context",
			shouldNeed: false,
		},
		{
			name: "long context triggers optimization",
			maConfig: MetaAgentConfig{
				Enabled:        true,
				MaxOptimizeLen: 100,
			},
			query:      "Find all errors and summarize the root causes from the log file",
			context:    string(make([]byte, 200)), // 200 bytes > 100 threshold
			shouldNeed: true,
		},
		{
			name: "always optimize when MaxOptimizeLen is 0",
			maConfig: MetaAgentConfig{
				Enabled:        true,
				MaxOptimizeLen: 0,
			},
			query:      "Extract the key takeaways and provide a comprehensive analysis of the conversation",
			context:    "short",
			shouldNeed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := NewNoopObserver()
			ma := NewMetaAgent(engine, tt.maConfig, obs)

			result := ma.needsOptimization(tt.query, tt.context)
			if result != tt.shouldNeed {
				t.Errorf("needsOptimization = %v, want %v", result, tt.shouldNeed)
			}
		})
	}
}

func TestMetaAgentBuildOptimizePrompt(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
	}
	engine := New("gpt-4o-mini", config)

	obs := NewNoopObserver()
	ma := NewMetaAgent(engine, MetaAgentConfig{Enabled: true}, obs)

	prompt := ma.buildOptimizePrompt("what are the key points?", "some long context here")

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}

	// Should contain the original query
	if !strings.Contains(prompt, "what are the key points?") {
		t.Error("prompt should contain the original query")
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
	}

	for _, tt := range tests {
		result := truncateStr(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestMetaAgentConfigParsing(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
		"meta_agent": map[string]interface{}{
			"enabled":          true,
			"model":            "gpt-4o",
			"max_optimize_len": 5000,
		},
	}

	parsed := ConfigFromMap(config)

	if parsed.MetaAgent == nil {
		t.Fatal("expected non-nil MetaAgent config")
	}
	if !parsed.MetaAgent.Enabled {
		t.Error("expected meta_agent.enabled to be true")
	}
	if parsed.MetaAgent.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", parsed.MetaAgent.Model)
	}
	if parsed.MetaAgent.MaxOptimizeLen != 5000 {
		t.Errorf("expected max_optimize_len 5000, got %d", parsed.MetaAgent.MaxOptimizeLen)
	}
}

func TestConfigWithObservability(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
		"debug":   true,
		"observability": map[string]interface{}{
			"trace_enabled": true,
			"service_name":  "test-rlm",
		},
	}

	parsed := ConfigFromMap(config)

	if parsed.Observability == nil {
		t.Fatal("expected non-nil Observability config")
	}
	if !parsed.Observability.Debug {
		t.Error("expected debug to be true")
	}
	if !parsed.Observability.TraceEnabled {
		t.Error("expected trace_enabled to be true")
	}
	if parsed.Observability.ServiceName != "test-rlm" {
		t.Errorf("expected service_name 'test-rlm', got '%s'", parsed.Observability.ServiceName)
	}
}

func TestEngineWithMetaAgent(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
		MetaAgent: &MetaAgentConfig{
			Enabled: true,
			Model:   "gpt-4o",
		},
	}

	engine := New("gpt-4o-mini", config)

	if engine.metaAgent == nil {
		t.Error("expected meta agent to be initialized")
	}
}

func TestEngineWithObservability(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
		Observability: &ObservabilityConfig{
			Debug: true,
		},
	}

	engine := New("gpt-4o-mini", config)
	defer engine.Shutdown()

	obs := engine.GetObserver()
	if obs == nil {
		t.Fatal("expected non-nil observer")
	}
	if !obs.config.Debug {
		t.Error("expected debug mode enabled")
	}
}

func TestEngineWithoutMetaAgent(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      5,
		MaxIterations: 30,
	}

	engine := New("gpt-4o-mini", config)

	if engine.metaAgent != nil {
		t.Error("expected meta agent to be nil when not configured")
	}
}
