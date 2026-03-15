package rlm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── LLM-Map Operator ──────────────────────────────────────────────────────
// Implements Operator-Level Recursion from the LCM paper (Section 3.1).
// Processes each item in a JSONL file via independent LLM calls with
// schema validation, retry logic, and database-backed execution tracking.

// LLMMapConfig configures an LLM-Map operation.
type LLMMapConfig struct {
	InputPath    string      `json:"input_path"`    // Path to JSONL input file
	OutputPath   string      `json:"output_path"`   // Path to JSONL output file
	Prompt       string      `json:"prompt"`        // Prompt template ({{item}} is replaced)
	OutputSchema *JSONSchema `json:"output_schema"` // Schema for validation
	Concurrency  int         `json:"concurrency"`   // Worker pool size (default: 16)
	MaxRetries   int         `json:"max_retries"`   // Per-item retry limit (default: 3)
	Model        string      `json:"model"`         // Model to use (default: engine model)
}

// LLMMapResult contains the results of an LLM-Map operation.
type LLMMapResult struct {
	TotalItems    int            `json:"total_items"`
	Completed     int            `json:"completed"`
	Failed        int            `json:"failed"`
	OutputPath    string         `json:"output_path"`
	Duration      time.Duration  `json:"duration"`
	TokensUsed    int            `json:"tokens_used"`
	ItemResults   []MapItemResult `json:"item_results,omitempty"`
}

// MapItemResult tracks the status of a single item in the map operation.
type MapItemResult struct {
	Index   int             `json:"index"`
	Status  MapItemStatus   `json:"status"`
	Output  json.RawMessage `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
	Retries int             `json:"retries"`
}

// MapItemStatus represents the execution status of a map item.
type MapItemStatus string

const (
	MapItemPending   MapItemStatus = "pending"
	MapItemRunning   MapItemStatus = "running"
	MapItemCompleted MapItemStatus = "completed"
	MapItemFailed    MapItemStatus = "failed"
)

// LLMMapper executes LLM-Map operations.
type LLMMapper struct {
	model       string
	apiBase     string
	apiKey      string
	timeout     int
	extraParams map[string]interface{}
	observer    *Observer
}

// NewLLMMapper creates a new LLM-Map executor.
func NewLLMMapper(model, apiBase, apiKey string, timeout int, extraParams map[string]interface{}, observer *Observer) *LLMMapper {
	return &LLMMapper{
		model:       model,
		apiBase:     apiBase,
		apiKey:      apiKey,
		timeout:     timeout,
		extraParams: extraParams,
		observer:    observer,
	}
}

// Execute runs an LLM-Map operation: parallel LLM calls over JSONL input.
func (m *LLMMapper) Execute(config LLMMapConfig) (*LLMMapResult, error) {
	start := time.Now()

	// Apply defaults
	if config.Concurrency <= 0 {
		config.Concurrency = 16
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	model := config.Model
	if model == "" {
		model = m.model
	}

	m.observer.Debug("lcm.map", "Starting LLM-Map: input=%s, concurrency=%d, model=%s",
		config.InputPath, config.Concurrency, model)

	// Read input items
	items, err := readJSONLFile(config.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	m.observer.Debug("lcm.map", "Read %d items from %s", len(items), config.InputPath)

	// Initialize results tracking
	results := make([]MapItemResult, len(items))
	for i := range results {
		results[i] = MapItemResult{
			Index:  i,
			Status: MapItemPending,
		}
	}

	// Worker pool
	var wg sync.WaitGroup
	itemChan := make(chan int, len(items))
	var totalTokens int64

	// Feed items to workers
	for i := range items {
		itemChan <- i
	}
	close(itemChan)

	// Spawn workers
	var mu sync.Mutex
	for w := 0; w < config.Concurrency && w < len(items); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range itemChan {
				result := m.processItem(items[idx], config.Prompt, config.OutputSchema, model, config.MaxRetries)
				atomic.AddInt64(&totalTokens, int64(result.tokensUsed))

				mu.Lock()
				results[idx] = MapItemResult{
					Index:   idx,
					Status:  result.status,
					Output:  result.output,
					Error:   result.errMsg,
					Retries: result.retries,
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Write output file
	if config.OutputPath != "" {
		if err := writeJSONLOutput(config.OutputPath, results); err != nil {
			return nil, fmt.Errorf("failed to write output: %w", err)
		}
	}

	// Count results
	completed, failed := 0, 0
	for _, r := range results {
		switch r.Status {
		case MapItemCompleted:
			completed++
		case MapItemFailed:
			failed++
		}
	}

	duration := time.Since(start)
	m.observer.Debug("lcm.map", "LLM-Map complete: %d/%d succeeded, %d failed in %s",
		completed, len(items), failed, duration)
	m.observer.Event("lcm.map_complete", map[string]string{
		"total_items": fmt.Sprintf("%d", len(items)),
		"completed":   fmt.Sprintf("%d", completed),
		"failed":      fmt.Sprintf("%d", failed),
		"duration_ms": fmt.Sprintf("%d", duration.Milliseconds()),
		"tokens_used": fmt.Sprintf("%d", totalTokens),
	})

	return &LLMMapResult{
		TotalItems:  len(items),
		Completed:   completed,
		Failed:      failed,
		OutputPath:  config.OutputPath,
		Duration:    duration,
		TokensUsed:  int(totalTokens),
		ItemResults: results,
	}, nil
}

// ─── Per-Item Processing ────────────────────────────────────────────────────

type itemProcessResult struct {
	status     MapItemStatus
	output     json.RawMessage
	errMsg     string
	retries    int
	tokensUsed int
}

func (m *LLMMapper) processItem(item json.RawMessage, promptTemplate string, schema *JSONSchema, model string, maxRetries int) itemProcessResult {
	// Build prompt by replacing {{item}} placeholder
	prompt := strings.ReplaceAll(promptTemplate, "{{item}}", string(item))

	var lastErr string
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Add validation feedback on retry
		if attempt > 0 && lastErr != "" {
			prompt = fmt.Sprintf("%s\n\nPrevious attempt failed validation: %s\nPlease fix the output to match the required schema.", prompt, lastErr)
		}

		request := ChatRequest{
			Model: model,
			Messages: []Message{
				{Role: "user", Content: prompt},
			},
			APIBase:     m.apiBase,
			APIKey:      m.apiKey,
			Timeout:     m.timeout,
			ExtraParams: m.extraParams,
		}

		result, err := CallChatCompletion(request)
		if err != nil {
			lastErr = err.Error()
			continue
		}

		tokensUsed := 0
		if result.Usage != nil {
			tokensUsed = result.Usage.TotalTokens
		}

		// Extract JSON from response
		output := extractJSON(result.Content)
		if output == nil {
			lastErr = "no valid JSON found in response"
			continue
		}

		// Validate against schema if provided
		if schema != nil {
			if validationErr := validateMapOutput(output, schema); validationErr != "" {
				lastErr = validationErr
				continue
			}
		}

		return itemProcessResult{
			status:     MapItemCompleted,
			output:     output,
			retries:    attempt,
			tokensUsed: tokensUsed,
		}
	}

	return itemProcessResult{
		status:  MapItemFailed,
		errMsg:  lastErr,
		retries: maxRetries,
	}
}

// ─── JSONL I/O ──────────────────────────────────────────────────────────────

func readJSONLFile(path string) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var items []json.RawMessage
	scanner := bufio.NewScanner(f)
	// Increase buffer for large lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		items = append(items, json.RawMessage(line))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading JSONL: %w", err)
	}

	return items, nil
}

func writeJSONLOutput(path string, results []MapItemResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for _, r := range results {
		if r.Status == MapItemCompleted && r.Output != nil {
			if _, writeErr := w.Write(r.Output); writeErr != nil {
				return writeErr
			}
			if _, writeErr := w.WriteString("\n"); writeErr != nil {
				return writeErr
			}
		} else {
			// Write error record
			errRecord := map[string]interface{}{
				"_error": r.Error,
				"_index": r.Index,
			}
			data, _ := json.Marshal(errRecord)
			if _, writeErr := w.Write(data); writeErr != nil {
				return writeErr
			}
			if _, writeErr := w.WriteString("\n"); writeErr != nil {
				return writeErr
			}
		}
	}

	return w.Flush()
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// extractJSON delegates to the shared ExtractFirstJSON (json_extraction.go).
var extractJSON = ExtractFirstJSON

// validateMapOutput validates JSON output against a schema.
func validateMapOutput(output json.RawMessage, schema *JSONSchema) string {
	var parsed interface{}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return fmt.Sprintf("invalid JSON: %v", err)
	}

	// Use the existing schema validation infrastructure
	if err := validateValue(parsed, schema); err != nil {
		return err.Error()
	}
	return ""
}
