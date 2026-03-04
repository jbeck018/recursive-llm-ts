package rlm

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// ─── TF-IDF Extractive Context Compression ──────────────────────────────────
//
// Pure Go, zero external dependencies, zero API calls.
// Scores sentences by Term Frequency - Inverse Document Frequency,
// selects top-K sentences that fit within a token budget,
// and preserves original document order.

// ScoredSentence holds a sentence with its TF-IDF score and original position.
type ScoredSentence struct {
	Text  string
	Score float64
	Index int // original position in the document (for order preservation)
}

// SplitSentences breaks text into sentences using punctuation boundaries.
// Handles ". ", "! ", "? " as sentence terminators, plus paragraph breaks.
func SplitSentences(text string) []string {
	if len(strings.TrimSpace(text)) == 0 {
		return nil
	}

	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		current.WriteRune(r)

		// Check for sentence-ending punctuation followed by space/newline/end
		isSentenceEnd := false
		if r == '.' || r == '!' || r == '?' {
			if i+1 >= len(runes) {
				isSentenceEnd = true
			} else {
				next := runes[i+1]
				if next == ' ' || next == '\n' || next == '\r' || next == '\t' {
					isSentenceEnd = true
				}
			}
		}

		// Also split on double newline (paragraph boundary)
		if r == '\n' && i+1 < len(runes) && runes[i+1] == '\n' {
			s := strings.TrimSpace(current.String())
			if len(s) > 0 {
				sentences = append(sentences, s)
			}
			current.Reset()
			i++ // skip the second newline
			continue
		}

		if isSentenceEnd {
			s := strings.TrimSpace(current.String())
			if len(s) > 0 {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}

	// Flush remaining text
	if remaining := strings.TrimSpace(current.String()); len(remaining) > 0 {
		sentences = append(sentences, remaining)
	}

	return sentences
}

// TokenizeWords splits text into lowercase word tokens, filtering non-alphanumeric characters.
func TokenizeWords(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// stopWords contains common English stop words to optionally filter for better TF-IDF scoring.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true, "have": true, "has": true,
	"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "shall": true,
	"can": true, "this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "i": true, "you": true, "he": true, "she": true,
	"we": true, "they": true, "me": true, "him": true, "her": true, "us": true,
	"them": true, "my": true, "your": true, "his": true, "our": true, "their": true,
	"not": true, "no": true, "if": true, "as": true, "so": true, "than": true,
	"then": true, "also": true, "just": true, "about": true, "into": true,
}

// FilterStopWords removes common stop words from a word list.
func FilterStopWords(words []string) []string {
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if !stopWords[w] && len(w) > 1 {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// ComputeTFIDF computes TF-IDF scores for each sentence in a document.
// Returns ScoredSentence slice with scores and original indices.
func ComputeTFIDF(sentences []string) []ScoredSentence {
	if len(sentences) == 0 {
		return nil
	}

	// Tokenize each sentence
	docWords := make([][]string, len(sentences))
	for i, s := range sentences {
		words := TokenizeWords(s)
		docWords[i] = FilterStopWords(words)
	}

	// Compute document frequency (how many sentences contain each word)
	df := make(map[string]int)
	for _, words := range docWords {
		seen := make(map[string]bool)
		for _, w := range words {
			if !seen[w] {
				df[w]++
				seen[w] = true
			}
		}
	}

	n := float64(len(sentences))

	// Score each sentence by TF-IDF
	scored := make([]ScoredSentence, len(sentences))
	for i, words := range docWords {
		score := 0.0
		tf := make(map[string]int)
		for _, w := range words {
			tf[w]++
		}
		for word, freq := range tf {
			// TF: term frequency in this sentence
			// IDF: log(N / df) where N = number of sentences
			idf := math.Log(n / float64(df[word]))
			score += float64(freq) * idf
		}
		// Normalize by sentence length to avoid bias toward long sentences
		if len(words) > 0 {
			score /= math.Sqrt(float64(len(words)))
		}
		scored[i] = ScoredSentence{
			Text:  sentences[i],
			Score: score,
			Index: i,
		}
	}

	return scored
}

// CompressContextTFIDF reduces context to fit within a token budget using
// extractive summarization via TF-IDF sentence scoring.
// Preserves original sentence order in the output.
// Returns the original context unchanged if it already fits.
func CompressContextTFIDF(text string, targetTokens int) string {
	if EstimateTokens(text) <= targetTokens {
		return text
	}

	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return text
	}

	scored := ComputeTFIDF(sentences)

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Greedily select top sentences until budget is reached
	var selected []ScoredSentence
	currentTokens := 0
	for _, s := range scored {
		sentTokens := EstimateTokens(s.Text)
		if currentTokens+sentTokens > targetTokens {
			continue
		}
		selected = append(selected, s)
		currentTokens += sentTokens
	}

	if len(selected) == 0 {
		// Budget too small for even one sentence - truncate the highest-scored
		if len(scored) > 0 {
			maxChars := targetTokens * 3 // Conservative chars/token
			if maxChars > len(scored[0].Text) {
				maxChars = len(scored[0].Text)
			}
			return scored[0].Text[:maxChars]
		}
		return text
	}

	// Re-sort by original index to preserve document order
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Index < selected[j].Index
	})

	parts := make([]string, len(selected))
	for i, s := range selected {
		parts[i] = s.Text
	}
	return strings.Join(parts, " ")
}
