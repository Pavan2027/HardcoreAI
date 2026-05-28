// Package retrieval orchestrates hybrid RAG retrieval pipelines.
package retrieval

import (
	"context"
	"fmt"

	"hardcoreai-rag/storage"
)

// Embedder is the interface that query embedders must satisfy.
type Embedder interface {
	EmbedQuery(ctx context.Context, query string) ([]float64, error)
}

// RetrievalEngine is the public interface for the unified retrieval pipeline.
type RetrievalEngine interface {
	Retrieve(ctx context.Context, query string, opts RetrievalOptions) (*RetrievalResult, error)
}

// Engine implements the RetrievalEngine interface.
type Engine struct {
	db       *storage.DB
	embedder Embedder
}

// NewEngine constructs a new RAG Engine wrapper around the DB and the query embedder.
func NewEngine(db *storage.DB, embedder Embedder) *Engine {
	return &Engine{
		db:       db,
		embedder: embedder,
	}
}

// Retrieve coordinates the full retrieval pipeline:
// 1. Embed query (via Embedder)
// 2. Hybrid RRF search (Vector + FTS)
// 3. Metadata Reranking (Additive boosting + Cap)
// 4. Token-budget Context Builder (Assembly + Trim)
func (e *Engine) Retrieve(ctx context.Context, query string, opts RetrievalOptions) (*RetrievalResult, error) {
	if query == "" {
		return nil, fmt.Errorf("retrieval.Engine: query must not be empty")
	}

	// 1. Run Hybrid Search (includes query embedding, Vector KNN, and FTS BM25 + RRF merge)
	res, err := HybridSearch(ctx, e.db, e.embedder, query, opts)
	if err != nil {
		return nil, fmt.Errorf("retrieval.Engine: hybrid search failed: %w", err)
	}

	// 2. Rerank using metadata-aware boosts
	reranked := Rerank(res.Chunks, query, opts.ChipFamily)

	// 3. Assemble and budget the LLM context string
	contextRes, err := BuildContext(reranked, opts.MaxTokens)
	if err != nil {
		return nil, fmt.Errorf("retrieval.Engine: build context failed: %w", err)
	}

	return &RetrievalResult{
		Chunks:        reranked,
		Context:       contextRes.Context,
		ChunksUsed:    contextRes.ChunksUsed,
		ChunksDropped: contextRes.ChunksDropped,
	}, nil
}
