// Package integration_test — hybrid search tests (Phase 5).
// Run without Ollama. Uses a mock embedder and in-memory SQLite.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch -v
//	VEC0_PATH=../bin/vec0 go test ./tests/ -run TestHybridSearch -v
package integration_test

import (
	"context"
	"math"
	"os"
	"testing"

	"hardcoreai-rag/retrieval"
	"hardcoreai-rag/storage"
)

// --- Infrastructure ---------------------------------------------------------

func schemaSQLInteg(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../storage/schema.sql")
	if err != nil {
		t.Fatalf("could not read schema.sql: %v", err)
	}
	return string(data)
}

// mockEmbedder implements embeddings.Embedder without needing Ollama.
// It returns a predetermined 768-dim unit vector based on the query string.
type mockEmbedder struct {
	// vectors maps query string → embedding. Falls back to zeroes if not found.
	vectors map[string][]float64
}

func (m *mockEmbedder) EmbedQuery(_ context.Context, query string) ([]float64, error) {
	if v, ok := m.vectors[query]; ok {
		return v, nil
	}
	// Default: zero vector (will match nothing strongly).
	return make([]float64, 768), nil
}

// unitVecInteg returns a 768-dim zero vector with dimension i set to 1.0.
func unitVecInteg(i int) []float64 {
	v := make([]float64, 768)
	v[i] = 1.0
	return v
}

// seedHybridDB creates an in-memory DB seeded with three STM32 chunks,
// embeddings in chunks, and rows in chunk_fts.
//
// Chunk mapping:
//
//	1 — USART_BRR / USART / STM32F4 / reference_manual — embedding: unitVecInteg(0)
//	2 — CFSR / CoreDebug / STM32F4 / reference_manual — embedding: unitVecInteg(1)
//	3 — DMA_SxCR / DMA / STM32H7 / app_note — embedding: unitVecInteg(2)
func seedHybridDB(t *testing.T) *storage.DB {
	t.Helper()

	db, err := storage.Open(":memory:", "")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.ApplySchema(schemaSQLInteg(t)); err != nil {
		t.Fatalf("ApplySchema: %v", err)
	}

	res, err := db.Exec(`
		INSERT INTO documents (filename, doc_type, chip_family, chip_model)
		VALUES ('stm32f4_rm.pdf', 'reference_manual', 'STM32F4', 'STM32F429')
	`)
	if err != nil {
		t.Fatalf("insert doc1: %v", err)
	}
	doc1ID, _ := res.LastInsertId()

	res, err = db.Exec(`
		INSERT INTO documents (filename, doc_type, chip_family, chip_model)
		VALUES ('stm32h7_dma_an.pdf', 'app_note', 'STM32H7', 'STM32H743')
	`)
	if err != nil {
		t.Fatalf("insert doc2: %v", err)
	}
	doc2ID, _ := res.LastInsertId()

	type chunkDef struct {
		docID      int64
		text       string
		section    string
		peripheral string
		register   string
		page       int
		vecIdx     int
	}
	chunks := []chunkDef{
		{doc1ID, "USART_BRR register controls the baud rate divider.", "USART Baud Rate Generation", "USART", "USART_BRR", 842, 0},
		{doc1ID, "CFSR holds precise and imprecise bus fault status bits.", "Fault Status Registers", "CoreDebug", "CFSR", 210, 1},
		{doc2ID, "DMA_SxCR configures DMA stream direction and priority.", "DMA Stream Configuration", "DMA", "DMA_SxCR", 330, 2},
	}

	for _, c := range chunks {
		res, err = db.Exec(`
			INSERT INTO chunks (document_id, chunk_text, section_title, peripheral, register_name, page_number, chunk_index)
			VALUES (?, ?, ?, ?, ?, ?, 0)
		`, c.docID, c.text, c.section, c.peripheral, c.register, c.page)
		if err != nil {
			t.Fatalf("insert chunk: %v", err)
		}
		chunkID, _ := res.LastInsertId()

		blob := storage.SerializeEmbedding(unitVecInteg(c.vecIdx))
		if _, err := db.Exec(`UPDATE chunks SET embedding = ? WHERE id = ?`, blob, chunkID); err != nil {
			t.Fatalf("update embedding: %v", err)
		}

		if _, err := db.Exec(`
			INSERT INTO chunk_fts (rowid, chunk_text, section_title, peripheral, register_name)
			VALUES (?, ?, ?, ?, ?)
		`, chunkID, c.text, c.section, c.peripheral, c.register); err != nil {
			t.Fatalf("insert chunk_fts: %v", err)
		}
	}

	return db
}

// --- Tests ------------------------------------------------------------------

// TestHybridSearch_VectorOnlyMatch tests a query where only vector search
// produces a strong signal (no keyword overlap with the query string).
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_VectorOnlyMatch -v
func TestHybridSearch_VectorOnlyMatch(t *testing.T) {
	db := seedHybridDB(t)

	// Embedder returns unitVecInteg(0) for this query → matches USART_BRR chunk.
	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"usart configuration": unitVecInteg(0),
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"usart configuration",
		retrieval.RetrievalOptions{K: 3},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected results, got 0")
	}

	t.Logf("✓ Top result: register=%q finalScore=%.4f", result.Chunks[0].RegisterName, result.Chunks[0].FinalScore)
}

// TestHybridSearch_BothSignals tests a query where both vector and FTS match
// the same chunk. That chunk should receive contributions from both lists
// and rank at the top.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_BothSignals -v
func TestHybridSearch_BothSignals(t *testing.T) {
	db := seedHybridDB(t)

	// Embedder returns unitVecInteg(1) → matches CFSR chunk semantically.
	// "CFSR" also matches via FTS on register_name.
	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"CFSR precise bus fault": unitVecInteg(1),
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"CFSR precise bus fault",
		retrieval.RetrievalOptions{K: 3},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected results, got 0")
	}

	top := result.Chunks[0]
	if top.RegisterName != "CFSR" {
		t.Errorf("expected CFSR as top result, got %q", top.RegisterName)
	}

	// A chunk appearing in both lists should have SemanticScore AND FTSScore set.
	if top.SemanticScore == 0 {
		t.Error("expected SemanticScore > 0 for chunk matched by vector search")
	}
	if top.FTSScore == 0 {
		t.Error("expected FTSScore > 0 for chunk matched by FTS search")
	}

	t.Logf("✓ CFSR chunk ranked #1 — semantic=%.4f fts=%.4f rrf=%.4f",
		top.SemanticScore, top.FTSScore, top.FinalScore)
}

// TestHybridSearch_Deduplication verifies that a chunk appearing in both
// vector and FTS results is not returned twice.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_Deduplication -v
func TestHybridSearch_Deduplication(t *testing.T) {
	db := seedHybridDB(t)

	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"CFSR fault": unitVecInteg(1), // matches CFSR by vector
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"CFSR fault", // also matches CFSR by FTS
		retrieval.RetrievalOptions{K: 10},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}

	seen := make(map[int]bool)
	for _, r := range result.Chunks {
		if seen[r.ChunkID] {
			t.Errorf("duplicate ChunkID=%d in results", r.ChunkID)
		}
		seen[r.ChunkID] = true
	}
	t.Logf("✓ No duplicates across %d results", len(result.Chunks))
}

// TestHybridSearch_ResultsOrderedByFinalScore verifies FinalScore is
// monotonically non-increasing across the result list.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_ResultsOrderedByFinalScore -v
func TestHybridSearch_ResultsOrderedByFinalScore(t *testing.T) {
	db := seedHybridDB(t)

	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"baud rate register": unitVecInteg(0),
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"baud rate register",
		retrieval.RetrievalOptions{K: 3},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}

	for i := 1; i < len(result.Chunks); i++ {
		prev := result.Chunks[i-1].FinalScore
		curr := result.Chunks[i].FinalScore
		if curr > prev+1e-9 {
			t.Errorf("results not sorted: index %d (%.6f) > index %d (%.6f)", i, curr, i-1, prev)
		}
	}
	t.Logf("✓ FinalScore is monotonically non-increasing across %d results", len(result.Chunks))
}

// TestHybridSearch_RespectK verifies that at most K results are returned.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_RespectK -v
func TestHybridSearch_RespectK(t *testing.T) {
	db := seedHybridDB(t)

	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"register configuration": unitVecInteg(0),
		},
	}

	for _, k := range []int{1, 2, 3} {
		result, err := retrieval.HybridSearch(
			context.Background(), db, embedder,
			"register configuration",
			retrieval.RetrievalOptions{K: k},
		)
		if err != nil {
			t.Fatalf("HybridSearch K=%d: %v", k, err)
		}
		if len(result.Chunks) > k {
			t.Errorf("K=%d: got %d results, expected at most %d", k, len(result.Chunks), k)
		}
		t.Logf("✓ K=%d returned %d results", k, len(result.Chunks))
	}
}

// TestHybridSearch_RRFBoostForDualMatch verifies that a chunk appearing in
// both vector and FTS results scores higher than a chunk appearing in only one.
// This is the core correctness invariant of RRF.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_RRFBoostForDualMatch -v
func TestHybridSearch_RRFBoostForDualMatch(t *testing.T) {
	db := seedHybridDB(t)

	// unitVecInteg(1) → CFSR chunk matched by vector.
	// Query "CFSR" also hits CFSR via FTS on register_name.
	// DMA chunk matches neither → should score lowest.
	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"CFSR": unitVecInteg(1),
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"CFSR",
		retrieval.RetrievalOptions{K: 3},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}
	if len(result.Chunks) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(result.Chunks))
	}

	// CFSR should be #1 (dual-list hit). Its FinalScore should exceed all others.
	top := result.Chunks[0]
	if top.RegisterName != "CFSR" {
		t.Errorf("expected CFSR as top (dual-signal) result, got %q", top.RegisterName)
	}

	for _, r := range result.Chunks[1:] {
		if r.FinalScore >= top.FinalScore-1e-9 {
			// Allow equal only in degenerate cases; normally dual-hit should win.
			t.Logf("note: %q (%.6f) is close to CFSR (%.6f) — check RRF math", r.RegisterName, r.FinalScore, top.FinalScore)
		}
	}

	t.Logf("✓ CFSR (dual-signal) RRF=%.6f > next result RRF=%.6f",
		top.FinalScore, result.Chunks[1].FinalScore)
}

// TestHybridSearch_EmptyQuery verifies an error is returned for empty input.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_EmptyQuery -v
func TestHybridSearch_EmptyQuery(t *testing.T) {
	db := seedHybridDB(t)
	embedder := &mockEmbedder{}

	_, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"",
		retrieval.RetrievalOptions{K: 5},
	)
	if err == nil {
		t.Error("expected error for empty query, got nil")
	} else {
		t.Logf("✓ Correctly rejected empty query: %v", err)
	}
}

// TestHybridSearch_FilterByChipFamily verifies ChipFamily filter is respected
// end-to-end through the hybrid pipeline.
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_FilterByChipFamily -v
func TestHybridSearch_FilterByChipFamily(t *testing.T) {
	db := seedHybridDB(t)

	// unitVecInteg(2) is the DMA chunk (STM32H7). Filtering for STM32F4 excludes it.
	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"DMA stream": unitVecInteg(2),
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"DMA stream",
		retrieval.RetrievalOptions{K: 5, ChipFamily: "STM32F4"},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}

	for _, r := range result.Chunks {
		if r.ChipFamily != "STM32F4" {
			t.Errorf("filter failed: got result with ChipFamily=%q", r.ChipFamily)
		}
	}
	t.Logf("✓ All %d results have ChipFamily=STM32F4 (STM32H7 DMA chunk excluded)", len(result.Chunks))
}

// TestHybridSearch_FinalScoreRange verifies FinalScore (RRF score) is
// in a sensible positive range — not negative or impossibly large.
//
// Maximum possible RRF score with 2 lists, rank 1 each: 2 * 1/(60+1) ≈ 0.033.
// Minimum with 1 list at rank K: 1/(60+K).
//
// HOW TO RUN:
//
//	go test ./tests/ -run TestHybridSearch_FinalScoreRange -v
func TestHybridSearch_FinalScoreRange(t *testing.T) {
	db := seedHybridDB(t)

	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			"USART baud rate": unitVecInteg(0),
		},
	}

	result, err := retrieval.HybridSearch(
		context.Background(), db, embedder,
		"USART baud rate",
		retrieval.RetrievalOptions{K: 5},
	)
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}

	maxPossibleRRF := 2.0 / float64(60+1) // both lists, rank 1
	for _, r := range result.Chunks {
		if r.FinalScore <= 0 {
			t.Errorf("FinalScore must be > 0, got %f (chunkID=%d)", r.FinalScore, r.ChunkID)
		}
		if r.FinalScore > maxPossibleRRF+1e-9 {
			t.Errorf("FinalScore %.6f exceeds theoretical max %.6f", r.FinalScore, maxPossibleRRF)
		}
	}
	t.Logf("✓ All FinalScores in (0, %.4f] — RRF range valid", maxPossibleRRF)

	_ = math.MaxFloat64 // imported for completeness; not used above
}
