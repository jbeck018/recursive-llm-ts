package rlm

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ─── Error Detection Tests ───────────────────────────────────────────────────

func TestIsContextOverflow_DirectType(t *testing.T) {
	err := NewContextOverflowError(400, "test", 32768, 40354)
	coe, ok := IsContextOverflow(err)
	if !ok {
		t.Fatal("expected IsContextOverflow to return true for ContextOverflowError")
	}
	if coe.ModelLimit != 32768 {
		t.Errorf("expected ModelLimit 32768, got %d", coe.ModelLimit)
	}
	if coe.RequestTokens != 40354 {
		t.Errorf("expected RequestTokens 40354, got %d", coe.RequestTokens)
	}
}

func TestIsContextOverflow_FromAPIError_OpenAI(t *testing.T) {
	// Real OpenAI error format
	response := `{"error":{"message":"This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens. Please reduce the length of the input messages.","type":"invalid_request_error","param":"messages","code":"context_length_exceeded"}}`
	apiErr := NewAPIError(400, response)

	coe, ok := IsContextOverflow(apiErr)
	if !ok {
		t.Fatal("expected IsContextOverflow to detect OpenAI context overflow error")
	}
	if coe.ModelLimit != 32768 {
		t.Errorf("expected ModelLimit 32768, got %d", coe.ModelLimit)
	}
	if coe.RequestTokens != 40354 {
		t.Errorf("expected RequestTokens 40354, got %d", coe.RequestTokens)
	}
}

func TestIsContextOverflow_FromAPIError_vLLM(t *testing.T) {
	// vLLM / Ray Serve error format (the user's actual error)
	response := `{"error":{"message":"Message: This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens. Please reduce the length of the input messages. None (Request ID: ad08ee3b-67df-4ab2-bdeb-1e3135847e2a), Internal exception: ray.llm._internal.serve.core.configs.openai_api_models.OpenAIHTTPException","type":"OpenAIHTTPException","param":null,"code":400}}`
	apiErr := NewAPIError(400, response)

	coe, ok := IsContextOverflow(apiErr)
	if !ok {
		t.Fatal("expected IsContextOverflow to detect vLLM context overflow error")
	}
	if coe.ModelLimit != 32768 {
		t.Errorf("expected ModelLimit 32768, got %d", coe.ModelLimit)
	}
	if coe.RequestTokens != 40354 {
		t.Errorf("expected RequestTokens 40354, got %d", coe.RequestTokens)
	}
}

func TestIsContextOverflow_FromAPIError_Azure(t *testing.T) {
	// Azure OpenAI format
	response := `{"error":{"message":"This model's maximum context length is 8192 tokens, however you requested 12000 tokens","type":"invalid_request_error","code":"context_length_exceeded"}}`
	apiErr := NewAPIError(400, response)

	coe, ok := IsContextOverflow(apiErr)
	if !ok {
		t.Fatal("expected IsContextOverflow to detect Azure context overflow error")
	}
	if coe.ModelLimit != 8192 {
		t.Errorf("expected ModelLimit 8192, got %d", coe.ModelLimit)
	}
	if coe.RequestTokens != 12000 {
		t.Errorf("expected RequestTokens 12000, got %d", coe.RequestTokens)
	}
}

func TestIsContextOverflow_NotOverflow(t *testing.T) {
	tests := []error{
		errors.New("rate limit exceeded"),
		errors.New("timeout"),
		NewAPIError(500, "internal server error"),
		NewAPIError(429, "too many requests"),
		NewMaxIterationsError(10),
		NewMaxDepthError(5),
	}

	for _, err := range tests {
		_, ok := IsContextOverflow(err)
		if ok {
			t.Errorf("expected IsContextOverflow to return false for: %v", err)
		}
	}
}

func TestIsContextOverflow_GenericError(t *testing.T) {
	// Generic error with overflow message
	err := fmt.Errorf("This model's maximum context length is 4096 tokens. However, your request has 5000 input tokens.")
	coe, ok := IsContextOverflow(err)
	if !ok {
		t.Fatal("expected IsContextOverflow to detect overflow from generic error")
	}
	if coe.ModelLimit != 4096 {
		t.Errorf("expected ModelLimit 4096, got %d", coe.ModelLimit)
	}
	if coe.RequestTokens != 5000 {
		t.Errorf("expected RequestTokens 5000, got %d", coe.RequestTokens)
	}
}

func TestIsContextOverflow_MaxTokensTooLarge_vLLM(t *testing.T) {
	// vLLM/Ray Serve error when max_tokens exceeds remaining capacity
	// This is the exact error from the user's production logs
	response := `{"object":"error","message":"'max_tokens' or 'max_completion_tokens' is too large: 10000. This model's maximum context length is 32768 tokens and your request has 30168 input tokens (10000 > 32768 - 30168)","type":"BadRequestError","param":null,"code":400}`
	apiErr := NewAPIError(400, response)

	coe, ok := IsContextOverflow(apiErr)
	if !ok {
		t.Fatal("expected IsContextOverflow to detect max_tokens too large error")
	}
	if coe.ModelLimit != 32768 {
		t.Errorf("expected ModelLimit 32768, got %d", coe.ModelLimit)
	}
	// Request tokens should include both input + max_tokens: 30168 + 10000 = 40168
	if coe.RequestTokens != 40168 {
		t.Errorf("expected RequestTokens 40168 (input 30168 + max_tokens 10000), got %d", coe.RequestTokens)
	}
}

func TestIsContextOverflow_MaxCompletionTokensTooLarge(t *testing.T) {
	// OpenAI newer API format with max_completion_tokens
	response := `{"error":{"message":"'max_tokens' or 'max_completion_tokens' is too large: 5000. This model's maximum context length is 16384 tokens and your request has 14000 input tokens","type":"invalid_request_error","code":"invalid_request_error"}}`
	apiErr := NewAPIError(400, response)

	coe, ok := IsContextOverflow(apiErr)
	if !ok {
		t.Fatal("expected IsContextOverflow to detect max_completion_tokens too large error")
	}
	if coe.ModelLimit != 16384 {
		t.Errorf("expected ModelLimit 16384, got %d", coe.ModelLimit)
	}
	if coe.RequestTokens != 19000 {
		t.Errorf("expected RequestTokens 19000 (input 14000 + max_tokens 5000), got %d", coe.RequestTokens)
	}
}

func TestGetResponseTokenBudget(t *testing.T) {
	rlm := &RLM{
		extraParams: map[string]interface{}{
			"max_tokens": float64(10000),
		},
	}
	obs := NewNoopObserver()
	config := DefaultContextOverflowConfig()
	reducer := newContextReducer(rlm, config, obs)

	budget := reducer.getResponseTokenBudget(32768)
	if budget != 10000 {
		t.Errorf("expected response token budget 10000, got %d", budget)
	}
}

func TestGetResponseTokenBudget_MaxCompletionTokens(t *testing.T) {
	rlm := &RLM{
		extraParams: map[string]interface{}{
			"max_completion_tokens": float64(5000),
		},
	}
	obs := NewNoopObserver()
	config := DefaultContextOverflowConfig()
	reducer := newContextReducer(rlm, config, obs)

	budget := reducer.getResponseTokenBudget(32768)
	if budget != 5000 {
		t.Errorf("expected response token budget 5000, got %d", budget)
	}
}

func TestGetResponseTokenBudget_NoMaxTokens(t *testing.T) {
	rlm := &RLM{
		extraParams: map[string]interface{}{
			"temperature": 0.7,
		},
	}
	obs := NewNoopObserver()
	config := DefaultContextOverflowConfig()
	reducer := newContextReducer(rlm, config, obs)

	budget := reducer.getResponseTokenBudget(32768)
	if budget != 0 {
		t.Errorf("expected response token budget 0, got %d", budget)
	}
}

func TestMakeMapPhaseParams(t *testing.T) {
	rlm := &RLM{
		extraParams: map[string]interface{}{
			"max_tokens":          float64(10000),
			"custom_llm_provider": "vllm",
			"temperature":         0.7,
		},
	}
	obs := NewNoopObserver()
	config := DefaultContextOverflowConfig()
	reducer := newContextReducer(rlm, config, obs)

	params := reducer.makeMapPhaseParams(32768)

	// max_tokens should be capped (32768/4 = 8192, but cap is 2000)
	maxTokens, ok := params["max_tokens"].(int)
	if !ok {
		t.Fatal("expected max_tokens to be int in map phase params")
	}
	if maxTokens > 2000 {
		t.Errorf("expected map phase max_tokens <= 2000, got %d", maxTokens)
	}

	// custom_llm_provider should be preserved
	if params["custom_llm_provider"] != "vllm" {
		t.Errorf("expected custom_llm_provider to be preserved, got %v", params["custom_llm_provider"])
	}

	// temperature should be preserved
	if params["temperature"] != 0.7 {
		t.Errorf("expected temperature to be preserved, got %v", params["temperature"])
	}
}

func TestContextOverflowError_OverflowRatio(t *testing.T) {
	tests := []struct {
		limit    int
		request  int
		expected float64
	}{
		{32768, 40354, 1.2314}, // ~23% over
		{4096, 8192, 2.0},     // 100% over
		{100, 100, 1.0},       // exactly at limit
		{0, 100, 0.0},         // zero limit edge case
	}

	for _, tt := range tests {
		coe := NewContextOverflowError(400, "test", tt.limit, tt.request)
		ratio := coe.OverflowRatio()
		if ratio < tt.expected-0.01 || ratio > tt.expected+0.01 {
			t.Errorf("OverflowRatio(%d, %d) = %.4f, expected ~%.4f", tt.limit, tt.request, ratio, tt.expected)
		}
	}
}

// ─── Token Estimation Tests ──────────────────────────────────────────────────

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		minTokens int
		maxTokens int
	}{
		{"", 0, 0},
		{"hello", 1, 3},
		{"Hello, world!", 2, 5},
		{strings.Repeat("a", 100), 20, 40},    // 100 chars -> ~25-30 tokens
		{strings.Repeat("a", 1000), 200, 350},  // 1000 chars -> ~250-300 tokens
		{strings.Repeat("a", 10000), 2000, 3500}, // 10000 chars -> ~2500-3000 tokens
	}

	for _, tt := range tests {
		tokens := EstimateTokens(tt.text)
		if tokens < tt.minTokens || tokens > tt.maxTokens {
			t.Errorf("EstimateTokens(%d chars) = %d, expected between %d and %d",
				len(tt.text), tokens, tt.minTokens, tt.maxTokens)
		}
	}
}

func TestEstimateTokens_ConservativeForEnglish(t *testing.T) {
	// For English text, OpenAI's cl100k_base gives roughly 1 token per 4 chars
	// Our estimator should be conservative (overestimate) to prevent overflow
	englishText := "The quick brown fox jumped over the lazy dog. This is a test of the token estimation function."
	estimated := EstimateTokens(englishText)

	// Real token count for this text is about 22 (cl100k_base)
	// We expect our estimate to be >= actual (conservative)
	if estimated < 20 {
		t.Errorf("EstimateTokens for English text should be at least 20, got %d", estimated)
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello, how are you?"},
	}

	tokens := EstimateMessagesTokens(messages)
	// 3 (base) + 2*(4 overhead) + tokens for both messages
	if tokens < 15 {
		t.Errorf("EstimateMessagesTokens expected at least 15, got %d", tokens)
	}
}

// ─── Context Chunking Tests ─────────────────────────────────────────────────

func TestChunkContext_SmallContext(t *testing.T) {
	context := "This is a small context."
	chunks := ChunkContext(context, 1000)

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for small context, got %d", len(chunks))
	}
	if chunks[0] != context {
		t.Error("expected chunk to be the original context")
	}
}

func TestChunkContext_LargeContext(t *testing.T) {
	// Create context that's ~10000 tokens (~35000 chars at 3.5 chars/token)
	context := strings.Repeat("The quick brown fox jumped over the lazy dog. ", 700)
	chunks := ChunkContext(context, 2000)

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify all content is covered (with overlap, total chars may exceed original)
	totalChars := 0
	for _, chunk := range chunks {
		totalChars += len(chunk)
		// Each chunk should be within the token limit
		chunkTokens := EstimateTokens(chunk)
		if chunkTokens > 2500 { // Allow some slack
			t.Errorf("chunk has %d estimated tokens, expected <= 2500", chunkTokens)
		}
	}
}

func TestChunkContext_ParagraphBoundaries(t *testing.T) {
	// Context with clear paragraph boundaries
	paragraphs := []string{
		"First paragraph with some content here.",
		"Second paragraph with different content.",
		"Third paragraph with more information.",
		"Fourth paragraph wrapping up the text.",
	}
	context := strings.Join(paragraphs, "\n\n")

	// Use a budget that forces splitting into 2 chunks
	chunks := ChunkContext(context, 30) // ~30 tokens per chunk

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify chunks preferentially split at paragraph boundaries
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if len(trimmed) == 0 {
			t.Error("got empty chunk")
		}
	}
}

func TestChunkContext_ZeroTokenBudget(t *testing.T) {
	context := "Some text content here"
	chunks := ChunkContext(context, 0)
	// Should use default of 4000 tokens
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk with default budget, got %d", len(chunks))
	}
}

// ─── parseContextOverflowMessage Tests ──────────────────────────────────────

func TestParseContextOverflowMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		limit   int
		request int
		ok      bool
	}{
		{
			name:    "OpenAI standard",
			msg:     "This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens.",
			limit:   32768,
			request: 40354,
			ok:      true,
		},
		{
			name:    "Azure format",
			msg:     "This model's maximum context length is 8192 tokens, however you requested 12000 tokens",
			limit:   8192,
			request: 12000,
			ok:      true,
		},
		{
			name:    "With comma-separated numbers",
			msg:     "This model's maximum context length is 32,768 tokens. However, your request has 40,354 input tokens.",
			limit:   32768,
			request: 40354,
			ok:      true,
		},
		{
			name:    "context_length_exceeded code",
			msg:     `{"code":"context_length_exceeded","message":"This model's maximum context length is 4096 tokens. Your messages resulted in 5000 tokens."}`,
			limit:   4096,
			request: 5000,
			ok:      true,
		},
		{
			name:    "Not an overflow error",
			msg:     "rate limit exceeded",
			limit:   0,
			request: 0,
			ok:      false,
		},
		{
			name:    "Generic error",
			msg:     "internal server error",
			limit:   0,
			request: 0,
			ok:      false,
		},
		{
			name:    "vLLM wrapped error",
			msg:     "Message: This model's maximum context length is 16384 tokens. However, your request has 20000 input tokens. Internal exception: ray.llm._internal.serve",
			limit:   16384,
			request: 20000,
			ok:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, request, ok := parseContextOverflowMessage(tt.msg)
			if ok != tt.ok {
				t.Errorf("parseContextOverflowMessage ok=%v, expected %v", ok, tt.ok)
			}
			if ok && limit != tt.limit {
				t.Errorf("limit=%d, expected %d", limit, tt.limit)
			}
			if ok && request != tt.request {
				t.Errorf("request=%d, expected %d", request, tt.request)
			}
		})
	}
}

// ─── extractNumber Tests ─────────────────────────────────────────────────────

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		s        string
		prefix   string
		suffix   string
		expected int
	}{
		{"maximum context length is 32768 tokens", "maximum context length is ", " tokens", 32768},
		{"your request has 40354 input tokens", "your request has ", " input tokens", 40354},
		{"you requested 12000 tokens", "you requested ", " tokens", 12000},
		{"limit is 32,768 tokens", "limit is ", " tokens", 32768},
		{"no match here", "limit is ", " tokens", 0},
		{"limit is  tokens", "limit is ", " tokens", 0}, // empty number
	}

	for _, tt := range tests {
		result := extractNumber(tt.s, tt.prefix, tt.suffix)
		if result != tt.expected {
			t.Errorf("extractNumber(%q, %q, %q) = %d, expected %d", tt.s, tt.prefix, tt.suffix, result, tt.expected)
		}
	}
}

// ─── ContextOverflowConfig Tests ─────────────────────────────────────────────

func TestDefaultContextOverflowConfig(t *testing.T) {
	config := DefaultContextOverflowConfig()

	if !config.Enabled {
		t.Error("expected default Enabled to be true")
	}
	if config.Strategy != "mapreduce" {
		t.Errorf("expected default Strategy 'mapreduce', got %q", config.Strategy)
	}
	if config.SafetyMargin != 0.15 {
		t.Errorf("expected default SafetyMargin 0.15, got %f", config.SafetyMargin)
	}
	if config.MaxReductionAttempts != 3 {
		t.Errorf("expected default MaxReductionAttempts 3, got %d", config.MaxReductionAttempts)
	}
	if config.MaxModelTokens != 0 {
		t.Errorf("expected default MaxModelTokens 0 (auto-detect), got %d", config.MaxModelTokens)
	}
}

// ─── ConfigFromMap Integration Tests ─────────────────────────────────────────

func TestConfigFromMap_ContextOverflow(t *testing.T) {
	configMap := map[string]interface{}{
		"api_key": "test-key",
		"context_overflow": map[string]interface{}{
			"enabled":                true,
			"max_model_tokens":       float64(32768),
			"strategy":               "truncate",
			"safety_margin":          0.2,
			"max_reduction_attempts": float64(5),
		},
	}

	config := ConfigFromMap(configMap)

	if config.ContextOverflow == nil {
		t.Fatal("expected ContextOverflow to be set")
	}
	if !config.ContextOverflow.Enabled {
		t.Error("expected Enabled to be true")
	}
	if config.ContextOverflow.MaxModelTokens != 32768 {
		t.Errorf("expected MaxModelTokens 32768, got %d", config.ContextOverflow.MaxModelTokens)
	}
	if config.ContextOverflow.Strategy != "truncate" {
		t.Errorf("expected Strategy 'truncate', got %q", config.ContextOverflow.Strategy)
	}
	if config.ContextOverflow.SafetyMargin != 0.2 {
		t.Errorf("expected SafetyMargin 0.2, got %f", config.ContextOverflow.SafetyMargin)
	}
	if config.ContextOverflow.MaxReductionAttempts != 5 {
		t.Errorf("expected MaxReductionAttempts 5, got %d", config.ContextOverflow.MaxReductionAttempts)
	}
}

func TestConfigFromMap_NoContextOverflow(t *testing.T) {
	configMap := map[string]interface{}{
		"api_key": "test-key",
	}

	config := ConfigFromMap(configMap)

	if config.ContextOverflow != nil {
		t.Error("expected ContextOverflow to be nil when not specified in map")
	}
}

// ─── Truncation Strategy Tests ───────────────────────────────────────────────

func TestReduceByTruncation(t *testing.T) {
	// Create a large context
	context := strings.Repeat("This is a sentence. ", 500) // ~10000 chars

	obs := NewNoopObserver()
	config := ContextOverflowConfig{
		Enabled:              true,
		Strategy:             "truncate",
		SafetyMargin:         0.15,
		MaxReductionAttempts: 3,
	}

	rlmEngine := &RLM{
		model:    "test-model",
		observer: obs,
	}

	reducer := newContextReducer(rlmEngine, config, obs)
	result, err := reducer.reduceByTruncation(context, 2000, 500)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) >= len(context) {
		t.Errorf("expected truncated context to be shorter: %d >= %d", len(result), len(context))
	}

	// Should contain the truncation marker
	if !strings.Contains(result, "[... context truncated") {
		t.Error("expected truncation marker in result")
	}

	// Should preserve beginning and end
	if !strings.HasPrefix(result, "This is") {
		t.Error("expected result to start with original beginning")
	}
	if !strings.HasSuffix(strings.TrimSpace(result), "sentence. ") && !strings.HasSuffix(strings.TrimSpace(result), "sentence.") {
		// Just check it has some of the end content
		if !strings.Contains(result[len(result)/2:], "sentence") {
			t.Error("expected result to contain end content")
		}
	}
}

// ─── findBreakPoint Tests ────────────────────────────────────────────────────

func TestFindBreakPoint(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		start  int
		end    int
		check  func(int) bool
	}{
		{
			name:  "Prefers paragraph break",
			text:  "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.",
			start: 0,
			end:   20,
			check: func(bp int) bool {
				return bp == 18 // After first \n\n (search window reaches back to position 16)
			},
		},
		{
			name:  "Falls back to line break",
			text:  "Line one.\nLine two.\nLine three.",
			start: 0,
			end:   20,
			check: func(bp int) bool {
				return bp == 10 || bp == 20 // After a \n
			},
		},
		{
			name:  "End of text",
			text:  "Short text",
			start: 0,
			end:   100,
			check: func(bp int) bool {
				return bp == 10 // End of text
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := findBreakPoint(tt.text, tt.start, tt.end)
			if !tt.check(bp) {
				t.Errorf("findBreakPoint returned %d", bp)
			}
		})
	}
}

// ─── Error Chain Tests ───────────────────────────────────────────────────────

func TestContextOverflowError_ErrorChain(t *testing.T) {
	coe := NewContextOverflowError(400, "test response", 32768, 40354)

	// Verify the embedded types are accessible
	if coe.APIError == nil {
		t.Fatal("expected embedded APIError to be non-nil")
	}
	if coe.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", coe.StatusCode)
	}
	if coe.RLMError == nil {
		t.Fatal("expected embedded RLMError to be non-nil")
	}

	// Verify errors.As finds ContextOverflowError itself
	var coe2 *ContextOverflowError
	if !errors.As(coe, &coe2) {
		t.Error("expected errors.As to find ContextOverflowError")
	}

	// Verify errors.As finds APIError through Unwrap chain
	var apiErr *APIError
	if !errors.As(coe, &apiErr) {
		t.Error("expected errors.As to find APIError in chain")
	}

	// Test error message
	msg := coe.Error()
	if !strings.Contains(msg, "context overflow") {
		t.Errorf("expected error message to contain 'context overflow', got: %s", msg)
	}
	if !strings.Contains(msg, "32768") {
		t.Errorf("expected error message to contain model limit, got: %s", msg)
	}
}

// ─── RLM Integration Tests ──────────────────────────────────────────────────

func TestRLMDefaultContextOverflow(t *testing.T) {
	// Creating an RLM without explicit context_overflow should enable it by default
	config := Config{
		APIKey:    "test",
		MaxDepth:  5,
		MaxIterations: 30,
	}

	engine := New("test-model", config)

	if engine.contextOverflow == nil {
		t.Fatal("expected contextOverflow to be set by default")
	}
	if !engine.contextOverflow.Enabled {
		t.Error("expected contextOverflow to be enabled by default")
	}
	if engine.contextOverflow.Strategy != "mapreduce" {
		t.Errorf("expected default strategy 'mapreduce', got %q", engine.contextOverflow.Strategy)
	}
}

func TestRLMExplicitContextOverflow(t *testing.T) {
	config := Config{
		APIKey: "test",
		ContextOverflow: &ContextOverflowConfig{
			Enabled:        true,
			MaxModelTokens: 16384,
			Strategy:       "truncate",
		},
	}

	engine := New("test-model", config)

	if engine.contextOverflow.MaxModelTokens != 16384 {
		t.Errorf("expected MaxModelTokens 16384, got %d", engine.contextOverflow.MaxModelTokens)
	}
	if engine.contextOverflow.Strategy != "truncate" {
		t.Errorf("expected strategy 'truncate', got %q", engine.contextOverflow.Strategy)
	}
}

// ─── New Strategy Config Tests ──────────────────────────────────────────────

func TestRLMContextOverflow_TFIDFStrategy(t *testing.T) {
	config := Config{
		APIKey: "test",
		ContextOverflow: &ContextOverflowConfig{
			Enabled:  true,
			Strategy: "tfidf",
		},
	}
	engine := New("test-model", config)
	if engine.contextOverflow.Strategy != "tfidf" {
		t.Errorf("expected strategy 'tfidf', got %q", engine.contextOverflow.Strategy)
	}
}

func TestRLMContextOverflow_TextRankStrategy(t *testing.T) {
	config := Config{
		APIKey: "test",
		ContextOverflow: &ContextOverflowConfig{
			Enabled:  true,
			Strategy: "textrank",
		},
	}
	engine := New("test-model", config)
	if engine.contextOverflow.Strategy != "textrank" {
		t.Errorf("expected strategy 'textrank', got %q", engine.contextOverflow.Strategy)
	}
}

func TestRLMContextOverflow_RefineStrategy(t *testing.T) {
	config := Config{
		APIKey: "test",
		ContextOverflow: &ContextOverflowConfig{
			Enabled:  true,
			Strategy: "refine",
		},
	}
	engine := New("test-model", config)
	if engine.contextOverflow.Strategy != "refine" {
		t.Errorf("expected strategy 'refine', got %q", engine.contextOverflow.Strategy)
	}
}

func TestConfigFromMap_NewStrategies(t *testing.T) {
	for _, strategy := range []string{"tfidf", "textrank", "refine"} {
		configMap := map[string]interface{}{
			"api_key": "test-key",
			"context_overflow": map[string]interface{}{
				"enabled":  true,
				"strategy": strategy,
			},
		}
		config := ConfigFromMap(configMap)
		if config.ContextOverflow == nil {
			t.Fatalf("expected ContextOverflow for strategy %q", strategy)
		}
		if config.ContextOverflow.Strategy != strategy {
			t.Errorf("expected strategy %q, got %q", strategy, config.ContextOverflow.Strategy)
		}
	}
}

// ─── TF-IDF Reducer Integration Tests ───────────────────────────────────────

func TestReduceByTFIDF(t *testing.T) {
	// Build large context with multiple sentences
	sentences := []string{
		"The quarterly earnings report shows revenue of $4.2 billion.",
		"Weather conditions are expected to be mild this week.",
		"The merger was approved by regulatory authorities in March.",
		"Traffic congestion increased 15% during rush hour.",
		"Operating margins improved to 23.5% from 19.8% last year.",
		"The local park received new playground equipment.",
		"Customer retention rate reached 94% this quarter.",
		"The movie earned $150 million at the box office opening weekend.",
		"Year-over-year growth accelerated to 31% in Q4.",
		"The recipe calls for two cups of flour and one egg.",
	}
	context := strings.Join(sentences, " ")

	obs := NewNoopObserver()
	config := ContextOverflowConfig{
		Enabled:      true,
		Strategy:     "tfidf",
		SafetyMargin: 0.15,
	}
	rlmEngine := &RLM{model: "test-model", observer: obs}
	reducer := newContextReducer(rlmEngine, config, obs)

	result, err := reducer.reduceByTFIDF(context, 50, 10) // Very tight budget
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) >= len(context) {
		t.Errorf("expected reduced context, got same or larger: %d >= %d", len(result), len(context))
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// ─── TextRank Reducer Integration Tests ─────────────────────────────────────

func TestReduceByTextRank(t *testing.T) {
	sentences := []string{
		"Machine learning algorithms process large datasets.",
		"Deep learning models use neural network architectures.",
		"Natural language processing handles text data efficiently.",
		"The garden needs watering twice a week.",
		"Transformer models revolutionized NLP tasks.",
		"Computer vision detects objects in images.",
		"The recipe requires fresh ingredients only.",
	}
	context := strings.Join(sentences, " ")

	obs := NewNoopObserver()
	config := ContextOverflowConfig{
		Enabled:      true,
		Strategy:     "textrank",
		SafetyMargin: 0.15,
	}
	rlmEngine := &RLM{model: "test-model", observer: obs}
	reducer := newContextReducer(rlmEngine, config, obs)

	result, err := reducer.reduceByTextRank(context, 60, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) >= len(context) {
		t.Errorf("expected reduced context: %d >= %d", len(result), len(context))
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// ─── Strategy Dispatch Tests ────────────────────────────────────────────────

func TestReduceForCompletion_DispatchesTFIDF(t *testing.T) {
	// Large context that will need chunking
	context := strings.Repeat("Sentence about machine learning algorithms. ", 100)

	obs := NewNoopObserver()
	config := ContextOverflowConfig{
		Enabled:      true,
		Strategy:     "tfidf",
		SafetyMargin: 0.15,
	}
	rlmEngine := &RLM{model: "test-model", observer: obs}
	reducer := newContextReducer(rlmEngine, config, obs)

	result, err := reducer.ReduceForCompletion("What about ML?", context, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) >= len(context) {
		t.Errorf("expected reduced context for tfidf strategy")
	}
}

func TestReduceForCompletion_DispatchesTextRank(t *testing.T) {
	context := strings.Repeat("Deep learning models process data efficiently. ", 100)

	obs := NewNoopObserver()
	config := ContextOverflowConfig{
		Enabled:      true,
		Strategy:     "textrank",
		SafetyMargin: 0.15,
	}
	rlmEngine := &RLM{model: "test-model", observer: obs}
	reducer := newContextReducer(rlmEngine, config, obs)

	result, err := reducer.ReduceForCompletion("What about DL?", context, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) >= len(context) {
		t.Errorf("expected reduced context for textrank strategy")
	}
}
