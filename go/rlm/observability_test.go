package rlm

import (
	"strings"
	"testing"
	"time"
)

func TestNewObserver(t *testing.T) {
	t.Run("with debug enabled", func(t *testing.T) {
		obs := NewObserver(ObservabilityConfig{Debug: true})
		if obs == nil {
			t.Fatal("expected non-nil observer")
		}
		if !obs.config.Debug {
			t.Error("expected debug to be enabled")
		}
	})

	t.Run("with tracing enabled", func(t *testing.T) {
		obs := NewObserver(ObservabilityConfig{
			TraceEnabled: true,
			ServiceName:  "test-rlm",
		})
		if obs == nil {
			t.Fatal("expected non-nil observer")
		}
		if obs.tracer == nil {
			t.Error("expected tracer to be initialized")
		}
		obs.Shutdown()
	})
}

func TestNewNoopObserver(t *testing.T) {
	obs := NewNoopObserver()
	if obs == nil {
		t.Fatal("expected non-nil observer")
	}

	// Should not panic with any operations
	ctx := obs.StartTrace("test", nil)
	obs.EndTrace(ctx)
	obs.Debug("test", "message %s", "arg")
	obs.Error("test", "error %s", "arg")
	obs.Event("test", map[string]string{"key": "value"})
	obs.LLMCall("model", 1, 0, time.Second, nil)
}

func TestObserverEvents(t *testing.T) {
	obs := NewObserver(ObservabilityConfig{Debug: true})

	obs.Event("test.event1", map[string]string{"key": "value1"})
	obs.Event("test.event2", map[string]string{"key": "value2"})

	events := obs.GetEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	if events[0].Name != "test.event1" {
		t.Errorf("expected first event name 'test.event1', got '%s'", events[0].Name)
	}
	if events[1].Name != "test.event2" {
		t.Errorf("expected second event name 'test.event2', got '%s'", events[1].Name)
	}
}

func TestObserverEventsJSON(t *testing.T) {
	obs := NewObserver(ObservabilityConfig{Debug: true})

	obs.Event("test.event", map[string]string{"key": "value"})

	jsonStr, err := obs.GetEventsJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(jsonStr, "test.event") {
		t.Error("expected JSON to contain event name")
	}
	if !strings.Contains(jsonStr, `"key"`) {
		t.Error("expected JSON to contain attribute key")
	}
}

func TestObserverLLMCall(t *testing.T) {
	obs := NewObserver(ObservabilityConfig{Debug: true})

	obs.LLMCall("gpt-4o-mini", 3, 150, 2*time.Second, nil)

	events := obs.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Type != "llm_call" {
		t.Errorf("expected type 'llm_call', got '%s'", event.Type)
	}
	if event.Attributes["model"] != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got '%s'", event.Attributes["model"])
	}
	if event.Attributes["message_count"] != "3" {
		t.Errorf("expected message_count '3', got '%s'", event.Attributes["message_count"])
	}
}

func TestObserverOnEventCallback(t *testing.T) {
	var receivedEvents []ObservabilityEvent

	obs := NewObserver(ObservabilityConfig{
		Debug: true,
		OnEvent: func(event ObservabilityEvent) {
			receivedEvents = append(receivedEvents, event)
		},
	})

	obs.Event("callback.test", map[string]string{"data": "test"})

	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 callback event, got %d", len(receivedEvents))
	}
	if receivedEvents[0].Name != "callback.test" {
		t.Errorf("expected event name 'callback.test', got '%s'", receivedEvents[0].Name)
	}
}

func TestObserverSpans(t *testing.T) {
	obs := NewObserver(ObservabilityConfig{
		TraceEnabled: true,
		ServiceName:  "test",
	})
	defer obs.Shutdown()

	traceCtx := obs.StartTrace("root", map[string]string{"op": "test"})
	spanCtx := obs.StartSpan("child", map[string]string{"step": "1"})
	obs.EndSpan(spanCtx)
	obs.EndTrace(traceCtx)

	events := obs.GetEvents()
	// Should have at least trace_start and span_start events
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
}

func TestObservabilityConfigFromMap(t *testing.T) {
	config := map[string]interface{}{
		"debug":         true,
		"trace_enabled": true,
		"service_name":  "my-service",
		"log_output":    "stderr",
	}

	obs := ObservabilityConfigFromMap(config)

	if !obs.Debug {
		t.Error("expected debug to be true")
	}
	if !obs.TraceEnabled {
		t.Error("expected trace_enabled to be true")
	}
	if obs.ServiceName != "my-service" {
		t.Errorf("expected service_name 'my-service', got '%s'", obs.ServiceName)
	}
	if obs.LogOutput != "stderr" {
		t.Errorf("expected log_output 'stderr', got '%s'", obs.LogOutput)
	}
}

func TestObservabilityConfigFromMap_Nil(t *testing.T) {
	obs := ObservabilityConfigFromMap(nil)
	if obs.Debug || obs.TraceEnabled {
		t.Error("expected all defaults for nil config")
	}
}

func TestExtractObservabilityConfig(t *testing.T) {
	fullConfig := map[string]interface{}{
		"debug":       true,
		"model":       "gpt-4o",
		"api_key":     "key",
		"service_name": "rlm-test",
		"observability": map[string]interface{}{
			"langfuse_enabled": true,
		},
	}

	obsConfig := ExtractObservabilityConfig(fullConfig)

	if v, ok := obsConfig["debug"].(bool); !ok || !v {
		t.Error("expected debug to be extracted")
	}
	if v, ok := obsConfig["service_name"].(string); !ok || v != "rlm-test" {
		t.Error("expected service_name to be extracted")
	}
	if v, ok := obsConfig["langfuse_enabled"].(bool); !ok || !v {
		t.Error("expected langfuse_enabled from nested observability config")
	}
	if _, ok := obsConfig["model"]; ok {
		t.Error("model should not be in observability config")
	}
}

func TestRedactSensitive(t *testing.T) {
	attrs := map[string]string{
		"model":   "gpt-4o",
		"api_key": "sk-12345",
		"secret":  "my-secret",
		"query":   "hello world",
	}

	redacted := RedactSensitive(attrs)

	if redacted["model"] != "gpt-4o" {
		t.Error("model should not be redacted")
	}
	if redacted["api_key"] != "[REDACTED]" {
		t.Error("api_key should be redacted")
	}
	if redacted["secret"] != "[REDACTED]" {
		t.Error("secret should be redacted")
	}
	if redacted["query"] != "hello world" {
		t.Error("query should not be redacted")
	}
}

func TestFormatStatsWithObservability(t *testing.T) {
	stats := RLMStats{
		LlmCalls:       5,
		Iterations:     3,
		Depth:          1,
		ParsingRetries: 2,
	}

	obs := NewObserver(ObservabilityConfig{Debug: true})
	obs.Event("test", map[string]string{"data": "value"})

	result := FormatStatsWithObservability(stats, obs)

	if result["llm_calls"] != 5 {
		t.Errorf("expected llm_calls 5, got %v", result["llm_calls"])
	}
	if result["parsing_retries"] != 2 {
		t.Errorf("expected parsing_retries 2, got %v", result["parsing_retries"])
	}
	if _, ok := result["trace_events"]; !ok {
		t.Error("expected trace_events in debug mode")
	}
}
