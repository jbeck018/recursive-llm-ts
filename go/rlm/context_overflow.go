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

// ReduceForCompletion handles context overflow for a regular completion.
// It chunks the context, summarizes each chunk, and combines the summaries.
func (cr *contextReducer) ReduceForCompletion(query string, context string, modelLimit int) (string, error) {
	cr.obs.Debug("overflow", "Starting MapReduce context reduction: %d estimated tokens, limit %d", EstimateTokens(context), modelLimit)

	// Calculate safe token budget per chunk
	// Reserve tokens for: system prompt (~500), query, overhead, safety margin
	queryTokens := EstimateTokens(query)
	overhead := 500 + queryTokens + int(float64(modelLimit)*cr.config.SafetyMargin)
	safeTokensPerChunk := modelLimit - overhead

	if safeTokensPerChunk <= 0 {
		safeTokensPerChunk = modelLimit / 2
	}

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
				ExtraParams: cr.rlm.extraParams,
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

	// Check for errors
	var mapErrors []string
	for _, err := range errs {
		if err != nil {
			mapErrors = append(mapErrors, err.Error())
		}
	}
	if len(mapErrors) > 0 {
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
				ExtraParams: cr.rlm.extraParams,
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
	for _, err := range errs {
		if err != nil {
			extractErrors = append(extractErrors, err.Error())
		}
	}
	if len(extractErrors) > 0 {
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
		ExtraParams: cr.rlm.extraParams,
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
			ExtraParams: cr.rlm.extraParams,
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

// ─── Adaptive Completion with Overflow Recovery ──────────────────────────────

// completionWithOverflowRecovery wraps a completion call with automatic overflow detection and retry.
// When a context overflow error is detected, it reduces the context and retries.
func (r *RLM) completionWithOverflowRecovery(query string, context string, overflowConfig ContextOverflowConfig) (string, RLMStats, error) {
	obs := r.observer
	if obs == nil {
		obs = NewNoopObserver()
	}

	// Try the normal completion first
	result, stats, err := r.Completion(query, context)
	if err == nil {
		return result, stats, nil
	}

	// Check if it's a context overflow error
	coe, isOverflow := IsContextOverflow(err)
	if !isOverflow {
		return "", stats, err // Not an overflow error, return original error
	}

	obs.Debug("overflow", "Context overflow detected: model limit %d, request %d tokens (%.1f%% over)",
		coe.ModelLimit, coe.RequestTokens, (coe.OverflowRatio()-1)*100)

	// Use detected limit or configured limit
	modelLimit := coe.ModelLimit
	if overflowConfig.MaxModelTokens > 0 {
		modelLimit = overflowConfig.MaxModelTokens
	}

	reducer := newContextReducer(r, overflowConfig, obs)

	// Attempt context reduction and retry
	for attempt := 0; attempt < overflowConfig.MaxReductionAttempts; attempt++ {
		obs.Debug("overflow", "Reduction attempt %d/%d", attempt+1, overflowConfig.MaxReductionAttempts)

		reducedContext, reduceErr := reducer.ReduceForCompletion(query, context, modelLimit)
		if reduceErr != nil {
			obs.Error("overflow", "Context reduction failed: %v", reduceErr)
			return "", stats, fmt.Errorf("context overflow recovery failed: %w", reduceErr)
		}

		obs.Debug("overflow", "Context reduced: %d -> %d chars", len(context), len(reducedContext))

		// Retry with reduced context
		result, stats, err = r.Completion(query, reducedContext)
		if err == nil {
			obs.Event("overflow.recovery_success", map[string]string{
				"attempt":          fmt.Sprintf("%d", attempt+1),
				"original_chars":   fmt.Sprintf("%d", len(context)),
				"reduced_chars":    fmt.Sprintf("%d", len(reducedContext)),
				"reduction_ratio":  fmt.Sprintf("%.2f", float64(len(reducedContext))/float64(len(context))),
			})
			return result, stats, nil
		}

		// If it overflows again, use the reduced context for the next attempt
		if _, stillOverflow := IsContextOverflow(err); stillOverflow {
			context = reducedContext
			continue
		}

		// Different error, return it
		return "", stats, err
	}

	return "", stats, fmt.Errorf("context overflow: exceeded %d reduction attempts, model limit is %d tokens", overflowConfig.MaxReductionAttempts, modelLimit)
}
