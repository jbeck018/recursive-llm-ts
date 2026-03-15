package rlm

import (
	"encoding/json"
	"strings"
)

// ─── Shared JSON Extraction Utilities ───────────────────────────────────────
// Consolidated from structured.go and lcm_map.go to eliminate duplication.
// Both the structured output parser and the LLM-Map operator need to extract
// valid JSON from LLM responses that may contain markdown, explanatory text,
// or malformed output.

// StripMarkdownCodeBlock removes markdown ``` fencing from LLM output.
func StripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 3 {
			s = strings.Join(lines[1:len(lines)-1], "\n")
			s = strings.TrimSpace(s)
		}
	}
	return s
}

// ExtractBalancedBraces finds the first balanced JSON object or array
// starting with startChar ('{' or '['). Handles nested structures,
// string escaping, and arbitrary depth.
// Returns the balanced substring or "" if no balanced match is found.
func ExtractBalancedBraces(s string, startChar byte) string {
	endChar := byte('}')
	if startChar == '[' {
		endChar = ']'
	}

	depth := 0
	inString := false
	escape := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inString {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case startChar:
			depth++
		case endChar:
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

// ExtractAllBalancedJSON finds all top-level JSON objects in a string by tracking
// balanced braces. Handles arbitrary nesting depth and string escaping.
// Used by structured output parsing which needs all candidates for schema matching.
func ExtractAllBalancedJSON(s string) []string {
	var results []string
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '{' && !inString {
			// Found start of a potential JSON object; extract balanced match
			balanced := ExtractBalancedBraces(s[i:], '{')
			if balanced != "" {
				results = append(results, balanced)
				i += len(balanced) - 1 // skip past this object
				continue
			}
		}

		// Track string state in the outer scan (for skipping { inside strings)
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
		}
	}

	return results
}

// ExtractFirstJSON finds the first valid JSON object or array in a string.
// Tries full content first, then searches for { or [ and attempts balanced extraction.
// Returns nil if no valid JSON is found.
func ExtractFirstJSON(content string) json.RawMessage {
	content = StripMarkdownCodeBlock(content)

	// Try to parse the whole content as JSON
	var js json.RawMessage
	if err := json.Unmarshal([]byte(content), &js); err == nil {
		return js
	}

	// Find first { or [ and try balanced extraction
	for _, startChar := range []byte{'{', '['} {
		idx := strings.IndexByte(content, startChar)
		if idx >= 0 {
			sub := content[idx:]
			// Try full remainder first
			if err := json.Unmarshal([]byte(sub), &js); err == nil {
				return js
			}
			// Try balanced brace extraction
			if balanced := ExtractBalancedBraces(sub, startChar); balanced != "" {
				if err := json.Unmarshal([]byte(balanced), &js); err == nil {
					return js
				}
			}
		}
	}

	return nil
}
