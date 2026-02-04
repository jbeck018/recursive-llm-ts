package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jbeck018/recursive-llm-ts/go/rlm"
)

type requestPayload struct {
	Model      string                 `json:"model"`
	Query      string                 `json:"query"`
	Context    string                 `json:"context"`
	Config     map[string]interface{} `json:"config"`
	Structured *structuredRequest     `json:"structured,omitempty"`
}

type structuredRequest struct {
	Schema            *rlm.JSONSchema `json:"schema"`
	ParallelExecution bool            `json:"parallelExecution"`
	MaxRetries        int             `json:"maxRetries"`
}

type responsePayload struct {
	Result           interface{}  `json:"result"`
	Stats            rlm.RLMStats `json:"stats"`
	StructuredResult bool         `json:"structured_result,omitempty"`
	TraceEvents      interface{}  `json:"trace_events,omitempty"`
}

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read stdin:", err)
		os.Exit(1)
	}

	var req requestPayload
	if err := json.Unmarshal(input, &req); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse input JSON:", err)
		os.Exit(1)
	}

	if req.Model == "" {
		fmt.Fprintln(os.Stderr, "Missing model in request payload")
		os.Exit(1)
	}

	config := rlm.ConfigFromMap(req.Config)
	engine := rlm.New(req.Model, config)
	defer engine.Shutdown()

	var resp responsePayload

	// Handle structured completion if requested
	if req.Structured != nil {
		structuredConfig := &rlm.StructuredConfig{
			Schema:            req.Structured.Schema,
			ParallelExecution: req.Structured.ParallelExecution,
			MaxRetries:        req.Structured.MaxRetries,
		}

		result, stats, err := engine.StructuredCompletion(req.Query, req.Context, structuredConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		resp = responsePayload{
			Result:           result,
			Stats:            stats,
			StructuredResult: true,
		}
	} else {
		// Regular completion
		result, stats, err := engine.Completion(req.Query, req.Context)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		resp = responsePayload{
			Result: result,
			Stats:  stats,
		}
	}

	// Include trace events if observability is enabled
	obs := engine.GetObserver()
	if obs != nil {
		events := obs.GetEvents()
		if len(events) > 0 {
			resp.TraceEvents = events
		}
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to encode response JSON:", err)
		os.Exit(1)
	}

	fmt.Println(string(payload))
}
