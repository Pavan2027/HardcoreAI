// Package storage_test — vector search tests (Phase 3).
// No vec0 extension, no Ollama, no API keys required.
//
// HOW TO RUN:
//
//	go test -tags "fts5" ./storage/ -run TestVectorSearch -v
package storage_test

import (
	"context"
	"math"
	"testing"

	"hardcoreai-rag/storage"
)

// unitVec returns a 768-dimensional zero vector with index i set to 1.0.
func unitVec(i int) []float64 {
	v := make([]float64, 768)
	v[i] = 1.0
	return v
}

// seedVectorDB opens an in-memory DB, applies the schema, and inserts three
// chunks with distinct unit-vector embeddings stored in chunks.embedding.
//
// Chunk layout:
//
//	id 1 — USART_BRR / USART / STM32F4 / reference_manual — unitVec(0)
//	id 2 — CFSR      / CoreDebug / STM32F4 / reference_manual — unitVec(1)
//	id 3 — DMA_SxCR  / DMA / STM32H7 / app_note — unitVec(2)
func seedVectorDB(t *testing.T) *storage.DB {
	t.Helper()

	db, err := storage.Open(":memory:", "")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.ApplySchema(schemaSQL(t)); err != nil {
		t.Fatalf("ApplySchema: %v", err)
	}

	res, err := db.Exec(`
		INSERT INTO documents (filename, doc_type, chip_family, chip_model)
		VALUES ('stm32f4_rm.pdf', 'reference_manual', 'STM32F4', 'STM32F429')`)
	if err != nil {
		t.Fatalf("insert doc1: %v", err)
	}
	doc1ID, _ := res.LastInsertId()

	res, err = db.Exec(`
		INSERT INTO documents (filename, doc_type, chip_family, chip_model)
		VALUES ('stm32h7_dma.pdf', 'app_note', 'STM32H7', 'STM32H743')`)
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
		vecIndex   int
	}
	chunks := []chunkDef{
		{doc1ID, "USART_BRR controls baud rate divider.", "USART Baud Rate", "USART", "USART_BRR", 842, 0},
		{doc1ID, "CFSR holds precise and imprecise bus fault status bits.", "Fault Status Registers", "CoreDebug", "CFSR", 210, 1},
		{doc2ID, "DMA_SxCR configures DMA stream direction and priority.", "DMA Stream Configuration", "DMA", "DMA_SxCR", 330, 2},
	}

	for _, c := range chunks {
		res, err = db.Exec(`
			INSERT INTO chunks (document_id, chunk_text, section_title, peripheral, register_name, page_number, chunk_index)
			VALUES (?, ?, ?, ?, ?, ?, 0)`,
			c.docID, c.text, c.section, c.peripheral, c.register, c.page)
		if err != nil {
			t.Fatalf("insert chunk: %v", err)
		}
		chunkID, _ := res.LastInsertId()

		// Write embedding BLOB directly to chunks.embedding column.
		blob := storage.SerializeEmbedding(unitVec(c.vecIndex))
		if _, err := db.Exec(`UPDATE chunks SET embedding = ? WHERE id = ?`, blob, chunkID); err != nil {
			t.Fatalf("update embedding chunkID=%d: %v", chunkID, err)
		}
	}

	return db
}

// TestVectorSearch_TopResult verifies the nearest chunk is returned first.
func TestVectorSearch_TopResult(t *testing.T) {
	db := seedVectorDB(t)

	results, err := db.VectorSearch(context.Background(), unitVec(0), storage.SearchOptions{K: 3})
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got 0")
	}
	if results[0].RegisterName != "USART_BRR" {
		t.Errorf("expected USART_BRR as top result, got %q", results[0].RegisterName)
	}
	t.Logf("✓ Top result: register=%q score=%.4f", results[0].RegisterName, results[0].SemanticScore)
}

// TestVectorSearch_ScoreNormalised verifies SemanticScore is in [0,1].
func TestVectorSearch_ScoreNormalised(t *testing.T) {
	db := seedVectorDB(t)
	results, err := db.VectorSearch(context.Background(), unitVec(0), storage.SearchOptions{K: 3})
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	for _, r := range results {
		if r.SemanticScore < 0 || r.SemanticScore > 1+1e-9 {
			t.Errorf("SemanticScore out of [0,1]: %f (chunkID=%d)", r.SemanticScore, r.ChunkID)
		}
	}
	t.Logf("✓ All %d scores in [0,1]", len(results))
}

// TestVectorSearch_ScoreOrdering verifies results are sorted best-first.
func TestVectorSearch_ScoreOrdering(t *testing.T) {
	db := seedVectorDB(t)
	results, err := db.VectorSearch(context.Background(), unitVec(0), storage.SearchOptions{K: 3})
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if results[i].SemanticScore > results[i-1].SemanticScore+1e-9 {
			t.Errorf("not sorted at index %d: %.4f > %.4f", i, results[i].SemanticScore, results[i-1].SemanticScore)
		}
	}
	t.Logf("✓ Scores monotonically non-increasing across %d results", len(results))
}

// TestVectorSearch_PerfectMatchScore verifies an identical vector scores ~1.0.
func TestVectorSearch_PerfectMatchScore(t *testing.T) {
	db := seedVectorDB(t)
	results, err := db.VectorSearch(context.Background(), unitVec(0), storage.SearchOptions{K: 1})
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if math.Abs(results[0].SemanticScore-1.0) > 0.001 {
		t.Errorf("expected score ~1.0 for identical vector, got %.6f", results[0].SemanticScore)
	}
	t.Logf("✓ Perfect-match SemanticScore = %.6f", results[0].SemanticScore)
}

// TestVectorSearch_FilterByChipFamily verifies ChipFamily filter excludes non-matching results.
func TestVectorSearch_FilterByChipFamily(t *testing.T) {
	db := seedVectorDB(t)
	results, err := db.VectorSearch(context.Background(), unitVec(2), storage.SearchOptions{
		K:          3,
		ChipFamily: "STM32F4",
	})
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	for _, r := range results {
		if r.ChipFamily != "STM32F4" {
			t.Errorf("filter failed: got ChipFamily=%q", r.ChipFamily)
		}
	}
	t.Logf("✓ All %d results have ChipFamily=STM32F4", len(results))
}

// TestVectorSearch_EmptyEmbedding verifies an error is returned for empty input.
func TestVectorSearch_EmptyEmbedding(t *testing.T) {
	db := seedVectorDB(t)
	_, err := db.VectorSearch(context.Background(), []float64{}, storage.SearchOptions{K: 5})
	if err == nil {
		t.Error("expected error for empty embedding, got nil")
	} else {
		t.Logf("✓ Correctly rejected empty embedding: %v", err)
	}
}