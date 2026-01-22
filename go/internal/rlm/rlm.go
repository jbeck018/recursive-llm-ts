package rlm

import (
	"errors"
	"fmt"
)

type RLM struct {
	model          string
	recursiveModel string
	apiBase        string
	apiKey         string
	maxDepth       int
	maxIterations  int
	timeoutSeconds int
	extraParams    map[string]interface{}
	currentDepth   int
	repl           *REPLExecutor
	stats          RLMStats
}

func New(model string, config Config) *RLM {
	recursiveModel := config.RecursiveModel
	if recursiveModel == "" {
		recursiveModel = model
	}

	return &RLM{
		model:          model,
		recursiveModel: recursiveModel,
		apiBase:        config.APIBase,
		apiKey:         config.APIKey,
		maxDepth:       config.MaxDepth,
		maxIterations:  config.MaxIterations,
		timeoutSeconds: config.TimeoutSeconds,
		extraParams:    config.ExtraParams,
		currentDepth:   0,
		repl:           NewREPLExecutor(),
		stats:          RLMStats{},
	}
}

func (r *RLM) Completion(query string, context string) (string, RLMStats, error) {
	if query != "" && context == "" {
		context = query
		query = ""
	}

	if r.currentDepth >= r.maxDepth {
		return "", r.stats, fmt.Errorf("max recursion depth (%d) exceeded", r.maxDepth)
	}

	r.stats.Depth = r.currentDepth
	replEnv := r.buildREPLEnv(query, context)
	systemPrompt := BuildSystemPrompt(len(context), r.currentDepth, query)
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	for iteration := 0; iteration < r.maxIterations; iteration++ {
		r.stats.Iterations = iteration + 1

		response, err := r.callLLM(messages)
		if err != nil {
			return "", r.stats, err
		}

		if IsFinal(response) {
			answer, ok := ParseResponse(response, replEnv)
			if ok {
				return answer, r.stats, nil
			}
		}

		execResult, err := r.repl.Execute(response, replEnv)
		if err != nil {
			execResult = fmt.Sprintf("Error: %s", err.Error())
		}

		messages = append(messages, Message{Role: "assistant", Content: response})
		messages = append(messages, Message{Role: "user", Content: execResult})
	}

	return "", r.stats, errors.New("max iterations exceeded without FINAL()")
}

func (r *RLM) callLLM(messages []Message) (string, error) {
	r.stats.LlmCalls++
	defaultModel := r.model
	if r.currentDepth > 0 {
		defaultModel = r.recursiveModel
	}

	request := ChatRequest{
		Model:       defaultModel,
		Messages:    messages,
		APIBase:     r.apiBase,
		APIKey:      r.apiKey,
		Timeout:     r.timeoutSeconds,
		ExtraParams: r.extraParams,
	}

	return CallChatCompletion(request)
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

		subConfig := Config{
			RecursiveModel: r.recursiveModel,
			APIBase:        r.apiBase,
			APIKey:         r.apiKey,
			MaxDepth:       r.maxDepth,
			MaxIterations:  r.maxIterations,
			TimeoutSeconds: r.timeoutSeconds,
			ExtraParams:    r.extraParams,
		}

		subRLM := New(r.recursiveModel, subConfig)
		subRLM.currentDepth = r.currentDepth + 1

		answer, _, err := subRLM.Completion(subQuery, subContext)
		if err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return answer
	}

	return env
}
