package rlm

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func episodeTestMsg(id, content string, tokens int, ts time.Time) *StoreMessage {
	return &StoreMessage{
		ID:        id,
		Role:      RoleUser,
		Content:   content,
		Tokens:    tokens,
		Timestamp: ts,
	}
}

func TestEpisodeManager_NewDefaults(t *testing.T) {
	m := NewEpisodeManager("sess-defaults", EpisodeConfig{})
	if m == nil {
		t.Fatal("NewEpisodeManager returned nil")
	}

	if m.config.MaxEpisodeTokens != 2000 {
		t.Errorf("MaxEpisodeTokens = %d, want 2000", m.config.MaxEpisodeTokens)
	}
	if m.config.MaxEpisodeMessages != 20 {
		t.Errorf("MaxEpisodeMessages = %d, want 20", m.config.MaxEpisodeMessages)
	}
	if m.config.TopicChangeThreshold != 0.5 {
		t.Errorf("TopicChangeThreshold = %f, want 0.5", m.config.TopicChangeThreshold)
	}
	if !m.config.AutoCompactAfterClose {
		t.Errorf("AutoCompactAfterClose = %v, want true", m.config.AutoCompactAfterClose)
	}
}

func TestEpisodeManager_AddMessage(t *testing.T) {
	m := NewEpisodeManager("sess-add", EpisodeConfig{
		MaxEpisodeMessages:    100,
		MaxEpisodeTokens:      10000,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: true,
	})

	ts1 := time.Now().Add(-2 * time.Minute)
	ts2 := ts1.Add(1 * time.Minute)

	m.AddMessage(episodeTestMsg("msg1", "Implement LCM episode manager tests", 7, ts1))
	m.AddMessage(episodeTestMsg("msg2", "Verify episode metadata updates", 5, ts2))

	ep := m.GetActiveEpisode()
	if ep == nil {
		t.Fatal("GetActiveEpisode returned nil")
	}
	if len(ep.MessageIDs) != 2 {
		t.Fatalf("len(MessageIDs) = %d, want 2", len(ep.MessageIDs))
	}
	if ep.MessageIDs[0] != "msg1" || ep.MessageIDs[1] != "msg2" {
		t.Errorf("MessageIDs = %v, want [msg1 msg2]", ep.MessageIDs)
	}
	if ep.Tokens != 12 {
		t.Errorf("Tokens = %d, want 12", ep.Tokens)
	}
	if !ep.StartTime.Equal(ts1) {
		t.Errorf("StartTime = %v, want %v", ep.StartTime, ts1)
	}
	if !ep.EndTime.Equal(ts2) {
		t.Errorf("EndTime = %v, want %v", ep.EndTime, ts2)
	}
	if strings.TrimSpace(ep.Title) == "" {
		t.Error("Title should be set from first message content")
	}
	if len(ep.Tags) == 0 {
		t.Error("Tags should be set from first message content")
	}
}

func TestEpisodeManager_AutoRotation(t *testing.T) {
	m := NewEpisodeManager("sess-rotate", EpisodeConfig{
		MaxEpisodeMessages:    3,
		MaxEpisodeTokens:      10000,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})

	base := time.Now()
	m.AddMessage(episodeTestMsg("m1", "first", 1, base))
	m.AddMessage(episodeTestMsg("m2", "second", 1, base.Add(time.Second)))
	m.AddMessage(episodeTestMsg("m3", "third", 1, base.Add(2*time.Second)))
	m.AddMessage(episodeTestMsg("m4", "fourth", 1, base.Add(3*time.Second)))

	episodes := m.GetAllEpisodes()
	if len(episodes) != 2 {
		t.Fatalf("len(GetAllEpisodes()) = %d, want 2", len(episodes))
	}

	first := episodes[0]
	second := episodes[1]
	if len(first.MessageIDs) != 3 {
		t.Errorf("first episode messages = %d, want 3", len(first.MessageIDs))
	}
	if len(second.MessageIDs) != 1 || second.MessageIDs[0] != "m4" {
		t.Errorf("second episode MessageIDs = %v, want [m4]", second.MessageIDs)
	}
	if m.GetActiveEpisode() == nil || m.GetActiveEpisode().ID != second.ID {
		t.Fatalf("active episode should be second episode")
	}
	if second.ParentEpisodeID != first.ID {
		t.Errorf("ParentEpisodeID = %s, want %s", second.ParentEpisodeID, first.ID)
	}
}

func TestEpisodeManager_CloseActiveEpisode(t *testing.T) {
	m := NewEpisodeManager("sess-close", EpisodeConfig{})
	msg := episodeTestMsg("m1", "close this episode", 4, time.Now())
	m.AddMessage(msg)

	ep := m.CloseActiveEpisode()
	if ep == nil {
		t.Fatal("CloseActiveEpisode returned nil")
	}
	if ep.Status != EpisodeCompacted {
		t.Errorf("Status = %s, want %s", ep.Status, EpisodeCompacted)
	}
	if strings.TrimSpace(ep.Summary) == "" {
		t.Error("Summary should be auto-generated when auto-compact is enabled")
	}
	if ep.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}
	if m.GetActiveEpisode() != nil {
		t.Error("active episode should be nil after close")
	}
}

func TestEpisodeManager_CompactEpisode(t *testing.T) {
	m := NewEpisodeManager("sess-compact", EpisodeConfig{
		MaxEpisodeTokens:      100,
		MaxEpisodeMessages:    10,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})
	m.AddMessage(episodeTestMsg("m1", "episode to compact", 3, time.Now()))
	ep := m.CloseActiveEpisode()
	if ep == nil {
		t.Fatal("CloseActiveEpisode returned nil")
	}

	summary := "Concise summary of the episode"
	if err := m.CompactEpisode(ep.ID, summary); err != nil {
		t.Fatalf("CompactEpisode returned error: %v", err)
	}

	updated, ok := m.GetEpisode(ep.ID)
	if !ok {
		t.Fatalf("GetEpisode(%s) returned not found", ep.ID)
	}
	if updated.Summary != summary {
		t.Errorf("Summary = %q, want %q", updated.Summary, summary)
	}
	if updated.SummaryTokens <= 0 {
		t.Errorf("SummaryTokens = %d, want > 0", updated.SummaryTokens)
	}
	if updated.Status != EpisodeCompacted {
		t.Errorf("Status = %s, want %s", updated.Status, EpisodeCompacted)
	}
}

func TestEpisodeManager_CompactEpisode_NotFound(t *testing.T) {
	m := NewEpisodeManager("sess-compact-missing", EpisodeConfig{})
	err := m.CompactEpisode("ep_missing", "summary")
	if err == nil {
		t.Fatal("CompactEpisode expected error for missing episode")
	}
	if !strings.Contains(err.Error(), "episode not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "episode not found")
	}
}

func TestEpisodeManager_CompactEpisode_EmptySummary(t *testing.T) {
	m := NewEpisodeManager("sess-compact-empty", EpisodeConfig{
		MaxEpisodeTokens:      100,
		MaxEpisodeMessages:    10,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})
	m.AddMessage(episodeTestMsg("m1", "episode to compact", 2, time.Now()))
	ep := m.CloseActiveEpisode()
	if ep == nil {
		t.Fatal("CloseActiveEpisode returned nil")
	}

	err := m.CompactEpisode(ep.ID, "   \n\t")
	if err == nil {
		t.Fatal("CompactEpisode expected error for empty summary")
	}
	if !strings.Contains(err.Error(), "summary cannot be empty") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "summary cannot be empty")
	}
}

func TestEpisodeManager_GetEpisodesForContext(t *testing.T) {
	m := NewEpisodeManager("sess-context", EpisodeConfig{
		MaxEpisodeTokens:      10000,
		MaxEpisodeMessages:    1,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})

	now := time.Now()
	m.AddMessage(episodeTestMsg("m1", "episode one", 10, now))
	m.AddMessage(episodeTestMsg("m2", "episode two", 8, now.Add(time.Second)))
	m.AddMessage(episodeTestMsg("m3", "episode three", 6, now.Add(2*time.Second)))

	episodes := m.GetAllEpisodes()
	if len(episodes) != 3 {
		t.Fatalf("len(GetAllEpisodes()) = %d, want 3", len(episodes))
	}

	// Force deterministic costs for non-active episodes.
	episodes[0].Status = EpisodeCompacted
	episodes[0].SummaryTokens = 3
	episodes[1].Status = EpisodeCompacted
	episodes[1].SummaryTokens = 4

	selected := m.GetEpisodesForContext(10)
	if len(selected) != 2 {
		t.Fatalf("len(GetEpisodesForContext(10)) = %d, want 2", len(selected))
	}
	if selected[0].ID != episodes[2].ID {
		t.Errorf("selected[0] = %s, want active %s", selected[0].ID, episodes[2].ID)
	}
	if selected[1].ID != episodes[1].ID {
		t.Errorf("selected[1] = %s, want %s", selected[1].ID, episodes[1].ID)
	}

	// Active episode should still be included even if it exceeds budget.
	smallBudget := m.GetEpisodesForContext(5)
	if len(smallBudget) != 1 {
		t.Fatalf("len(GetEpisodesForContext(5)) = %d, want 1", len(smallBudget))
	}
	if smallBudget[0].ID != episodes[2].ID {
		t.Errorf("smallBudget[0] = %s, want active %s", smallBudget[0].ID, episodes[2].ID)
	}
}

func TestEpisodeManager_GetAllEpisodes(t *testing.T) {
	m := NewEpisodeManager("sess-all", EpisodeConfig{
		MaxEpisodeTokens:      10000,
		MaxEpisodeMessages:    1,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})

	now := time.Now()
	m.AddMessage(episodeTestMsg("m1", "first", 1, now))
	m.AddMessage(episodeTestMsg("m2", "second", 1, now.Add(time.Second)))
	m.AddMessage(episodeTestMsg("m3", "third", 1, now.Add(2*time.Second)))

	episodes := m.GetAllEpisodes()
	if len(episodes) != 3 {
		t.Fatalf("len(GetAllEpisodes()) = %d, want 3", len(episodes))
	}

	for i, ep := range episodes {
		if len(ep.MessageIDs) != 1 {
			t.Fatalf("episode %d should have 1 message, got %d", i, len(ep.MessageIDs))
		}
		wantMsgID := "m" + string(rune('1'+i))
		if ep.MessageIDs[0] != wantMsgID {
			t.Errorf("episode %d message ID = %s, want %s", i, ep.MessageIDs[0], wantMsgID)
		}
	}
}

func TestEpisodeManager_ParentChaining(t *testing.T) {
	m := NewEpisodeManager("sess-parent", EpisodeConfig{
		MaxEpisodeTokens:      10000,
		MaxEpisodeMessages:    1,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})

	now := time.Now()
	m.AddMessage(episodeTestMsg("m1", "parent one", 1, now))
	m.AddMessage(episodeTestMsg("m2", "parent two", 1, now.Add(time.Second)))
	m.AddMessage(episodeTestMsg("m3", "parent three", 1, now.Add(2*time.Second)))

	episodes := m.GetAllEpisodes()
	if len(episodes) != 3 {
		t.Fatalf("len(GetAllEpisodes()) = %d, want 3", len(episodes))
	}

	if episodes[0].ParentEpisodeID != "" {
		t.Errorf("episodes[0].ParentEpisodeID = %q, want empty", episodes[0].ParentEpisodeID)
	}
	if episodes[1].ParentEpisodeID != episodes[0].ID {
		t.Errorf("episodes[1].ParentEpisodeID = %q, want %q", episodes[1].ParentEpisodeID, episodes[0].ID)
	}
	if episodes[2].ParentEpisodeID != episodes[1].ID {
		t.Errorf("episodes[2].ParentEpisodeID = %q, want %q", episodes[2].ParentEpisodeID, episodes[1].ID)
	}
}

func TestEpisodeManager_NilMessage(t *testing.T) {
	m := NewEpisodeManager("sess-nil", EpisodeConfig{})
	m.AddMessage(nil)

	if m.GetActiveEpisode() != nil {
		t.Error("active episode should remain nil after AddMessage(nil)")
	}
	if len(m.GetAllEpisodes()) != 0 {
		t.Errorf("len(GetAllEpisodes()) = %d, want 0", len(m.GetAllEpisodes()))
	}
}

func TestBuildEpisodeTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty content",
			content: "   ",
			want:    "Untitled Episode",
		},
		{
			name:    "short title",
			content: "Investigate LCM regression",
			want:    "Investigate LCM regression",
		},
		{
			name:    "long title truncates to eight words",
			content: "one two three four five six seven eight nine ten",
			want:    "one two three four five six seven eight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildEpisodeTitle(tt.content)
			if got != tt.want {
				t.Errorf("buildEpisodeTitle(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestBuildEpisodeTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "empty",
			content: "",
			want:    nil,
		},
		{
			name:    "lowercase and punctuation stripping",
			content: "Go, go! TEST test cases extra",
			want:    []string{"go", "test"},
		},
		{
			name:    "max three input words before cleanup",
			content: "alpha beta gamma delta epsilon",
			want:    []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildEpisodeTags(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildEpisodeTags(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
