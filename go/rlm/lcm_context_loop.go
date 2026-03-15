package rlm

import (
	"fmt"
	"sync"
)

// ─── LCM Context Control Loop ───────────────────────────────────────────────
// Implements the dual-threshold context management from the LCM paper:
// - Below τ_soft: no overhead (zero-cost continuity)
// - τ_soft ≤ tokens < τ_hard: async compaction between turns
// - tokens ≥ τ_hard: blocking compaction before next LLM call

// LCMConfig configures the Lossless Context Management engine.
type LCMConfig struct {
	// Enabled activates LCM context management (default: false for backward compat)
	Enabled bool `json:"enabled"`

	// SoftThreshold is τ_soft: token count above which async compaction begins.
	// Default: 70% of model limit.
	SoftThreshold int `json:"soft_threshold,omitempty"`

	// HardThreshold is τ_hard: token count above which blocking compaction occurs.
	// Default: 90% of model limit.
	HardThreshold int `json:"hard_threshold,omitempty"`

	// CompactionBlockSize is how many messages to compact at once.
	// Default: 10 messages.
	CompactionBlockSize int `json:"compaction_block_size,omitempty"`

	// SummaryTargetTokens is the target size for each summary node.
	// Default: 500 tokens.
	SummaryTargetTokens int `json:"summary_target_tokens,omitempty"`
}

// DefaultLCMConfig returns default LCM configuration.
func DefaultLCMConfig() LCMConfig {
	return LCMConfig{
		Enabled:             false,
		CompactionBlockSize: 10,
		SummaryTargetTokens: 500,
	}
}

// LCMEngine is the main LCM context management engine.
// It wraps the store, summarizer, and context control loop.
type LCMEngine struct {
	config     LCMConfig
	store      *LCMStore
	summarizer *LCMSummarizer
	observer   *Observer
	modelLimit int

	// Async compaction state
	compactMu     sync.Mutex
	compacting    bool
	compactResult chan *compactionResult
}

type compactionResult struct {
	summary *SummaryNode
	err     error
}

// NewLCMEngine creates a new LCM engine with the given configuration.
func NewLCMEngine(config LCMConfig, store *LCMStore, summarizer *LCMSummarizer, observer *Observer, modelLimit int) *LCMEngine {
	// Apply defaults based on model limit
	if config.SoftThreshold == 0 && modelLimit > 0 {
		config.SoftThreshold = int(float64(modelLimit) * 0.70)
	}
	if config.HardThreshold == 0 && modelLimit > 0 {
		config.HardThreshold = int(float64(modelLimit) * 0.90)
	}
	if config.CompactionBlockSize == 0 {
		config.CompactionBlockSize = 10
	}
	if config.SummaryTargetTokens == 0 {
		config.SummaryTargetTokens = 500
	}

	return &LCMEngine{
		config:     config,
		store:      store,
		summarizer: summarizer,
		observer:   observer,
		modelLimit: modelLimit,
	}
}

// ─── Context Control Loop (Algorithm 2 from paper) ──────────────────────────

// OnNewItem is called after each new message is added to the store.
// It implements the context control loop from Figure 2 of the LCM paper.
// Returns nil if no compaction was needed or if async compaction was triggered.
func (e *LCMEngine) OnNewItem() error {
	if !e.config.Enabled {
		return nil
	}

	// Check if async compaction has completed
	e.applyPendingCompaction()

	tokens := e.store.ActiveContextTokens()

	// Below soft threshold: zero-cost continuity
	if tokens <= e.config.SoftThreshold {
		return nil
	}

	// Soft threshold exceeded: trigger async compaction (non-blocking)
	if tokens < e.config.HardThreshold {
		e.observer.Debug("lcm.control", "Soft threshold exceeded (%d > %d), triggering async compaction",
			tokens, e.config.SoftThreshold)
		e.triggerAsyncCompaction()
		return nil
	}

	// Hard threshold exceeded: blocking compaction
	e.observer.Debug("lcm.control", "Hard threshold exceeded (%d >= %d), blocking compaction",
		tokens, e.config.HardThreshold)
	return e.blockingCompaction()
}

// ─── Async Compaction ───────────────────────────────────────────────────────

func (e *LCMEngine) triggerAsyncCompaction() {
	e.compactMu.Lock()
	if e.compacting {
		e.compactMu.Unlock()
		return // Already compacting
	}
	e.compacting = true
	e.compactResult = make(chan *compactionResult, 1)
	e.compactMu.Unlock()

	go func() {
		result := e.performCompaction()
		e.compactResult <- result
	}()
}

func (e *LCMEngine) applyPendingCompaction() {
	e.compactMu.Lock()
	if !e.compacting || e.compactResult == nil {
		e.compactMu.Unlock()
		return
	}

	// Non-blocking check
	select {
	case result := <-e.compactResult:
		e.compacting = false
		e.compactMu.Unlock()

		if result.err != nil {
			e.observer.Error("lcm.control", "Async compaction failed: %v", result.err)
			return
		}
		if result.summary != nil {
			removed := e.store.CompactOldestBlock(result.summary)
			e.observer.Debug("lcm.control", "Async compaction applied: replaced %d messages with summary %s",
				len(removed), result.summary.ID)
			e.observer.Event("lcm.compaction", map[string]string{
				"type":           "async",
				"summary_id":     result.summary.ID,
				"messages_compacted": fmt.Sprintf("%d", len(removed)),
				"summary_tokens": fmt.Sprintf("%d", result.summary.Tokens),
				"level":          fmt.Sprintf("%d", result.summary.Level),
			})
		}
	default:
		e.compactMu.Unlock()
		// Not done yet, continue
	}
}

// ─── Blocking Compaction ────────────────────────────────────────────────────

func (e *LCMEngine) blockingCompaction() error {
	// Keep compacting until under hard threshold
	for e.store.ActiveContextTokens() >= e.config.HardThreshold {
		result := e.performCompaction()
		if result.err != nil {
			return fmt.Errorf("blocking compaction failed: %w", result.err)
		}
		if result.summary == nil {
			break // Nothing more to compact
		}

		removed := e.store.CompactOldestBlock(result.summary)
		e.observer.Debug("lcm.control", "Blocking compaction: replaced %d messages with summary %s (%d tokens)",
			len(removed), result.summary.ID, result.summary.Tokens)
		e.observer.Event("lcm.compaction", map[string]string{
			"type":               "blocking",
			"summary_id":         result.summary.ID,
			"messages_compacted": fmt.Sprintf("%d", len(removed)),
			"summary_tokens":     fmt.Sprintf("%d", result.summary.Tokens),
			"level":              fmt.Sprintf("%d", result.summary.Level),
		})
	}
	return nil
}

// ─── Core Compaction ────────────────────────────────────────────────────────

func (e *LCMEngine) performCompaction() *compactionResult {
	active := e.store.GetActiveContext()

	// Find the oldest block of raw messages to compact (skip system prompt)
	var block []*StoreMessage
	for _, item := range active {
		if item.IsMessage() {
			if item.Message.Role == RoleSystem {
				continue // Never compact system prompt
			}
			block = append(block, item.Message)
			if len(block) >= e.config.CompactionBlockSize {
				break
			}
		}
	}

	if len(block) == 0 {
		return &compactionResult{summary: nil, err: nil}
	}

	// Apply three-level escalation
	result, err := e.summarizer.SummarizeMessages(block, e.config.SummaryTargetTokens)
	if err != nil {
		return &compactionResult{err: err}
	}

	// Create summary node in the DAG
	var msgIDs []string
	for _, msg := range block {
		msgIDs = append(msgIDs, msg.ID)
	}

	summary := e.store.CreateLeafSummary(msgIDs, result.Content, result.Level)

	return &compactionResult{summary: summary}
}

// ─── Condensed Summaries (DAG depth > 1) ────────────────────────────────────

// CondenseOldSummaries finds summary nodes in the active context and merges them
// into a higher-order condensed summary. This creates DAG depth > 1.
func (e *LCMEngine) CondenseOldSummaries() error {
	active := e.store.GetActiveContext()

	// Collect summary items
	var summaryItems []*ActiveContextItem
	for _, item := range active {
		if !item.IsMessage() && item.Summary != nil {
			summaryItems = append(summaryItems, item)
		}
	}

	// Need at least 2 summaries to condense
	if len(summaryItems) < 2 {
		return nil
	}

	// Condense the oldest summaries
	condenseCount := len(summaryItems)
	if condenseCount > e.config.CompactionBlockSize {
		condenseCount = e.config.CompactionBlockSize
	}
	toCondense := summaryItems[:condenseCount]

	// Build combined content for re-summarization
	var combined string
	var childIDs []string
	for _, item := range toCondense {
		combined += item.Summary.Content + "\n\n"
		childIDs = append(childIDs, item.Summary.ID)
	}

	// Summarize the combined summaries
	result, err := e.summarizer.Summarize(combined, e.config.SummaryTargetTokens)
	if err != nil {
		return fmt.Errorf("condensation failed: %w", err)
	}

	// Create condensed summary node
	condensed := e.store.CreateCondensedSummary(childIDs, result.Content, result.Level)

	e.observer.Debug("lcm.control", "Condensed %d summaries into %s (%d tokens)",
		len(childIDs), condensed.ID, condensed.Tokens)

	return nil
}

// ─── Query Helpers ──────────────────────────────────────────────────────────

// GetStore returns the underlying LCM store.
func (e *LCMEngine) GetStore() *LCMStore {
	return e.store
}

// GetConfig returns the LCM configuration.
func (e *LCMEngine) GetConfig() LCMConfig {
	return e.config
}

// IsEnabled returns whether LCM is active.
func (e *LCMEngine) IsEnabled() bool {
	return e.config.Enabled
}
