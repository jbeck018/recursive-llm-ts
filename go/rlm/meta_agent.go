package rlm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MetaAgentConfig configures the meta-agent behavior.
type MetaAgentConfig struct {
	Enabled        bool   `json:"enabled"`
	Model          string `json:"model,omitempty"`           // Model to use for meta-agent (defaults to main model)
	MaxOptimizeLen int    `json:"max_optimize_len,omitempty"` // Max context length before optimization (0 = always optimize)
}

// MetaAgent optimizes queries before passing them to the RLM engine.
// It analyzes raw, non-optimized messages and rewrites them for better
// recursive decomposition and structured extraction.
type MetaAgent struct {
	config  MetaAgentConfig
	rlm     *RLM
	obs     *Observer
}

// NewMetaAgent creates a MetaAgent wrapping an RLM engine.
func NewMetaAgent(rlm *RLM, config MetaAgentConfig, obs *Observer) *MetaAgent {
	if config.Model == "" {
		config.Model = rlm.model
	}
	return &MetaAgent{
		config: config,
		rlm:    rlm,
		obs:    obs,
	}
}

// OptimizeQuery takes a raw query and context, and returns an optimized query
// that is better suited for RLM processing.
func (ma *MetaAgent) OptimizeQuery(query string, context string) (string, error) {
	ctx := ma.obs.StartSpan("meta_agent.optimize_query", map[string]string{
		"query_length":   fmt.Sprintf("%d", len(query)),
		"context_length": fmt.Sprintf("%d", len(context)),
	})
	defer ma.obs.EndSpan(ctx)

	ma.obs.Debug("meta_agent", "Optimizing query: %s", truncateStr(query, 200))

	// Skip optimization for short, already-specific queries
	if !ma.needsOptimization(query, context) {
		ma.obs.Debug("meta_agent", "Query does not need optimization, passing through")
		return query, nil
	}

	optimizePrompt := ma.buildOptimizePrompt(query, context)

	messages := []Message{
		{Role: "system", Content: metaAgentSystemPrompt},
		{Role: "user", Content: optimizePrompt},
	}

	request := ChatRequest{
		Model:       ma.config.Model,
		Messages:    messages,
		APIBase:     ma.rlm.apiBase,
		APIKey:      ma.rlm.apiKey,
		Timeout:     ma.rlm.timeoutSeconds,
		ExtraParams: ma.rlm.extraParams,
	}

	result, err := CallChatCompletion(request)
	if err != nil {
		ma.obs.Error("meta_agent", "Failed to optimize query: %v", err)
		// Fall back to original query on error
		return query, nil
	}

	optimized := strings.TrimSpace(result)
	ma.obs.Debug("meta_agent", "Optimized query: %s", truncateStr(optimized, 200))
	ma.obs.Event("meta_agent.query_optimized", map[string]string{
		"original_length":  fmt.Sprintf("%d", len(query)),
		"optimized_length": fmt.Sprintf("%d", len(optimized)),
	})

	return optimized, nil
}

// OptimizeForStructured takes a raw query and schema, and returns an optimized
// query specifically designed for structured extraction.
func (ma *MetaAgent) OptimizeForStructured(query string, context string, schema *JSONSchema) (string, error) {
	ctx := ma.obs.StartSpan("meta_agent.optimize_structured", map[string]string{
		"query_length":   fmt.Sprintf("%d", len(query)),
		"context_length": fmt.Sprintf("%d", len(context)),
		"schema_type":    schema.Type,
	})
	defer ma.obs.EndSpan(ctx)

	ma.obs.Debug("meta_agent", "Optimizing for structured extraction: %s", truncateStr(query, 200))

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return query, nil
	}

	optimizePrompt := fmt.Sprintf(
		"I need to extract structured data from a document. Please optimize my query "+
			"for better extraction accuracy.\n\n"+
			"Original query: %s\n\n"+
			"Target JSON Schema:\n%s\n\n"+
			"Context preview (first 500 chars):\n%s\n\n"+
			"Please provide an optimized extraction query that:\n"+
			"1. Explicitly references each required field from the schema\n"+
			"2. Provides clear extraction instructions for complex types (arrays, nested objects)\n"+
			"3. Specifies expected formats and constraints\n"+
			"4. Includes hints about where to find the data in the context\n\n"+
			"Return ONLY the optimized query text, nothing else.",
		query, string(schemaJSON), truncateStr(context, 500),
	)

	messages := []Message{
		{Role: "system", Content: metaAgentStructuredPrompt},
		{Role: "user", Content: optimizePrompt},
	}

	request := ChatRequest{
		Model:       ma.config.Model,
		Messages:    messages,
		APIBase:     ma.rlm.apiBase,
		APIKey:      ma.rlm.apiKey,
		Timeout:     ma.rlm.timeoutSeconds,
		ExtraParams: ma.rlm.extraParams,
	}

	result, err := CallChatCompletion(request)
	if err != nil {
		ma.obs.Error("meta_agent", "Failed to optimize structured query: %v", err)
		return query, nil
	}

	optimized := strings.TrimSpace(result)
	ma.obs.Debug("meta_agent", "Optimized structured query: %s", truncateStr(optimized, 200))
	ma.obs.Event("meta_agent.structured_query_optimized", map[string]string{
		"original_length":  fmt.Sprintf("%d", len(query)),
		"optimized_length": fmt.Sprintf("%d", len(optimized)),
	})

	return optimized, nil
}

// needsOptimization determines if a query would benefit from meta-agent optimization.
func (ma *MetaAgent) needsOptimization(query string, context string) bool {
	// Always optimize if MaxOptimizeLen is 0 (default)
	if ma.config.MaxOptimizeLen == 0 {
		return true
	}

	// Optimize if context is large
	if len(context) > ma.config.MaxOptimizeLen {
		return true
	}

	// Skip if query is already very specific (contains keywords suggesting structure)
	specificKeywords := []string{
		"extract", "parse", "find all", "identify",
		"list the", "count the", "summarize",
	}
	queryLower := strings.ToLower(query)
	for _, kw := range specificKeywords {
		if strings.Contains(queryLower, kw) {
			return false
		}
	}

	// Optimize if query is vague or short
	if len(query) < 50 {
		return true
	}

	return true
}

// buildOptimizePrompt creates the prompt for query optimization.
func (ma *MetaAgent) buildOptimizePrompt(query string, context string) string {
	contextPreview := truncateStr(context, 500)

	return fmt.Sprintf(
		"Please optimize the following query for recursive language model processing.\n\n"+
			"Original query: %s\n\n"+
			"Context preview (first 500 chars):\n%s\n\n"+
			"The context has %d total characters.\n\n"+
			"Please provide an optimized version of this query that:\n"+
			"1. Is specific and actionable\n"+
			"2. Breaks down complex questions into clear sub-questions\n"+
			"3. Specifies what format the answer should be in\n"+
			"4. Provides hints about what to look for in the context\n"+
			"5. Includes any relevant constraints or requirements\n\n"+
			"Return ONLY the optimized query text, nothing else.",
		query, contextPreview, len(context),
	)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

const metaAgentSystemPrompt = `You are a query optimization assistant for a Recursive Language Model (RLM) system.
Your job is to take raw, potentially vague queries and rewrite them to be more specific and actionable.

The RLM system processes large documents by:
1. Breaking them into chunks
2. Using JavaScript REPL to search and extract information
3. Making recursive sub-queries for complex analysis
4. Returning a final answer using FINAL()

Your optimized queries should be clear, specific, and structured for this processing pattern.

Rules:
- Return ONLY the optimized query text
- Do not include explanations or metadata
- Keep the core intent of the original query
- Make implicit requirements explicit
- Add format specifications when helpful`

const metaAgentStructuredPrompt = `You are a structured data extraction query optimizer for a Recursive Language Model (RLM) system.
Your job is to optimize queries specifically for extracting structured JSON data from documents.

The RLM system extracts data by:
1. Analyzing the document against a JSON schema
2. Extracting each field individually or in parallel
3. Validating the result against the schema
4. Retrying with feedback if validation fails

Your optimized queries should:
- Explicitly reference required fields from the schema
- Provide clear extraction instructions per field
- Specify data types and constraints
- Include hints about where to find data in the document

Rules:
- Return ONLY the optimized query text
- Do not include explanations or metadata
- Reference schema fields by name
- Include type expectations (string, number, array, etc.)`
