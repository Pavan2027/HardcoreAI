package retrieval_test

import (
	"math"
	"testing"

	"hardcoreai-rag/retrieval"
	"hardcoreai-rag/storage"
)

// TestRerank_ExactRegisterBoost verifies that a chunk matching the exact register
// name in the query receives a boost of +0.30, and is ranked first even if it has
// a lower initial semantic score.
func TestRerank_ExactRegisterBoost(t *testing.T) {
	results := []storage.SearchResult{
		{
			ChunkID:       1,
			RegisterName:  "USART_CR1",
			SemanticScore: 0.65, // Slightly higher semantic score
			FTSScore:      0.50,
		},
		{
			ChunkID:       2,
			RegisterName:  "USART_BRR",
			SemanticScore: 0.60, // Lower semantic score
			FTSScore:      0.50,
		},
	}

	// Query has "USART_BRR" so chunk 2 should get the exact register boost (+0.30)
	// Chunk 2: Final = 0.60*0.6 + 0.50*0.2 + 0.30*0.2 = 0.36 + 0.10 + 0.06 = 0.52
	// Chunk 1: Final = 0.65*0.6 + 0.50*0.2 + 0.00*0.2 = 0.39 + 0.10 + 0.00 = 0.49
	reranked := retrieval.Rerank(results, "USART_BRR configuration", "")

	if len(reranked) != 2 {
		t.Fatalf("expected 2 results, got %d", len(reranked))
	}

	// Check that chunk 2 was boosted to the top
	if reranked[0].ChunkID != 2 {
		t.Errorf("expected chunk 2 to be ranked first, got chunk %d", reranked[0].ChunkID)
	}

	// Expected boost for chunk 2: exact register match (+0.30) -> metadata boost = 0.30
	// Expected boost for chunk 1: no match -> metadata boost = 0.0
	if math.Abs(reranked[0].MetadataBoost-0.30) > 1e-9 {
		t.Errorf("expected chunk 2 metadata boost to be 0.30, got %f", reranked[0].MetadataBoost)
	}
	if math.Abs(reranked[1].MetadataBoost-0.0) > 1e-9 {
		t.Errorf("expected chunk 1 metadata boost to be 0.0, got %f", reranked[1].MetadataBoost)
	}
}

// TestRerank_PeripheralBoost verifies that a chunk with a peripheral matching
// the query receives a boost of +0.20.
func TestRerank_PeripheralBoost(t *testing.T) {
	results := []storage.SearchResult{
		{
			ChunkID:       1,
			Peripheral:    "DMA",
			SemanticScore: 0.70,
			FTSScore:      0.40,
		},
		{
			ChunkID:       2,
			Peripheral:    "USART",
			SemanticScore: 0.65, // Lower semantic score
			FTSScore:      0.40,
		},
	}

	// Query has "USART" so chunk 2 should get peripheral boost (+0.20)
	// Chunk 2: Final = 0.65*0.6 + 0.40*0.2 + 0.20*0.2 = 0.39 + 0.08 + 0.04 = 0.51
	// Chunk 1: Final = 0.70*0.6 + 0.40*0.2 + 0.00*0.2 = 0.42 + 0.08 + 0.00 = 0.50
	reranked := retrieval.Rerank(results, "USART serial communication", "")

	if reranked[0].ChunkID != 2 {
		t.Errorf("expected chunk 2 to be ranked first, got chunk %d", reranked[0].ChunkID)
	}

	if math.Abs(reranked[0].MetadataBoost-0.20) > 1e-9 {
		t.Errorf("expected chunk 2 metadata boost to be 0.20, got %f", reranked[0].MetadataBoost)
	}
}

// TestRerank_SectionTitleBoost verifies that a chunk with a section title matching
// non-stop-words in the query receives a boost of +0.15.
func TestRerank_SectionTitleBoost(t *testing.T) {
	results := []storage.SearchResult{
		{
			ChunkID:       1,
			SectionTitle:  "Fault Status Registers",
			SemanticScore: 0.65,
			FTSScore:      0.30,
		},
		{
			ChunkID:       2,
			SectionTitle:  "USART Baud Rate Generation",
			SemanticScore: 0.61, // Lower semantic score
			FTSScore:      0.30,
		},
	}

	// Query has "baud" so chunk 2 should get section title boost (+0.15)
	// "the" and "in" are stop words and should be ignored.
	// Chunk 2: Final = 0.61*0.6 + 0.30*0.2 + 0.15*0.2 = 0.366 + 0.06 + 0.03 = 0.456
	// Chunk 1: Final = 0.65*0.6 + 0.30*0.2 + 0.00*0.2 = 0.390 + 0.06 + 0.00 = 0.450
	reranked := retrieval.Rerank(results, "how to configure the baud rate in microcontroller", "")

	if reranked[0].ChunkID != 2 {
		t.Errorf("expected chunk 2 to be ranked first, got chunk %d", reranked[0].ChunkID)
	}

	if math.Abs(reranked[0].MetadataBoost-0.15) > 1e-9 {
		t.Errorf("expected chunk 2 metadata boost to be 0.15, got %f", reranked[0].MetadataBoost)
	}
}

// TestRerank_DocTypeBoost verifies that reference manuals receive a boost of +0.10.
func TestRerank_DocTypeBoost(t *testing.T) {
	results := []storage.SearchResult{
		{
			ChunkID:       1,
			Peripheral:    "USART",
			DocType:       "app_note",
			SemanticScore: 0.75,
			FTSScore:      0.50,
		},
		{
			ChunkID:       2,
			Peripheral:    "USART",
			DocType:       "reference_manual",
			SemanticScore: 0.73, // Lower semantic score
			FTSScore:      0.50,
		},
	}

	// Query has "USART" so both get peripheral boost (+0.20)
	// Chunk 2 also gets DocType boost (+0.10) because it's a reference_manual
	// Chunk 2: Final = 0.73*0.6 + 0.50*0.2 + 0.30*0.2 = 0.438 + 0.10 + 0.06 = 0.598
	// Chunk 1: Final = 0.75*0.6 + 0.50*0.2 + 0.20*0.2 = 0.450 + 0.10 + 0.04 = 0.590
	reranked := retrieval.Rerank(results, "USART configuration", "")

	if reranked[0].ChunkID != 2 {
		t.Errorf("expected chunk 2 to be ranked first, got chunk %d", reranked[0].ChunkID)
	}

	if math.Abs(reranked[0].MetadataBoost-0.30) > 1e-9 {
		t.Errorf("expected chunk 2 metadata boost to be 0.30, got %f", reranked[0].MetadataBoost)
	}
	if math.Abs(reranked[1].MetadataBoost-0.20) > 1e-9 {
		t.Errorf("expected chunk 1 metadata boost to be 0.20, got %f", reranked[1].MetadataBoost)
	}
}

// TestRerank_ActiveChipFamilyBoost verifies that chunks belonging to the active
// chip family filter receive a boost of +0.05.
func TestRerank_ActiveChipFamilyBoost(t *testing.T) {
	results := []storage.SearchResult{
		{
			ChunkID:       1,
			Peripheral:    "USART",
			ChipFamily:    "STM32H7",
			SemanticScore: 0.70,
			FTSScore:      0.40,
		},
		{
			ChunkID:       2,
			Peripheral:    "USART",
			ChipFamily:    "STM32F4",
			SemanticScore: 0.69, // Lower semantic score
			FTSScore:      0.40,
		},
	}

	// Active chip family is STM32F4, so chunk 2 should receive the active chip family boost (+0.05)
	// Both get peripheral boost (+0.20)
	// Chunk 2: Final = 0.69*0.6 + 0.40*0.2 + 0.25*0.2 = 0.414 + 0.08 + 0.05 = 0.544
	// Chunk 1: Final = 0.70*0.6 + 0.40*0.2 + 0.20*0.2 = 0.420 + 0.08 + 0.04 = 0.540
	reranked := retrieval.Rerank(results, "USART communication", "STM32F4")

	if reranked[0].ChunkID != 2 {
		t.Errorf("expected chunk 2 to be ranked first, got chunk %d", reranked[0].ChunkID)
	}

	if math.Abs(reranked[0].MetadataBoost-0.25) > 1e-9 {
		t.Errorf("expected chunk 2 metadata boost to be 0.25, got %f", reranked[0].MetadataBoost)
	}
	if math.Abs(reranked[1].MetadataBoost-0.20) > 1e-9 {
		t.Errorf("expected chunk 1 metadata boost to be 0.20, got %f", reranked[1].MetadataBoost)
	}
}

// TestRerank_AdditiveBoostCap verifies that the total metadata boost is capped at 1.0.
func TestRerank_AdditiveBoostCap(t *testing.T) {
	results := []storage.SearchResult{
		{
			ChunkID:      1,
			RegisterName: "USART_BRR",
			Peripheral:   "USART",
			SectionTitle: "USART Baud Rate Generation",
			DocType:      "reference_manual",
			ChipFamily:   "STM32F4",
		},
	}

	// Additive boost: 0.30 (reg) + 0.20 (peri) + 0.15 (sect) + 0.10 (doc) + 0.05 (fam) = 0.80
	reranked := retrieval.Rerank(results, "USART_BRR baud rate configuration on STM32F4", "STM32F4")

	expectedBoost := 0.30 + 0.20 + 0.15 + 0.10 + 0.05 // 0.80
	if math.Abs(reranked[0].MetadataBoost-expectedBoost) > 1e-9 {
		t.Errorf("expected metadata boost to be %f, got %f", expectedBoost, reranked[0].MetadataBoost)
	}
}
