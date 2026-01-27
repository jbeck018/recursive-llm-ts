package rlm

import (
	"testing"
)

func TestIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{"FINAL with double quotes", `FINAL("answer")`, true},
		{"FINAL with single quotes", `FINAL('answer')`, true},
		{"FINAL with triple double quotes", `FINAL("""answer""")`, true},
		{"FINAL_VAR", `FINAL_VAR(result)`, true},
		{"No FINAL", `x = 1`, false},
		{"Contains FINAL as substring", `This is FINALLY done`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFinal(tt.response); got != tt.want {
				t.Errorf("IsFinal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractFinal(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
		wantOk   bool
	}{
		{
			name:     "Double quotes",
			response: `FINAL("The answer is 42")`,
			want:     "The answer is 42",
			wantOk:   true,
		},
		{
			name:     "Single quotes",
			response: `FINAL('The answer is 42')`,
			want:     "The answer is 42",
			wantOk:   true,
		},
		{
			name:     "Triple double quotes",
			response: `FINAL("""The answer is 42""")`,
			want:     "The answer is 42",
			wantOk:   true,
		},
		{
			name:     "Triple single quotes",
			response: `FINAL('''The answer is 42''')`,
			want:     "The answer is 42",
			wantOk:   true,
		},
		{
			name: "Multiline with triple quotes",
			response: `FINAL("""Line 1
Line 2
Line 3""")`,
			want:   "Line 1\nLine 2\nLine 3",
			wantOk: true,
		},
		{
			name:     "With whitespace",
			response: `FINAL(  "The answer"  )`,
			want:     "The answer",
			wantOk:   true,
		},
		{
			name:     "No FINAL",
			response: `x = 1`,
			want:     "",
			wantOk:   false,
		},
		{
			name:     "FINAL_VAR should not match",
			response: `FINAL_VAR(result)`,
			want:     "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOk := extractFinal(tt.response)
			if gotOk != tt.wantOk {
				t.Errorf("extractFinal() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("extractFinal() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractFinalVar(t *testing.T) {
	tests := []struct {
		name     string
		response string
		env      map[string]interface{}
		want     string
		wantOk   bool
	}{
		{
			name:     "Simple variable",
			response: `FINAL_VAR(result)`,
			env:      map[string]interface{}{"result": "The answer"},
			want:     "The answer",
			wantOk:   true,
		},
		{
			name:     "Integer variable",
			response: `FINAL_VAR(count)`,
			env:      map[string]interface{}{"count": 42},
			want:     "42",
			wantOk:   true,
		},
		{
			name:     "Variable not found",
			response: `FINAL_VAR(missing)`,
			env:      map[string]interface{}{"result": "The answer"},
			want:     "",
			wantOk:   false,
		},
		{
			name:     "With whitespace",
			response: `FINAL_VAR( result )`,
			env:      map[string]interface{}{"result": "The answer"},
			want:     "The answer",
			wantOk:   true,
		},
		{
			name:     "No FINAL_VAR",
			response: `x = 1`,
			env:      map[string]interface{}{},
			want:     "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOk := extractFinalVar(tt.response, tt.env)
			if gotOk != tt.wantOk {
				t.Errorf("extractFinalVar() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("extractFinalVar() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		env      map[string]interface{}
		want     string
		wantOk   bool
	}{
		{
			name:     "FINAL takes precedence",
			response: `FINAL("Direct answer")`,
			env:      map[string]interface{}{"result": "Var answer"},
			want:     "Direct answer",
			wantOk:   true,
		},
		{
			name:     "FINAL_VAR fallback",
			response: `FINAL_VAR(result)`,
			env:      map[string]interface{}{"result": "Var answer"},
			want:     "Var answer",
			wantOk:   true,
		},
		{
			name:     "Neither",
			response: `x = 1`,
			env:      map[string]interface{}{},
			want:     "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOk := ParseResponse(tt.response, tt.env)
			if gotOk != tt.wantOk {
				t.Errorf("ParseResponse() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("ParseResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}
