package rlm

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Token Tracking Unit Tests ──────────────────────────────────────────────

func TestTokenUsage_ParsedFromAPIResponse(t *testing.T) {
	// Verify that CallChatCompletion correctly parses the usage field from API responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Hello world"}},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     150,
				"completion_tokens": 25,
				"total_tokens":      175,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := CallChatCompletion(ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "test"}},
		APIBase:  server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %q", result.Content)
	}
	if result.Usage == nil {
		t.Fatal("expected usage to be non-nil")
	}
	if result.Usage.PromptTokens != 150 {
		t.Errorf("expected 150 prompt tokens, got %d", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 25 {
		t.Errorf("expected 25 completion tokens, got %d", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 175 {
		t.Errorf("expected 175 total tokens, got %d", result.Usage.TotalTokens)
	}
}

func TestTokenUsage_NilWhenAPIDoesNotReturnUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Hello"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := CallChatCompletion(ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "test"}},
		APIBase:  server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage != nil {
		t.Errorf("expected usage to be nil when API doesn't return it, got %+v", result.Usage)
	}
}

func TestRLMStats_TokenAccumulation(t *testing.T) {
	// Test that token usage accumulates across multiple LLM calls
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": fmt.Sprintf(`FINAL("answer from call %d")`, callCount)}},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     100 * callCount,
				"completion_tokens": 20 * callCount,
				"total_tokens":      120 * callCount,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := New("test-model", Config{
		APIBase:       server.URL,
		MaxDepth:      5,
		MaxIterations: 10,
	})

	_, stats, err := engine.Completion("test query", "test context")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First call should have returned FINAL, so 1 LLM call
	if stats.LlmCalls != 1 {
		t.Errorf("expected 1 LLM call, got %d", stats.LlmCalls)
	}
	if stats.TotalTokens != 120 {
		t.Errorf("expected 120 total tokens, got %d", stats.TotalTokens)
	}
	if stats.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", stats.PromptTokens)
	}
	if stats.CompletionTokens != 20 {
		t.Errorf("expected 20 completion tokens, got %d", stats.CompletionTokens)
	}
}

func TestRLMStats_TokenAccumulation_MultipleIterations(t *testing.T) {
	// Simulates an RLM completion that takes 3 iterations before producing FINAL
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		content := "context.indexOf('test')"
		if callCount >= 3 {
			content = `FINAL("done after 3 calls")`
		}
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": content}},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     200,
				"completion_tokens": 50,
				"total_tokens":      250,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := New("test-model", Config{
		APIBase:       server.URL,
		MaxDepth:      5,
		MaxIterations: 10,
	})

	_, stats, err := engine.Completion("test query", "test context for searching")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.LlmCalls != 3 {
		t.Errorf("expected 3 LLM calls, got %d", stats.LlmCalls)
	}
	// 3 calls * 250 tokens each = 750 total
	if stats.TotalTokens != 750 {
		t.Errorf("expected 750 total tokens (3 calls * 250), got %d", stats.TotalTokens)
	}
	if stats.PromptTokens != 600 {
		t.Errorf("expected 600 prompt tokens (3 calls * 200), got %d", stats.PromptTokens)
	}
	if stats.CompletionTokens != 150 {
		t.Errorf("expected 150 completion tokens (3 calls * 50), got %d", stats.CompletionTokens)
	}
}

func TestRLMStats_TokensInJSONOutput(t *testing.T) {
	// Verify token fields are serialized in the JSON output
	stats := RLMStats{
		LlmCalls:         3,
		Iterations:       2,
		Depth:            0,
		TotalTokens:      750,
		PromptTokens:     600,
		CompletionTokens: 150,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal stats: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}

	if v, ok := parsed["total_tokens"].(float64); !ok || int(v) != 750 {
		t.Errorf("expected total_tokens=750 in JSON, got %v", parsed["total_tokens"])
	}
	if v, ok := parsed["prompt_tokens"].(float64); !ok || int(v) != 600 {
		t.Errorf("expected prompt_tokens=600 in JSON, got %v", parsed["prompt_tokens"])
	}
	if v, ok := parsed["completion_tokens"].(float64); !ok || int(v) != 150 {
		t.Errorf("expected completion_tokens=150 in JSON, got %v", parsed["completion_tokens"])
	}
}

func TestRLMStats_ZeroTokensOmittedFromJSON(t *testing.T) {
	// When no tokens are tracked, fields should be omitted (omitempty)
	stats := RLMStats{
		LlmCalls:   1,
		Iterations: 1,
		Depth:      0,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal stats: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "total_tokens") {
		t.Errorf("expected total_tokens to be omitted when zero, got: %s", jsonStr)
	}
	if strings.Contains(jsonStr, "prompt_tokens") {
		t.Errorf("expected prompt_tokens to be omitted when zero, got: %s", jsonStr)
	}
	if strings.Contains(jsonStr, "completion_tokens") {
		t.Errorf("expected completion_tokens to be omitted when zero, got: %s", jsonStr)
	}
}

func TestFormatStatsWithObservability_IncludesTokens(t *testing.T) {
	stats := RLMStats{
		LlmCalls:         2,
		Iterations:       1,
		Depth:            0,
		TotalTokens:      500,
		PromptTokens:     400,
		CompletionTokens: 100,
	}

	obs := NewNoopObserver()
	formatted := FormatStatsWithObservability(stats, obs)

	if v, ok := formatted["total_tokens"].(int); !ok || v != 500 {
		t.Errorf("expected total_tokens=500, got %v", formatted["total_tokens"])
	}
	if v, ok := formatted["prompt_tokens"].(int); !ok || v != 400 {
		t.Errorf("expected prompt_tokens=400, got %v", formatted["prompt_tokens"])
	}
	if v, ok := formatted["completion_tokens"].(int); !ok || v != 100 {
		t.Errorf("expected completion_tokens=100, got %v", formatted["completion_tokens"])
	}
}

func TestFormatStatsWithObservability_OmitsZeroTokens(t *testing.T) {
	stats := RLMStats{
		LlmCalls:   1,
		Iterations: 1,
		Depth:      0,
	}

	obs := NewNoopObserver()
	formatted := FormatStatsWithObservability(stats, obs)

	if _, exists := formatted["total_tokens"]; exists {
		t.Errorf("expected total_tokens to be absent when zero, got %v", formatted["total_tokens"])
	}
}

// ─── Token Efficiency Tests ─────────────────────────────────────────────────
//
// These tests prove that RLM context reduction strategies process fewer tokens
// than passing an entire large document through as raw context.

// generateLargeContext creates a realistic document of approximately targetTokens tokens.
// It generates structured content with numbered paragraphs to make it easy to verify
// that reduction strategies preserve key information.
func generateLargeContext(targetTokens int) string {
	// ~3.5 chars per token is our estimation ratio
	targetChars := int(float64(targetTokens) * 3.5)

	var sb strings.Builder
	sb.WriteString("# Technical Report: System Performance Analysis\n\n")
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString("This comprehensive report analyzes the performance characteristics of the distributed system ")
	sb.WriteString("deployed across three data centers. Key findings include a 15% improvement in latency, ")
	sb.WriteString("23% reduction in error rates, and significant cost savings through resource optimization.\n\n")

	paragraphNum := 1
	for sb.Len() < targetChars {
		// Generate diverse paragraph types to simulate realistic documents
		switch paragraphNum % 5 {
		case 0:
			fmt.Fprintf(&sb, "### Section %d: Database Performance Metrics\n\n", paragraphNum)
			fmt.Fprintf(&sb, "In quarter Q%d, the primary database cluster processed an average of %d,000 queries per second "+
				"with a p99 latency of %d.%d milliseconds. The read-to-write ratio was approximately %d:%d. "+
				"Connection pool utilization peaked at %d%% during high-traffic periods, with %d active connections "+
				"out of a configured maximum of %d. Index hit ratios remained above %d%% for all primary tables, "+
				"though the secondary indexes on the analytics tables showed degradation to %d%% during batch "+
				"processing windows. This resulted in an overall throughput improvement of %d.%d%% compared to "+
				"the previous quarter's baseline measurements.\n\n",
				paragraphNum%4+1, paragraphNum*12+50, paragraphNum%10+1, paragraphNum%99,
				paragraphNum%7+3, 1, paragraphNum%30+70, paragraphNum*3+100, paragraphNum*5+200,
				paragraphNum%5+95, paragraphNum%20+75, paragraphNum%15+5, paragraphNum%99)
		case 1:
			fmt.Fprintf(&sb, "### Section %d: API Gateway Statistics\n\n", paragraphNum)
			fmt.Fprintf(&sb, "The API gateway handled %d.%dM requests during the reporting period. Rate limiting "+
				"was triggered %d times for %d unique clients. The top 5 endpoints by traffic volume were: "+
				"/api/v2/users (%d.%d%%), /api/v2/products (%d.%d%%), /api/v2/orders (%d.%d%%), "+
				"/api/v2/analytics (%d.%d%%), and /api/v2/search (%d.%d%%). Authentication failures "+
				"decreased from %d to %d per day after implementing the new token refresh mechanism. "+
				"The overall API availability was %d.%d%% with %d minutes of total downtime.\n\n",
				paragraphNum*5+10, paragraphNum%99, paragraphNum*7+20, paragraphNum*3+5,
				paragraphNum%20+20, paragraphNum%99, paragraphNum%15+15, paragraphNum%99,
				paragraphNum%10+10, paragraphNum%99, paragraphNum%8+5, paragraphNum%99,
				paragraphNum%5+3, paragraphNum%99, paragraphNum*2+50, paragraphNum+10,
				99, paragraphNum%10+90, paragraphNum%30+5)
		case 2:
			fmt.Fprintf(&sb, "### Section %d: Memory and CPU Utilization\n\n", paragraphNum)
			fmt.Fprintf(&sb, "Across all %d nodes in the cluster, average memory utilization was %d.%d%%. "+
				"Node %d consistently showed the highest memory consumption at %d.%d%%, primarily due to "+
				"in-memory caching of frequently accessed data structures. CPU utilization averaged %d.%d%% "+
				"with peaks reaching %d.%d%% during the daily ETL batch processing window between "+
				"%d:00 and %d:00 UTC. Garbage collection pauses were reduced from an average of %dms to %dms "+
				"after tuning the JVM parameters. Thread pool saturation events decreased from %d per hour "+
				"to %d per hour following the implementation of adaptive thread pool sizing.\n\n",
				paragraphNum*2+20, paragraphNum%40+50, paragraphNum%99, paragraphNum%20+1,
				paragraphNum%15+80, paragraphNum%99, paragraphNum%30+40, paragraphNum%99,
				paragraphNum%20+75, paragraphNum%99, paragraphNum%6+2, paragraphNum%6+4,
				paragraphNum%50+100, paragraphNum%30+20, paragraphNum%10+5, paragraphNum%5+1)
		case 3:
			fmt.Fprintf(&sb, "### Section %d: Error Analysis and Incident Report\n\n", paragraphNum)
			fmt.Fprintf(&sb, "During the period, %d unique error types were observed across the system. "+
				"The most frequent error (ERR-%04d) was a transient connection timeout to the Redis cluster, "+
				"occurring %d times with a mean time to recovery of %d.%d seconds. Error category breakdown: "+
				"network errors (%d%%), application errors (%d%%), database errors (%d%%), "+
				"authentication errors (%d%%), and other (%d%%). The total error budget consumed was %d.%d%% "+
				"of the allocated %d.%d%% for the quarter. Two P2 incidents were recorded on days %d and %d, "+
				"with root causes traced to upstream provider instability and a misconfigured load balancer "+
				"health check interval respectively.\n\n",
				paragraphNum*3+15, paragraphNum+1000, paragraphNum*50+200, paragraphNum%10+1, paragraphNum%99,
				paragraphNum%30+30, paragraphNum%25+20, paragraphNum%20+15, paragraphNum%10+5,
				paragraphNum%10+5, paragraphNum%3, paragraphNum%99, paragraphNum%5, paragraphNum%99,
				paragraphNum%28+1, paragraphNum%28+15)
		case 4:
			fmt.Fprintf(&sb, "### Section %d: Cost Optimization Results\n\n", paragraphNum)
			fmt.Fprintf(&sb, "Infrastructure costs for the period totaled $%d,%03d.%02d, representing a "+
				"%d.%d%% decrease from the previous quarter. Key savings were achieved through: "+
				"reserved instance utilization (saving $%d,%03d), right-sizing %d underutilized instances "+
				"(saving $%d,%03d), implementing spot instances for batch workloads (saving $%d,%03d), "+
				"and optimizing data transfer routes (saving $%d,%03d). The cost per million API requests "+
				"decreased from $%d.%02d to $%d.%02d. Projected annual savings based on current trends: "+
				"$%d,%03d. Storage costs increased by %d.%d%% due to expanded logging retention requirements.\n\n",
				paragraphNum*100+500, paragraphNum%1000, paragraphNum%100, paragraphNum%15+5, paragraphNum%99,
				paragraphNum*20+100, paragraphNum%1000, paragraphNum*3+10, paragraphNum*10+50, paragraphNum%1000,
				paragraphNum*8+30, paragraphNum%1000, paragraphNum*5+20, paragraphNum%1000,
				paragraphNum%50+10, paragraphNum%100, paragraphNum%40+5, paragraphNum%100,
				paragraphNum*300+1000, paragraphNum%1000, paragraphNum%10+2, paragraphNum%99)
		}
		paragraphNum++
	}

	return sb.String()
}

func TestTokenEfficiency_TFIDFUsesFewerTokens(t *testing.T) {
	// Generate a large context (~35,000 tokens, well over 32k)
	largeContext := generateLargeContext(35000)
	originalTokens := EstimateTokens(largeContext)

	if originalTokens < 32000 {
		t.Fatalf("generated context is too small: %d tokens, need at least 32000", originalTokens)
	}
	t.Logf("Original context: %d chars, ~%d estimated tokens", len(largeContext), originalTokens)

	// Apply TF-IDF compression to fit within a 32k token budget
	modelLimit := 32768
	overhead := 1000 // System prompt + query overhead
	availableTokens := modelLimit - overhead

	compressed := CompressContextTFIDF(largeContext, availableTokens)
	compressedTokens := EstimateTokens(compressed)

	t.Logf("TF-IDF compressed: %d chars, ~%d estimated tokens", len(compressed), compressedTokens)
	t.Logf("Token reduction: %d -> %d (%.1f%% reduction)",
		originalTokens, compressedTokens,
		(1.0-float64(compressedTokens)/float64(originalTokens))*100)

	// Core assertion: TF-IDF MUST produce fewer tokens than the original
	if compressedTokens >= originalTokens {
		t.Errorf("TF-IDF failed to reduce tokens: original=%d, compressed=%d", originalTokens, compressedTokens)
	}

	// And it must fit within our budget
	if compressedTokens > availableTokens {
		t.Errorf("TF-IDF output exceeds budget: %d tokens > %d available", compressedTokens, availableTokens)
	}

	// Verify meaningful compression (at least 5% reduction for a context that's over budget)
	reductionPct := (1.0 - float64(compressedTokens)/float64(originalTokens)) * 100
	if reductionPct < 5.0 {
		t.Errorf("TF-IDF compression too weak: only %.1f%% reduction", reductionPct)
	}
}

func TestTokenEfficiency_TextRankUsesFewerTokens(t *testing.T) {
	largeContext := generateLargeContext(35000)
	originalTokens := EstimateTokens(largeContext)

	if originalTokens < 32000 {
		t.Fatalf("generated context is too small: %d tokens, need at least 32000", originalTokens)
	}
	t.Logf("Original context: %d chars, ~%d estimated tokens", len(largeContext), originalTokens)

	modelLimit := 32768
	overhead := 1000
	availableTokens := modelLimit - overhead

	compressed := CompressContextTextRank(largeContext, availableTokens)
	compressedTokens := EstimateTokens(compressed)

	t.Logf("TextRank compressed: %d chars, ~%d estimated tokens", len(compressed), compressedTokens)
	t.Logf("Token reduction: %d -> %d (%.1f%% reduction)",
		originalTokens, compressedTokens,
		(1.0-float64(compressedTokens)/float64(originalTokens))*100)

	if compressedTokens >= originalTokens {
		t.Errorf("TextRank failed to reduce tokens: original=%d, compressed=%d", originalTokens, compressedTokens)
	}

	if compressedTokens > availableTokens {
		t.Errorf("TextRank output exceeds budget: %d tokens > %d available", compressedTokens, availableTokens)
	}

	reductionPct := (1.0 - float64(compressedTokens)/float64(originalTokens)) * 100
	if reductionPct < 5.0 {
		t.Errorf("TextRank compression too weak: only %.1f%% reduction", reductionPct)
	}
}

func TestTokenEfficiency_TruncateUsesFewerTokens(t *testing.T) {
	largeContext := generateLargeContext(35000)
	originalTokens := EstimateTokens(largeContext)

	if originalTokens < 32000 {
		t.Fatalf("generated context is too small: %d tokens, need at least 32000", originalTokens)
	}

	modelLimit := 32768
	overhead := 1000

	// Create a reducer with truncation strategy
	engine := New("test-model", Config{
		MaxDepth:      5,
		MaxIterations: 10,
		ContextOverflow: &ContextOverflowConfig{
			Enabled:      true,
			Strategy:     "truncate",
			SafetyMargin: 0.15,
		},
	})

	reducer := newContextReducer(engine, *engine.contextOverflow, NewNoopObserver())
	truncated, err := reducer.reduceByTruncation(largeContext, modelLimit, overhead)
	if err != nil {
		t.Fatalf("truncation failed: %v", err)
	}

	truncatedTokens := EstimateTokens(truncated)

	t.Logf("Truncate: %d -> %d estimated tokens (%.1f%% reduction)",
		originalTokens, truncatedTokens,
		(1.0-float64(truncatedTokens)/float64(originalTokens))*100)

	if truncatedTokens >= originalTokens {
		t.Errorf("truncation failed to reduce tokens: original=%d, truncated=%d", originalTokens, truncatedTokens)
	}
}

func TestTokenEfficiency_ChunkingProducesSmallChunks(t *testing.T) {
	largeContext := generateLargeContext(35000)
	originalTokens := EstimateTokens(largeContext)

	if originalTokens < 32000 {
		t.Fatalf("generated context is too small: %d tokens, need at least 32000", originalTokens)
	}

	// Chunk with a 8k token budget per chunk
	chunkBudget := 8000
	chunks := ChunkContext(largeContext, chunkBudget)

	t.Logf("Chunked %d tokens into %d chunks (budget: %d tokens/chunk)", originalTokens, len(chunks), chunkBudget)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for %d token context, got %d", originalTokens, len(chunks))
	}

	// Each chunk must be smaller than the original
	for i, chunk := range chunks {
		chunkTokens := EstimateTokens(chunk)
		if chunkTokens >= originalTokens {
			t.Errorf("chunk %d is not smaller than original: %d tokens >= %d", i, chunkTokens, originalTokens)
		}
		t.Logf("  Chunk %d: %d estimated tokens", i, chunkTokens)
	}
}

func TestTokenEfficiency_PreemptiveReduction(t *testing.T) {
	// Test that PreemptiveReduceContext actually reduces a large context
	largeContext := generateLargeContext(35000)
	originalTokens := EstimateTokens(largeContext)

	engine := New("gpt-4o-mini", Config{
		MaxDepth:      5,
		MaxIterations: 10,
		ContextOverflow: &ContextOverflowConfig{
			Enabled:      true,
			Strategy:     "tfidf",
			SafetyMargin: 0.15,
		},
	})

	reduced, wasReduced, err := engine.PreemptiveReduceContext("Summarize the key findings", largeContext, 0)
	if err != nil {
		t.Fatalf("preemptive reduction failed: %v", err)
	}

	// gpt-4o-mini has 128k limit, so 35k should NOT trigger reduction
	if wasReduced {
		t.Logf("context was unexpectedly reduced for 35k input with 128k model limit")
	} else {
		t.Logf("correctly skipped reduction: 35k tokens fits within gpt-4o-mini's 128k limit")
	}

	// Force a smaller model limit to ensure reduction triggers
	engine2 := New("gpt-4", Config{
		MaxDepth:      5,
		MaxIterations: 10,
		ContextOverflow: &ContextOverflowConfig{
			Enabled:          true,
			Strategy:         "tfidf",
			SafetyMargin:     0.15,
			MaxModelTokens:   16000, // Force small limit
		},
	})

	reduced2, wasReduced2, err := engine2.PreemptiveReduceContext("Summarize the key findings", largeContext, 0)
	if err != nil {
		t.Fatalf("preemptive reduction failed: %v", err)
	}

	if !wasReduced2 {
		t.Error("expected context to be reduced when model limit is 16k and context is 35k tokens")
	}

	reducedTokens := EstimateTokens(reduced2)
	t.Logf("Preemptive TF-IDF: %d -> %d estimated tokens (%.1f%% reduction)",
		originalTokens, reducedTokens,
		(1.0-float64(reducedTokens)/float64(originalTokens))*100)

	if reducedTokens >= originalTokens {
		t.Errorf("preemptive reduction failed: original=%d, reduced=%d", originalTokens, reducedTokens)
	}

	_ = reduced // used above
}

func TestTokenEfficiency_AllStrategiesCompared(t *testing.T) {
	// Generate a 40k token context (well over 32k limit)
	largeContext := generateLargeContext(40000)
	originalTokens := EstimateTokens(largeContext)

	if originalTokens < 35000 {
		t.Fatalf("generated context is too small: %d tokens, need at least 35000", originalTokens)
	}

	modelLimit := 32768
	overhead := 1000

	t.Logf("Original context: %d chars, ~%d estimated tokens", len(largeContext), originalTokens)
	t.Logf("Model limit: %d tokens, overhead: %d, available: %d", modelLimit, overhead, modelLimit-overhead)

	// Track results for each strategy
	type strategyResult struct {
		name          string
		reducedTokens int
		reductionPct  float64
		requiresLLM   bool
	}
	var results []strategyResult

	availableTokens := modelLimit - overhead

	// TF-IDF (pure algorithmic)
	tfidfResult := CompressContextTFIDF(largeContext, availableTokens)
	tfidfTokens := EstimateTokens(tfidfResult)
	results = append(results, strategyResult{
		name:          "tfidf",
		reducedTokens: tfidfTokens,
		reductionPct:  (1.0 - float64(tfidfTokens)/float64(originalTokens)) * 100,
		requiresLLM:   false,
	})

	// TextRank (pure algorithmic)
	textRankResult := CompressContextTextRank(largeContext, availableTokens)
	textRankTokens := EstimateTokens(textRankResult)
	results = append(results, strategyResult{
		name:          "textrank",
		reducedTokens: textRankTokens,
		reductionPct:  (1.0 - float64(textRankTokens)/float64(originalTokens)) * 100,
		requiresLLM:   false,
	})

	// Truncation
	engine := New("test-model", Config{
		MaxDepth:      5,
		MaxIterations: 10,
		ContextOverflow: &ContextOverflowConfig{
			Enabled:      true,
			Strategy:     "truncate",
			SafetyMargin: 0.15,
		},
	})
	reducer := newContextReducer(engine, *engine.contextOverflow, NewNoopObserver())
	truncResult, _ := reducer.reduceByTruncation(largeContext, modelLimit, overhead)
	truncTokens := EstimateTokens(truncResult)
	results = append(results, strategyResult{
		name:          "truncate",
		reducedTokens: truncTokens,
		reductionPct:  (1.0 - float64(truncTokens)/float64(originalTokens)) * 100,
		requiresLLM:   false,
	})

	// Print comparison table
	t.Logf("\n--- Token Efficiency Comparison ---")
	t.Logf("%-12s | %12s | %10s | %s", "Strategy", "Tokens Used", "Reduction", "Requires LLM")
	t.Logf("%-12s | %12s | %10s | %s", "------------", "------------", "----------", "------------")
	t.Logf("%-12s | %12d | %9s | %s", "raw (none)", originalTokens, "0.0%", "no")
	for _, r := range results {
		llmStr := "no"
		if r.requiresLLM {
			llmStr = "yes"
		}
		t.Logf("%-12s | %12d | %9.1f%% | %s", r.name, r.reducedTokens, r.reductionPct, llmStr)
	}

	// Assert ALL strategies use fewer tokens than raw
	for _, r := range results {
		if r.reducedTokens >= originalTokens {
			t.Errorf("strategy %q failed: %d tokens >= original %d tokens", r.name, r.reducedTokens, originalTokens)
		}
	}

	// Assert all strategies fit within the model limit
	for _, r := range results {
		if r.reducedTokens > availableTokens {
			t.Errorf("strategy %q exceeds budget: %d tokens > %d available", r.name, r.reducedTokens, availableTokens)
		}
	}
}

func TestTokenEfficiency_VeryLargeContext_100kTokens(t *testing.T) {
	// Test with a very large context (~100k tokens) to prove scaling
	largeContext := generateLargeContext(100000)
	originalTokens := EstimateTokens(largeContext)

	if originalTokens < 90000 {
		t.Fatalf("generated context is too small: %d tokens, need at least 90000", originalTokens)
	}

	modelLimit := 32768
	overhead := 1000
	availableTokens := modelLimit - overhead

	t.Logf("Original: ~%d estimated tokens (3x over 32k limit)", originalTokens)

	// TF-IDF
	tfidfResult := CompressContextTFIDF(largeContext, availableTokens)
	tfidfTokens := EstimateTokens(tfidfResult)

	// TextRank
	textRankResult := CompressContextTextRank(largeContext, availableTokens)
	textRankTokens := EstimateTokens(textRankResult)

	t.Logf("TF-IDF:    %d tokens (%.1f%% reduction)", tfidfTokens, (1.0-float64(tfidfTokens)/float64(originalTokens))*100)
	t.Logf("TextRank:  %d tokens (%.1f%% reduction)", textRankTokens, (1.0-float64(textRankTokens)/float64(originalTokens))*100)

	// Both must be significantly smaller
	if tfidfTokens >= originalTokens/2 {
		t.Errorf("TF-IDF should reduce 100k context by at least 50%%: got %d tokens", tfidfTokens)
	}
	if textRankTokens >= originalTokens/2 {
		t.Errorf("TextRank should reduce 100k context by at least 50%%: got %d tokens", textRankTokens)
	}

	// Both must fit within budget
	if tfidfTokens > availableTokens {
		t.Errorf("TF-IDF exceeds budget: %d > %d", tfidfTokens, availableTokens)
	}
	if textRankTokens > availableTokens {
		t.Errorf("TextRank exceeds budget: %d > %d", textRankTokens, availableTokens)
	}
}

func TestTokenEfficiency_MapReduceTracksTokens(t *testing.T) {
	// Test that mapreduce strategy properly accumulates token usage from multiple chunks
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Simulate summarization - return a short summary for each chunk
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": fmt.Sprintf("Summary of chunk %d: key finding was performance improvement.", callCount)}},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     500 + callCount*50,
				"completion_tokens": 30,
				"total_tokens":      530 + callCount*50,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := New("test-model", Config{
		APIBase:       server.URL,
		MaxDepth:      5,
		MaxIterations: 10,
		ContextOverflow: &ContextOverflowConfig{
			Enabled:      true,
			Strategy:     "mapreduce",
			SafetyMargin: 0.15,
		},
	})

	// Create a large context that will be split into multiple chunks
	largeContext := generateLargeContext(40000)
	query := "Summarize the key findings"

	reducer := newContextReducer(engine, *engine.contextOverflow, NewNoopObserver())
	reduced, err := reducer.ReduceForCompletion(query, largeContext, 16000)
	if err != nil {
		t.Fatalf("mapreduce reduction failed: %v", err)
	}

	// Verify that token usage was accumulated
	if engine.stats.TotalTokens == 0 {
		t.Error("expected total_tokens > 0 after mapreduce reduction, got 0")
	}
	if engine.stats.PromptTokens == 0 {
		t.Error("expected prompt_tokens > 0 after mapreduce reduction, got 0")
	}
	if engine.stats.CompletionTokens == 0 {
		t.Error("expected completion_tokens > 0 after mapreduce reduction, got 0")
	}

	t.Logf("MapReduce token tracking: %d total tokens (%d prompt, %d completion) across %d LLM calls",
		engine.stats.TotalTokens, engine.stats.PromptTokens, engine.stats.CompletionTokens, engine.stats.LlmCalls)
	t.Logf("Reduced context: %d chars", len(reduced))

	// The reduced context should be much smaller than the original
	if len(reduced) >= len(largeContext) {
		t.Errorf("mapreduce failed to reduce context: %d chars >= original %d chars", len(reduced), len(largeContext))
	}
}

func TestTokenEfficiency_StructuredCompletion_TracksTokens(t *testing.T) {
	// Verify structured completion accumulates tokens
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": `{"summary": "test result", "score": 8}`}},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     300,
				"completion_tokens": 15,
				"total_tokens":      315,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine := New("test-model", Config{
		APIBase:       server.URL,
		MaxDepth:      5,
		MaxIterations: 10,
	})

	schema := &StructuredConfig{
		Schema: &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"summary": {Type: "string"},
				"score":   {Type: "number"},
			},
			Required: []string{"summary", "score"},
		},
		MaxRetries: 3,
	}

	result, stats, err := engine.StructuredCompletion("Analyze this", "Some test context", schema)
	if err != nil {
		t.Fatalf("structured completion failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if stats.TotalTokens == 0 {
		t.Error("expected total_tokens > 0 after structured completion, got 0")
	}
	if stats.PromptTokens == 0 {
		t.Error("expected prompt_tokens > 0 after structured completion")
	}
	if stats.CompletionTokens == 0 {
		t.Error("expected completion_tokens > 0 after structured completion")
	}

	t.Logf("Structured completion: %d total tokens (%d prompt, %d completion)", stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens)
}

// ─── Token Estimation Accuracy Tests ─────────────────────────────────────────

func TestEstimateTokens_AccuracyForLargeContent(t *testing.T) {
	// Verify that our estimation stays reasonable for large content
	content := generateLargeContext(32000)
	estimated := EstimateTokens(content)

	// Real tokenizer would give different results, but our estimation should be
	// within a reasonable range. The key property: conservative (over-estimates slightly)
	charToTokenRatio := float64(len(content)) / float64(estimated)

	// Our estimator uses 3.5 chars/token, so ratio should be ~3.5
	if math.Abs(charToTokenRatio-3.5) > 0.5 {
		t.Errorf("char-to-token ratio %.2f deviates too far from expected ~3.5", charToTokenRatio)
	}

	t.Logf("Large content: %d chars, %d estimated tokens, ratio: %.2f chars/token",
		len(content), estimated, charToTokenRatio)
}
