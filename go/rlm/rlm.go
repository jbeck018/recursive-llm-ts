package rlm

import (
	"fmt"
	"time"
)

type RLM struct {
	model            string
	recursiveModel   string
	apiBase          string
	apiKey           string
	maxDepth         int
	maxIterations    int
	timeoutSeconds   int
	useMetacognitive bool
	extraParams      map[string]interface{}
	currentDepth     int
	repl             *REPLExecutor
	stats            RLMStats
	observer         *Observer
	metaAgent        *MetaAgent
}

func New(model string, config Config) *RLM {
	recursiveModel := config.RecursiveModel
	if recursiveModel == "" {
		recursiveModel = model
	}

	// Setup observer
	var obs *Observer
	if config.Observability != nil {
		obs = NewObserver(*config.Observability)
	} else {
		obs = NewNoopObserver()
	}

	r := &RLM{
		model:            model,
		recursiveModel:   recursiveModel,
		apiBase:          config.APIBase,
		apiKey:           config.APIKey,
		maxDepth:         config.MaxDepth,
		maxIterations:    config.MaxIterations,
		timeoutSeconds:   config.TimeoutSeconds,
		useMetacognitive: config.UseMetacognitive,
		extraParams:      config.ExtraParams,
		currentDepth:     0,
		repl:             NewREPLExecutor(),
		stats:            RLMStats{},
		observer:         obs,
	}

	// Setup meta-agent if enabled
	if config.MetaAgent != nil && config.MetaAgent.Enabled {
		r.metaAgent = NewMetaAgent(r, *config.MetaAgent, obs)
	}

	return r
}

func (r *RLM) Completion(query string, context string) (string, RLMStats, error) {
	ctx := r.observer.StartTrace("rlm.completion", map[string]string{
		"model":          r.model,
		"query_length":   fmt.Sprintf("%d", len(query)),
		"context_length": fmt.Sprintf("%d", len(context)),
		"depth":          fmt.Sprintf("%d", r.currentDepth),
	})
	defer r.observer.EndTrace(ctx)

	if query != "" && context == "" {
		context = query
		query = ""
	}

	if r.currentDepth >= r.maxDepth {
		return "", r.stats, NewMaxDepthError(r.maxDepth)
	}

	// Apply meta-agent optimization if enabled
	if r.metaAgent != nil && r.currentDepth == 0 {
		optimized, err := r.metaAgent.OptimizeQuery(query, context)
		if err == nil && optimized != "" {
			r.observer.Debug("rlm", "Using meta-agent optimized query")
			query = optimized
		}
	}

	r.stats.Depth = r.currentDepth
	replEnv := r.buildREPLEnv(query, context)
	systemPrompt := BuildSystemPrompt(len(context), r.currentDepth, query, r.useMetacognitive)
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	for iteration := 0; iteration < r.maxIterations; iteration++ {
		r.stats.Iterations = iteration + 1
		r.observer.Debug("rlm", "Iteration %d/%d at depth %d", iteration+1, r.maxIterations, r.currentDepth)

		response, err := r.callLLM(messages)
		if err != nil {
			r.observer.Error("rlm", "LLM call failed on iteration %d: %v", iteration+1, err)
			return "", r.stats, err
		}

		if IsFinal(response) {
			answer, ok := ParseResponse(response, replEnv)
			if ok {
				r.observer.Debug("rlm", "FINAL answer found on iteration %d", iteration+1)
				r.observer.Event("rlm.completion_success", map[string]string{
					"iterations": fmt.Sprintf("%d", iteration+1),
					"llm_calls":  fmt.Sprintf("%d", r.stats.LlmCalls),
				})
				return answer, r.stats, nil
			}
		}

		execResult, err := r.repl.Execute(response, replEnv)
		if err != nil {
			r.observer.Debug("rlm", "REPL execution error: %v", err)
			execResult = fmt.Sprintf("Error: %s", err.Error())
		} else {
			r.observer.Debug("rlm", "REPL output: %s", truncateStr(execResult, 200))
		}

		messages = append(messages, Message{Role: "assistant", Content: response})
		messages = append(messages, Message{Role: "user", Content: execResult})
	}

	return "", r.stats, NewMaxIterationsError(r.maxIterations)
}

func (r *RLM) callLLM(messages []Message) (string, error) {
	r.stats.LlmCalls++
	defaultModel := r.model
	if r.currentDepth > 0 {
		defaultModel = r.recursiveModel
	}

	r.observer.Debug("llm", "Calling %s with %d messages", defaultModel, len(messages))

	start := time.Now()

	request := ChatRequest{
		Model:       defaultModel,
		Messages:    messages,
		APIBase:     r.apiBase,
		APIKey:      r.apiKey,
		Timeout:     r.timeoutSeconds,
		ExtraParams: r.extraParams,
	}

	result, err := CallChatCompletion(request)
	duration := time.Since(start)

	r.observer.LLMCall(defaultModel, len(messages), 0, duration, err)

	if err != nil {
		return "", err
	}

	r.observer.Debug("llm", "Response received (%d chars) in %s", len(result), duration)
	return result, nil
}

func (r *RLM) buildREPLEnv(query string, context string) map[string]interface{} {
	env := map[string]interface{}{
		"context": context,
		"query":   query,
	}

	env["re"] = NewRegexHelper()
	env["recursive_llm"] = func(subQuery string, subContext string) string {
		if r.currentDepth+1 >= r.maxDepth {
			return fmt.Sprintf("Max recursion depth (%d) reached", r.maxDepth)
		}

		r.observer.Debug("rlm", "Recursive call at depth %d: %s", r.currentDepth+1, truncateStr(subQuery, 100))

		subConfig := Config{
			RecursiveModel:   r.recursiveModel,
			APIBase:          r.apiBase,
			APIKey:           r.apiKey,
			MaxDepth:         r.maxDepth,
			MaxIterations:    r.maxIterations,
			TimeoutSeconds:   r.timeoutSeconds,
			UseMetacognitive: r.useMetacognitive,
			ExtraParams:      r.extraParams,
		}

		subRLM := New(r.recursiveModel, subConfig)
		subRLM.currentDepth = r.currentDepth + 1
		subRLM.observer = r.observer // Share observer for trace continuity

		answer, _, err := subRLM.Completion(subQuery, subContext)
		if err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return answer
	}

	return env
}

// GetObserver returns the observer for external access to events/traces.
func (r *RLM) GetObserver() *Observer {
	return r.observer
}

// Shutdown gracefully shuts down the RLM engine and its observer.
func (r *RLM) Shutdown() {
	if r.observer != nil {
		r.observer.Shutdown()
	}
}
