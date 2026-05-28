// Package storage_test — FTS search tests (Phase 4).
// Run without Ollama. Uses in-memory SQLite.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch -v
//	VEC0_PATH=../bin/vec0 go test ./storage/ -run TestFTSSearch -v
package storage_test

import (
	"context"
	"testing"

	"hardcoreai-rag/storage"
)

// seedFTSDB sets up an in-memory DB with three chunks populated in both
// the chunks table and the chunk_fts content table.
//
// Chunk layout:
//
//	chunkID 1 — USART_BRR / baud rate / STM32F4 / reference_manual
//	chunkID 2 — CFSR / precise bus fault / STM32F4 / reference_manual
//	chunkID 3 — DMA_SxCR / stream priority / STM32H7 / app_note
func seedFTSDB(t *testing.T) *storage.DB {
	t.Helper()

	db, err := storage.Open(":memory:", "")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.ApplySchema(schemaSQL(t)); err != nil {
		t.Fatalf("ApplySchema: %v", err)
	}

	// Document 1 — STM32F4 Reference Manual
	res, err := db.Exec(`
		INSERT INTO documents (filename, doc_type, chip_family, chip_model)
		VALUES ('stm32f4_rm.pdf', 'reference_manual', 'STM32F4', 'STM32F429')
	`)
	if err != nil {
		t.Fatalf("insert doc1: %v", err)
	}
	doc1ID, _ := res.LastInsertId()

	// Document 2 — STM32H7 App Note
	res, err = db.Exec(`
		INSERT INTO documents (filename, doc_type, chip_family, chip_model)
		VALUES ('stm32h7_dma_an.pdf', 'app_note', 'STM32H7', 'STM32H743')
	`)
	if err != nil {
		t.Fatalf("insert doc2: %v", err)
	}
	doc2ID, _ := res.LastInsertId()

	type chunk struct {
		docID      int64
		text       string
		section    string
		peripheral string
		register   string
		page       int
	}
	chunks := []chunk{
		{doc1ID, "The USART_BRR register controls the baud rate divider value.", "USART Baud Rate Generation", "USART", "USART_BRR", 842},
		{doc1ID, "The CFSR register holds precise and imprecise bus fault status bits.", "Fault Status Registers", "CoreDebug", "CFSR", 210},
		{doc2ID, "DMA_SxCR configures the DMA stream transfer direction and channel priority.", "DMA Stream Configuration", "DMA", "DMA_SxCR", 330},
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

		// FTS5 content tables must be populated explicitly.
		if _, err := db.Exec(`
			INSERT INTO chunk_fts (rowid, chunk_text, section_title, peripheral, register_name)
			VALUES (?, ?, ?, ?, ?)
		`, chunkID, c.text, c.section, c.peripheral, c.register); err != nil {
			t.Fatalf("insert chunk_fts chunkID=%d: %v", chunkID, err)
		}
	}

	return db
}

// --- Tests ------------------------------------------------------------------

// TestFTSSearch_RegisterNameExact verifies exact register name lookup.
// This is the primary use case FTS covers that vector search handles poorly.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_RegisterNameExact -v
func TestFTSSearch_RegisterNameExact(t *testing.T) {
	db := seedFTSDB(t)
	ctx := context.Background()

	results, err := db.FTSSearch(ctx, "USART_BRR", storage.SearchOptions{K: 5})
	if err != nil {
		t.Fatalf("FTSSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for USART_BRR, got 0")
	}

	if results[0].RegisterName != "USART_BRR" {
		t.Errorf("expected top result USART_BRR, got %q", results[0].RegisterName)
	}
	t.Logf("✓ Top result for 'USART_BRR': register=%q score=%.4f", results[0].RegisterName, results[0].FTSScore)
}

// TestFTSSearch_KeywordInText verifies that a keyword appearing in chunk_text
// is retrieved even without an exact register match.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_KeywordInText -v
func TestFTSSearch_KeywordInText(t *testing.T) {
	db := seedFTSDB(t)
	ctx := context.Background()

	results, err := db.FTSSearch(ctx, "baud rate", storage.SearchOptions{K: 5})
	if err != nil {
		t.Fatalf("FTSSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'baud rate', got 0")
	}

	// The USART baud rate chunk should be the top hit.
	if results[0].Peripheral != "USART" {
		t.Errorf("expected USART peripheral as top result, got %q", results[0].Peripheral)
	}
	t.Logf("✓ Top result for 'baud rate': peripheral=%q section=%q", results[0].Peripheral, results[0].SectionTitle)
}

// TestFTSSearch_FaultRegister verifies retrieval of fault-related content.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_FaultRegister -v
func TestFTSSearch_FaultRegister(t *testing.T) {
	db := seedFTSDB(t)
	ctx := context.Background()

	results, err := db.FTSSearch(ctx, "CFSR precise bus fault", storage.SearchOptions{K: 5})
	if err != nil {
		t.Fatalf("FTSSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for CFSR query, got 0")
	}

	if results[0].RegisterName != "CFSR" {
		t.Errorf("expected top result CFSR, got %q", results[0].RegisterName)
	}
	t.Logf("✓ Top result for 'CFSR precise bus fault': register=%q", results[0].RegisterName)
}

// TestFTSSearch_ScoreNormalised verifies all FTSScores are in [0,1].
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_ScoreNormalised -v
func TestFTSSearch_ScoreNormalised(t *testing.T) {
	db := seedFTSDB(t)
	ctx := context.Background()

	results, err := db.FTSSearch(ctx, "DMA stream", storage.SearchOptions{K: 5})
	if err != nil {
		t.Fatalf("FTSSearch: %v", err)
	}

	for _, r := range results {
		if r.FTSScore < 0 || r.FTSScore > 1 {
			t.Errorf("FTSScore out of [0,1]: %f (chunkID=%d)", r.FTSScore, r.ChunkID)
		}
	}
	t.Logf("✓ All %d FTS scores in [0,1]", len(results))
}

// TestFTSSearch_SanitizeSpecialChars verifies queries with FTS5 operator
// characters don't cause a query error.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_SanitizeSpecialChars -v
func TestFTSSearch_SanitizeSpecialChars(t *testing.T) {
	db := seedFTSDB(t)
	ctx := context.Background()

	// Quotes, parens, and asterisks are FTS5 syntax — must not crash.
	_, err := db.FTSSearch(ctx, `"USART*" AND (BRR OR CR1)`, storage.SearchOptions{K: 5})
	if err != nil {
		t.Errorf("sanitized query caused error: %v", err)
	} else {
		t.Log("✓ Special-character query handled without error")
	}
}

// TestFTSSearch_EmptyQuery verifies an error is returned for empty input.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_EmptyQuery -v
func TestFTSSearch_EmptyQuery(t *testing.T) {
	db := seedFTSDB(t)
	_, err := db.FTSSearch(context.Background(), "", storage.SearchOptions{K: 5})
	if err == nil {
		t.Error("expected error for empty query, got nil")
	} else {
		t.Logf("✓ Correctly rejected empty query: %v", err)
	}
}

// TestFTSSearch_FilterByDocType verifies DocTypes filter excludes app_note chunks.
//
// HOW TO RUN:
//
//	go test ./storage/ -run TestFTSSearch_FilterByDocType -v
func TestFTSSearch_FilterByDocType(t *testing.T) {
	db := seedFTSDB(t)
	ctx := context.Background()

	results, err := db.FTSSearch(ctx, "DMA stream", storage.SearchOptions{
		K:        5,
		DocTypes: []string{"reference_manual"},
	})
	if err != nil {
		t.Fatalf("FTSSearch: %v", err)
	}

	for _, r := range results {
		if r.DocType != "reference_manual" {
			t.Errorf("filter failed: got result with DocType=%q", r.DocType)
		}
	}
	t.Logf("✓ All %d results have DocType=reference_manual", len(results))
}