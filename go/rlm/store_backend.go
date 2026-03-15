package rlm

import "io"

// ─── Store Backend Interface ─────────────────────────────────────────────────
// Abstracts the persistence layer for LCM's dual-state memory.
// The in-memory implementation (LCMStore) remains the default.
// A SQLite implementation provides crash recovery, indexed full-text search,
// and transactional writes as described in the LCM paper (Section 2.1).

// StoreBackend defines the persistence operations for the LCM store.
// Implementations must be safe for concurrent use.
type StoreBackend interface {
	// ─── Message Operations ──────────────────────────────────────────
	// PersistMessage stores a message in the immutable store.
	PersistMessage(msg *StoreMessage) error
	// GetMessage retrieves a message by ID.
	GetMessage(id string) (*StoreMessage, error)
	// GetAllMessages returns all messages in chronological order.
	GetAllMessages() ([]*StoreMessage, error)
	// MessageCount returns the total number of persisted messages.
	MessageCount() (int, error)

	// ─── Summary Operations ─────────────────────────────────────────
	// PersistSummary stores a summary node in the DAG.
	PersistSummary(node *SummaryNode) error
	// GetSummary retrieves a summary by ID.
	GetSummary(id string) (*SummaryNode, error)
	// GetAllSummaries returns all summary nodes.
	GetAllSummaries() ([]*SummaryNode, error)
	// UpdateSummaryParent sets the parent ID on a summary (for condensation).
	UpdateSummaryParent(summaryID, parentID string) error

	// ─── Search ─────────────────────────────────────────────────────
	// GrepMessages searches message content with a regex pattern.
	// Returns matching messages with optional summary scope filtering.
	GrepMessages(pattern string, summaryScope *string, maxResults int) ([]*StoreMessage, error)

	// ─── Lifecycle ──────────────────────────────────────────────────
	io.Closer
}

// ─── In-Memory Backend ───────────────────────────────────────────────────────
// MemoryBackend wraps the existing in-memory maps as a StoreBackend.
// This is the default backend and requires no external dependencies.

type MemoryBackend struct {
	messages   map[string]*StoreMessage
	messageSeq []*StoreMessage
	summaries  map[string]*SummaryNode
}

// NewMemoryBackend creates a new in-memory backend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		messages:   make(map[string]*StoreMessage),
		messageSeq: make([]*StoreMessage, 0),
		summaries:  make(map[string]*SummaryNode),
	}
}

func (m *MemoryBackend) PersistMessage(msg *StoreMessage) error {
	m.messages[msg.ID] = msg
	m.messageSeq = append(m.messageSeq, msg)
	return nil
}

func (m *MemoryBackend) GetMessage(id string) (*StoreMessage, error) {
	msg, ok := m.messages[id]
	if !ok {
		return nil, nil
	}
	return msg, nil
}

func (m *MemoryBackend) GetAllMessages() ([]*StoreMessage, error) {
	result := make([]*StoreMessage, len(m.messageSeq))
	copy(result, m.messageSeq)
	return result, nil
}

func (m *MemoryBackend) MessageCount() (int, error) {
	return len(m.messageSeq), nil
}

func (m *MemoryBackend) PersistSummary(node *SummaryNode) error {
	m.summaries[node.ID] = node
	return nil
}

func (m *MemoryBackend) GetSummary(id string) (*SummaryNode, error) {
	sum, ok := m.summaries[id]
	if !ok {
		return nil, nil
	}
	return sum, nil
}

func (m *MemoryBackend) GetAllSummaries() ([]*SummaryNode, error) {
	result := make([]*SummaryNode, 0, len(m.summaries))
	for _, s := range m.summaries {
		result = append(result, s)
	}
	return result, nil
}

func (m *MemoryBackend) UpdateSummaryParent(summaryID, parentID string) error {
	if sum, ok := m.summaries[summaryID]; ok {
		sum.ParentID = parentID
	}
	return nil
}

func (m *MemoryBackend) GrepMessages(pattern string, summaryScope *string, maxResults int) ([]*StoreMessage, error) {
	// For in-memory, just return all messages (filtering happens in LCMStore.Grep)
	return m.messageSeq, nil
}

func (m *MemoryBackend) Close() error {
	return nil
}
