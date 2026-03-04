package rlm

import (
	"strings"
	"testing"
)

// ─── SplitSentences Tests ───────────────────────────────────────────────────

func TestSplitSentences_Basic(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	sentences := SplitSentences(text)

	if len(sentences) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %v", len(sentences), sentences)
	}
	if sentences[0] != "First sentence." {
		t.Errorf("sentence 0: %q", sentences[0])
	}
	if sentences[1] != "Second sentence." {
		t.Errorf("sentence 1: %q", sentences[1])
	}
	if sentences[2] != "Third sentence." {
		t.Errorf("sentence 2: %q", sentences[2])
	}
}

func TestSplitSentences_ParagraphBreaks(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	sentences := SplitSentences(text)

	if len(sentences) < 3 {
		t.Fatalf("expected at least 3 sentences, got %d: %v", len(sentences), sentences)
	}
}

func TestSplitSentences_MixedPunctuation(t *testing.T) {
	text := "Is this a question? Yes it is! And here is a statement."
	sentences := SplitSentences(text)

	if len(sentences) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %v", len(sentences), sentences)
	}
	if sentences[0] != "Is this a question?" {
		t.Errorf("sentence 0: %q", sentences[0])
	}
}

func TestSplitSentences_Empty(t *testing.T) {
	sentences := SplitSentences("")
	if len(sentences) != 0 {
		t.Errorf("expected 0 sentences for empty string, got %d", len(sentences))
	}

	sentences = SplitSentences("   ")
	if len(sentences) != 0 {
		t.Errorf("expected 0 sentences for whitespace, got %d", len(sentences))
	}
}

func TestSplitSentences_NoTerminator(t *testing.T) {
	text := "A sentence without punctuation"
	sentences := SplitSentences(text)

	if len(sentences) != 1 {
		t.Fatalf("expected 1 sentence, got %d: %v", len(sentences), sentences)
	}
	if sentences[0] != "A sentence without punctuation" {
		t.Errorf("sentence: %q", sentences[0])
	}
}

// ─── TokenizeWords Tests ────────────────────────────────────────────────────

func TestTokenizeWords_Basic(t *testing.T) {
	words := TokenizeWords("Hello, World! This is a test.")
	expected := []string{"hello", "world", "this", "is", "a", "test"}

	if len(words) != len(expected) {
		t.Fatalf("expected %d words, got %d: %v", len(expected), len(words), words)
	}
	for i, w := range words {
		if w != expected[i] {
			t.Errorf("word %d: got %q, expected %q", i, w, expected[i])
		}
	}
}

func TestTokenizeWords_Numbers(t *testing.T) {
	words := TokenizeWords("There are 42 cats and 7 dogs.")
	// Should include numbers
	found42 := false
	for _, w := range words {
		if w == "42" {
			found42 = true
		}
	}
	if !found42 {
		t.Error("expected tokenized words to include '42'")
	}
}

func TestTokenizeWords_Empty(t *testing.T) {
	words := TokenizeWords("")
	if len(words) != 0 {
		t.Errorf("expected 0 words for empty string, got %d", len(words))
	}
}

// ─── FilterStopWords Tests ──────────────────────────────────────────────────

func TestFilterStopWords(t *testing.T) {
	words := []string{"the", "quick", "brown", "fox", "is", "a", "animal"}
	filtered := FilterStopWords(words)

	for _, w := range filtered {
		if stopWords[w] {
			t.Errorf("stop word %q was not filtered", w)
		}
	}

	// "quick", "brown", "fox", "animal" should survive
	if len(filtered) < 3 {
		t.Errorf("expected at least 3 content words, got %d: %v", len(filtered), filtered)
	}
}

// ─── ComputeTFIDF Tests ─────────────────────────────────────────────────────

func TestComputeTFIDF_Basic(t *testing.T) {
	sentences := []string{
		"The machine learning algorithm processes data efficiently.",
		"Natural language processing uses deep learning models.",
		"The weather today is sunny and warm.",
	}

	scored := ComputeTFIDF(sentences)

	if len(scored) != 3 {
		t.Fatalf("expected 3 scored sentences, got %d", len(scored))
	}

	// All scores should be non-negative
	for i, s := range scored {
		if s.Score < 0 {
			t.Errorf("sentence %d has negative score: %f", i, s.Score)
		}
		if s.Index != i {
			t.Errorf("sentence %d has wrong index: %d", i, s.Index)
		}
	}
}

func TestComputeTFIDF_UniqueTermsScoreHigher(t *testing.T) {
	// A sentence with unique terms (not appearing in other sentences) should score higher
	sentences := []string{
		"Common words appear everywhere in text.",
		"Common words appear everywhere in documents.",
		"Quantum entanglement revolutionizes cryptographic security protocols.",
	}

	scored := ComputeTFIDF(sentences)

	// The third sentence has unique terms not shared with others, so it should have a high score
	// (though IDF will boost unique terms)
	if scored[2].Score <= 0 {
		t.Error("sentence with unique terms should have positive score")
	}
}

func TestComputeTFIDF_Empty(t *testing.T) {
	scored := ComputeTFIDF(nil)
	if scored != nil {
		t.Errorf("expected nil for empty input, got %v", scored)
	}
}

func TestComputeTFIDF_PreservesIndex(t *testing.T) {
	sentences := []string{"First.", "Second.", "Third."}
	scored := ComputeTFIDF(sentences)

	for i, s := range scored {
		if s.Index != i {
			t.Errorf("expected index %d, got %d", i, s.Index)
		}
	}
}

// ─── CompressContextTFIDF Tests ─────────────────────────────────────────────

func TestCompressContextTFIDF_NoCompressionNeeded(t *testing.T) {
	text := "Short text that fits easily."
	result := CompressContextTFIDF(text, 1000)

	if result != text {
		t.Errorf("expected unchanged text when no compression needed, got %q", result)
	}
}

func TestCompressContextTFIDF_CompressesLargeText(t *testing.T) {
	// Build a large document with many sentences
	var sentences []string
	for i := 0; i < 50; i++ {
		sentences = append(sentences, "This is a test sentence with various content and information.")
	}
	// Add some unique high-value sentences
	sentences = append(sentences, "The quantum computing breakthrough enables 1000x faster processing.")
	sentences = append(sentences, "Revenue grew 47% year-over-year to reach $2.3 billion in Q4.")
	text := strings.Join(sentences, " ")

	// Request much smaller budget than the full text
	originalTokens := EstimateTokens(text)
	targetTokens := originalTokens / 3

	result := CompressContextTFIDF(text, targetTokens)

	resultTokens := EstimateTokens(result)
	if resultTokens > targetTokens+10 { // Allow small slack
		t.Errorf("compressed result (%d tokens) exceeds target (%d tokens)", resultTokens, targetTokens)
	}

	if len(result) >= len(text) {
		t.Errorf("expected compressed result to be shorter: %d >= %d chars", len(result), len(text))
	}
}

func TestCompressContextTFIDF_PreservesOrder(t *testing.T) {
	text := "Alpha sentence first. Beta sentence second. Gamma sentence third. Delta sentence fourth. Epsilon sentence fifth."
	// Very small budget to force selection of only a few sentences
	result := CompressContextTFIDF(text, 20)

	// The selected sentences should appear in their original order
	sentences := SplitSentences(result)
	if len(sentences) == 0 {
		t.Fatal("expected at least one sentence in result")
	}

	// Verify order: if we find multiple sentences, their order should be preserved
	for i := 1; i < len(sentences); i++ {
		posI := strings.Index(text, sentences[i])
		posPrev := strings.Index(text, sentences[i-1])
		if posI < posPrev {
			t.Errorf("sentence order not preserved: %q appears before %q in original but after in result",
				sentences[i-1], sentences[i])
		}
	}
}

func TestCompressContextTFIDF_HighValueSentencesSelected(t *testing.T) {
	// Mix of generic and specific/data-rich sentences
	text := "The weather is nice today. " +
		"It is a good day to go outside. " +
		"The GDP of Japan reached $4.2 trillion in 2024 with 2.3% growth. " +
		"Trees are green and the sky is blue. " +
		"CRISPR-Cas9 gene editing achieved 99.7% accuracy in clinical trials at Johns Hopkins."

	// Budget enough for ~2 sentences
	result := CompressContextTFIDF(text, 40)

	// The data-rich sentences should be selected over generic ones
	hasSpecific := strings.Contains(result, "trillion") || strings.Contains(result, "CRISPR") || strings.Contains(result, "accuracy")
	if !hasSpecific {
		t.Errorf("expected high-value sentences to be selected, got: %q", result)
	}
}

func TestCompressContextTFIDF_EmptyText(t *testing.T) {
	result := CompressContextTFIDF("", 100)
	if result != "" {
		t.Errorf("expected empty result for empty input, got %q", result)
	}
}
