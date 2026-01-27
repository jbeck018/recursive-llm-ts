// Package rlm provides a Recursive Language Model (RLM) engine for Go.
//
// RLM enables language models to use tools and make recursive calls to themselves,
// allowing for complex multi-step reasoning and task decomposition. This implements
// the technique from the paper "Recursive Language Models" by Alex Zhang and Omar Khattab (MIT, 2025).
//
// # Installation
//
// To use this package in your Go project:
//
//	go get github.com/jbeck018/recursive-llm-ts/go
//
// # Basic Usage
//
// Create an RLM engine and execute a completion:
//
//	import "github.com/jbeck018/recursive-llm-ts/go/rlm"
//
//	config := rlm.Config{
//	    MaxDepth:      5,
//	    MaxIterations: 30,
//	    APIKey:        os.Getenv("OPENAI_API_KEY"),
//	}
//
//	engine := rlm.New("gpt-4", config)
//	answer, stats, err := engine.Completion("What is 2+2?", "")
//
// # Structured Output
//
// For structured JSON output with validation:
//
//	schema := &rlm.JSONSchema{
//	    Type: "object",
//	    Properties: map[string]*rlm.JSONSchema{
//	        "name": {Type: "string"},
//	        "age":  {Type: "number"},
//	    },
//	    Required: []string{"name", "age"},
//	}
//
//	config := &rlm.StructuredConfig{
//	    Schema:     schema,
//	    MaxRetries: 3,
//	}
//
//	result, stats, err := engine.StructuredCompletion(
//	    "Extract person info",
//	    "John is 30 years old",
//	    config,
//	)
//
// # Recursive Calls
//
// The LLM can make recursive calls to itself using the recursive_llm() function
// available in the JavaScript REPL environment during execution. This enables
// complex multi-step reasoning and task decomposition.
//
// # Supported Providers
//
// RLM works with any OpenAI-compatible API:
//   - OpenAI (default)
//   - Azure OpenAI
//   - Ollama
//   - LiteLLM
//   - Any OpenAI-compatible endpoint
//
// Configure the provider using Config.APIBase:
//
//	config := rlm.Config{
//	    APIBase: "https://your-azure-endpoint.openai.azure.com/v1",
//	    APIKey:  os.Getenv("AZURE_API_KEY"),
//	}
//
// # Error Handling
//
// All methods return typed errors that can be checked with errors.As():
//
//	var maxDepthErr *rlm.MaxDepthError
//	if errors.As(err, &maxDepthErr) {
//	    fmt.Printf("Hit max depth: %d\n", maxDepthErr.MaxDepth)
//	}
package rlm
