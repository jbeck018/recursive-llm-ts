package rlm

import (
	"strings"
	"testing"
)

func TestREPLExecutor_BasicExecution(t *testing.T) {
	repl := NewREPLExecutor()

	tests := []struct {
		name    string
		code    string
		env     map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name: "Simple print",
			code: `console.log("Hello World")`,
			env:  map[string]interface{}{},
			want: "Hello World",
		},
		{
			name: "Variable access",
			code: `console.log(context)`,
			env:  map[string]interface{}{"context": "Test Context"},
			want: "Test Context",
		},
		{
			name: "String slicing",
			code: `console.log(context.slice(0, 5))`,
			env:  map[string]interface{}{"context": "Hello World"},
			want: "Hello",
		},
		{
			name: "len function",
			code: `console.log(len(context))`,
			env:  map[string]interface{}{"context": "Hello"},
			want: "5",
		},
		{
			name: "regex findall",
			code: `console.log(re.findall("ERROR", context))`,
			env:  map[string]interface{}{"context": "ERROR 1 ERROR 2", "re": NewRegexHelper()},
			want: "ERROR,ERROR",
		},
		{
			name: "Last expression evaluation",
			code: `const x = 42; x`,
			env:  map[string]interface{}{},
			want: "42",
		},
		{
			name: "Array operations",
			code: `const arr = [1, 2, 3]; console.log(arr.length)`,
			env:  map[string]interface{}{},
			want: "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repl.Execute(tt.code, tt.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Execute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestREPLExecutor_CodeExtraction(t *testing.T) {
	repl := NewREPLExecutor()

	tests := []struct {
		name string
		code string
		env  map[string]interface{}
		want string
	}{
		{
			name: "JavaScript code block",
			code: "```javascript\nconsole.log('test')\n```",
			env:  map[string]interface{}{},
			want: "test",
		},
		{
			name: "Python code block (should still extract)",
			code: "```python\nconsole.log('test')\n```",
			env:  map[string]interface{}{},
			want: "test",
		},
		{
			name: "Generic code block",
			code: "```\nconsole.log('test')\n```",
			env:  map[string]interface{}{},
			want: "test",
		},
		{
			name: "No code block",
			code: "console.log('test')",
			env:  map[string]interface{}{},
			want: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repl.Execute(tt.code, tt.env)
			if err != nil {
				t.Errorf("Execute() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Execute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestREPLExecutor_JSBootstrap(t *testing.T) {
	repl := NewREPLExecutor()

	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "json.loads",
			code: `const obj = json.loads('{"key":"value"}'); console.log(obj.key)`,
			want: "value",
		},
		{
			name: "json.dumps",
			code: `console.log(json.dumps({key: "value"}))`,
			want: `{"key":"value"}`,
		},
		{
			name: "range",
			code: `console.log(range(5).length)`,
			want: "5",
		},
		{
			name: "sum",
			code: `console.log(sum([1, 2, 3]))`,
			want: "6",
		},
		{
			name: "Counter",
			code: `const c = Counter("hello"); console.log(c.l)`,
			want: "2",
		},
		{
			name: "sorted",
			code: `console.log(sorted([3, 1, 2]).join(','))`,
			want: "1,2,3",
		},
		{
			name: "enumerate",
			code: `console.log(enumerate(['a', 'b']).length)`,
			want: "2",
		},
		{
			name: "zip",
			code: `console.log(zip([1, 2], ['a', 'b']).length)`,
			want: "2",
		},
		{
			name: "any",
			code: `console.log(any([false, true, false]))`,
			want: "true",
		},
		{
			name: "all",
			code: `console.log(all([true, true, true]))`,
			want: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repl.Execute(tt.code, map[string]interface{}{})
			if err != nil {
				t.Errorf("Execute() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Execute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestREPLExecutor_OutputTruncation(t *testing.T) {
	repl := NewREPLExecutor()
	repl.maxOutputChars = 50

	longOutput := strings.Repeat("x", 100)
	code := `console.log("` + longOutput + `")`

	got, err := repl.Execute(code, map[string]interface{}{})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
		return
	}

	if len(got) <= repl.maxOutputChars {
		t.Errorf("Expected truncation, but got length %d", len(got))
	}

	if !strings.Contains(got, "[Output truncated") {
		t.Errorf("Expected truncation message in output")
	}
}

func TestREPLExecutor_ErrorHandling(t *testing.T) {
	repl := NewREPLExecutor()

	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{
			name:    "Syntax error",
			code:    `const x = ;`,
			wantErr: true,
		},
		{
			name:    "Reference error",
			code:    `console.log(undefinedVariable)`,
			wantErr: true,
		},
		{
			name:    "Valid code",
			code:    `console.log("ok")`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repl.Execute(tt.code, map[string]interface{}{})
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegexHelper(t *testing.T) {
	re := NewRegexHelper()

	t.Run("findall", func(t *testing.T) {
		result := re["findall"]("ERROR", "ERROR 1 ERROR 2 WARNING")
		matches, ok := result.([]string)
		if !ok {
			t.Fatal("Expected []string from findall")
		}
		if len(matches) != 2 {
			t.Errorf("Expected 2 matches, got %d", len(matches))
		}
	})

	t.Run("search", func(t *testing.T) {
		result := re["search"]("ERROR", "INFO ERROR WARNING")
		match, ok := result.(string)
		if !ok {
			t.Fatal("Expected string from search")
		}
		if match != "ERROR" {
			t.Errorf("Expected 'ERROR', got %q", match)
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := re["search"]("ERROR", "INFO WARNING")
		match, ok := result.(string)
		if !ok {
			t.Fatal("Expected string from search")
		}
		if match != "" {
			t.Errorf("Expected empty string, got %q", match)
		}
	})
}
