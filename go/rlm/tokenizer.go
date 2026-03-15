package rlm

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

// ─── Tokenizer Interface ─────────────────────────────────────────────────────
// Provides accurate token counting with model-specific BPE encoding.
// Replaces the heuristic ~3.5 chars/token estimation with real tokenization.

// Tokenizer counts tokens in text strings.
type Tokenizer interface {
	CountTokens(text string) int
}

// ─── Tiktoken BPE Tokenizer ──────────────────────────────────────────────────

// TiktokenTokenizer uses the tiktoken BPE encoding for accurate token counting.
type TiktokenTokenizer struct {
	encoding *tiktoken.Tiktoken
	name     string
}

// modelEncodingMap maps model name prefixes to their tiktoken encoding names.
var modelEncodingMap = map[string]string{
	// OpenAI o200k_base models
	"gpt-4o":     "o200k_base",
	"gpt-4o-":    "o200k_base",
	"o1":         "o200k_base",
	"o3":         "o200k_base",
	"o4":         "o200k_base",
	// OpenAI cl100k_base models
	"gpt-4-":     "cl100k_base",
	"gpt-4":      "cl100k_base",
	"gpt-3.5":    "cl100k_base",
	// Anthropic (closest approximation)
	"claude":     "cl100k_base",
	// Meta Llama
	"llama":      "cl100k_base",
	// Mistral
	"mistral":    "cl100k_base",
	"mixtral":    "cl100k_base",
	// Qwen
	"qwen":       "cl100k_base",
}

// encodingForModel returns the tiktoken encoding name for a given model.
func encodingForModel(model string) string {
	lower := strings.ToLower(model)

	// Try exact match first
	if enc, ok := modelEncodingMap[lower]; ok {
		return enc
	}

	// Try prefix matching (longest prefix wins)
	bestMatch := ""
	bestEnc := "cl100k_base"
	for prefix, enc := range modelEncodingMap {
		if strings.HasPrefix(lower, prefix) && len(prefix) > len(bestMatch) {
			bestMatch = prefix
			bestEnc = enc
		}
	}

	return bestEnc
}

// NewTiktokenTokenizer creates a tokenizer using the appropriate BPE encoding for the model.
// Returns nil and an error if the encoding cannot be loaded.
func NewTiktokenTokenizer(model string) (*TiktokenTokenizer, error) {
	encName := encodingForModel(model)
	enc, err := tiktoken.GetEncoding(encName)
	if err != nil {
		return nil, err
	}
	return &TiktokenTokenizer{
		encoding: enc,
		name:     encName,
	}, nil
}

// CountTokens returns the exact BPE token count for the text.
func (t *TiktokenTokenizer) CountTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// EncodingName returns the name of the encoding used.
func (t *TiktokenTokenizer) EncodingName() string {
	return t.name
}

// ─── Heuristic Tokenizer (Fallback) ──────────────────────────────────────────

// HeuristicTokenizer uses the character-to-token ratio heuristic.
// This is the original EstimateTokens logic, kept as a fallback
// when tiktoken encodings are unavailable.
type HeuristicTokenizer struct{}

// CountTokens provides a fast approximation of token count.
// Uses ~3.5 chars/token ratio, intentionally conservative (over-estimates).
func (h *HeuristicTokenizer) CountTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text)*10 + 34) / 35
}

// ─── Cached Tokenizer ────────────────────────────────────────────────────────

const maxCacheSize = 10000

// CachedTokenizer wraps another Tokenizer with an LRU-style cache.
// Uses xxhash for fast key hashing and sync.Map for concurrent access.
type CachedTokenizer struct {
	inner    Tokenizer
	cache    sync.Map // map[uint64]int
	size     atomic.Int64
}

// NewCachedTokenizer wraps a tokenizer with caching.
func NewCachedTokenizer(inner Tokenizer) *CachedTokenizer {
	return &CachedTokenizer{
		inner: inner,
	}
}

// CountTokens returns the cached token count, computing and caching on miss.
func (c *CachedTokenizer) CountTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	// Hash the text for cache key
	key := xxhash.Sum64String(text)

	// Check cache
	if val, ok := c.cache.Load(key); ok {
		return val.(int)
	}

	// Compute
	count := c.inner.CountTokens(text)

	// Store if under max size; evict all if over (simple strategy)
	if c.size.Load() < maxCacheSize {
		if _, loaded := c.cache.LoadOrStore(key, count); !loaded {
			c.size.Add(1)
		}
	} else {
		// Simple eviction: clear the cache when full
		c.cache.Clear()
		c.size.Store(0)
		c.cache.Store(key, count)
		c.size.Add(1)
	}

	return count
}

// CacheSize returns the current number of cached entries.
func (c *CachedTokenizer) CacheSize() int64 {
	return c.size.Load()
}

// Inner returns the underlying tokenizer.
func (c *CachedTokenizer) Inner() Tokenizer {
	return c.inner
}

// ─── Global Default Tokenizer ────────────────────────────────────────────────

var (
	defaultTokenizer Tokenizer = &HeuristicTokenizer{}
	tokenizerMu      sync.RWMutex
)

// SetDefaultTokenizer configures the global tokenizer for a given model.
// Tries tiktoken BPE first, falls back to heuristic on failure.
// The tokenizer is wrapped with caching for performance.
func SetDefaultTokenizer(model string) {
	tokenizerMu.Lock()
	defer tokenizerMu.Unlock()

	tok, err := NewTiktokenTokenizer(model)
	if err != nil {
		// Fall back to heuristic with caching
		defaultTokenizer = NewCachedTokenizer(&HeuristicTokenizer{})
		return
	}

	defaultTokenizer = NewCachedTokenizer(tok)
}

// GetTokenizer returns the current global tokenizer.
func GetTokenizer() Tokenizer {
	tokenizerMu.RLock()
	defer tokenizerMu.RUnlock()
	return defaultTokenizer
}

// ResetDefaultTokenizer resets to the heuristic tokenizer (used in tests).
func ResetDefaultTokenizer() {
	tokenizerMu.Lock()
	defer tokenizerMu.Unlock()
	defaultTokenizer = &HeuristicTokenizer{}
}
