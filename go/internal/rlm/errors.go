package rlm

import "fmt"

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
