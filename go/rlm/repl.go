package rlm

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dop251/goja"
)

type REPLExecutor struct {
	maxOutputChars int
}

func NewREPLExecutor() *REPLExecutor {
	return &REPLExecutor{
		maxOutputChars: 2000,
	}
}

func (r *REPLExecutor) Execute(code string, env map[string]interface{}) (string, error) {
	code = extractCode(code)
	if strings.TrimSpace(code) == "" {
		return "No code to execute", nil
	}

	vm := goja.New()
	var output strings.Builder

	for key, value := range env {
		if err := vm.Set(key, value); err != nil {
			return "", fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	writeOutput := func(call goja.FunctionCall) goja.Value {
		parts := make([]string, 0, len(call.Arguments))
		for _, arg := range call.Arguments {
			parts = append(parts, arg.String())
		}
		output.WriteString(strings.Join(parts, " "))
		output.WriteString("\n")
		return goja.Undefined()
	}

	console := map[string]func(goja.FunctionCall) goja.Value{
		"log": writeOutput,
	}

	if err := vm.Set("console", console); err != nil {
		return "", fmt.Errorf("failed to set console: %w", err)
	}
	if err := vm.Set("print", writeOutput); err != nil {
		return "", fmt.Errorf("failed to set print: %w", err)
	}
	if err := vm.Set("len", func(value goja.Value) int {
		if value == nil || value == goja.Undefined() || value == goja.Null() {
			return 0
		}
		if exported := value.Export(); exported != nil {
			switch typed := exported.(type) {
			case string:
				return len(typed)
			case []interface{}:
				return len(typed)
			case map[string]interface{}:
				return len(typed)
			}
		}
		return len(value.String())
	}); err != nil {
		return "", fmt.Errorf("failed to set len: %w", err)
	}

	if _, err := vm.RunString(jsBootstrap); err != nil {
		return "", NewREPLError("Bootstrap execution error", jsBootstrap, err)
	}

	if _, err := vm.RunString(code); err != nil {
		return "", NewREPLError("Code execution error", code, err)
	}

	if output.Len() == 0 {
		lastLine := getLastLine(code)
		if looksLikeExpression(lastLine) {
			value, err := vm.RunString(lastLine)
			if err == nil && value != nil && value != goja.Undefined() {
				output.WriteString(value.String())
				output.WriteString("\n")
			}
		}
	}

	if output.Len() == 0 {
		return "Code executed successfully (no output)", nil
	}

	rawOutput := output.String()
	trimmedOutput := strings.TrimSpace(rawOutput)
	if len(rawOutput) > r.maxOutputChars {
		truncated := rawOutput[:r.maxOutputChars]
		return fmt.Sprintf("%s\n\n[Output truncated: %d chars total, showing first %d]", strings.TrimSpace(truncated), len(rawOutput), r.maxOutputChars), nil
	}

	return trimmedOutput, nil
}

func extractCode(text string) string {
	if strings.Contains(text, "```python") {
		return extractBlock(text, "```python")
	}
	if strings.Contains(text, "```javascript") {
		return extractBlock(text, "```javascript")
	}
	if strings.Contains(text, "```js") {
		return extractBlock(text, "```js")
	}
	if strings.Contains(text, "```") {
		return extractBlock(text, "```")
	}
	return text
}

const jsBootstrap = `
const json = {
  loads: (text) => JSON.parse(text),
  dumps: (value, replacer, space) => JSON.stringify(value, replacer, space),
};
const math = Math;
const datetime = Date;
const Counter = (iterable) => {
  const counts = {};
  if (iterable == null) {
    return counts;
  }
  const items = typeof iterable === "string" ? iterable.split("") : iterable;
  for (const item of items) {
    const key = String(item);
    counts[key] = (counts[key] || 0) + 1;
  }
  return counts;
};
const defaultdict = (defaultFactory) => new Proxy({}, {
  get(target, prop) {
    if (!(prop in target)) {
      target[prop] = typeof defaultFactory === "function" ? defaultFactory() : defaultFactory;
    }
    return target[prop];
  },
});
const range = (start, stop, step) => {
  if (stop === undefined) {
    stop = start;
    start = 0;
  }
  if (step === undefined) {
    step = 1;
  }
  if (step === 0) {
    return [];
  }
  const result = [];
  if (step > 0) {
    for (let i = start; i < stop; i += step) {
      result.push(i);
    }
  } else {
    for (let i = start; i > stop; i += step) {
      result.push(i);
    }
  }
  return result;
};
const sorted = (iterable, compareFn) => [...iterable].sort(compareFn);
const sum = (iterable) => (iterable || []).reduce((acc, value) => acc + Number(value), 0);
const min = (iterable) => Math.min(...iterable);
const max = (iterable) => Math.max(...iterable);
const enumerate = (iterable) => (iterable || []).map((value, index) => [index, value]);
const zip = (...iterables) => {
  const length = Math.min(...iterables.map((items) => items.length));
  const result = [];
  for (let i = 0; i < length; i++) {
    result.push(iterables.map((items) => items[i]));
  }
  return result;
};
const any = (iterable) => (iterable || []).some(Boolean);
const all = (iterable) => (iterable || []).every(Boolean);
`

func extractBlock(text string, marker string) string {
	start := strings.Index(text, marker)
	if start == -1 {
		return text
	}
	start += len(marker)
	end := strings.Index(text[start:], "```")
	if end == -1 {
		return text[start:]
	}
	return strings.TrimSpace(text[start : start+end])
}

func getLastLine(code string) string {
	lines := strings.Split(strings.TrimSpace(code), "\n")
	if len(lines) == 0 {
		return ""
	}
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	// Handle multiple statements on same line separated by semicolon
	if strings.Contains(lastLine, ";") {
		parts := strings.Split(lastLine, ";")
		// Get the last non-empty part
		for i := len(parts) - 1; i >= 0; i-- {
			if trimmed := strings.TrimSpace(parts[i]); trimmed != "" {
				return trimmed
			}
		}
	}
	return lastLine
}

func looksLikeExpression(line string) bool {
	if line == "" {
		return false
	}
	// Check for statements that shouldn't be evaluated as expressions
	keywords := []string{"const ", "let ", "var ", "function ", "if ", "for ", "while ", "class ", "return "}
	for _, keyword := range keywords {
		if strings.HasPrefix(strings.TrimSpace(line), keyword) {
			return false
		}
	}
	// Check for assignment (but not == or === comparison)
	if strings.Contains(line, "=") && !strings.Contains(line, "==") && !strings.Contains(line, "===") {
		return false
	}
	return true
}

func NewRegexHelper() map[string]func(string, string) interface{} {
	return map[string]func(string, string) interface{}{
		"findall": func(pattern string, text string) interface{} {
			re, err := regexpFromPattern(pattern)
			if err != nil {
				return []string{}
			}
			return re.FindAllString(text, -1)
		},
		"search": func(pattern string, text string) interface{} {
			re, err := regexpFromPattern(pattern)
			if err != nil {
				return ""
			}
			match := re.FindString(text)
			return match
		},
	}
}

func regexpFromPattern(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.New("invalid regex pattern")
	}
	return re, nil
}
