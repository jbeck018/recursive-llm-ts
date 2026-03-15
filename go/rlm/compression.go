package rlm

// ─── Shared Text Compression Utilities ──────────────────────────────────────
// Consolidated from context_overflow.go and lcm_summarizer.go to eliminate
// duplication of the "keep start + end, truncate middle" strategy.

// TruncateTextParams configures deterministic text truncation.
type TruncateTextParams struct {
	// MaxTokens is the target token count.
	MaxTokens int
	// MarkerText is inserted at the truncation point.
	// Default: "\n[... content truncated ...]\n"
	MarkerText string
	// StartFraction is the fraction of budget kept from the start (default: 2/3).
	StartFraction float64
}

// TruncateText performs deterministic truncation using the "keep start + end"
// strategy, which addresses the "lost in the middle" problem by preserving
// both the beginning and end of the text.
//
// This is used as the guaranteed-convergence fallback in both:
// - LCM's three-level escalation (Level 3)
// - Context overflow's truncation strategy
func TruncateText(text string, params TruncateTextParams) string {
	// Apply defaults
	if params.StartFraction <= 0 || params.StartFraction >= 1 {
		params.StartFraction = 2.0 / 3.0
	}
	if params.MarkerText == "" {
		params.MarkerText = "\n[... content truncated ...]\n"
	}

	// Conservative chars-to-tokens ratio: 3 chars per token
	maxChars := params.MaxTokens * 3
	if len(text) <= maxChars {
		return text
	}

	startChars := int(float64(maxChars) * params.StartFraction)
	endChars := maxChars - startChars

	// Guard against edge cases
	if startChars > len(text) {
		startChars = len(text)
	}
	if endChars > len(text)-startChars {
		endChars = len(text) - startChars
	}
	if endChars < 0 {
		endChars = 0
	}

	if endChars == 0 {
		return text[:startChars] + params.MarkerText
	}

	return text[:startChars] + params.MarkerText + text[len(text)-endChars:]
}
