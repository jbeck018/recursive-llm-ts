package rlm

import (
	"fmt"
	"strings"
	"sync"
)

// ContextOverflowConfig configures automatic context overflow handling.
type ContextOverflowConfig struct {
	// Enabled turns on automatic overflow detection and recovery (default: true when config present)
	Enabled bool `json:"enabled"`
	// MaxModelTokens overrides the detected model token limit (0 = auto-detect from API errors)
	MaxModelTokens int `json:"max_model_tokens,omitempty"`
	// Strategy for reducing context: "mapreduce" (default), "truncate", "chunked", "tfidf", "textrank", "refine"
	Strategy string `json:"strategy,omitempty"`
	// SafetyMargin is the fraction of token budget to reserve for prompts/overhead (default: 0.15)
	SafetyMargin float64 `json:"safety_margin,omitempty"`
	// MaxReductionAttempts is how many times to retry with smaller context (default: 3)
	MaxReductionAttempts int `json:"max_reduction_attempts,omitempty"`
}

// DefaultContextOverflowConfig returns sensible defaults for overflow handling.
func DefaultContextOverflowConfig() ContextOverflowConfig {
	return ContextOverflowConfig{
		Enabled:              true,
		Strategy:             "mapreduce",
		SafetyMargin:         0.15,
		MaxReductionAttempts: 3,
	}
}

// ─── Model Token Limits ──────────────────────────────────────────────────────

// modelTokenLimits maps known model name patterns to their maximum context window sizes.
// Used for pre-emptive overflow detection so we don't need to wait for API errors.
var modelTokenLimits = map[string]int{
	// OpenAI
	"gpt-4o":            128000,
	"gpt-4o-mini":       128000,
	"gpt-4-turbo":       128000,
	"gpt-4":             8192,
	"gpt-4-32k":         32768,
	"gpt-3.5-turbo":     16385,
	"gpt-3.5-turbo-16k": 16385,
	"o1":                200000,
	"o1-mini":           128000,
	"o1-preview":        128000,
	"o3-mini":           200000,
	// Anthropic (via LiteLLM/proxy)
	"claude-3-opus":       200000,
	"claude-3-sonnet":     200000,
	"claude-3-haiku":      200000,
	"claude-3.5-sonnet":   200000,
	"claude-3.5-haiku":    200000,
	"claude-sonnet-4":     200000,
	"claude-opus-4":       200000,
	// Llama (common vLLM deployments)
	"llama-3":     8192,
	"llama-3.1":   128000,
	"llama-3.2":   128000,
	"llama-3.3":   128000,
	// Mistral
	"mistral-7b":    32768,
	"mixtral-8x7b":  32768,
	"mistral-large": 128000,
	"mistral-small": 128000,
	// Qwen
	"qwen-2":   32768,
	"qwen-2.5": 128000,
}

// LookupModelTokenLimit returns the known token limit for a model, or 0 if unknown.
// Matches by prefix so "gpt-4o-mini-2024-07-18" matches "gpt-4o-mini".
func LookupModelTokenLimit(model string) int {
	lowerModel := strings.ToLower(model)

	// Try exact match first
	if limit, ok := modelTokenLimits[lowerModel]; ok {
		return limit
	}

	// Try prefix matching (longest prefix wins)
	bestMatch := ""
	bestLimit := 0
	for pattern, limit := range modelTokenLimits {
		if strings.HasPrefix(lowerModel, pattern) && len(pattern) > len(bestMatch) {
			bestMatch = pattern
			bestLimit = limit
		}
	}

	return bestLimit
}

// getModelTokenLimit returns the effective token limit for pre-emptive overflow checks.
// Priority: config override > model name lookup > 0 (disabled).
func (r *RLM) getModelTokenLimit() int {
	if r.contextOverflow != nil && r.contextOverflow.MaxModelTokens > 0 {
		return r.contextOverflow.MaxModelTokens
	}
	return LookupModelTokenLimit(r.model)
}

// ─── Pre-emptive Overflow Check ──────────────────────────────────────────────

// structuredPromptOverhead is the approximate token overhead for structured completion prompts
// (instructions, schema constraints, JSON formatting directives).
const structuredPromptOverhead = 350

// PreemptiveReduceContext checks if the context would overflow the model's token limit
// and reduces it proactively BEFORE building the prompt. Returns the (possibly reduced)
// context, or an error if reduction fails.
//
// This is called before the first LLM call, unlike post-hoc overflow recovery which
// only triggers after an API error. Following the RLM paper's principle that
// "the context window of the root LM is rarely clogged."
func (r *RLM) PreemptiveReduceContext(query string, context string, extraOverhead int) (string, bool, error) {
	modelLimit := r.getModelTokenLimit()
	if modelLimit == 0 {
		// No known limit; skip pre-emptive check (will rely on post-hoc recovery)
		return context, false, nil
	}

	if r.contextOverflow == nil || !r.contextOverflow.Enabled {
		return context, false, nil
	}

	// Estimate total token budget needed
	contextTokens := EstimateTokens(context)
	queryTokens := EstimateTokens(query)
	responseTokens := r.getResponseTokenBudget()
	safetyMargin := r.contextOverflow.SafetyMargin
	if safetyMargin == 0 {
		safetyMargin = 0.15
	}

	totalEstimate := contextTokens + queryTokens + extraOverhead + responseTokens +
		int(float64(modelLimit)*safetyMargin)

	r.observer.Debug("overflow", "Pre-emptive check: context=%d query=%d overhead=%d response=%d safety=%d total=%d limit=%d",
		contextTokens, queryTokens, extraOverhead, responseTokens,
		int(float64(modelLimit)*safetyMargin), totalEstimate, modelLimit)

	if totalEstimate <= modelLimit {
		return context, false, nil
	}

	// Context would overflow — reduce it proactively
	r.observer.Debug("overflow", "Pre-emptive reduction needed: estimated %d tokens > limit %d", totalEstimate, modelLimit)

	reducer := newContextReducer(r, *r.contextOverflow, r.observer)
	reduced, err := reducer.ReduceForCompletion(query, context, modelLimit)
	if err != nil {
		return context, false, fmt.Errorf("pre-emptive context reduction failed: %w", err)
	}

	r.observer.Debug("overflow", "Pre-emptive reduction: %d -> %d chars", len(context), len(reduced))
	return reduced, true, nil
}

// getResponseTokenBudget extracts max_tokens or max_completion_tokens from ExtraParams.
func (r *RLM) getResponseTokenBudget() int {
	if r.extraParams == nil {
		return 0
	}
	for _, key := range []string{"max_completion_tokens", "max_tokens"} {
		if v, ok := r.extraParams[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			case int64:
				return int(n)
			}
		}
	}
	return 0
}

// ─── Token Estimation ────────────────────────────────────────────────────────

// EstimateTokens provides a fast approximation of token count for a string.
// Uses a character-to-token ratio heuristic. This is intentionally conservative
// (over-estimates slightly) to avoid overflow.
//
// Approximate ratios for common encodings:
//   - English text: ~4 chars/token (cl100k_base)
//   - JSON/code:    ~3.5 chars/token
//   - CJK text:     ~1.5 chars/token
//   - Mixed:        ~3.5 chars/token (safe default)
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Use 3.5 chars/token as conservative estimate
	return (len(text)*10 + 34) / 35 // equivalent to ceil(len/3.5)
}

// EstimateMessagesTokens estimates the total tokens for a set of chat messages.
// Includes per-message overhead (~4 tokens per message for role + formatting).
func EstimateMessagesTokens(messages []Message) int {
	total := 3 // Every reply is primed with <|im_start|>assistant<|im_sep|>
	for _, msg := range messages {
		total += 4 // role + formatting overhead
		total += EstimateTokens(msg.Content)
	}
	return total
}

// ─── Context Chunking ────────────────────────────────────────────────────────

// ChunkContext splits context into chunks that fit within a token budget.
// Uses paragraph/sentence boundaries when possible, with overlap for context continuity.
func ChunkContext(context string, maxTokensPerChunk int) []string {
	if maxTokensPerChunk <= 0 {
		maxTokensPerChunk = 4000
	}

	// Estimate max chars per chunk (slightly conservative)
	maxCharsPerChunk := maxTokensPerChunk * 3 // Use 3 chars/token to leave room

	if len(context) <= maxCharsPerChunk {
		return []string{context}
	}

	var chunks []string
	overlapChars := maxCharsPerChunk / 10 // 10% overlap for context continuity

	pos := 0
	for pos < len(context) {
		end := pos + maxCharsPerChunk
		if end >= len(context) {
			chunks = append(chunks, context[pos:])
			break
		}

		// Try to find a good break point (paragraph boundary first, then sentence, then word)
		breakPoint := findBreakPoint(context, pos, end)
		chunks = append(chunks, context[pos:breakPoint])

		// Move position back by overlap amount to maintain context continuity
		pos = breakPoint - overlapChars
		if pos < 0 {
			pos = 0
		}
		// Ensure we make forward progress
		if pos <= (breakPoint - maxCharsPerChunk) {
			pos = breakPoint
		}
	}

	return chunks
}

// findBreakPoint finds the best position to split text near the target end position.
// Prefers paragraph breaks (\n\n), then line breaks (\n), then sentence ends (. ! ?), then word breaks.
func findBreakPoint(text string, start int, targetEnd int) int {
	if targetEnd >= len(text) {
		return len(text)
	}

	// Search window: look back from targetEnd up to 20% of the chunk
	searchStart := targetEnd - (targetEnd-start)/5
	if searchStart < start {
		searchStart = start
	}

	searchRegion := text[searchStart:targetEnd]

	// Try paragraph break first
	if idx := strings.LastIndex(searchRegion, "\n\n"); idx >= 0 {
		return searchStart + idx + 2
	}

	// Try line break
	if idx := strings.LastIndex(searchRegion, "\n"); idx >= 0 {
		return searchStart + idx + 1
	}

	// Try sentence end
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.LastIndex(searchRegion, sep); idx >= 0 {
			return searchStart + idx + len(sep)
		}
	}

	// Try word break
	if idx := strings.LastIndex(searchRegion, " "); idx >= 0 {
		return searchStart + idx + 1
	}

	// No good break point, just split at target
	return targetEnd
}

// ─── MapReduce Context Reduction ─────────────────────────────────────────────

// MapReduceResult holds the result of a MapReduce context reduction
type MapReduceResult struct {
	ReducedContext string
	ChunkCount     int
	OriginalTokens int
	ReducedTokens  int
}

// contextReducer manages context reduction for overflow recovery
type contextReducer struct {
	rlm    *RLM
	config ContextOverflowConfig
	obs    *Observer
}

// newContextReducer creates a reducer bound to an RLM engine
func newContextReducer(rlm *RLM, config ContextOverflowConfig, obs *Observer) *contextReducer {
	return &contextReducer{rlm: rlm, config: config, obs: obs}
}

// getResponseTokenBudget delegates to the RLM engine's method.
func (cr *contextReducer) getResponseTokenBudget() int {
	return cr.rlm.getResponseTokenBudget()
}

// makeMapPhaseParams creates ExtraParams suitable for map-phase LLM calls (summarization).
// It copies the user's ExtraParams but overrides max_tokens to a smaller value since
// summaries don't need as many tokens as the original completion.
func (cr *contextReducer) makeMapPhaseParams(modelLimit int) map[string]interface{} {
	params := make(map[string]interface{})
	// Copy all user params (custom_llm_provider, temperature, etc.)
	for k, v := range cr.rlm.extraParams {
		params[k] = v
	}
	// Override max_tokens for map-phase: use at most 1/4 of model limit or 2000, whichever is smaller
	mapMaxTokens := modelLimit / 4
	if mapMaxTokens > 2000 {
		mapMaxTokens = 2000
	}
	if mapMaxTokens < 256 {
		mapMaxTokens = 256
	}
	params["max_tokens"] = mapMaxTokens
	// Remove max_completion_tokens if present to avoid conflicts
	delete(params, "max_completion_tokens")
	return params
}

// ReduceForCompletion handles context overflow for a regular completion.
// It chunks the context, summarizes each chunk, and combines the summaries.
func (cr *contextReducer) ReduceForCompletion(query string, context string, modelLimit int) (string, error) {
	cr.obs.Debug("overflow", "Starting context reduction: %d estimated tokens, limit %d", EstimateTokens(context), modelLimit)

	// Calculate safe token budget per chunk
	// Reserve tokens for: system prompt (~500), query, overhead, safety margin, response budget
	queryTokens := EstimateTokens(query)
	responseTokens := cr.getResponseTokenBudget()
	overhead := 500 + queryTokens + int(float64(modelLimit)*cr.config.SafetyMargin) + responseTokens
	safeTokensPerChunk := modelLimit - overhead

	if safeTokensPerChunk <= 0 {
		safeTokensPerChunk = modelLimit / 4
	}

	cr.obs.Debug("overflow", "Budget: overhead=%d (query=%d, response=%d, safety=%d), chunk budget=%d",
		overhead, queryTokens, responseTokens, int(float64(modelLimit)*cr.config.SafetyMargin), safeTokensPerChunk)

	chunks := ChunkContext(context, safeTokensPerChunk)
	cr.obs.Debug("overflow", "Split context into %d chunks (budget: %d tokens/chunk)", len(chunks), safeTokensPerChunk)

	if len(chunks) == 1 {
		// Context is already small enough (or couldn't be meaningfully split)
		return context, nil
	}

	switch cr.config.Strategy {
	case "truncate":
		return cr.reduceByTruncation(context, modelLimit, overhead)
	case "chunked":
		return cr.reduceByChunkedExtraction(query, chunks, modelLimit, overhead)
	case "tfidf":
		return cr.reduceByTFIDF(context, modelLimit, overhead)
	case "textrank":
		return cr.reduceByTextRank(context, modelLimit, overhead)
	case "refine":
		return cr.reduceByRefine(query, chunks, modelLimit, overhead)
	default: // "mapreduce"
		return cr.reduceByMapReduce(query, chunks, modelLimit, overhead)
	}
}

// reduceByMapReduce summarizes each chunk and combines the summaries.
func (cr *contextReducer) reduceByMapReduce(query string, chunks []string, modelLimit int, overhead int) (string, error) {
	cr.obs.Debug("overflow", "Using MapReduce strategy with %d chunks", len(chunks))

	// Use map-phase-specific params with reduced max_tokens for summarization
	mapPhaseParams := cr.makeMapPhaseParams(modelLimit)

	summaries := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	var wg sync.WaitGroup

	// Map phase: summarize each chunk in parallel
	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, chunkText string) {
			defer wg.Done()

			mapPrompt := fmt.Sprintf(
				"Summarize the following text chunk, preserving all key facts, data points, names, numbers, and specific details that would be needed to answer the question: %q\n\n"+
					"IMPORTANT: Be thorough and retain specific data. Do not omit numbers, percentages, dates, or named entities.\n\n"+
					"Text chunk (%d of %d):\n%s",
				query, idx+1, len(chunks), chunkText,
			)

			messages := []Message{
				{Role: "system", Content: "You are a precise summarization assistant. Preserve all factual details, data points, and specific information."},
				{Role: "user", Content: mapPrompt},
			}

			result, err := CallChatCompletion(ChatRequest{
				Model:       cr.rlm.model,
				Messages:    messages,
				APIBase:     cr.rlm.apiBase,
				APIKey:      cr.rlm.apiKey,
				Timeout:     cr.rlm.timeoutSeconds,
				ExtraParams: mapPhaseParams,
			})
			if err != nil {
				errs[idx] = fmt.Errorf("map phase chunk %d: %w", idx+1, err)
				return
			}

			cr.rlm.stats.LlmCalls++
			summaries[idx] = result
			cr.obs.Debug("overflow", "Chunk %d/%d summarized: %d -> %d chars", idx+1, len(chunks), len(chunkText), len(result))
		}(i, chunk)
	}

	wg.Wait()

	// Check for errors - if map phase overflows, fall back to tfidf
	var mapErrors []string
	hasOverflow := false
	for _, err := range errs {
		if err != nil {
			mapErrors = append(mapErrors, err.Error())
			if _, isOverflow := IsContextOverflow(err); isOverflow {
				hasOverflow = true
			}
		}
	}
	if len(mapErrors) > 0 {
		if hasOverflow {
			cr.obs.Debug("overflow", "MapReduce map phase hit overflow, falling back to TF-IDF strategy")
			return cr.reduceByTFIDF(strings.Join(chunks, "\n\n"), modelLimit, overhead)
		}
		return "", fmt.Errorf("MapReduce map phase failed: %s", strings.Join(mapErrors, "; "))
	}

	// Reduce phase: combine summaries
	combined := strings.Join(summaries, "\n\n---\n\n")

	// Check if combined summaries fit in the budget
	if EstimateTokens(combined)+overhead < modelLimit {
		cr.obs.Debug("overflow", "MapReduce complete: %d -> %d estimated tokens", EstimateTokens(strings.Join(chunks, "")), EstimateTokens(combined))
		return combined, nil
	}

	// If summaries are still too large, recursively reduce
	cr.obs.Debug("overflow", "Combined summaries still too large (%d tokens), reducing recursively", EstimateTokens(combined))
	return cr.ReduceForCompletion(query, combined, modelLimit)
}

// reduceByTruncation simply truncates context to fit within the limit.
func (cr *contextReducer) reduceByTruncation(context string, modelLimit int, overhead int) (string, error) {
	cr.obs.Debug("overflow", "Using truncation strategy")

	availableTokens := modelLimit - overhead
	maxChars := availableTokens * 3 // Conservative chars-to-tokens

	if maxChars >= len(context) {
		return context, nil
	}

	// Keep beginning and end, truncate middle (addresses "lost in the middle" problem)
	keepFromStart := maxChars * 2 / 3
	keepFromEnd := maxChars / 3

	truncated := context[:keepFromStart] +
		"\n\n[... context truncated due to token limit ...]\n\n" +
		context[len(context)-keepFromEnd:]

	cr.obs.Debug("overflow", "Truncated context: %d -> %d chars", len(context), len(truncated))
	return truncated, nil
}

// reduceByChunkedExtraction processes each chunk independently and returns all extracted content.
func (cr *contextReducer) reduceByChunkedExtraction(query string, chunks []string, modelLimit int, overhead int) (string, error) {
	cr.obs.Debug("overflow", "Using chunked extraction strategy with %d chunks", len(chunks))

	// Use map-phase-specific params with reduced max_tokens
	mapPhaseParams := cr.makeMapPhaseParams(modelLimit)

	results := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, chunkText string) {
			defer wg.Done()

			extractPrompt := fmt.Sprintf(
				"Extract all information relevant to the following question from this text chunk. "+
					"Include specific data, facts, quotes, and details. If nothing relevant is found, respond with 'NO_RELEVANT_CONTENT'.\n\n"+
					"Question: %s\n\nText chunk (%d of %d):\n%s",
				query, idx+1, len(chunks), chunkText,
			)

			messages := []Message{
				{Role: "system", Content: "You are a precise information extraction assistant. Extract only relevant information."},
				{Role: "user", Content: extractPrompt},
			}

			result, err := CallChatCompletion(ChatRequest{
				Model:       cr.rlm.model,
				Messages:    messages,
				APIBase:     cr.rlm.apiBase,
				APIKey:      cr.rlm.apiKey,
				Timeout:     cr.rlm.timeoutSeconds,
				ExtraParams: mapPhaseParams,
			})
			if err != nil {
				errs[idx] = fmt.Errorf("chunked extraction chunk %d: %w", idx+1, err)
				return
			}

			cr.rlm.stats.LlmCalls++
			if strings.TrimSpace(result) != "NO_RELEVANT_CONTENT" {
				results[idx] = result
			}
		}(i, chunk)
	}

	wg.Wait()

	var extractErrors []string
	hasOverflow := false
	for _, err := range errs {
		if err != nil {
			extractErrors = append(extractErrors, err.Error())
			if _, isOverflow := IsContextOverflow(err); isOverflow {
				hasOverflow = true
			}
		}
	}
	if len(extractErrors) > 0 {
		if hasOverflow {
			cr.obs.Debug("overflow", "Chunked extraction hit overflow, falling back to TF-IDF strategy")
			return cr.reduceByTFIDF(strings.Join(chunks, "\n\n"), modelLimit, overhead)
		}
		return "", fmt.Errorf("chunked extraction failed: %s", strings.Join(extractErrors, "; "))
	}

	// Combine non-empty results
	var parts []string
	for _, r := range results {
		if r != "" {
			parts = append(parts, r)
		}
	}

	if len(parts) == 0 {
		return "No relevant content found across all chunks.", nil
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// ─── Refine Sequential Strategy ─────────────────────────────────────────────

// reduceByRefine processes chunks sequentially, where the first chunk generates
// an initial answer and each subsequent chunk refines it. This approach has the
// highest information fidelity because every chunk is processed in context of
// the cumulative answer, but is sequential (not parallelizable).
func (cr *contextReducer) reduceByRefine(query string, chunks []string, modelLimit int, overhead int) (string, error) {
	cr.obs.Debug("overflow", "Using refine strategy with %d chunks", len(chunks))

	if len(chunks) == 0 {
		return "", fmt.Errorf("refine strategy: no chunks to process")
	}

	// Use map-phase-specific params with reduced max_tokens
	mapPhaseParams := cr.makeMapPhaseParams(modelLimit)

	// Phase 1: Generate initial answer from the first chunk
	initialPrompt := fmt.Sprintf(
		"Using the following context, provide a comprehensive answer to the question.\n"+
			"Preserve all key facts, data points, names, numbers, and specific details.\n\n"+
			"Question: %s\n\nContext:\n%s",
		query, chunks[0],
	)

	messages := []Message{
		{Role: "system", Content: "You are a precise information synthesis assistant. Preserve all factual details and specific data points."},
		{Role: "user", Content: initialPrompt},
	}

	currentAnswer, err := CallChatCompletion(ChatRequest{
		Model:       cr.rlm.model,
		Messages:    messages,
		APIBase:     cr.rlm.apiBase,
		APIKey:      cr.rlm.apiKey,
		Timeout:     cr.rlm.timeoutSeconds,
		ExtraParams: mapPhaseParams,
	})
	if err != nil {
		return "", fmt.Errorf("refine initial chunk: %w", err)
	}
	cr.rlm.stats.LlmCalls++
	cr.obs.Debug("overflow", "Refine: initial answer from chunk 1/%d (%d chars)", len(chunks), len(currentAnswer))

	// Phase 2: Refine the answer with each subsequent chunk
	for i := 1; i < len(chunks); i++ {
		refinePrompt := fmt.Sprintf(
			"You have an existing answer to the question: %q\n\n"+
				"Existing answer:\n%s\n\n"+
				"Now you have additional context that may contain new information, corrections, or supporting details.\n"+
				"Refine the existing answer by incorporating any relevant new information from this context.\n"+
				"If this context adds nothing new, return the existing answer unchanged.\n"+
				"IMPORTANT: Never remove information from the existing answer unless it is contradicted by the new context.\n\n"+
				"Additional context (chunk %d of %d):\n%s",
			query, currentAnswer, i+1, len(chunks), chunks[i],
		)

		messages := []Message{
			{Role: "system", Content: "You are a precise information synthesis assistant. Refine answers by incorporating new context without losing existing information."},
			{Role: "user", Content: refinePrompt},
		}

		refined, err := CallChatCompletion(ChatRequest{
			Model:       cr.rlm.model,
			Messages:    messages,
			APIBase:     cr.rlm.apiBase,
			APIKey:      cr.rlm.apiKey,
			Timeout:     cr.rlm.timeoutSeconds,
			ExtraParams: mapPhaseParams,
		})
		if err != nil {
			cr.obs.Debug("overflow", "Refine: chunk %d/%d failed: %v, keeping current answer", i+1, len(chunks), err)
			// On error, keep current answer rather than failing entirely
			continue
		}
		cr.rlm.stats.LlmCalls++
		currentAnswer = refined
		cr.obs.Debug("overflow", "Refine: incorporated chunk %d/%d (%d chars)", i+1, len(chunks), len(currentAnswer))
	}

	// Verify the refined answer fits within budget
	if EstimateTokens(currentAnswer)+overhead < modelLimit {
		cr.obs.Debug("overflow", "Refine complete: answer is %d estimated tokens", EstimateTokens(currentAnswer))
		return currentAnswer, nil
	}

	// If the refined answer is still too large, truncate it
	cr.obs.Debug("overflow", "Refine answer too large (%d tokens), truncating", EstimateTokens(currentAnswer))
	return cr.reduceByTruncation(currentAnswer, modelLimit, overhead)
}

// ─── TF-IDF Strategy (wrapper for contextReducer) ───────────────────────────

// reduceByTFIDF uses TF-IDF extractive compression - pure algorithmic, no API calls.
func (cr *contextReducer) reduceByTFIDF(context string, modelLimit int, overhead int) (string, error) {
	cr.obs.Debug("overflow", "Using TF-IDF extractive strategy")

	availableTokens := modelLimit - overhead
	if availableTokens <= 0 {
		availableTokens = modelLimit / 2
	}

	result := CompressContextTFIDF(context, availableTokens)
	cr.obs.Debug("overflow", "TF-IDF compressed: %d -> %d chars (%d -> %d est. tokens)",
		len(context), len(result), EstimateTokens(context), EstimateTokens(result))
	return result, nil
}

// ─── TextRank Strategy (wrapper for contextReducer) ─────────────────────────

// reduceByTextRank uses TextRank graph-based ranking - pure algorithmic, no API calls.
func (cr *contextReducer) reduceByTextRank(context string, modelLimit int, overhead int) (string, error) {
	cr.obs.Debug("overflow", "Using TextRank graph-based strategy")

	availableTokens := modelLimit - overhead
	if availableTokens <= 0 {
		availableTokens = modelLimit / 2
	}

	result := CompressContextTextRank(context, availableTokens)
	cr.obs.Debug("overflow", "TextRank compressed: %d -> %d chars (%d -> %d est. tokens)",
		len(context), len(result), EstimateTokens(context), EstimateTokens(result))
	return result, nil
}

