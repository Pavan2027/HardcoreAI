package retrieval

import "hardcoreai-rag/storage"

// RetrievalOptions controls the full hybrid retrieval pipeline.
type RetrievalOptions struct {
	// K is the maximum number of results returned after merging.
	// Defaults to 10 if zero.
	K int

	// ChipFamily restricts results to a specific STM32 family (e.g. "STM32F4").
	// Empty means no restriction.
	ChipFamily string

	// DocTypes restricts results to specific document types
	// (e.g. []string{"reference_manual"}). Empty means no restriction.
	DocTypes []string

	// MaxTokens is the maximum number of tokens in the context window.
	// Defaults to 3000 if zero.
	MaxTokens int
}

// RetrievalResult is the output of the hybrid retrieval pipeline.
type RetrievalResult struct {
	// Chunks holds the merged, deduplicated results ordered by FinalScore descending.
	Chunks []storage.SearchResult

	// Context is the structured, LLM-ready context string.
	Context string

	// ChunksUsed is the number of chunks successfully included in the context.
	ChunksUsed int

	// ChunksDropped is the number of chunks trimmed due to token budget limits.
	ChunksDropped int
}