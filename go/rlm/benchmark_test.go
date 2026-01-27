package rlm

import (
	"strings"
	"testing"
)

// Benchmark parser performance
func BenchmarkIsFinal(b *testing.B) {
	responses := []string{
		`FINAL("answer")`,
		`FINAL_VAR(result)`,
		`x = 1`,
		`console.log("test")`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsFinal(responses[i%len(responses)])
	}
}

func BenchmarkExtractFinal(b *testing.B) {
	response := `FINAL("This is a test answer with some content")`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractFinal(response)
	}
}

func BenchmarkParseResponse(b *testing.B) {
	response := `FINAL("Test answer")`
	env := map[string]interface{}{
		"result": "test",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseResponse(response, env)
	}
}

// Benchmark REPL performance
func BenchmarkREPLSimpleExecution(b *testing.B) {
	repl := NewREPLExecutor()
	code := `console.log("Hello World")`
	env := map[string]interface{}{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repl.Execute(code, env)
	}
}

func BenchmarkREPLContextAccess(b *testing.B) {
	repl := NewREPLExecutor()
	code := `console.log(context.slice(0, 10))`
	env := map[string]interface{}{
		"context": strings.Repeat("Lorem ipsum dolor sit amet. ", 1000),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repl.Execute(code, env)
	}
}

func BenchmarkREPLRegex(b *testing.B) {
	repl := NewREPLExecutor()
	code := `const matches = re.findall("ERROR", context); console.log(matches.length)`
	context := strings.Repeat("INFO ERROR WARNING ", 100)
	env := map[string]interface{}{
		"context": context,
		"re":      NewRegexHelper(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repl.Execute(code, env)
	}
}

func BenchmarkREPLJSBootstrap(b *testing.B) {
	repl := NewREPLExecutor()
	code := `const arr = range(100); const s = sum(arr); console.log(s)`
	env := map[string]interface{}{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repl.Execute(code, env)
	}
}

// Benchmark regex helper
func BenchmarkRegexFindall(b *testing.B) {
	re := NewRegexHelper()
	text := strings.Repeat("ERROR INFO WARNING ERROR ", 100)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re["findall"]("ERROR", text)
	}
}

func BenchmarkRegexSearch(b *testing.B) {
	re := NewRegexHelper()
	text := strings.Repeat("INFO WARNING ", 50) + "ERROR" + strings.Repeat(" INFO WARNING", 50)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re["search"]("ERROR", text)
	}
}

// Benchmark config parsing
func BenchmarkConfigFromMap(b *testing.B) {
	config := map[string]interface{}{
		"recursive_model": "gpt-4o-mini",
		"api_base":        "https://api.openai.com/v1",
		"api_key":         "sk-test",
		"max_depth":       5,
		"max_iterations":  30,
		"temperature":     0.7,
		"extra_param":     "value",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConfigFromMap(config)
	}
}

// Benchmark code extraction
func BenchmarkExtractCode(b *testing.B) {
	code := "```javascript\nconsole.log('test')\nconst x = 42\n```"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractCode(code)
	}
}

// Memory allocation benchmarks
func BenchmarkREPLMemoryAllocation(b *testing.B) {
	repl := NewREPLExecutor()
	code := `const arr = []; for (let i = 0; i < 1000; i++) arr.push(i); console.log(arr.length)`
	env := map[string]interface{}{}
	
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repl.Execute(code, env)
	}
}

func BenchmarkLargeContextAccess(b *testing.B) {
	repl := NewREPLExecutor()
	// Simulate 100KB context
	largeContext := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 2000)
	code := `const first = context.slice(0, 100); const last = context.slice(-100); console.log(first.length + last.length)`
	env := map[string]interface{}{
		"context": largeContext,
	}
	
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repl.Execute(code, env)
	}
}
