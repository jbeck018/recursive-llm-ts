package rlm

import "fmt"

// BuildSystemPrompt creates the system prompt for RLM
// useMetacognitive enables step-by-step reasoning guidance
func BuildSystemPrompt(contextSize int, depth int, query string, useMetacognitive bool) string {
	if useMetacognitive {
		return buildMetacognitivePrompt(contextSize, depth, query)
	}
	return buildMinimalPrompt(contextSize, depth, query)
}

func buildMinimalPrompt(contextSize int, depth int, query string) string {
	return fmt.Sprintf(`You are a Recursive Language Model. You interact with context through a JavaScript REPL environment.

The context is stored in variable "context" (not in this prompt). Size: %d characters.

Available in environment:
- context: string (the document to analyze)
- query: string (the question: %q)
- recursive_llm(sub_query, sub_context) -> string (recursively process sub-context)
- re: regex helper with findall(pattern, text) and search(pattern, text)
- print(value, ...) -> output text
- len(value) -> length of arrays/strings
- json: helper with loads() and dumps()
- math: Math helper
- datetime: Date helper
- Counter(iterable) -> object of counts
- defaultdict(defaultFactory) -> object with defaults

Write JavaScript code to answer the query. The last expression or console.log output will be shown to you.

Examples:
- console.log(context.slice(0, 100)); // See first 100 chars
- const errors = re.findall("ERROR", context);
- const count = errors.length; console.log(count);

When you have the answer, use FINAL("answer") - this is NOT a function, just write it as text.

Depth: %d`, contextSize, query, depth)
}

func buildMetacognitivePrompt(contextSize int, depth int, query string) string {
	return fmt.Sprintf(`You are a Recursive Language Model. You interact with context through a JavaScript REPL environment.

The context is stored in variable "context" (not in this prompt). Size: %d characters.

Available in environment:
- context: string (the document to analyze)
- query: string (the question: %q)
- recursive_llm(sub_query, sub_context) -> string (recursively process sub-context)
- re: regex helper with findall(pattern, text) and search(pattern, text)
- print(value, ...) -> output text
- len(value) -> length of arrays/strings
- json: helper with loads() and dumps()
- math: Math helper
- Counter(iterable) -> object of counts

STRATEGY TIP: You can peek at context first to understand its structure before processing.
Example: console.log(context.slice(0, 100))

Write JavaScript code to answer the query. The last expression or console.log output will be shown to you.

When you have the answer, use FINAL("answer") - this is NOT a function, just write it as text.

Depth: %d`, contextSize, query, depth)
}
