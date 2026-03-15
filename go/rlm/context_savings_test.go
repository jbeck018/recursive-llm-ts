package rlm

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func deterministicSentence(idx int) string {
	topics := []string{
		"architecture", "testing", "performance", "reliability", "observability",
		"security", "scalability", "maintainability", "usability", "automation",
	}
	details := []string{
		"input validation", "error handling", "resource limits", "data flow", "boundary conditions",
		"traceability", "deployment safety", "schema consistency", "latency targets", "integration behavior",
	}

	topic := topics[idx%len(topics)]
	detail := details[(idx*7)%len(details)]
	return fmt.Sprintf("Sentence %d discusses topic %s with details about %s. ", idx, topic, detail)
}

func generateDeterministicContext(targetTokens int) string {
	if targetTokens <= 0 {
		return ""
	}

	var b strings.Builder
	total := 0
	for i := 1; total < targetTokens; i++ {
		s := deterministicSentence(i)
		b.WriteString(s)
		total += EstimateTokens(s)
	}
	return b.String()
}

func fixedEnglishProse500Words() string {
	words := []string{
		"software", "teams", "benefit", "from", "clear", "requirements", "because", "stable", "interfaces", "reduce",
		"rework", "and", "improve", "delivery", "predictability", "when", "engineers", "document", "assumptions", "carefully",
		"review", "cycles", "become", "faster", "while", "quality", "signals", "remain", "visible", "across",
		"planning", "implementation", "testing", "and", "maintenance", "phases", "in", "long", "lived", "systems",
	}

	var b strings.Builder
	for i := 0; i < 500; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		w := words[i%len(words)]
		if (i+1)%25 == 0 {
			w += "."
		}
		b.WriteString(w)
	}
	return b.String()
}

func percentDifference(base, compare int) float64 {
	if base == 0 {
		return 0
	}
	return (float64(compare-base) / float64(base)) * 100
}

func percentSavings(original, reduced int) float64 {
	if original <= 0 {
		return 0
	}
	return (float64(original-reduced) / float64(original)) * 100
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func preservesOriginalSentences(original, reduced string) bool {
	originalSentences := SplitSentences(original)
	if len(originalSentences) == 0 {
		return true
	}

	origSet := make(map[string]bool, len(originalSentences))
	for _, s := range originalSentences {
		origSet[strings.TrimSpace(s)] = true
	}

	for _, s := range SplitSentences(reduced) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.Contains(s, "content truncated") {
			continue
		}
		if !origSet[s] {
			return false
		}
	}
	return true
}

func episodeContextCost(episodes []*Episode) int {
	total := 0
	for _, ep := range episodes {
		cost := ep.Tokens
		if ep.Status != EpisodeActive && ep.SummaryTokens > 0 {
			cost = ep.SummaryTokens
		}
		total += cost
	}
	return total
}

func TestContextSavings_TokenizerAccuracy(t *testing.T) {
	useHeuristicTokenizerForTest(t)

	bpeTokenizer, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		t.Fatalf("failed to create BPE tokenizer: %v", err)
	}

	goSnippet := `package main

import (
	"fmt"
	"strings"
)

func summarize(items []string) map[string]int {
	result := map[string]int{}
	for _, item := range items {
		normalized := strings.TrimSpace(strings.ToLower(item))
		if normalized == "" {
			continue
		}
		result[normalized]++
	}
	return result
}

func main() {
	data := []string{"alpha", "beta", "alpha", "gamma", "beta", "alpha"}
	stats := summarize(data)
	fmt.Println("stats:", stats)
}
`

	jsonData := `{
  "project": "recursive-llm-ts",
  "version": "1.0.0",
  "features": {
    "lcm": true,
    "observability": true,
    "context_overflow": {
      "enabled": true,
      "strategy": "tfidf",
      "max_reduction_attempts": 3
    }
  },
  "items": [
    {"id": 1, "name": "alpha", "priority": "high"},
    {"id": 2, "name": "beta", "priority": "medium"},
    {"id": 3, "name": "gamma", "priority": "low"}
  ]
}`

	cjkText := "这是一个固定的中文测试句子，用于衡量分词稳定性。日本語の固定テスト文を使ってトークン数を比較します。고정된 한국어 문장으로 토큰 계산 결과를 확인합니다。"

	testCases := []struct {
		name    string
		content string
	}{
		{name: "english_prose", content: fixedEnglishProse500Words()},
		{name: "go_code", content: goSnippet},
		{name: "json", content: jsonData},
		{name: "cjk", content: cjkText},
	}

	t.Logf("Tokenizer accuracy comparison (heuristic default + direct BPE)")
	for _, tc := range testCases {
		heuristic := EstimateTokens(tc.content)
		bpe := bpeTokenizer.CountTokens(tc.content)
		chars := len([]rune(tc.content))
		diffPct := percentDifference(bpe, heuristic)

		t.Logf("type=%-14s chars=%5d heuristic=%5d bpe=%5d diff=%7.2f%%", tc.name, chars, heuristic, bpe, diffPct)

		if heuristic <= 0 {
			t.Fatalf("heuristic token count should be > 0 for %s", tc.name)
		}
		if bpe <= 0 {
			t.Fatalf("BPE token count should be > 0 for %s", tc.name)
		}
	}
}

func TestContextSavings_FiveLevelEscalation(t *testing.T) {
	useHeuristicTokenizerForTest(t)

	original := generateDeterministicContext(5000)
	originalTokens := EstimateTokens(original)

	level3 := CompressContextTFIDF(original, 2000)
	level4 := CompressContextTextRank(original, 2000)
	level5 := TruncateText(original, TruncateTextParams{MaxTokens: 2000})

	level3Tokens := EstimateTokens(level3)
	level4Tokens := EstimateTokens(level4)
	level5Tokens := EstimateTokens(level5)

	t.Logf("Five-level non-LLM escalation comparison")
	t.Logf("original_tokens=%d", originalTokens)
	t.Logf("level=3 strategy=tfidf   tokens=%d reduction=%6.2f%% sentence_preserved=%s", level3Tokens, percentSavings(originalTokens, level3Tokens), yesNo(preservesOriginalSentences(original, level3)))
	t.Logf("level=4 strategy=textrank tokens=%d reduction=%6.2f%% sentence_preserved=%s", level4Tokens, percentSavings(originalTokens, level4Tokens), yesNo(preservesOriginalSentences(original, level4)))
	t.Logf("level=5 strategy=truncate tokens=%d reduction=%6.2f%% sentence_preserved=%s", level5Tokens, percentSavings(originalTokens, level5Tokens), yesNo(preservesOriginalSentences(original, level5)))

	if level3Tokens >= originalTokens {
		t.Fatalf("expected TF-IDF to reduce tokens: original=%d level3=%d", originalTokens, level3Tokens)
	}
	if level4Tokens >= originalTokens {
		t.Fatalf("expected TextRank to reduce tokens: original=%d level4=%d", originalTokens, level4Tokens)
	}
	if level5Tokens >= originalTokens {
		t.Fatalf("expected Truncate to reduce tokens: original=%d level5=%d", originalTokens, level5Tokens)
	}
}

func TestContextSavings_EpisodicMemoryBudget(t *testing.T) {
	useHeuristicTokenizerForTest(t)

	manager := NewEpisodeManager("ctx-savings-episodes", EpisodeConfig{
		MaxEpisodeMessages:    5,
		MaxEpisodeTokens:      500,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})

	baseTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	rawTokens := 0
	for i := 0; i < 50; i++ {
		content := fmt.Sprintf("Message %d. %s", i+1, generateDeterministicContext(100))
		tokens := EstimateTokens(content)
		rawTokens += tokens

		manager.AddMessage(&StoreMessage{
			ID:        fmt.Sprintf("msg_%03d", i+1),
			Role:      RoleUser,
			Content:   content,
			Tokens:    tokens,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		})
	}

	episodes := manager.GetAllEpisodes()
	t.Logf("episodes_created=%d (expected around 10)", len(episodes))
	if len(episodes) < 9 || len(episodes) > 11 {
		t.Fatalf("expected around 10 episodes, got %d", len(episodes))
	}

	for i := 0; i < len(episodes)-1; i++ {
		summary := fmt.Sprintf("Episode %d summary. %s", i+1, generateDeterministicContext(30))
		if err := manager.CompactEpisode(episodes[i].ID, summary); err != nil {
			t.Fatalf("failed to compact episode %s: %v", episodes[i].ID, err)
		}
	}

	budgets := []int{200, 500, 1000, 2000}
	for _, budget := range budgets {
		selected := manager.GetEpisodesForContext(budget)
		contextTokens := episodeContextCost(selected)
		savings := percentSavings(rawTokens, contextTokens)
		t.Logf("budget=%4d episodes=%2d context_tokens=%5d raw_tokens=%5d savings=%6.2f%%", budget, len(selected), contextTokens, rawTokens, savings)

		if len(selected) == 0 {
			t.Fatalf("expected at least one episode for budget %d", budget)
		}
	}
}

func TestContextSavings_AllStrategiesComparison(t *testing.T) {
	useHeuristicTokenizerForTest(t)

	original := generateDeterministicContext(35000)
	originalTokens := EstimateTokens(original)
	target := 16000

	tfidf := CompressContextTFIDF(original, target)
	textrank := CompressContextTextRank(original, target)
	truncated := TruncateText(original, TruncateTextParams{MaxTokens: target})

	results := []struct {
		strategy   string
		content    string
		tokens     int
		preserved  bool
	}{
		{strategy: "TF-IDF", content: tfidf, tokens: EstimateTokens(tfidf), preserved: preservesOriginalSentences(original, tfidf)},
		{strategy: "TextRank", content: textrank, tokens: EstimateTokens(textrank), preserved: preservesOriginalSentences(original, textrank)},
		{strategy: "Truncate", content: truncated, tokens: EstimateTokens(truncated), preserved: preservesOriginalSentences(original, truncated)},
	}

	t.Logf("strategy comparison for target=%d tokens (original=%d)", target, originalTokens)
	t.Logf("strategy   output_tokens  reduction%%  sentence_preserved")
	for _, r := range results {
		t.Logf("%-9s %12d  %9.2f%%  %s", r.strategy, r.tokens, percentSavings(originalTokens, r.tokens), yesNo(r.preserved))
		if r.tokens >= originalTokens {
			t.Fatalf("strategy %s did not reduce tokens: original=%d output=%d", r.strategy, originalTokens, r.tokens)
		}
	}
}

func TestContextSavings_CombinedPipeline(t *testing.T) {
	useHeuristicTokenizerForTest(t)

	manager := NewEpisodeManager("ctx-savings-pipeline", EpisodeConfig{
		MaxEpisodeMessages:    10,
		MaxEpisodeTokens:      1000000,
		TopicChangeThreshold:  0.5,
		AutoCompactAfterClose: false,
	})

	baseTime := time.Date(2024, 5, 10, 9, 30, 0, 0, time.UTC)
	messageContentByID := make(map[string]string)
	rawTokens := 0

	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("pipeline_msg_%03d", i+1)
		content := fmt.Sprintf("Message %d segment. %s", i+1, generateDeterministicContext(500))
		tokens := EstimateTokens(content)
		rawTokens += tokens
		messageContentByID[id] = content

		manager.AddMessage(&StoreMessage{
			ID:        id,
			Role:      RoleUser,
			Content:   content,
			Tokens:    tokens,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		})
	}

	episodes := manager.GetAllEpisodes()
	if len(episodes) != 10 {
		t.Fatalf("expected 10 episodes from 100 messages with MaxEpisodeMessages=10, got %d", len(episodes))
	}

	afterGrouping := episodeContextCost(episodes)

	for i := 0; i < len(episodes)-1; i++ {
		ep := episodes[i]
		var b strings.Builder
		for _, msgID := range ep.MessageIDs {
			b.WriteString(messageContentByID[msgID])
			b.WriteString("\n")
		}
		summary := CompressContextTFIDF(b.String(), 300)
		if err := manager.CompactEpisode(ep.ID, summary); err != nil {
			t.Fatalf("failed to compact episode %s: %v", ep.ID, err)
		}
	}

	afterCompaction := episodeContextCost(manager.GetAllEpisodes())
	selected := manager.GetEpisodesForContext(8000)
	afterBudgetSelection := episodeContextCost(selected)
	totalSavings := percentSavings(rawTokens, afterBudgetSelection)

	t.Logf("Combined pipeline results")
	t.Logf("original_total_tokens=%d", rawTokens)
	t.Logf("after_episodic_grouping=%d", afterGrouping)
	t.Logf("after_compaction=%d", afterCompaction)
	t.Logf("after_budget_selection=%d", afterBudgetSelection)
	t.Logf("total_savings=%6.2f%%", totalSavings)

	if afterCompaction >= afterGrouping {
		t.Fatalf("expected compaction to reduce context tokens: grouped=%d compacted=%d", afterGrouping, afterCompaction)
	}
	if afterBudgetSelection > 8000 && len(selected) > 0 && selected[0].Status != EpisodeActive {
		t.Fatalf("expected selected context <= budget when active episode is not the reason for overflow: selected=%d budget=8000", afterBudgetSelection)
	}
}
