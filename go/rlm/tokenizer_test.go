package rlm

import (
	"strings"
	"testing"
)

// ─── Tiktoken BPE Tokenizer Tests ────────────────────────────────────────────

func TestTiktokenTokenizer_English(t *testing.T) {
	tok, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	text := "Hello, world! This is a test of the tokenizer."
	bpeCount := tok.CountTokens(text)
	heuristic := (&HeuristicTokenizer{}).CountTokens(text)

	t.Logf("English text: BPE=%d, Heuristic=%d", bpeCount, heuristic)

	if bpeCount <= 0 {
		t.Error("BPE count should be > 0")
	}
	// BPE and heuristic should give different counts for most text
	// (they may coincidentally match for very short strings, so just verify BPE is reasonable)
	if bpeCount > len(text) {
		t.Errorf("BPE count %d should not exceed character count %d", bpeCount, len(text))
	}
}

func TestTiktokenTokenizer_Code(t *testing.T) {
	tok, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	code := `func main() {
	fmt.Println("Hello, World!")
	for i := 0; i < 100; i++ {
		result = append(result, processItem(items[i]))
	}
}`
	bpeCount := tok.CountTokens(code)
	heuristic := (&HeuristicTokenizer{}).CountTokens(code)

	t.Logf("Code: BPE=%d, Heuristic=%d (chars=%d)", bpeCount, heuristic, len(code))

	if bpeCount <= 0 {
		t.Error("BPE count should be > 0 for code")
	}
}

func TestTiktokenTokenizer_JSON(t *testing.T) {
	tok, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	jsonData := `{"name": "test", "values": [1, 2, 3, 4, 5], "nested": {"key": "value", "count": 42}}`
	bpeCount := tok.CountTokens(jsonData)
	heuristic := (&HeuristicTokenizer{}).CountTokens(jsonData)

	t.Logf("JSON: BPE=%d, Heuristic=%d (chars=%d)", bpeCount, heuristic, len(jsonData))

	if bpeCount <= 0 {
		t.Error("BPE count should be > 0 for JSON")
	}
}

func TestTiktokenTokenizer_CJK(t *testing.T) {
	tok, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	// Chinese, Japanese, and Korean text
	cjkText := "这是一个测试。日本語のテスト。한국어 테스트입니다。"
	bpeCount := tok.CountTokens(cjkText)
	heuristic := (&HeuristicTokenizer{}).CountTokens(cjkText)

	t.Logf("CJK text: BPE=%d, Heuristic=%d (chars=%d, bytes=%d)", bpeCount, heuristic, len([]rune(cjkText)), len(cjkText))

	if bpeCount <= 0 {
		t.Error("BPE count should be > 0 for CJK")
	}
	// CJK text has ~1.5 chars per token but heuristic assumes ~3.5
	// So BPE should count MORE tokens than heuristic for CJK
	if bpeCount <= heuristic {
		t.Logf("WARNING: BPE (%d) should typically be > heuristic (%d) for CJK text", bpeCount, heuristic)
	}
}

func TestTiktokenTokenizer_Empty(t *testing.T) {
	tok, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	if tok.CountTokens("") != 0 {
		t.Error("empty string should return 0 tokens")
	}
}

func TestTiktokenTokenizer_EncodingSelection(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o", "o200k_base"},
		{"gpt-4o-mini", "o200k_base"},
		{"gpt-4o-mini-2024-07-18", "o200k_base"},
		{"gpt-4", "cl100k_base"},
		{"gpt-4-turbo", "cl100k_base"},
		{"gpt-3.5-turbo", "cl100k_base"},
		{"claude-3-opus", "cl100k_base"},
		{"claude-sonnet-4", "cl100k_base"},
		{"o1", "o200k_base"},
		{"o3-mini", "o200k_base"},
		{"llama-3.1", "cl100k_base"},
		{"unknown-model", "cl100k_base"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			enc := encodingForModel(tt.model)
			if enc != tt.expected {
				t.Errorf("encodingForModel(%q) = %q, want %q", tt.model, enc, tt.expected)
			}
		})
	}
}

// ─── Heuristic Tokenizer Tests ───────────────────────────────────────────────

func TestHeuristicTokenizer_Fallback(t *testing.T) {
	h := &HeuristicTokenizer{}

	if h.CountTokens("") != 0 {
		t.Error("empty string should be 0")
	}

	// "Hello" = 5 chars, ceil(5/3.5) = 2
	count := h.CountTokens("Hello")
	if count <= 0 {
		t.Error("should count > 0 tokens for 'Hello'")
	}

	// Longer text
	longText := strings.Repeat("word ", 1000)
	longCount := h.CountTokens(longText)
	expected := (len(longText)*10 + 34) / 35
	if longCount != expected {
		t.Errorf("heuristic count %d != expected %d", longCount, expected)
	}
}

// ─── Cached Tokenizer Tests ─────────────────────────────────────────────────

func TestCachedTokenizer_CacheHit(t *testing.T) {
	callCount := 0
	inner := &countingTokenizer{fn: func(text string) int {
		callCount++
		return len(text) / 4
	}}

	cached := NewCachedTokenizer(inner)

	text := "This is a test string for caching"

	// First call: cache miss
	count1 := cached.CountTokens(text)
	if callCount != 1 {
		t.Errorf("expected 1 inner call, got %d", callCount)
	}

	// Second call: cache hit (inner should NOT be called again)
	count2 := cached.CountTokens(text)
	if callCount != 1 {
		t.Errorf("expected still 1 inner call after cache hit, got %d", callCount)
	}

	if count1 != count2 {
		t.Errorf("cache returned different values: %d vs %d", count1, count2)
	}

	if cached.CacheSize() != 1 {
		t.Errorf("cache size should be 1, got %d", cached.CacheSize())
	}
}

func TestCachedTokenizer_Empty(t *testing.T) {
	inner := &HeuristicTokenizer{}
	cached := NewCachedTokenizer(inner)

	if cached.CountTokens("") != 0 {
		t.Error("empty string should return 0 without caching")
	}
	if cached.CacheSize() != 0 {
		t.Error("cache should not store empty strings")
	}
}

func TestCachedTokenizer_DifferentStrings(t *testing.T) {
	inner := &HeuristicTokenizer{}
	cached := NewCachedTokenizer(inner)

	cached.CountTokens("string one")
	cached.CountTokens("string two")
	cached.CountTokens("string three")

	if cached.CacheSize() != 3 {
		t.Errorf("cache size should be 3, got %d", cached.CacheSize())
	}
}

func TestCachedTokenizer_Inner(t *testing.T) {
	inner := &HeuristicTokenizer{}
	cached := NewCachedTokenizer(inner)

	if cached.Inner() != inner {
		t.Error("Inner() should return the wrapped tokenizer")
	}
}

// countingTokenizer tracks how many times CountTokens is called.
type countingTokenizer struct {
	fn func(string) int
}

func (c *countingTokenizer) CountTokens(text string) int {
	return c.fn(text)
}

// ─── Global Default Tokenizer Tests ──────────────────────────────────────────

func TestSetDefaultTokenizer_KnownModel(t *testing.T) {
	defer ResetDefaultTokenizer()

	SetDefaultTokenizer("gpt-4o")
	tok := GetTokenizer()

	// Should be a CachedTokenizer wrapping a TiktokenTokenizer
	cached, ok := tok.(*CachedTokenizer)
	if !ok {
		t.Fatalf("expected CachedTokenizer, got %T", tok)
	}

	inner, ok := cached.Inner().(*TiktokenTokenizer)
	if !ok {
		t.Fatalf("expected inner TiktokenTokenizer, got %T", cached.Inner())
	}

	if inner.EncodingName() != "o200k_base" {
		t.Errorf("expected o200k_base encoding, got %s", inner.EncodingName())
	}

	// Verify it actually counts tokens
	count := tok.CountTokens("Hello, world!")
	if count <= 0 {
		t.Error("tokenizer should count > 0 tokens")
	}
}

func TestSetDefaultTokenizer_UnknownModel(t *testing.T) {
	defer ResetDefaultTokenizer()

	// Even unknown models should work because we default to cl100k_base
	SetDefaultTokenizer("totally-unknown-model-xyz")
	tok := GetTokenizer()

	count := tok.CountTokens("Hello, world!")
	if count <= 0 {
		t.Error("tokenizer should count > 0 tokens even for unknown model")
	}
}

func TestEstimateTokens_UsesDefaultTokenizer(t *testing.T) {
	defer ResetDefaultTokenizer()

	// With heuristic default
	heuristicCount := EstimateTokens("Hello, world! This is a test.")

	// Switch to BPE
	SetDefaultTokenizer("gpt-4o")
	bpeCount := EstimateTokens("Hello, world! This is a test.")

	t.Logf("EstimateTokens: heuristic=%d, bpe=%d", heuristicCount, bpeCount)

	// Both should be > 0
	if heuristicCount <= 0 || bpeCount <= 0 {
		t.Errorf("both counts should be > 0: heuristic=%d, bpe=%d", heuristicCount, bpeCount)
	}
}

func TestResetDefaultTokenizer(t *testing.T) {
	SetDefaultTokenizer("gpt-4o")
	ResetDefaultTokenizer()

	tok := GetTokenizer()
	_, ok := tok.(*HeuristicTokenizer)
	if !ok {
		t.Errorf("after reset, expected HeuristicTokenizer, got %T", tok)
	}
}
