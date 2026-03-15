package rlm

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ─── LCM Data Model ─────────────────────────────────────────────────────────
// Implements the dual-state memory architecture from the LCM paper:
// - Immutable Store: every message persisted verbatim, never modified
// - Active Context: assembled from recent messages + summary nodes

// MessageRole represents the role of a message sender.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
	RoleSystem    MessageRole = "system"
)

// StoreMessage is an immutable record in the LCM store.
// Once persisted, it is never modified or deleted.
type StoreMessage struct {
	ID        string      `json:"id"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	Tokens    int         `json:"tokens"`
	Timestamp time.Time   `json:"timestamp"`
	FileIDs   []string    `json:"file_ids,omitempty"` // Referenced file IDs
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SummaryKind distinguishes leaf from condensed summaries in the DAG.
type SummaryKind string

const (
	SummaryLeaf      SummaryKind = "leaf"      // Direct summary of a span of messages
	SummaryCondensed SummaryKind = "condensed" // Higher-order summary of other summaries
)

// SummaryNode is a node in the hierarchical summary DAG.
// It represents a compressed view of a span of messages or other summaries.
type SummaryNode struct {
	ID          string      `json:"id"`
	Kind        SummaryKind `json:"kind"`
	Content     string      `json:"content"`     // The summary text
	Tokens      int         `json:"tokens"`      // Token count of this summary
	Level       int         `json:"level"`       // Escalation level that produced this (1, 2, or 3)
	CreatedAt   time.Time   `json:"created_at"`

	// Provenance: what this summary covers
	MessageIDs  []string    `json:"message_ids"`  // IDs of messages directly summarized (leaf)
	ChildIDs    []string    `json:"child_ids"`    // IDs of child summary nodes (condensed)
	ParentID    string      `json:"parent_id,omitempty"` // Parent summary node (if further condensed)

	// File IDs propagated from summarized messages
	FileIDs     []string    `json:"file_ids,omitempty"`
}

// ActiveContextItem represents one item in the active context window.
// It's either a raw message or a pointer to a summary node.
type ActiveContextItem struct {
	// Exactly one of these is set
	Message   *StoreMessage `json:"message,omitempty"`
	Summary   *SummaryNode  `json:"summary,omitempty"`

	// Position in the active context (for ordering)
	Position  int           `json:"position"`
}

// IsMessage returns true if this item is a raw message (not summarized).
func (i *ActiveContextItem) IsMessage() bool {
	return i.Message != nil
}

// GetTokens returns the token count for this item.
func (i *ActiveContextItem) GetTokens() int {
	if i.Message != nil {
		return i.Message.Tokens
	}
	if i.Summary != nil {
		return i.Summary.Tokens
	}
	return 0
}

// GetContent returns the content text.
func (i *ActiveContextItem) GetContent() string {
	if i.Message != nil {
		return i.Message.Content
	}
	if i.Summary != nil {
		return i.Summary.Content
	}
	return ""
}

// GetFileIDs returns file IDs from this item.
func (i *ActiveContextItem) GetFileIDs() []string {
	if i.Message != nil {
		return i.Message.FileIDs
	}
	if i.Summary != nil {
		return i.Summary.FileIDs
	}
	return nil
}

// ─── LCM Store ──────────────────────────────────────────────────────────────

// LCMStore is the persistent, transactional store for LCM sessions.
// It maintains the immutable message history and the derived summary DAG.
// This is an in-memory implementation; production could use embedded PostgreSQL.
type LCMStore struct {
	mu         sync.RWMutex

	// Immutable Store: messages indexed by ID
	messages   map[string]*StoreMessage
	messageSeq []*StoreMessage // Chronological order

	// Summary DAG: summary nodes indexed by ID
	summaries  map[string]*SummaryNode

	// Active Context: the window sent to the LLM
	active     []*ActiveContextItem

	// Counters
	nextMsgID  int
	nextSumID  int

	// Session metadata
	sessionID  string
	createdAt  time.Time
}

// NewLCMStore creates a new empty LCM store for a session.
func NewLCMStore(sessionID string) *LCMStore {
	return &LCMStore{
		messages:   make(map[string]*StoreMessage),
		messageSeq: make([]*StoreMessage, 0),
		summaries:  make(map[string]*SummaryNode),
		active:     make([]*ActiveContextItem, 0),
		sessionID:  sessionID,
		createdAt:  time.Now(),
	}
}

// ─── Immutable Store Operations ─────────────────────────────────────────────

// PersistMessage adds a message to the immutable store and active context.
// The message is never modified after this call.
func (s *LCMStore) PersistMessage(role MessageRole, content string, fileIDs []string) *StoreMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextMsgID++
	msg := &StoreMessage{
		ID:        fmt.Sprintf("msg_%s_%d", s.sessionID, s.nextMsgID),
		Role:      role,
		Content:   content,
		Tokens:    EstimateTokens(content),
		Timestamp: time.Now(),
		FileIDs:   fileIDs,
	}

	s.messages[msg.ID] = msg
	s.messageSeq = append(s.messageSeq, msg)

	// Add to active context as a raw message pointer
	s.active = append(s.active, &ActiveContextItem{
		Message:  msg,
		Position: len(s.active),
	})

	return msg
}

// GetMessage retrieves a message by ID from the immutable store.
func (s *LCMStore) GetMessage(id string) (*StoreMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msg, ok := s.messages[id]
	return msg, ok
}

// GetAllMessages returns all messages in chronological order.
func (s *LCMStore) GetAllMessages() []*StoreMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*StoreMessage, len(s.messageSeq))
	copy(result, s.messageSeq)
	return result
}

// MessageCount returns the number of messages in the store.
func (s *LCMStore) MessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messageSeq)
}

// ─── Summary DAG Operations ─────────────────────────────────────────────────

// CreateLeafSummary creates a summary node from a span of messages.
func (s *LCMStore) CreateLeafSummary(messageIDs []string, content string, level int) *SummaryNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextSumID++
	node := &SummaryNode{
		ID:         fmt.Sprintf("sum_%s_%d", s.sessionID, s.nextSumID),
		Kind:       SummaryLeaf,
		Content:    content,
		Tokens:     EstimateTokens(content),
		Level:      level,
		CreatedAt:  time.Now(),
		MessageIDs: messageIDs,
	}

	// Propagate file IDs from summarized messages
	fileIDSet := make(map[string]bool)
	for _, msgID := range messageIDs {
		if msg, ok := s.messages[msgID]; ok {
			for _, fid := range msg.FileIDs {
				fileIDSet[fid] = true
			}
		}
	}
	for fid := range fileIDSet {
		node.FileIDs = append(node.FileIDs, fid)
	}

	s.summaries[node.ID] = node
	return node
}

// CreateCondensedSummary creates a higher-order summary from existing summaries.
func (s *LCMStore) CreateCondensedSummary(childIDs []string, content string, level int) *SummaryNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextSumID++
	node := &SummaryNode{
		ID:        fmt.Sprintf("sum_%s_%d", s.sessionID, s.nextSumID),
		Kind:      SummaryCondensed,
		Content:   content,
		Tokens:    EstimateTokens(content),
		Level:     level,
		CreatedAt: time.Now(),
		ChildIDs:  childIDs,
	}

	// Propagate file IDs from child summaries and set parent pointers
	fileIDSet := make(map[string]bool)
	for _, childID := range childIDs {
		if child, ok := s.summaries[childID]; ok {
			child.ParentID = node.ID
			for _, fid := range child.FileIDs {
				fileIDSet[fid] = true
			}
		}
	}
	for fid := range fileIDSet {
		node.FileIDs = append(node.FileIDs, fid)
	}

	s.summaries[node.ID] = node
	return node
}

// GetSummary retrieves a summary node by ID.
func (s *LCMStore) GetSummary(id string) (*SummaryNode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sum, ok := s.summaries[id]
	return sum, ok
}

// ExpandSummary returns the original messages that a summary covers.
// For leaf summaries, returns the directly summarized messages.
// For condensed summaries, recursively expands all children.
// NOTE: This is the unrestricted internal method. External callers should use
// ExpandSummaryRestricted which enforces the sub-agent-only policy from the LCM paper.
func (s *LCMStore) ExpandSummary(summaryID string) ([]*StoreMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sum, ok := s.summaries[summaryID]
	if !ok {
		return nil, fmt.Errorf("summary %s not found", summaryID)
	}

	return s.expandSummaryLocked(sum)
}

// ExpandSummaryRestricted enforces the LCM paper's restriction (Appendix C.1):
// lcm_expand can only be called by sub-agents (depth > 0), not the main agent.
// This prevents uncontrolled context growth in the primary interaction loop.
// When the main agent needs to inspect compacted history, it must delegate
// the expansion to a sub-agent via the Task tool.
func (s *LCMStore) ExpandSummaryRestricted(summaryID string, callerDepth int) ([]*StoreMessage, error) {
	if callerDepth == 0 {
		return nil, &ExpandRestrictionError{
			SummaryID: summaryID,
		}
	}
	return s.ExpandSummary(summaryID)
}

// ExpandRestrictionError is returned when the main agent tries to call lcm_expand.
type ExpandRestrictionError struct {
	SummaryID string
}

func (e *ExpandRestrictionError) Error() string {
	return fmt.Sprintf(
		"lcm_expand(%s) rejected: only sub-agents can expand summaries. "+
			"Delegate the expansion to a sub-agent via the Task tool, which will "+
			"process the expanded content in its own context window and return only the relevant findings.",
		e.SummaryID,
	)
}

func (s *LCMStore) expandSummaryLocked(sum *SummaryNode) ([]*StoreMessage, error) {
	if sum.Kind == SummaryLeaf {
		var msgs []*StoreMessage
		for _, msgID := range sum.MessageIDs {
			if msg, ok := s.messages[msgID]; ok {
				msgs = append(msgs, msg)
			}
		}
		return msgs, nil
	}

	// Condensed: recursively expand children
	var allMsgs []*StoreMessage
	for _, childID := range sum.ChildIDs {
		child, ok := s.summaries[childID]
		if !ok {
			continue
		}
		childMsgs, err := s.expandSummaryLocked(child)
		if err != nil {
			return nil, err
		}
		allMsgs = append(allMsgs, childMsgs...)
	}
	return allMsgs, nil
}

// ─── Active Context Operations ──────────────────────────────────────────────

// GetActiveContext returns the current active context items.
func (s *LCMStore) GetActiveContext() []*ActiveContextItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*ActiveContextItem, len(s.active))
	copy(result, s.active)
	return result
}

// ActiveContextTokens returns the total token count of the active context.
func (s *LCMStore) ActiveContextTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, item := range s.active {
		total += item.GetTokens()
	}
	return total
}

// CompactOldestBlock replaces the oldest block of raw messages in active context
// with a summary node. Returns the IDs of compacted messages.
func (s *LCMStore) CompactOldestBlock(summary *SummaryNode) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the oldest contiguous block of raw messages (skip first item if system prompt)
	startIdx := 0
	for startIdx < len(s.active) {
		if s.active[startIdx].IsMessage() && s.active[startIdx].Message.Role == RoleSystem {
			startIdx++ // Skip system prompt
			continue
		}
		break
	}

	// Collect message IDs from the block that this summary covers
	compactedIDs := make(map[string]bool)
	for _, msgID := range summary.MessageIDs {
		compactedIDs[msgID] = true
	}

	// Build new active context replacing compacted messages with summary
	var newActive []*ActiveContextItem
	summaryInserted := false
	var removedIDs []string

	for _, item := range s.active {
		if item.IsMessage() && compactedIDs[item.Message.ID] {
			removedIDs = append(removedIDs, item.Message.ID)
			if !summaryInserted {
				newActive = append(newActive, &ActiveContextItem{
					Summary:  summary,
					Position: len(newActive),
				})
				summaryInserted = true
			}
		} else {
			item.Position = len(newActive)
			newActive = append(newActive, item)
		}
	}

	s.active = newActive
	return removedIDs
}

// BuildMessages converts the active context into a Messages slice for LLM calls.
// Summary nodes include their IDs as annotations for deterministic retrievability.
func (s *LCMStore) BuildMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var msgs []Message
	for _, item := range s.active {
		if item.IsMessage() {
			msgs = append(msgs, Message{
				Role:    string(item.Message.Role),
				Content: item.Message.Content,
			})
		} else if item.Summary != nil {
			// Annotate summary with IDs for deterministic retrievability
			content := fmt.Sprintf("[Summary %s | covers %d items]\n%s",
				item.Summary.ID,
				len(item.Summary.MessageIDs)+len(item.Summary.ChildIDs),
				item.Summary.Content,
			)
			// Summary nodes are presented as system-level context
			msgs = append(msgs, Message{
				Role:    "system",
				Content: content,
			})
		}
	}
	return msgs
}

// ─── LCM Grep (Search) ─────────────────────────────────────────────────────

// GrepResult represents a search hit from lcm_grep.
type GrepResult struct {
	Message   *StoreMessage `json:"message"`
	SummaryID string        `json:"summary_id,omitempty"` // Summary that covers this message
	MatchLine string        `json:"match_line"`
}

// Grep searches the immutable store for messages matching a regex pattern.
// Results are grouped by the summary node that currently covers them.
func (s *LCMStore) Grep(pattern string, maxResults int) ([]GrepResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	if maxResults <= 0 {
		maxResults = 50 // Default pagination
	}

	// Build reverse index: message ID → covering summary ID
	coveringSum := make(map[string]string)
	for _, sum := range s.summaries {
		if sum.Kind == SummaryLeaf {
			for _, msgID := range sum.MessageIDs {
				// Use the deepest (most specific) summary
				if _, exists := coveringSum[msgID]; !exists {
					coveringSum[msgID] = sum.ID
				}
			}
		}
	}

	var results []GrepResult
	for _, msg := range s.messageSeq {
		if len(results) >= maxResults {
			break
		}
		if re.MatchString(msg.Content) {
			// Extract matching line
			matchLine := ""
			for _, line := range strings.Split(msg.Content, "\n") {
				if re.MatchString(line) {
					matchLine = line
					break
				}
			}
			results = append(results, GrepResult{
				Message:   msg,
				SummaryID: coveringSum[msg.ID],
				MatchLine: matchLine,
			})
		}
	}

	return results, nil
}

// ─── LCM Describe ──────────────────────────────────────────────────────────

// DescribeResult contains metadata about an LCM identifier.
type DescribeResult struct {
	Type       string            `json:"type"` // "message" or "summary"
	ID         string            `json:"id"`
	Tokens     int               `json:"tokens"`
	Metadata   map[string]string `json:"metadata,omitempty"`

	// Message-specific
	Role       string            `json:"role,omitempty"`
	Timestamp  *time.Time        `json:"timestamp,omitempty"`

	// Summary-specific
	Kind       string            `json:"kind,omitempty"`
	Level      int               `json:"level,omitempty"`
	CoveredIDs []string          `json:"covered_ids,omitempty"`
	FileIDs    []string          `json:"file_ids,omitempty"`
	Content    string            `json:"content,omitempty"` // Summary text (not for messages)
}

// Describe returns metadata for any LCM identifier (message or summary).
func (s *LCMStore) Describe(id string) (*DescribeResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if msg, ok := s.messages[id]; ok {
		return &DescribeResult{
			Type:      "message",
			ID:        msg.ID,
			Tokens:    msg.Tokens,
			Role:      string(msg.Role),
			Timestamp: &msg.Timestamp,
			FileIDs:   msg.FileIDs,
			Metadata:  msg.Metadata,
		}, nil
	}

	if sum, ok := s.summaries[id]; ok {
		coveredIDs := sum.MessageIDs
		if sum.Kind == SummaryCondensed {
			coveredIDs = sum.ChildIDs
		}
		return &DescribeResult{
			Type:       "summary",
			ID:         sum.ID,
			Tokens:     sum.Tokens,
			Kind:       string(sum.Kind),
			Level:      sum.Level,
			CoveredIDs: coveredIDs,
			FileIDs:    sum.FileIDs,
			Content:    sum.Content,
		}, nil
	}

	return nil, fmt.Errorf("LCM identifier %s not found", id)
}

// ─── Statistics ─────────────────────────────────────────────────────────────

// LCMStoreStats contains runtime statistics about the store.
type LCMStoreStats struct {
	TotalMessages        int `json:"total_messages"`
	TotalSummaries       int `json:"total_summaries"`
	ActiveContextItems   int `json:"active_context_items"`
	ActiveContextTokens  int `json:"active_context_tokens"`
	ImmutableStoreTokens int `json:"immutable_store_tokens"`
	CompressionRatio     float64 `json:"compression_ratio"` // active/total
}

// Stats returns current statistics about the store.
func (s *LCMStore) Stats() LCMStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalTokens := 0
	for _, msg := range s.messageSeq {
		totalTokens += msg.Tokens
	}

	activeTokens := 0
	for _, item := range s.active {
		activeTokens += item.GetTokens()
	}

	ratio := 0.0
	if totalTokens > 0 {
		ratio = float64(activeTokens) / float64(totalTokens)
	}

	return LCMStoreStats{
		TotalMessages:        len(s.messageSeq),
		TotalSummaries:       len(s.summaries),
		ActiveContextItems:   len(s.active),
		ActiveContextTokens:  activeTokens,
		ImmutableStoreTokens: totalTokens,
		CompressionRatio:     ratio,
	}
}
