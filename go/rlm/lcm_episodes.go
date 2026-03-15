package rlm

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// EpisodeStatus describes lifecycle state for an episode.
type EpisodeStatus string

const (
	// EpisodeActive is currently accumulating messages.
	EpisodeActive EpisodeStatus = "active"
	// EpisodeCompacted has a generated summary while messages remain available.
	EpisodeCompacted EpisodeStatus = "compacted"
	// EpisodeArchived is deeply compressed and represented by summary in active context.
	EpisodeArchived EpisodeStatus = "archived"
)

// Episode is a coherent interaction unit that groups related messages.
type Episode struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	MessageIDs      []string      `json:"message_ids"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Tokens          int           `json:"tokens"`
	Summary         string        `json:"summary,omitempty"`
	SummaryTokens   int           `json:"summary_tokens,omitempty"`
	Status          EpisodeStatus `json:"status"`
	Tags            []string      `json:"tags,omitempty"`
	ParentEpisodeID string        `json:"parent_episode_id,omitempty"`
}

// EpisodeConfig controls episode boundaries and behavior.
type EpisodeConfig struct {
	MaxEpisodeTokens      int
	MaxEpisodeMessages    int
	TopicChangeThreshold  float64
	AutoCompactAfterClose bool
}

// EpisodeManager manages episode creation, compaction, and retrieval.
type EpisodeManager struct {
	mu            sync.RWMutex
	episodes      map[string]*Episode
	episodeSeq    []*Episode
	activeEpisode *Episode
	nextID        int
	sessionID     string
	config        EpisodeConfig
}

// NewEpisodeManager creates a manager with defaults applied.
func NewEpisodeManager(sessionID string, config EpisodeConfig) *EpisodeManager {
	zeroConfig := config == (EpisodeConfig{})

	if config.MaxEpisodeTokens <= 0 {
		config.MaxEpisodeTokens = 2000
	}
	if config.MaxEpisodeMessages <= 0 {
		config.MaxEpisodeMessages = 20
	}
	if config.TopicChangeThreshold <= 0 || config.TopicChangeThreshold > 1 {
		config.TopicChangeThreshold = 0.5
	}
	if zeroConfig {
		config.AutoCompactAfterClose = true
	}

	return &EpisodeManager{
		episodes:   make(map[string]*Episode),
		episodeSeq: make([]*Episode, 0),
		sessionID:  sessionID,
		config:     config,
	}
}

// AddMessage adds a message into the active episode, rotating when needed.
func (m *EpisodeManager) AddMessage(msg *StoreMessage) {
	if msg == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeEpisode == nil {
		m.activeEpisode = m.newEpisodeLocked("")
	}

	if m.shouldCloseEpisodeLocked() {
		closed := m.closeActiveEpisodeLocked()
		parentID := ""
		if closed != nil {
			parentID = closed.ID
		}
		m.activeEpisode = m.newEpisodeLocked(parentID)
	}

	ep := m.activeEpisode
	ep.MessageIDs = append(ep.MessageIDs, msg.ID)
	ep.Tokens += msg.Tokens
	if ep.StartTime.IsZero() {
		ep.StartTime = msg.Timestamp
	}
	ep.EndTime = msg.Timestamp
	if strings.TrimSpace(ep.Title) == "" {
		ep.Title = buildEpisodeTitle(msg.Content)
	}
	if len(ep.Tags) == 0 {
		ep.Tags = buildEpisodeTags(msg.Content)
	}
}

// CloseActiveEpisode closes the current active episode.
func (m *EpisodeManager) CloseActiveEpisode() *Episode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeActiveEpisodeLocked()
}

// GetEpisode retrieves an episode by id.
func (m *EpisodeManager) GetEpisode(id string) (*Episode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ep, ok := m.episodes[id]
	return ep, ok
}

// GetAllEpisodes returns all episodes in chronological order.
func (m *EpisodeManager) GetAllEpisodes() []*Episode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Episode, len(m.episodeSeq))
	copy(out, m.episodeSeq)
	return out
}

// GetActiveEpisode returns the currently active episode.
func (m *EpisodeManager) GetActiveEpisode() *Episode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeEpisode
}

// CompactEpisode compresses an episode into a summary.
func (m *EpisodeManager) CompactEpisode(episodeID string, summary string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ep, ok := m.episodes[episodeID]
	if !ok {
		return fmt.Errorf("episode not found: %s", episodeID)
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("summary cannot be empty")
	}

	ep.Summary = summary
	ep.SummaryTokens = EstimateTokens(summary)
	ep.Status = EpisodeCompacted
	if strings.TrimSpace(ep.Title) == "" {
		ep.Title = buildEpisodeTitle(summary)
	}
	if len(ep.Tags) == 0 {
		ep.Tags = buildEpisodeTags(summary)
	}

	return nil
}

// GetEpisodesForContext returns reverse-chronological episodes within budget.
// Active episode is always included when present.
func (m *EpisodeManager) GetEpisodesForContext(tokenBudget int) []*Episode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if tokenBudget < 0 {
		tokenBudget = 0
	}

	result := make([]*Episode, 0)
	included := make(map[string]bool)
	remaining := tokenBudget

	if m.activeEpisode != nil {
		result = append(result, m.activeEpisode)
		included[m.activeEpisode.ID] = true
		remaining -= m.activeEpisode.Tokens
		if remaining <= 0 {
			return result
		}
	}

	for i := len(m.episodeSeq) - 1; i >= 0; i-- {
		ep := m.episodeSeq[i]
		if ep == nil || included[ep.ID] {
			continue
		}

		cost := ep.Tokens
		if ep.Status != EpisodeActive {
			if ep.SummaryTokens > 0 {
				cost = ep.SummaryTokens
			}
		}

		if cost > remaining {
			continue
		}

		result = append(result, ep)
		remaining -= cost
		if remaining <= 0 {
			break
		}
	}

	return result
}

func (m *EpisodeManager) shouldCloseEpisodeLocked() bool {
	if m.activeEpisode == nil {
		return false
	}
	if m.config.MaxEpisodeTokens > 0 && m.activeEpisode.Tokens >= m.config.MaxEpisodeTokens {
		return true
	}
	if m.config.MaxEpisodeMessages > 0 && len(m.activeEpisode.MessageIDs) >= m.config.MaxEpisodeMessages {
		return true
	}
	return false
}

func (m *EpisodeManager) closeActiveEpisodeLocked() *Episode {
	if m.activeEpisode == nil {
		return nil
	}

	ep := m.activeEpisode
	if ep.EndTime.IsZero() {
		ep.EndTime = time.Now()
	}
	if ep.Status == EpisodeActive && m.config.AutoCompactAfterClose {
		if strings.TrimSpace(ep.Summary) == "" {
			ep.Summary = fmt.Sprintf("Episode %s (%d messages)", ep.ID, len(ep.MessageIDs))
		}
		ep.SummaryTokens = EstimateTokens(ep.Summary)
		ep.Status = EpisodeCompacted
	}

	m.activeEpisode = nil
	return ep
}

func (m *EpisodeManager) newEpisodeLocked(parentEpisodeID string) *Episode {
	m.nextID++
	ep := &Episode{
		ID:              fmt.Sprintf("ep_%s_%d", m.sessionID, m.nextID),
		Title:           "",
		MessageIDs:      make([]string, 0),
	StartTime:       time.Time{},
	EndTime:         time.Time{},
		Tokens:          0,
		Summary:         "",
		SummaryTokens:   0,
		Status:          EpisodeActive,
		Tags:            make([]string, 0),
		ParentEpisodeID: parentEpisodeID,
	}
	m.episodes[ep.ID] = ep
	m.episodeSeq = append(m.episodeSeq, ep)
	return ep
}

func buildEpisodeTitle(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "Untitled Episode"
	}
	parts := strings.Fields(trimmed)
	if len(parts) > 8 {
		parts = parts[:8]
	}
	return strings.Join(parts, " ")
}

func buildEpisodeTags(content string) []string {
	trimmed := strings.ToLower(strings.TrimSpace(content))
	if trimmed == "" {
		return nil
	}
	parts := strings.Fields(trimmed)
	if len(parts) > 3 {
		parts = parts[:3]
	}
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, p := range parts {
		p = strings.Trim(p, ",.;:!?()[]{}\"'")
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}
