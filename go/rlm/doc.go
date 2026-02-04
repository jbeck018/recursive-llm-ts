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
// # Meta-Agent Mode
//
// The meta-agent optimizes queries before they reach the RLM engine. Short or
// vague queries are automatically rewritten for better recursive processing:
//
//	config := rlm.Config{
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	    MetaAgent: &rlm.MetaAgentConfig{
//	        Enabled: true,
//	        Model:   "gpt-4o-mini", // optional, defaults to engine model
//	    },
//	}
//
//	engine := rlm.New("gpt-4o", config)
//	// "summarize this" becomes a detailed, optimized query
//	answer, stats, err := engine.Completion("summarize this", longDocument)
//
// For structured queries, the meta-agent references schema fields explicitly:
//
//	result, stats, err := engine.StructuredCompletion(
//	    "get info",
//	    context,
//	    &rlm.StructuredConfig{Schema: schema},
//	)
//
// # Observability
//
// RLM provides built-in observability via OpenTelemetry tracing, Langfuse
// integration, and debug logging. All internal operations (LLM calls, REPL
// execution, meta-agent optimization, validation) emit trace events.
//
// Debug mode logs all operations:
//
//	config := rlm.Config{
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	    Observability: &rlm.ObservabilityConfig{
//	        Debug: true,
//	        LogOutput: "stderr", // or "stdout", or a file path
//	    },
//	}
//
// OpenTelemetry tracing:
//
//	config := rlm.Config{
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	    Observability: &rlm.ObservabilityConfig{
//	        TraceEnabled:  true,
//	        TraceEndpoint: "http://localhost:4318",
//	        ServiceName:   "my-rlm-service",
//	    },
//	}
//
// Langfuse integration:
//
//	config := rlm.Config{
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	    Observability: &rlm.ObservabilityConfig{
//	        LangfuseEnabled:   true,
//	        LangfusePublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
//	        LangfuseSecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
//	        LangfuseHost:      "https://cloud.langfuse.com",
//	    },
//	}
//
// Trace events are accessible after completion via the Observer:
//
//	obs := engine.GetObserver()
//	events := obs.Events()
//	defer engine.Shutdown() // flush OTEL spans
//
// # Structured Output
//
// For structured JSON output with schema validation using Google jsonschema-go:
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
