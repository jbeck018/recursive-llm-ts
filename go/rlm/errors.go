package rlm

import (
	"errors"
	"fmt"
	"strings"
)

// RLMError is the base error type for all RLM errors
type RLMError struct {
	Message string
	Cause   error
}

func (e *RLMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *RLMError) Unwrap() error {
	return e.Cause
}

// MaxIterationsError is returned when max iterations are exceeded
type MaxIterationsError struct {
	MaxIterations int
	*RLMError
}

func NewMaxIterationsError(maxIterations int) *MaxIterationsError {
	return &MaxIterationsError{
		MaxIterations: maxIterations,
		RLMError: &RLMError{
			Message: fmt.Sprintf("max iterations (%d) exceeded without FINAL()", maxIterations),
		},
	}
}

// MaxDepthError is returned when max recursion depth is exceeded
type MaxDepthError struct {
	MaxDepth int
	*RLMError
}

func NewMaxDepthError(maxDepth int) *MaxDepthError {
	return &MaxDepthError{
		MaxDepth: maxDepth,
		RLMError: &RLMError{
			Message: fmt.Sprintf("max recursion depth (%d) exceeded", maxDepth),
		},
	}
}

// REPLError is returned when REPL execution fails
type REPLError struct {
	Code string
	*RLMError
}

func NewREPLError(message string, code string, cause error) *REPLError {
	return &REPLError{
		Code: code,
		RLMError: &RLMError{
			Message: message,
			Cause:   cause,
		},
	}
}

// APIError is returned when LLM API calls fail
type APIError struct {
	StatusCode int
	Response   string
	*RLMError
}

func NewAPIError(statusCode int, response string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Response:   response,
		RLMError: &RLMError{
			Message: fmt.Sprintf("LLM request failed (%d): %s", statusCode, response),
		},
	}
}

// ContextOverflowError is returned when the request exceeds the model's context window
type ContextOverflowError struct {
	ModelLimit    int // Maximum tokens the model supports
	RequestTokens int // Number of tokens in the request
	*APIError
}

// NewContextOverflowError creates a ContextOverflowError from parsed API response details
func NewContextOverflowError(statusCode int, response string, modelLimit, requestTokens int) *ContextOverflowError {
	return &ContextOverflowError{
		ModelLimit:    modelLimit,
		RequestTokens: requestTokens,
		APIError: &APIError{
			StatusCode: statusCode,
			Response:   response,
			RLMError: &RLMError{
				Message: fmt.Sprintf("context overflow: model limit is %d tokens but request has %d tokens (overflow by %d)",
					modelLimit, requestTokens, requestTokens-modelLimit),
			},
		},
	}
}

// Unwrap returns the embedded APIError so errors.As can find it in the chain.
func (e *ContextOverflowError) Unwrap() error {
	return e.APIError
}

// OverflowRatio returns how much the request exceeds the limit (e.g., 1.23 means 23% over)
func (e *ContextOverflowError) OverflowRatio() float64 {
	if e.ModelLimit == 0 {
		return 0
	}
	return float64(e.RequestTokens) / float64(e.ModelLimit)
}

// IsContextOverflow checks if an error is a context overflow error.
// It detects both explicit ContextOverflowError types and parses API error messages.
func IsContextOverflow(err error) (*ContextOverflowError, bool) {
	// Direct type check
	var coe *ContextOverflowError
	if errors.As(err, &coe) {
		return coe, true
	}

	// Parse from APIError message
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if limit, request, ok := parseContextOverflowMessage(apiErr.Response); ok {
			return NewContextOverflowError(apiErr.StatusCode, apiErr.Response, limit, request), true
		}
		// Also check the error message itself
		if limit, request, ok := parseContextOverflowMessage(apiErr.Error()); ok {
			return NewContextOverflowError(apiErr.StatusCode, apiErr.Response, limit, request), true
		}
	}

	// Parse from generic error message
	if limit, request, ok := parseContextOverflowMessage(err.Error()); ok {
		return NewContextOverflowError(0, err.Error(), limit, request), true
	}

	return nil, false
}

// parseContextOverflowMessage extracts token limits from common API error message patterns.
// Supports OpenAI, Azure, vLLM, and other OpenAI-compatible API error formats.
func parseContextOverflowMessage(msg string) (modelLimit int, requestTokens int, ok bool) {
	// Common patterns:
	// OpenAI: "This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens."
	// Azure: "This model's maximum context length is 32768 tokens, however you requested 40354 tokens"
	// vLLM: "This model's maximum context length is 32768 tokens. However, your request has 40354 input tokens."
	// Anthropic: "max_tokens: ... exceeds the maximum"

	lowerMsg := strings.ToLower(msg)

	// Pattern 1: "maximum context length is X tokens"
	if strings.Contains(lowerMsg, "maximum context length") {
		limit := extractNumber(msg, "maximum context length is ", " tokens")
		if limit > 0 {
			// Try various patterns for the request size
			request := extractNumber(msg, "your request has ", " input tokens")
			if request == 0 {
				request = extractNumber(msg, "your request has ", " tokens")
			}
			if request == 0 {
				request = extractNumber(msg, "you requested ", " tokens")
			}
			if request == 0 {
				request = extractNumber(msg, "requested ", " tokens")
			}
			if request > 0 && request > limit {
				return limit, request, true
			}
		}
	}

	// Pattern 2: "context_length_exceeded" error code
	if strings.Contains(lowerMsg, "context_length_exceeded") {
		limit := extractNumber(msg, "maximum context length is ", " tokens")
		request := extractNumber(msg, "resulted in ", " tokens")
		if limit > 0 && request > 0 {
			return limit, request, true
		}
	}

	// Pattern 3: "max_tokens is too large" - response budget exceeds remaining capacity
	// vLLM/OpenAI: "max_tokens' or 'max_completion_tokens' is too large: 10000.
	//   This model's maximum context length is 32768 tokens and your request has 30168 input tokens"
	// In this case, input tokens < model limit, but input + max_tokens > model limit.
	// We report the effective total (input + max_tokens) as requestTokens.
	if strings.Contains(lowerMsg, "max_tokens") && strings.Contains(lowerMsg, "too large") {
		limit := extractNumber(msg, "maximum context length is ", " tokens")
		inputTokens := extractNumber(msg, "your request has ", " input tokens")
		if inputTokens == 0 {
			inputTokens = extractNumber(msg, "your request has ", " tokens")
		}
		maxTokens := extractNumber(msg, "too large: ", ".")
		if maxTokens == 0 {
			maxTokens = extractNumber(msg, "too large: ", " ")
		}
		if limit > 0 && inputTokens > 0 && maxTokens > 0 {
			return limit, inputTokens + maxTokens, true
		}
		// Fallback: if we got limit and input tokens, treat input as the overflow
		if limit > 0 && inputTokens > 0 {
			return limit, inputTokens, true
		}
	}

	// Pattern 4: "input too long" / "too many tokens" generic patterns
	if strings.Contains(lowerMsg, "input too long") || strings.Contains(lowerMsg, "too many tokens") || strings.Contains(lowerMsg, "too many input tokens") {
		limit := extractNumber(msg, "limit is ", " tokens")
		if limit == 0 {
			limit = extractNumber(msg, "maximum of ", " tokens")
		}
		request := extractNumber(msg, "has ", " tokens")
		if request == 0 {
			request = extractNumber(msg, "requested ", " tokens")
		}
		if limit > 0 && request > 0 {
			return limit, request, true
		}
	}

	return 0, 0, false
}

// extractNumber finds a number between a prefix and suffix in a string
func extractNumber(s string, prefix string, suffix string) int {
	lowerS := strings.ToLower(s)
	lowerPrefix := strings.ToLower(prefix)

	idx := strings.Index(lowerS, lowerPrefix)
	if idx < 0 {
		return 0
	}

	start := idx + len(lowerPrefix)
	remaining := s[start:]

	// Find the suffix
	lowerSuffix := strings.ToLower(suffix)
	endIdx := strings.Index(strings.ToLower(remaining), lowerSuffix)
	if endIdx < 0 {
		return 0
	}

	numStr := strings.TrimSpace(remaining[:endIdx])
	// Remove commas from numbers like "32,768"
	numStr = strings.ReplaceAll(numStr, ",", "")

	var n int
	_, err := fmt.Sscanf(numStr, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}
