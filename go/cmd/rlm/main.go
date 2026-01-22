package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"recursive-llm-go/internal/rlm"
)

type requestPayload struct {
	Model   string                 `json:"model"`
	Query   string                 `json:"query"`
	Context string                 `json:"context"`
	Config  map[string]interface{} `json:"config"`
}

type responsePayload struct {
	Result string      `json:"result"`
	Stats  rlm.RLMStats `json:"stats"`
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

	result, stats, err := engine.Completion(req.Query, req.Context)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	resp := responsePayload{
		Result: result,
		Stats:  stats,
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to encode response JSON:", err)
		os.Exit(1)
	}

	fmt.Println(string(payload))
}
