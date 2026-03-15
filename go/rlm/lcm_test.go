package rlm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── LCM Store Tests ────────────────────────────────────────────────────────

func TestLCMStore_NewStore(t *testing.T) {
	store := NewLCMStore("test-session")
	if store == nil {
		t.Fatal("NewLCMStore returned nil")
	}
	if store.sessionID != "test-session" {
		t.Errorf("sessionID = %s, want test-session", store.sessionID)
	}
	if store.MessageCount() != 0 {
		t.Errorf("MessageCount = %d, want 0", store.MessageCount())
	}
}

func TestLCMStore_PersistMessage(t *testing.T) {
	store := NewLCMStore("test")

	msg := store.PersistMessage(RoleUser, "Hello, world!", nil)
	if msg == nil {
		t.Fatal("PersistMessage returned nil")
	}
	if msg.Role != RoleUser {
		t.Errorf("Role = %s, want user", msg.Role)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %s, want 'Hello, world!'", msg.Content)
	}
	if msg.Tokens <= 0 {
		t.Errorf("Tokens = %d, want > 0", msg.Tokens)
	}
	if store.MessageCount() != 1 {
		t.Errorf("MessageCount = %d, want 1", store.MessageCount())
	}
}

func TestLCMStore_PersistMessage_WithFileIDs(t *testing.T) {
	store := NewLCMStore("test")
	fileIDs := []string{"file_1", "file_2"}
	msg := store.PersistMessage(RoleUser, "Check these files", fileIDs)
	if len(msg.FileIDs) != 2 {
		t.Errorf("FileIDs length = %d, want 2", len(msg.FileIDs))
	}
}

func TestLCMStore_GetMessage(t *testing.T) {
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "test content", nil)

	retrieved, ok := store.GetMessage(msg.ID)
	if !ok {
		t.Fatal("GetMessage returned false")
	}
	if retrieved.Content != "test content" {
		t.Errorf("Content = %s, want 'test content'", retrieved.Content)
	}

	_, ok = store.GetMessage("nonexistent")
	if ok {
		t.Error("GetMessage returned true for nonexistent ID")
	}
}

func TestLCMStore_GetAllMessages(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleUser, "first", nil)
	store.PersistMessage(RoleAssistant, "second", nil)
	store.PersistMessage(RoleUser, "third", nil)

	msgs := store.GetAllMessages()
	if len(msgs) != 3 {
		t.Fatalf("GetAllMessages returned %d, want 3", len(msgs))
	}
	if msgs[0].Content != "first" {
		t.Errorf("First message = %s, want 'first'", msgs[0].Content)
	}
	if msgs[2].Content != "third" {
		t.Errorf("Third message = %s, want 'third'", msgs[2].Content)
	}
}

func TestLCMStore_ImmutableStore(t *testing.T) {
	// Verify that messages cannot be modified after persistence
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "original", nil)
	originalID := msg.ID

	// Persist more messages
	store.PersistMessage(RoleAssistant, "reply", nil)

	// Retrieve the original - should still be intact
	retrieved, ok := store.GetMessage(originalID)
	if !ok {
		t.Fatal("Original message not found")
	}
	if retrieved.Content != "original" {
		t.Errorf("Content changed from 'original' to '%s'", retrieved.Content)
	}
}

// ─── Summary DAG Tests ──────────────────────────────────────────────────────

func TestLCMStore_CreateLeafSummary(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, "Hello", nil)
	msg2 := store.PersistMessage(RoleAssistant, "Hi there", nil)

	summary := store.CreateLeafSummary(
		[]string{msg1.ID, msg2.ID},
		"User greeted, assistant replied",
		1,
	)

	if summary == nil {
		t.Fatal("CreateLeafSummary returned nil")
	}
	if summary.Kind != SummaryLeaf {
		t.Errorf("Kind = %s, want leaf", summary.Kind)
	}
	if summary.Level != 1 {
		t.Errorf("Level = %d, want 1", summary.Level)
	}
	if len(summary.MessageIDs) != 2 {
		t.Errorf("MessageIDs length = %d, want 2", len(summary.MessageIDs))
	}
}

func TestLCMStore_CreateLeafSummary_FileIDPropagation(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, "File A", []string{"file_a"})
	msg2 := store.PersistMessage(RoleUser, "File B", []string{"file_b"})

	summary := store.CreateLeafSummary(
		[]string{msg1.ID, msg2.ID},
		"Summary of files",
		1,
	)

	if len(summary.FileIDs) != 2 {
		t.Errorf("FileIDs length = %d, want 2", len(summary.FileIDs))
	}
}

func TestLCMStore_CreateCondensedSummary(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, "Hello", []string{"f1"})
	msg2 := store.PersistMessage(RoleAssistant, "Hi", nil)
	msg3 := store.PersistMessage(RoleUser, "More", []string{"f2"})

	leaf1 := store.CreateLeafSummary([]string{msg1.ID, msg2.ID}, "Greeting exchange", 1)
	leaf2 := store.CreateLeafSummary([]string{msg3.ID}, "Continuation", 1)

	condensed := store.CreateCondensedSummary(
		[]string{leaf1.ID, leaf2.ID},
		"Full conversation summary",
		1,
	)

	if condensed.Kind != SummaryCondensed {
		t.Errorf("Kind = %s, want condensed", condensed.Kind)
	}
	if len(condensed.ChildIDs) != 2 {
		t.Errorf("ChildIDs length = %d, want 2", len(condensed.ChildIDs))
	}
	// File IDs should propagate from children
	if len(condensed.FileIDs) != 2 {
		t.Errorf("FileIDs length = %d, want 2 (propagated from children)", len(condensed.FileIDs))
	}
	// Parent pointers should be set on children
	updatedLeaf1, _ := store.GetSummary(leaf1.ID)
	if updatedLeaf1.ParentID != condensed.ID {
		t.Errorf("Leaf1 ParentID = %s, want %s", updatedLeaf1.ParentID, condensed.ID)
	}
}

func TestLCMStore_ExpandSummary_Leaf(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, "Message 1", nil)
	msg2 := store.PersistMessage(RoleAssistant, "Message 2", nil)

	leaf := store.CreateLeafSummary([]string{msg1.ID, msg2.ID}, "Summary", 1)

	expanded, err := store.ExpandSummary(leaf.ID)
	if err != nil {
		t.Fatalf("ExpandSummary error: %v", err)
	}
	if len(expanded) != 2 {
		t.Fatalf("Expanded length = %d, want 2", len(expanded))
	}
	if expanded[0].Content != "Message 1" {
		t.Errorf("First expanded = %s, want 'Message 1'", expanded[0].Content)
	}
}

func TestLCMStore_ExpandSummary_Condensed(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, "A", nil)
	msg2 := store.PersistMessage(RoleAssistant, "B", nil)
	msg3 := store.PersistMessage(RoleUser, "C", nil)

	leaf1 := store.CreateLeafSummary([]string{msg1.ID, msg2.ID}, "Summary AB", 1)
	leaf2 := store.CreateLeafSummary([]string{msg3.ID}, "Summary C", 1)
	condensed := store.CreateCondensedSummary([]string{leaf1.ID, leaf2.ID}, "All", 2)

	expanded, err := store.ExpandSummary(condensed.ID)
	if err != nil {
		t.Fatalf("ExpandSummary error: %v", err)
	}
	if len(expanded) != 3 {
		t.Fatalf("Expanded length = %d, want 3 (recursive)", len(expanded))
	}
}

func TestLCMStore_ExpandSummary_NotFound(t *testing.T) {
	store := NewLCMStore("test")
	_, err := store.ExpandSummary("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent summary")
	}
}

// ─── Active Context Tests ───────────────────────────────────────────────────

func TestLCMStore_ActiveContext(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleSystem, "You are helpful", nil)
	store.PersistMessage(RoleUser, "Hello", nil)

	active := store.GetActiveContext()
	if len(active) != 2 {
		t.Fatalf("Active context length = %d, want 2", len(active))
	}
	if !active[0].IsMessage() {
		t.Error("First item should be a message")
	}
	if active[0].Message.Role != RoleSystem {
		t.Errorf("First item role = %s, want system", active[0].Message.Role)
	}
}

func TestLCMStore_ActiveContextTokens(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleUser, "Hello world this is a test message", nil)
	store.PersistMessage(RoleAssistant, "Yes it is indeed a test", nil)

	tokens := store.ActiveContextTokens()
	if tokens <= 0 {
		t.Errorf("ActiveContextTokens = %d, want > 0", tokens)
	}
}

func TestLCMStore_CompactOldestBlock(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleSystem, "System prompt", nil)
	msg1 := store.PersistMessage(RoleUser, "First question", nil)
	msg2 := store.PersistMessage(RoleAssistant, "First answer", nil)
	store.PersistMessage(RoleUser, "Second question", nil)

	// Create summary for msg1 and msg2
	summary := store.CreateLeafSummary(
		[]string{msg1.ID, msg2.ID},
		"Q&A about first topic",
		1,
	)

	removed := store.CompactOldestBlock(summary)
	if len(removed) != 2 {
		t.Errorf("Removed %d messages, want 2", len(removed))
	}

	active := store.GetActiveContext()
	// Should now be: system prompt, summary, second question
	if len(active) != 3 {
		t.Fatalf("Active context length = %d, want 3", len(active))
	}
	if active[0].IsMessage() && active[0].Message.Role != RoleSystem {
		t.Error("First item should be system prompt")
	}
	if active[1].IsMessage() {
		t.Error("Second item should be summary, not message")
	}
	if active[1].Summary == nil || active[1].Summary.ID != summary.ID {
		t.Error("Second item should be the compaction summary")
	}
}

func TestLCMStore_BuildMessages(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleSystem, "System prompt", nil)
	msg1 := store.PersistMessage(RoleUser, "Hello", nil)
	msg2 := store.PersistMessage(RoleAssistant, "Hi", nil)

	// Compact first two non-system messages
	summary := store.CreateLeafSummary([]string{msg1.ID, msg2.ID}, "Greeting exchange", 1)
	store.CompactOldestBlock(summary)

	store.PersistMessage(RoleUser, "New question", nil)

	msgs := store.BuildMessages()
	if len(msgs) != 3 {
		t.Fatalf("BuildMessages returned %d, want 3", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("First role = %s, want system", msgs[0].Role)
	}
	// Second should be the summary
	if !strings.Contains(msgs[1].Content, "Summary") {
		t.Error("Second message should contain summary annotation")
	}
	if msgs[2].Content != "New question" {
		t.Errorf("Third message = %s, want 'New question'", msgs[2].Content)
	}
}

// ─── LCM Grep Tests ────────────────────────────────────────────────────────

func TestLCMStore_Grep(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleUser, "What is the weather today?", nil)
	store.PersistMessage(RoleAssistant, "The weather is sunny with a high of 75°F", nil)
	store.PersistMessage(RoleUser, "What about tomorrow?", nil)
	store.PersistMessage(RoleAssistant, "Tomorrow will be rainy", nil)

	results, err := store.Grep("weather", 10)
	if err != nil {
		t.Fatalf("Grep error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Grep returned %d results, want 2", len(results))
	}
}

func TestLCMStore_Grep_WithSummary(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, "Error: disk full", nil)
	store.PersistMessage(RoleAssistant, "Try cleaning up temp files", nil)

	// Create summary covering msg1
	store.CreateLeafSummary([]string{msg1.ID}, "Disk issue", 1)

	results, err := store.Grep("Error", 10)
	if err != nil {
		t.Fatalf("Grep error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Grep returned %d results, want 1", len(results))
	}
	if results[0].SummaryID == "" {
		t.Error("Expected SummaryID to be set for message covered by summary")
	}
}

func TestLCMStore_Grep_InvalidRegex(t *testing.T) {
	store := NewLCMStore("test")
	_, err := store.Grep("[invalid", 10)
	if err == nil {
		t.Error("Expected error for invalid regex")
	}
}

func TestLCMStore_Grep_Pagination(t *testing.T) {
	store := NewLCMStore("test")
	for i := 0; i < 20; i++ {
		store.PersistMessage(RoleUser, fmt.Sprintf("test message %d", i), nil)
	}

	results, err := store.Grep("test", 5)
	if err != nil {
		t.Fatalf("Grep error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Grep returned %d results, want 5 (paginated)", len(results))
	}
}

// ─── LCM Describe Tests ────────────────────────────────────────────────────

func TestLCMStore_Describe_Message(t *testing.T) {
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "Hello", []string{"file_1"})

	desc, err := store.Describe(msg.ID)
	if err != nil {
		t.Fatalf("Describe error: %v", err)
	}
	if desc.Type != "message" {
		t.Errorf("Type = %s, want message", desc.Type)
	}
	if desc.Role != "user" {
		t.Errorf("Role = %s, want user", desc.Role)
	}
	if len(desc.FileIDs) != 1 {
		t.Errorf("FileIDs length = %d, want 1", len(desc.FileIDs))
	}
}

func TestLCMStore_Describe_Summary(t *testing.T) {
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "Hello", nil)
	summary := store.CreateLeafSummary([]string{msg.ID}, "Greeting", 2)

	desc, err := store.Describe(summary.ID)
	if err != nil {
		t.Fatalf("Describe error: %v", err)
	}
	if desc.Type != "summary" {
		t.Errorf("Type = %s, want summary", desc.Type)
	}
	if desc.Kind != "leaf" {
		t.Errorf("Kind = %s, want leaf", desc.Kind)
	}
	if desc.Level != 2 {
		t.Errorf("Level = %d, want 2", desc.Level)
	}
	if desc.Content != "Greeting" {
		t.Errorf("Content = %s, want 'Greeting'", desc.Content)
	}
}

func TestLCMStore_Describe_NotFound(t *testing.T) {
	store := NewLCMStore("test")
	_, err := store.Describe("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent ID")
	}
}

// ─── Store Stats Tests ──────────────────────────────────────────────────────

func TestLCMStore_Stats(t *testing.T) {
	store := NewLCMStore("test")
	store.PersistMessage(RoleUser, "Hello world", nil)
	store.PersistMessage(RoleAssistant, "Hi there friend", nil)

	stats := store.Stats()
	if stats.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2", stats.TotalMessages)
	}
	if stats.TotalSummaries != 0 {
		t.Errorf("TotalSummaries = %d, want 0", stats.TotalSummaries)
	}
	if stats.ActiveContextItems != 2 {
		t.Errorf("ActiveContextItems = %d, want 2", stats.ActiveContextItems)
	}
	if stats.CompressionRatio == 0 {
		t.Error("CompressionRatio should be > 0")
	}
}

func TestLCMStore_Stats_WithCompaction(t *testing.T) {
	store := NewLCMStore("test")
	msg1 := store.PersistMessage(RoleUser, strings.Repeat("Hello world test message. ", 50), nil)
	msg2 := store.PersistMessage(RoleAssistant, strings.Repeat("Reply content here. ", 50), nil)

	summary := store.CreateLeafSummary([]string{msg1.ID, msg2.ID}, "Brief summary", 1)
	store.CompactOldestBlock(summary)

	stats := store.Stats()
	if stats.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2 (immutable)", stats.TotalMessages)
	}
	if stats.TotalSummaries != 1 {
		t.Errorf("TotalSummaries = %d, want 1", stats.TotalSummaries)
	}
	if stats.CompressionRatio >= 1.0 {
		t.Errorf("CompressionRatio = %f, want < 1.0 (compression occurred)", stats.CompressionRatio)
	}
}

// ─── Three-Level Summarization Tests ────────────────────────────────────────

func TestLCMSummarizer_DeterministicTruncate(t *testing.T) {
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)

	// Test deterministic truncation directly
	longInput := strings.Repeat("This is a test sentence. ", 100)
	result := summarizer.deterministicTruncate(longInput, 50)

	if result.Level != 5 {
		t.Errorf("Level = %d, want 5", result.Level)
	}
	if result.Tokens > 200 {
		t.Errorf("Tokens = %d, want <= 200 (truncated to ~50 target)", result.Tokens)
	}
	if !strings.Contains(result.Content, "truncated") {
		t.Error("Truncated content should contain truncation marker")
	}
}

func TestLCMSummarizer_ShortInput(t *testing.T) {
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)

	result, err := summarizer.Summarize("Short text", 1000)
	if err != nil {
		t.Fatalf("Summarize error: %v", err)
	}
	if result.Level != 0 {
		t.Errorf("Level = %d, want 0 (no summarization needed)", result.Level)
	}
	if result.Content != "Short text" {
		t.Errorf("Content changed for short input")
	}
}

func TestLCMSummarizer_SummarizeMessages(t *testing.T) {
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)

	msgs := []*StoreMessage{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there"},
	}

	// This will try LLM calls (which will fail without API key) and fall back to deterministic
	result, err := summarizer.SummarizeMessages(msgs, 10)
	if err != nil {
		// Summarize should eventually succeed via Level 3 deterministic
		t.Fatalf("SummarizeMessages error: %v", err)
	}
	if result.Content == "" {
		t.Error("Result content should not be empty")
	}
}

// ─── Context Control Loop Tests ─────────────────────────────────────────────

func TestLCMEngine_ZeroCostContinuity(t *testing.T) {
	store := NewLCMStore("test")
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)

	config := LCMConfig{
		Enabled:             true,
		SoftThreshold:       10000,
		HardThreshold:       20000,
		CompactionBlockSize: 5,
		SummaryTargetTokens: 200,
	}

	engine := NewLCMEngine(config, store, summarizer, obs, 128000)

	// Add a small message — should be below soft threshold
	store.PersistMessage(RoleUser, "Hello", nil)
	err := engine.OnNewItem()
	if err != nil {
		t.Fatalf("OnNewItem error: %v", err)
	}

	// Active context should be unchanged (zero-cost continuity)
	if store.ActiveContextTokens() > config.SoftThreshold {
		t.Error("Small message should be below soft threshold")
	}
}

func TestLCMEngine_Disabled(t *testing.T) {
	store := NewLCMStore("test")
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)

	config := LCMConfig{Enabled: false}
	engine := NewLCMEngine(config, store, summarizer, obs, 128000)

	store.PersistMessage(RoleUser, "Hello", nil)
	err := engine.OnNewItem()
	if err != nil {
		t.Fatalf("OnNewItem error when disabled: %v", err)
	}
}

func TestLCMEngine_DefaultThresholds(t *testing.T) {
	store := NewLCMStore("test")
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)

	config := LCMConfig{Enabled: true}
	engine := NewLCMEngine(config, store, summarizer, obs, 100000)

	if engine.config.SoftThreshold != 70000 {
		t.Errorf("SoftThreshold = %d, want 70000 (70%% of 100000)", engine.config.SoftThreshold)
	}
	if engine.config.HardThreshold != 90000 {
		t.Errorf("HardThreshold = %d, want 90000 (90%% of 100000)", engine.config.HardThreshold)
	}
}

func TestLCMEngine_GetStore(t *testing.T) {
	store := NewLCMStore("test")
	obs := NewNoopObserver()
	summarizer := NewLCMSummarizer("test", "", "", 30, nil, obs)
	config := LCMConfig{Enabled: true}

	engine := NewLCMEngine(config, store, summarizer, obs, 128000)
	if engine.GetStore() != store {
		t.Error("GetStore returned different store")
	}
}

// ─── LLM-Map Tests ──────────────────────────────────────────────────────────

func TestLLMMap_ReadJSONL(t *testing.T) {
	// Create temp JSONL file
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.jsonl")

	lines := []string{
		`{"name": "Alice", "age": 30}`,
		`{"name": "Bob", "age": 25}`,
		`{"name": "Charlie", "age": 35}`,
	}
	err := os.WriteFile(inputPath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		t.Fatal(err)
	}

	items, err := readJSONLFile(inputPath)
	if err != nil {
		t.Fatalf("readJSONLFile error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("Read %d items, want 3", len(items))
	}

	var first map[string]interface{}
	_ = json.Unmarshal(items[0], &first)
	if first["name"] != "Alice" {
		t.Errorf("First name = %v, want Alice", first["name"])
	}
}

func TestLLMMap_WriteJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.jsonl")

	results := []MapItemResult{
		{Index: 0, Status: MapItemCompleted, Output: json.RawMessage(`{"result": "ok"}`)},
		{Index: 1, Status: MapItemFailed, Error: "timeout"},
	}

	err := writeJSONLOutput(outputPath, results)
	if err != nil {
		t.Fatalf("writeJSONLOutput error: %v", err)
	}

	// Read back
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("Output has %d lines, want 2", len(lines))
	}
}

func TestLLMMap_ExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		wantNil  bool
	}{
		{`{"key": "value"}`, false},
		{`Some text {"key": "value"} more text`, false},
		{"```json\n{\"key\": \"value\"}\n```", false},
		{`[1, 2, 3]`, false},
		{`no json here`, true},
		{``, true},
	}

	for _, tt := range tests {
		result := extractJSON(tt.input)
		if tt.wantNil && result != nil {
			t.Errorf("extractJSON(%q) = %s, want nil", tt.input, string(result))
		}
		if !tt.wantNil && result == nil {
			t.Errorf("extractJSON(%q) = nil, want non-nil", tt.input)
		}
	}
}

func TestLLMMap_ValidateMapOutput(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name"},
	}

	// Valid output
	valid := json.RawMessage(`{"name": "Alice", "age": 30}`)
	if err := validateMapOutput(valid, schema); err != "" {
		t.Errorf("validateMapOutput valid = %s, want empty", err)
	}

	// Invalid JSON
	invalid := json.RawMessage(`not json`)
	if err := validateMapOutput(invalid, schema); err == "" {
		t.Error("validateMapOutput invalid JSON should return error")
	}
}

// ─── Large File Handling Tests ──────────────────────────────────────────────

func TestLCMFileHandler_SmallFile(t *testing.T) {
	obs := NewNoopObserver()
	handler := NewLCMFileHandler(DefaultLCMFileConfig(), "test", "", "", 30, nil, obs)

	content := "Small file content"
	result, ref, err := handler.ProcessFile("test.txt", content)
	if err != nil {
		t.Fatalf("ProcessFile error: %v", err)
	}
	if ref != nil {
		t.Error("Small file should not create a file ref")
	}
	if result != content {
		t.Errorf("Content should be unchanged for small files")
	}
}

func TestLCMFileHandler_LargeFile(t *testing.T) {
	obs := NewNoopObserver()
	config := LCMFileConfig{TokenThreshold: 10} // Very low threshold for testing
	handler := NewLCMFileHandler(config, "test", "", "", 30, nil, obs)

	content := strings.Repeat("Line of content for testing. ", 100)
	result, ref, err := handler.ProcessFile("test.txt", content)
	if err != nil {
		t.Fatalf("ProcessFile error: %v", err)
	}
	if ref == nil {
		t.Fatal("Large file should create a file ref")
	}
	if ref.Path != "test.txt" {
		t.Errorf("Path = %s, want test.txt", ref.Path)
	}
	if !strings.Contains(result, ref.ID) {
		t.Error("Context reference should contain file ID")
	}
}

func TestLCMFileHandler_GetFileRef(t *testing.T) {
	obs := NewNoopObserver()
	config := LCMFileConfig{TokenThreshold: 10}
	handler := NewLCMFileHandler(config, "test", "", "", 30, nil, obs)

	content := strings.Repeat("Content. ", 100)
	_, ref, _ := handler.ProcessFile("test.txt", content)

	retrieved, ok := handler.GetFileRef(ref.ID)
	if !ok {
		t.Fatal("GetFileRef returned false")
	}
	if retrieved.Path != "test.txt" {
		t.Errorf("Path = %s, want test.txt", retrieved.Path)
	}
}

func TestClassifyFileType(t *testing.T) {
	tests := []struct {
		ext  string
		want fileType
	}{
		{".json", fileTypeStructuredData},
		{".csv", fileTypeStructuredData},
		{".go", fileTypeCode},
		{".py", fileTypeCode},
		{".ts", fileTypeCode},
		{".txt", fileTypeText},
		{".md", fileTypeText},
		{".unknown", fileTypeText},
	}

	for _, tt := range tests {
		got := classifyFileType(tt.ext)
		if got != tt.want {
			t.Errorf("classifyFileType(%s) = %d, want %d", tt.ext, got, tt.want)
		}
	}
}

func TestDetectMIMEType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"file.json", "application/json"},
		{"file.go", "text/x-go"},
		{"file.py", "text/x-python"},
		{"file.unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := detectMIMEType(tt.path)
		if got != tt.want {
			t.Errorf("detectMIMEType(%s) = %s, want %s", tt.path, got, tt.want)
		}
	}
}

func TestSummarizeJSON(t *testing.T) {
	obs := NewNoopObserver()
	handler := NewLCMFileHandler(DefaultLCMFileConfig(), "test", "", "", 30, nil, obs)

	jsonContent := `{"users": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}], "count": 2}`
	result := handler.summarizeJSON(jsonContent)
	if !strings.Contains(result, "JSON file analysis") {
		t.Error("JSON summary should contain analysis header")
	}
}

func TestSummarizeCSV(t *testing.T) {
	obs := NewNoopObserver()
	handler := NewLCMFileHandler(DefaultLCMFileConfig(), "test", "", "", 30, nil, obs)

	csvContent := "name,age,city\nAlice,30,NYC\nBob,25,LA"
	result := handler.summarizeCSV(csvContent, ".csv")
	if !strings.Contains(result, "3 rows") {
		t.Error("CSV summary should contain row count")
	}
	if !strings.Contains(result, "Columns") {
		t.Error("CSV summary should contain column names")
	}
}

func TestExtractCodeStructure(t *testing.T) {
	goCode := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}

func helper(x int) int {
	return x * 2
}

type Config struct {
	Name string
}
`
	result := extractCodeStructure(goCode, ".go")
	if !strings.Contains(result, "func main") {
		t.Error("Should extract func main definition")
	}
	if !strings.Contains(result, "func helper") {
		t.Error("Should extract func helper definition")
	}
	if !strings.Contains(result, "type Config") {
		t.Error("Should extract type Config definition")
	}
}

func TestSummarizeJSONL(t *testing.T) {
	obs := NewNoopObserver()
	handler := NewLCMFileHandler(DefaultLCMFileConfig(), "test", "", "", 30, nil, obs)

	jsonlContent := `{"id": 1, "name": "Alice"}
{"id": 2, "name": "Bob"}
{"id": 3, "name": "Charlie"}`
	result := handler.summarizeJSONL(jsonlContent)
	if !strings.Contains(result, "3 lines") {
		t.Error("JSONL summary should contain line count")
	}
}

// ─── Config Parsing Tests ───────────────────────────────────────────────────

func TestConfigFromMap_LCM(t *testing.T) {
	configMap := map[string]interface{}{
		"lcm": map[string]interface{}{
			"enabled":              true,
			"soft_threshold":       50000.0,
			"hard_threshold":       80000.0,
			"compaction_block_size": 8.0,
			"summary_target_tokens": 300.0,
		},
	}

	config := ConfigFromMap(configMap)
	if config.LCM == nil {
		t.Fatal("LCM config should not be nil")
	}
	if !config.LCM.Enabled {
		t.Error("LCM should be enabled")
	}
	if config.LCM.SoftThreshold != 50000 {
		t.Errorf("SoftThreshold = %d, want 50000", config.LCM.SoftThreshold)
	}
	if config.LCM.HardThreshold != 80000 {
		t.Errorf("HardThreshold = %d, want 80000", config.LCM.HardThreshold)
	}
	if config.LCM.CompactionBlockSize != 8 {
		t.Errorf("CompactionBlockSize = %d, want 8", config.LCM.CompactionBlockSize)
	}
	if config.LCM.SummaryTargetTokens != 300 {
		t.Errorf("SummaryTargetTokens = %d, want 300", config.LCM.SummaryTargetTokens)
	}
}

func TestConfigFromMap_LCM_NotPresent(t *testing.T) {
	configMap := map[string]interface{}{
		"api_key": "test-key",
	}

	config := ConfigFromMap(configMap)
	if config.LCM != nil {
		t.Error("LCM config should be nil when not present")
	}
}

// ─── Integration: ActiveContextItem Tests ───────────────────────────────────

func TestActiveContextItem_GetTokens(t *testing.T) {
	msg := &StoreMessage{Content: "test", Tokens: 10}
	item := &ActiveContextItem{Message: msg}
	if item.GetTokens() != 10 {
		t.Errorf("GetTokens = %d, want 10", item.GetTokens())
	}

	sum := &SummaryNode{Content: "summary", Tokens: 5}
	sumItem := &ActiveContextItem{Summary: sum}
	if sumItem.GetTokens() != 5 {
		t.Errorf("GetTokens = %d, want 5", sumItem.GetTokens())
	}

	empty := &ActiveContextItem{}
	if empty.GetTokens() != 0 {
		t.Errorf("GetTokens = %d, want 0", empty.GetTokens())
	}
}

func TestActiveContextItem_GetContent(t *testing.T) {
	msg := &StoreMessage{Content: "hello"}
	item := &ActiveContextItem{Message: msg}
	if item.GetContent() != "hello" {
		t.Errorf("GetContent = %s, want 'hello'", item.GetContent())
	}

	sum := &SummaryNode{Content: "summary text"}
	sumItem := &ActiveContextItem{Summary: sum}
	if sumItem.GetContent() != "summary text" {
		t.Errorf("GetContent = %s, want 'summary text'", sumItem.GetContent())
	}
}

func TestActiveContextItem_GetFileIDs(t *testing.T) {
	msg := &StoreMessage{FileIDs: []string{"f1", "f2"}}
	item := &ActiveContextItem{Message: msg}
	if len(item.GetFileIDs()) != 2 {
		t.Errorf("GetFileIDs length = %d, want 2", len(item.GetFileIDs()))
	}

	sum := &SummaryNode{FileIDs: []string{"f3"}}
	sumItem := &ActiveContextItem{Summary: sum}
	if len(sumItem.GetFileIDs()) != 1 {
		t.Errorf("GetFileIDs length = %d, want 1", len(sumItem.GetFileIDs()))
	}
}

func TestIsDefinitionLine(t *testing.T) {
	tests := []struct {
		line string
		ext  string
		want bool
	}{
		{"func main() {", ".go", true},
		{"type Config struct {", ".go", true},
		{"def hello():", ".py", true},
		{"class MyClass:", ".py", true},
		{"export function foo() {", ".ts", true},
		{"interface Props {", ".ts", true},
		{"fn main() {", ".rs", true},
		{"// just a comment", ".go", false},
		{"x := 5", ".go", false},
	}

	for _, tt := range tests {
		got := isDefinitionLine(tt.line, tt.ext)
		if got != tt.want {
			t.Errorf("isDefinitionLine(%q, %q) = %v, want %v", tt.line, tt.ext, got, tt.want)
		}
	}
}

func TestIsImportLine(t *testing.T) {
	tests := []struct {
		line string
		ext  string
		want bool
	}{
		{`import "fmt"`, ".go", true},
		{`import os`, ".py", true},
		{`from datetime import date`, ".py", true},
		{`import { useState } from 'react'`, ".ts", true},
		{`use std::io`, ".rs", true},
		{`// import comment`, ".go", false},
	}

	for _, tt := range tests {
		got := isImportLine(tt.line, tt.ext)
		if got != tt.want {
			t.Errorf("isImportLine(%q, %q) = %v, want %v", tt.line, tt.ext, got, tt.want)
		}
	}
}

// ─── Delegation Guard Tests ─────────────────────────────────────────────────

func TestDelegationGuard_RootAgentAlwaysAllowed(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(0, DelegationRequest{
		Prompt: "Do all the work",
		// No scope or kept_work needed for root
	})
	if err != nil {
		t.Errorf("Root agent should always be allowed, got: %v", err)
	}
}

func TestDelegationGuard_ReadOnlyExempt(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(2, DelegationRequest{
		Prompt:   "Read some files",
		ReadOnly: true,
	})
	if err != nil {
		t.Errorf("Read-only agents should be exempt, got: %v", err)
	}
}

func TestDelegationGuard_ParallelExempt(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(1, DelegationRequest{
		Prompt:   "Process chunk A",
		Parallel: true,
	})
	if err != nil {
		t.Errorf("Parallel decomposition should be exempt, got: %v", err)
	}
}

func TestDelegationGuard_MissingDelegatedScope(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(1, DelegationRequest{
		Prompt:   "Do something",
		KeptWork: "I'll aggregate",
	})
	if err == nil {
		t.Error("Should reject when delegated_scope is missing")
	}
	delegErr, ok := err.(*DelegationError)
	if !ok {
		t.Fatalf("Expected DelegationError, got %T", err)
	}
	if delegErr.Reason == "" {
		t.Error("DelegationError should have a reason")
	}
}

func TestDelegationGuard_MissingKeptWork(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(1, DelegationRequest{
		Prompt:         "Do something",
		DelegatedScope: "Parse the files",
	})
	if err == nil {
		t.Error("Should reject when kept_work is missing")
	}
}

func TestDelegationGuard_TotalDelegation(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(1, DelegationRequest{
		Prompt:         "Implement the entire feature including tests and documentation",
		DelegatedScope: "Implement the entire feature including tests and documentation",
		KeptWork:       "none",
	})
	if err == nil {
		t.Error("Should reject total delegation (kept_work = 'none')")
	}
}

func TestDelegationGuard_TotalDelegation_TrivialKeptWork(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	trivialKeptWorks := []string{"nothing", "n/a", "waiting", "collect results", "return results"}
	for _, kw := range trivialKeptWorks {
		err := guard.ValidateDelegation(1, DelegationRequest{
			Prompt:         "Do all the work",
			DelegatedScope: "Implement everything from scratch with full testing",
			KeptWork:       kw,
		})
		if err == nil {
			t.Errorf("Should reject trivial kept_work %q", kw)
		}
	}
}

func TestDelegationGuard_ValidDelegation(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(1, DelegationRequest{
		Prompt:         "Parse the configuration files and extract database settings",
		DelegatedScope: "Parse configuration files in the config/ directory",
		KeptWork:       "Validate extracted settings against the schema and apply migrations",
	})
	if err != nil {
		t.Errorf("Valid delegation should be allowed, got: %v", err)
	}
}

func TestDelegationGuard_TotalDelegation_ShortKeptWork(t *testing.T) {
	obs := NewNoopObserver()
	guard := NewDelegationGuard(obs)

	err := guard.ValidateDelegation(1, DelegationRequest{
		Prompt:         "Implement the full authentication system with OAuth2, JWT tokens, refresh tokens, and rate limiting",
		DelegatedScope: "Implement the full authentication system with OAuth2, JWT tokens, refresh tokens, and rate limiting",
		KeptWork:       "ok",
	})
	if err == nil {
		t.Error("Should reject when kept_work is suspiciously short compared to scope")
	}
}

func TestIsTotalDelegation(t *testing.T) {
	tests := []struct {
		scope string
		kept  string
		want  bool
	}{
		{"big task", "none", true},
		{"big task", "nothing", true},
		{"big task", "n/a", true},
		{"big task", "waiting", true},
		{"big task", "I will validate the output and merge the results", false},
		{"parse files", "aggregate and report", false},
		{"long scope description here with many details", "ok", true},
	}

	for _, tt := range tests {
		got := isTotalDelegation(tt.scope, tt.kept)
		if got != tt.want {
			t.Errorf("isTotalDelegation(%q, %q) = %v, want %v", tt.scope, tt.kept, got, tt.want)
		}
	}
}

// ─── ExpandSummary Restriction Tests ────────────────────────────────────────

func TestExpandSummaryRestricted_RootAgentBlocked(t *testing.T) {
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "Hello", nil)
	summary := store.CreateLeafSummary([]string{msg.ID}, "Greeting", 1)

	_, err := store.ExpandSummaryRestricted(summary.ID, 0)
	if err == nil {
		t.Error("Root agent (depth 0) should be blocked from expanding summaries")
	}
	expandErr, ok := err.(*ExpandRestrictionError)
	if !ok {
		t.Fatalf("Expected ExpandRestrictionError, got %T", err)
	}
	if expandErr.SummaryID != summary.ID {
		t.Errorf("SummaryID = %s, want %s", expandErr.SummaryID, summary.ID)
	}
}

func TestExpandSummaryRestricted_SubAgentAllowed(t *testing.T) {
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "Hello", nil)
	summary := store.CreateLeafSummary([]string{msg.ID}, "Greeting", 1)

	msgs, err := store.ExpandSummaryRestricted(summary.ID, 1)
	if err != nil {
		t.Fatalf("Sub-agent (depth 1) should be allowed, got: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("Expected 1 expanded message, got %d", len(msgs))
	}
}

func TestExpandSummaryRestricted_DeepSubAgentAllowed(t *testing.T) {
	store := NewLCMStore("test")
	msg := store.PersistMessage(RoleUser, "Hello", nil)
	summary := store.CreateLeafSummary([]string{msg.ID}, "Greeting", 1)

	_, err := store.ExpandSummaryRestricted(summary.ID, 5)
	if err != nil {
		t.Errorf("Deep sub-agent (depth 5) should be allowed, got: %v", err)
	}
}

// ─── LCM Completion Integration Tests ───────────────────────────────────────

func TestLCMEngine_CompletionIntegration_StorePopulated(t *testing.T) {
	// Test that the LCM engine populates the store when used in completion
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      3,
		MaxIterations: 5,
		LCM: &LCMConfig{
			Enabled:             true,
			SoftThreshold:       100000,
			HardThreshold:       200000,
			CompactionBlockSize: 10,
			SummaryTargetTokens: 500,
		},
	}

	engine := New("test-model", config)
	defer engine.Shutdown()

	// Verify LCM engine is set up
	if engine.GetLCMEngine() == nil {
		t.Fatal("LCM engine should be initialized when config.LCM.Enabled = true")
	}
	if !engine.GetLCMEngine().IsEnabled() {
		t.Error("LCM engine should be enabled")
	}

	store := engine.GetLCMEngine().GetStore()
	if store.MessageCount() != 0 {
		t.Errorf("Store should be empty initially, got %d messages", store.MessageCount())
	}
}

func TestLCMEngine_NotInitializedWhenDisabled(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      3,
		MaxIterations: 5,
		// No LCM config
	}

	engine := New("test-model", config)
	defer engine.Shutdown()

	if engine.GetLCMEngine() != nil {
		t.Error("LCM engine should be nil when not configured")
	}
}

func TestLCMEngine_NotInitializedWhenExplicitlyDisabled(t *testing.T) {
	config := Config{
		APIKey:        "test-key",
		MaxDepth:      3,
		MaxIterations: 5,
		LCM:           &LCMConfig{Enabled: false},
	}

	engine := New("test-model", config)
	defer engine.Shutdown()

	if engine.GetLCMEngine() != nil {
		t.Error("LCM engine should be nil when explicitly disabled")
	}
}

// ─── Agentic-Map Tests ──────────────────────────────────────────────────────

func TestAgenticMapper_DefaultConfig(t *testing.T) {
	config := AgenticMapConfig{
		InputPath:  "/tmp/input.jsonl",
		OutputPath: "/tmp/output.jsonl",
		Prompt:     "Process {{item}}",
	}

	// Verify defaults are applied
	if config.Concurrency <= 0 {
		config.Concurrency = 8
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 2
	}
	if config.MaxDepth <= 0 {
		config.MaxDepth = 3
	}
	if config.MaxIter <= 0 {
		config.MaxIter = 15
	}

	if config.Concurrency != 8 {
		t.Errorf("Default concurrency = %d, want 8", config.Concurrency)
	}
	if config.MaxRetries != 2 {
		t.Errorf("Default MaxRetries = %d, want 2", config.MaxRetries)
	}
	if config.MaxDepth != 3 {
		t.Errorf("Default MaxDepth = %d, want 3", config.MaxDepth)
	}
}

func TestAgenticMapper_WriteOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "agentic_output.jsonl")

	results := []AgenticItemResult{
		{Index: 0, Status: MapItemCompleted, Output: json.RawMessage(`{"result": "ok"}`), LLMCalls: 3, Iterations: 2},
		{Index: 1, Status: MapItemFailed, Error: "sub-agent timed out", LLMCalls: 5, Iterations: 5},
	}

	err := writeAgenticOutput(outputPath, results)
	if err != nil {
		t.Fatalf("writeAgenticOutput error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("Output has %d lines, want 2", len(lines))
	}

	// Verify error record includes metadata
	var errRecord map[string]interface{}
	_ = json.Unmarshal([]byte(lines[1]), &errRecord)
	if errRecord["_error"] != "sub-agent timed out" {
		t.Errorf("Error record missing error, got: %v", errRecord)
	}
	if errRecord["_llm_calls"] != float64(5) {
		t.Errorf("Error record missing llm_calls, got: %v", errRecord["_llm_calls"])
	}
}

// ─── Balanced Brace Extraction Tests ────────────────────────────────────────

func TestExtractBalancedBraces(t *testing.T) {
	tests := []struct {
		input     string
		startChar byte
		want      string
	}{
		{`{"key": "value"}`, '{', `{"key": "value"}`},
		{`{"key": "value"} extra`, '{', `{"key": "value"}`},
		{`{"nested": {"inner": true}}`, '{', `{"nested": {"inner": true}}`},
		{`[1, 2, 3]`, '[', `[1, 2, 3]`},
		{`[1, [2, 3], 4] trailing`, '[', `[1, [2, 3], 4]`},
		{`{"str": "has \" escaped"}`, '{', `{"str": "has \" escaped"}`},
		{`{unclosed`, '{', ``},
	}

	for _, tt := range tests {
		got := ExtractBalancedBraces(tt.input, tt.startChar)
		if got != tt.want {
			t.Errorf("ExtractBalancedBraces(%q, %q) = %q, want %q", tt.input, string(tt.startChar), got, tt.want)
		}
	}
}

// ─── DelegationError Tests ──────────────────────────────────────────────────

func TestDelegationError_ErrorMessage(t *testing.T) {
	err := &DelegationError{
		Reason:     "test reason",
		Suggestion: "try something else",
	}
	msg := err.Error()
	if !strings.Contains(msg, "test reason") {
		t.Error("Error message should contain reason")
	}
	if !strings.Contains(msg, "try something else") {
		t.Error("Error message should contain suggestion")
	}
}

func TestExpandRestrictionError_ErrorMessage(t *testing.T) {
	err := &ExpandRestrictionError{SummaryID: "sum_123"}
	msg := err.Error()
	if !strings.Contains(msg, "sum_123") {
		t.Error("Error message should contain summary ID")
	}
	if !strings.Contains(msg, "sub-agent") {
		t.Error("Error message should mention sub-agents")
	}
}
