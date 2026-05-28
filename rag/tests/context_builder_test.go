// Package integration_test — Context Builder and Engine tests (Phase 8).
package integration_test

import (
	"context"
	"strings"
	"testing"

	"hardcoreai-rag/retrieval"
	"hardcoreai-rag/storage"
	"hardcoreai-rag/utils"
)

// --- Tokenizer Tests --------------------------------------------------------

func TestCountTokens_Basic(t *testing.T) {
	// A simple test query
	text := "What causes a precise BusFault in STM32?"
	
	// Expect standard BPE tokenizer or fallback estimation to return > 0
	tokens := utils.CountTokens(text)
	if tokens <= 0 {
		t.Errorf("expected token count to be greater than 0, got %d", tokens)
	}
	
	// Check offline fallback estimation specifically by passing empty or huge inputs
	emptyTokens := utils.CountTokens("")
	if emptyTokens != 1 && emptyTokens != 0 {
		t.Errorf("expected empty token count to be 0 or 1 fallback, got %d", emptyTokens)
	}
}

// --- Context Builder Tests --------------------------------------------------

func TestBuildContext_Formatting(t *testing.T) {
	chunks := []storage.SearchResult{
		{
			ChunkID:      1,
			ChunkText:    "The USART_BRR register controls the baud rate.",
			SectionTitle: "USART Baud Rate Generation",
			Peripheral:   "USART",
			RegisterName: "USART_BRR",
			PageNumber:   842,
			Filename:     "stm32f4_rm.pdf",
			DocType:      "reference_manual",
			ChipFamily:   "STM32F4",
			FinalScore:   0.95,
		},
	}

	res, err := retrieval.BuildContext(chunks, 3000)
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	if res.ChunksUsed != 1 {
		t.Errorf("expected 1 chunk used, got %d", res.ChunksUsed)
	}
	if res.ChunksDropped != 0 {
		t.Errorf("expected 0 chunks dropped, got %d", res.ChunksDropped)
	}

	// Verify exact output formatting structure
	expectedHeader1 := "[Source: stm32f4_rm.pdf | doc_type: reference_manual]"
	expectedHeader2 := "Section: USART Baud Rate Generation"
	expectedHeader3 := "Register: USART_BRR | Page: 842"
	expectedSeparator := "---"

	if !strings.Contains(res.Context, expectedHeader1) {
		t.Errorf("context missing source line. Got:\n%s", res.Context)
	}
	if !strings.Contains(res.Context, expectedHeader2) {
		t.Errorf("context missing section line. Got:\n%s", res.Context)
	}
	if !strings.Contains(res.Context, expectedHeader3) {
		t.Errorf("context missing register/page line. Got:\n%s", res.Context)
	}
	if !strings.Contains(res.Context, expectedSeparator) {
		t.Errorf("context missing separator line. Got:\n%s", res.Context)
	}
}

func TestBuildContext_TokenBudgeting(t *testing.T) {
	chunks := []storage.SearchResult{
		{
			ChunkID:      1,
			ChunkText:    "This is the first high-scoring chunk. High priority.",
			SectionTitle: "Section 1",
			PageNumber:   10,
			Filename:     "doc1.pdf",
			DocType:      "reference_manual",
			FinalScore:   0.95,
		},
		{
			ChunkID:      2,
			ChunkText:    "This is the second medium-scoring chunk. Medium priority.",
			SectionTitle: "Section 2",
			PageNumber:   20,
			Filename:     "doc1.pdf",
			DocType:      "reference_manual",
			FinalScore:   0.85,
		},
		{
			ChunkID:      3,
			ChunkText:    "This is the third low-scoring chunk that should exceed the budget.",
			SectionTitle: "Section 3",
			PageNumber:   30,
			Filename:     "doc1.pdf",
			DocType:      "reference_manual",
			FinalScore:   0.75,
		},
	}

	// Set a very small token budget that only fits the first two formatted chunks
	formattedChunk1 := "[Source: doc1.pdf | doc_type: reference_manual]\nSection: Section 1\nRegister: N/A | Page: 10\n\nThis is the first high-scoring chunk. High priority.\n\n---\n"
	formattedChunk2 := "[Source: doc1.pdf | doc_type: reference_manual]\nSection: Section 2\nRegister: N/A | Page: 20\n\nThis is the second medium-scoring chunk. Medium priority.\n\n---\n"
	
	budgetLimit := utils.CountTokens(formattedChunk1) + utils.CountTokens(formattedChunk2) + 2

	res, err := retrieval.BuildContext(chunks, budgetLimit)
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	// Verify that chunks were correctly trimmed and lower-scoring was dropped
	if res.ChunksUsed != 2 {
		t.Errorf("expected exactly 2 chunks used under budget, got %d", res.ChunksUsed)
	}
	if res.ChunksDropped != 1 {
		t.Errorf("expected exactly 1 chunk dropped, got %d", res.ChunksDropped)
	}
	if strings.Contains(res.Context, "Section 3") {
		t.Errorf("context should not contain the low-scoring chunk. Got:\n%s", res.Context)
	}
}

// --- Unified Engine Pipeline Tests ------------------------------------------

func TestEngine_Retrieve(t *testing.T) {
	// Initialize database
	db := seedHybridDB(t)

	// Mock embedding mapping for standard test query
	query := "USART register description"
	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			query: unitVecInteg(0), // maps to USART_BRR unit vector
		},
	}

	// Setup Engine orchestrator
	engine := retrieval.NewEngine(db, embedder)

	opts := retrieval.RetrievalOptions{
		K:          2,
		ChipFamily: "STM32F4",
		MaxTokens:  2000,
	}

	result, err := engine.Retrieve(context.Background(), query, opts)
	if err != nil {
		t.Fatalf("Engine.Retrieve failed: %v", err)
	}

	// Verify Engine successfully executed all phases end-to-end
	if len(result.Chunks) == 0 {
		t.Fatal("expected chunks retrieved, got empty slice")
	}

	// First chunk should have metadata-boost from exact register match
	first := result.Chunks[0]
	if first.RegisterName != "USART_BRR" {
		t.Errorf("expected top result to be USART_BRR register match, got %s", first.RegisterName)
	}

	// Should contain the structured LLM-ready Context
	if result.Context == "" {
		t.Errorf("expected non-empty context string")
	}
	if !strings.Contains(result.Context, "USART_BRR") {
		t.Errorf("context missing register description. Context:\n%s", result.Context)
	}

	if result.ChunksUsed <= 0 {
		t.Errorf("expected ChunksUsed > 0, got %d", result.ChunksUsed)
	}
}
