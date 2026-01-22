package rlm

import (
	"fmt"
	"strconv"
)

type RLMStats struct {
	LlmCalls       int `json:"llm_calls"`
	Iterations     int `json:"iterations"`
	Depth          int `json:"depth"`
	ParsingRetries int `json:"parsing_retries,omitempty"`
}

type JSONSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]*JSONSchema `json:"properties,omitempty"`
	Items      *JSONSchema            `json:"items,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Enum       []string               `json:"enum,omitempty"`
	Nullable   bool                   `json:"nullable,omitempty"`
}

type SubTask struct {
	ID           string
	Query        string
	Schema       *JSONSchema
	Dependencies []string
	Path         []string
}

type StructuredConfig struct {
	Schema            *JSONSchema
	ParallelExecution bool
	MaxRetries        int
}

type Config struct {
	RecursiveModel    string
	APIBase           string
	APIKey            string
	MaxDepth          int
	MaxIterations     int
	TimeoutSeconds    int
	Parallel          bool // Enable parallel recursive calls with goroutines
	UseMetacognitive  bool // Enable step-by-step reasoning guidance in prompts
	Structured        *StructuredConfig
	ExtraParams       map[string]interface{}
}

func ConfigFromMap(config map[string]interface{}) Config {
	parsed := Config{
		MaxDepth:      5,
		MaxIterations: 30,
		ExtraParams:   map[string]interface{}{},
	}

	if config == nil {
		return parsed
	}

	for key, value := range config {
		switch key {
		case "recursive_model":
			parsed.RecursiveModel = toString(value)
		case "api_base":
			parsed.APIBase = toString(value)
		case "api_key":
			parsed.APIKey = toString(value)
		case "max_depth":
			if v, ok := toInt(value); ok {
				parsed.MaxDepth = v
			}
		case "max_iterations":
			if v, ok := toInt(value); ok {
				parsed.MaxIterations = v
			}
		case "timeout":
			if v, ok := toInt(value); ok {
				parsed.TimeoutSeconds = v
			}
		case "parallel":
			if v, ok := value.(bool); ok {
				parsed.Parallel = v
			}
		case "use_metacognitive", "metacognitive":
			if v, ok := value.(bool); ok {
				parsed.UseMetacognitive = v
			}
		case "pythonia_timeout", "go_binary_path", "bridge", "structured":
			// ignore bridge-only config and structured (handled separately)
		default:
			parsed.ExtraParams[key] = value
		}
	}

	return parsed
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed, true
		}
	default:
		return 0, false
	}

	return 0, false
}
