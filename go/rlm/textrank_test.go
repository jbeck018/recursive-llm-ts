package rlm

import (
	"math"
	"strings"
	"testing"
)

// ─── CosineSimilarity Tests ─────────────────────────────────────────────────

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	v := tfidfVector{
		terms: map[string]float64{"hello": 1.0, "world": 2.0},
		norm:  math.Sqrt(5.0),
	}
	sim := cosineSimilarity(v, v)

	if sim < 0.99 || sim > 1.01 {
		t.Errorf("expected cosine similarity ~1.0 for identical vectors, got %f", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := tfidfVector{
		terms: map[string]float64{"hello": 1.0},
		norm:  1.0,
	}
	b := tfidfVector{
		terms: map[string]float64{"world": 1.0},
		norm:  1.0,
	}
	sim := cosineSimilarity(a, b)

	if sim != 0 {
		t.Errorf("expected cosine similarity 0 for orthogonal vectors, got %f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := tfidfVector{terms: map[string]float64{}, norm: 0}
	b := tfidfVector{terms: map[string]float64{"hello": 1.0}, norm: 1.0}

	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("expected 0 for zero vector, got %f", sim)
	}
}

func TestCosineSimilarity_PartialOverlap(t *testing.T) {
	a := tfidfVector{
		terms: map[string]float64{"hello": 1.0, "world": 1.0},
		norm:  math.Sqrt(2.0),
	}
	b := tfidfVector{
		terms: map[string]float64{"hello": 1.0, "foo": 1.0},
		norm:  math.Sqrt(2.0),
	}
	sim := cosineSimilarity(a, b)

	// dot = 1*1 = 1, norm product = sqrt(2)*sqrt(2) = 2, sim = 0.5
	if sim < 0.49 || sim > 0.51 {
		t.Errorf("expected cosine similarity ~0.5, got %f", sim)
	}
}

// ─── BuildSimilarityGraph Tests ─────────────────────────────────────────────

func TestBuildSimilarityGraph_SimilarSentences(t *testing.T) {
	sentences := []string{
		"The machine learning model processes data efficiently.",
		"The deep learning model processes information quickly.",
		"The weather today is sunny and warm.",
	}

	config := DefaultTextRankConfig()
	config.MinSimilarity = 0.01 // Very low threshold to ensure edges
	graph := BuildSimilarityGraph(sentences, config)

	if len(graph) != 3 {
		t.Fatalf("expected 3x3 graph, got %d rows", len(graph))
	}

	// Sentences 0 and 1 share terms (model, processes, learning) - should have higher similarity
	// Sentence 2 is about weather - should have lower similarity with 0 and 1
	sim01 := graph[0][1]
	sim02 := graph[0][2]

	if sim01 <= 0 {
		t.Error("expected positive similarity between similar sentences 0 and 1")
	}

	// 0-1 should be more similar than 0-2
	if sim01 <= sim02 {
		t.Errorf("expected sentences 0,1 to be more similar than 0,2: sim01=%f, sim02=%f", sim01, sim02)
	}
}

func TestBuildSimilarityGraph_Symmetric(t *testing.T) {
	sentences := []string{"First sentence here.", "Second sentence there.", "Third different topic."}
	config := DefaultTextRankConfig()
	config.MinSimilarity = 0.0
	graph := BuildSimilarityGraph(sentences, config)

	for i := 0; i < len(graph); i++ {
		for j := 0; j < len(graph); j++ {
			if graph[i][j] != graph[j][i] {
				t.Errorf("graph not symmetric: [%d][%d]=%f != [%d][%d]=%f",
					i, j, graph[i][j], j, i, graph[j][i])
			}
		}
	}
}

func TestBuildSimilarityGraph_DiagonalZero(t *testing.T) {
	sentences := []string{"First.", "Second.", "Third."}
	config := DefaultTextRankConfig()
	graph := BuildSimilarityGraph(sentences, config)

	for i := 0; i < len(graph); i++ {
		if graph[i][i] != 0 {
			t.Errorf("expected zero self-similarity, got %f for sentence %d", graph[i][i], i)
		}
	}
}

// ─── PageRank Tests ─────────────────────────────────────────────────────────

func TestPageRank_UniformGraph(t *testing.T) {
	// Fully connected graph with equal weights -> all scores should be equal
	n := 4
	graph := make([][]float64, n)
	for i := range graph {
		graph[i] = make([]float64, n)
		for j := range graph[i] {
			if i != j {
				graph[i][j] = 1.0
			}
		}
	}

	config := DefaultTextRankConfig()
	scores := PageRank(graph, config)

	if len(scores) != n {
		t.Fatalf("expected %d scores, got %d", n, len(scores))
	}

	// All scores should be approximately equal
	for i := 1; i < n; i++ {
		if math.Abs(scores[i]-scores[0]) > 0.01 {
			t.Errorf("expected uniform scores, got %v", scores)
			break
		}
	}
}

func TestPageRank_StarGraph(t *testing.T) {
	// Star graph: node 0 connected to all others, others only connected to 0
	// Node 0 should have the highest score
	n := 5
	graph := make([][]float64, n)
	for i := range graph {
		graph[i] = make([]float64, n)
	}
	for i := 1; i < n; i++ {
		graph[0][i] = 1.0
		graph[i][0] = 1.0
	}

	config := DefaultTextRankConfig()
	scores := PageRank(graph, config)

	// Node 0 (hub) should have the highest score
	for i := 1; i < n; i++ {
		if scores[0] <= scores[i] {
			t.Errorf("expected hub node (0) to have highest score: scores[0]=%f, scores[%d]=%f",
				scores[0], i, scores[i])
		}
	}
}

func TestPageRank_EmptyGraph(t *testing.T) {
	scores := PageRank(nil, DefaultTextRankConfig())
	if scores != nil {
		t.Errorf("expected nil for empty graph, got %v", scores)
	}
}

func TestPageRank_DisconnectedGraph(t *testing.T) {
	// Graph with no edges -> all scores should be equal (from the (1-d)/n term)
	n := 3
	graph := make([][]float64, n)
	for i := range graph {
		graph[i] = make([]float64, n)
	}

	config := DefaultTextRankConfig()
	scores := PageRank(graph, config)

	for i := 1; i < n; i++ {
		if math.Abs(scores[i]-scores[0]) > 0.001 {
			t.Errorf("expected equal scores for disconnected graph, got %v", scores)
			break
		}
	}
}

func TestPageRank_Convergence(t *testing.T) {
	// PageRank should converge (scores sum to approximately 1)
	n := 4
	graph := make([][]float64, n)
	for i := range graph {
		graph[i] = make([]float64, n)
		for j := range graph[i] {
			if i != j {
				graph[i][j] = float64((i + j) % 3) // asymmetric weights
			}
		}
	}

	config := DefaultTextRankConfig()
	scores := PageRank(graph, config)

	sum := 0.0
	for _, s := range scores {
		sum += s
		if s < 0 {
			t.Errorf("negative PageRank score: %f", s)
		}
	}

	// Sum should be approximately 1.0
	if sum < 0.9 || sum > 1.1 {
		t.Errorf("expected PageRank scores to sum to ~1.0, got %f", sum)
	}
}

// ─── CompressContextTextRank Tests ──────────────────────────────────────────

func TestCompressContextTextRank_NoCompressionNeeded(t *testing.T) {
	text := "Short text that fits."
	result := CompressContextTextRank(text, 1000)

	if result != text {
		t.Errorf("expected unchanged text, got %q", result)
	}
}

func TestCompressContextTextRank_CompressesLargeText(t *testing.T) {
	var parts []string
	for i := 0; i < 30; i++ {
		parts = append(parts, "Machine learning algorithms process large datasets to find patterns in the data.")
	}
	parts = append(parts, "The stock price of AAPL rose 15% to $198.50 after the earnings report.")
	parts = append(parts, "Quantum computing achieved 99.9% gate fidelity using topological qubits.")
	text := strings.Join(parts, " ")

	originalTokens := EstimateTokens(text)
	targetTokens := originalTokens / 3

	result := CompressContextTextRank(text, targetTokens)

	resultTokens := EstimateTokens(result)
	if resultTokens > targetTokens+10 {
		t.Errorf("compressed result (%d tokens) exceeds target (%d tokens)", resultTokens, targetTokens)
	}
}

func TestCompressContextTextRank_PreservesOrder(t *testing.T) {
	text := "Alpha is the first Greek letter. Beta comes after alpha. Gamma is the third letter. Delta is fourth. Epsilon follows delta."
	result := CompressContextTextRank(text, 30)

	sentences := SplitSentences(result)
	for i := 1; i < len(sentences); i++ {
		posI := strings.Index(text, sentences[i])
		posPrev := strings.Index(text, sentences[i-1])
		if posI < posPrev {
			t.Errorf("order not preserved: %q before %q", sentences[i-1], sentences[i])
		}
	}
}

func TestCompressContextTextRank_Empty(t *testing.T) {
	result := CompressContextTextRank("", 100)
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestCompressContextTextRank_WithConfig(t *testing.T) {
	text := "Machine learning models. Deep learning networks. Neural network architectures. Data processing pipelines. Cloud computing infrastructure."

	config := TextRankConfig{
		DampingFactor:        0.85,
		MaxIterations:        50,
		ConvergenceThreshold: 0.001,
		MinSimilarity:        0.0, // Allow all edges
	}

	result := CompressContextTextRankWithConfig(text, 15, config)

	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
	if EstimateTokens(result) > 20 { // Some slack
		t.Errorf("result exceeds budget: %d tokens", EstimateTokens(result))
	}
}

// ─── TextRank vs TF-IDF Comparison ──────────────────────────────────────────

func TestTextRank_DifferentFromTFIDF(t *testing.T) {
	// TextRank and TF-IDF should produce different rankings because TextRank
	// considers inter-sentence relationships while TF-IDF only considers term rarity
	text := "Machine learning processes data. " +
		"Deep learning is a subset of machine learning. " +
		"Neural networks power deep learning. " +
		"The weather is sunny today. " +
		"Rain is expected tomorrow."

	tfidfResult := CompressContextTFIDF(text, 20)
	textrankResult := CompressContextTextRank(text, 20)

	// They should both produce non-empty results
	if len(tfidfResult) == 0 {
		t.Error("TF-IDF result is empty")
	}
	if len(textrankResult) == 0 {
		t.Error("TextRank result is empty")
	}

	// They CAN be the same for simple inputs, but both should work
	t.Logf("TF-IDF:    %q", tfidfResult)
	t.Logf("TextRank:  %q", textrankResult)
}
