package retrieval

import (
	"context"
	"fmt"
	"sort"

	"hardcoreai-rag/storage"
)

// rrfK is the RRF constant from the original paper.
// 60 is the standard value — higher values reduce the impact of rank differences.
const rrfK = 60

// HybridSearch is the entry point for the retrieval pipeline.
//
// It runs vector search and FTS search, then merges the two ranked lists using
// Reciprocal Rank Fusion (RRF). The merged list is deduplicated, trimmed to
// opts.K, and ready to be passed to Rerank.
//
// Each result's FinalScore holds its RRF score at this stage. Rerank (Phase 6)
// will overwrite FinalScore with the weighted additive score.
//
// Deprecated logic has been fully removed for portability.
func HybridSearch(
	ctx context.Context,
	db *storage.DB,
	embedder Embedder,
	query string,
	opts RetrievalOptions,
) (*RetrievalResult, error) {
	if query == "" {
		return nil, fmt.Errorf("retrieval.HybridSearch: query must not be empty")
	}

	k := opts.K
	if k <= 0 {
		k = 10
	}

	// Over-fetch so RRF has a wider candidate pool to merge from.
	storageOpts := StorageOptions(opts, k*2)

	vec, err := embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("retrieval.HybridSearch: embed query: %w", err)
	}

	vectorResults, err := db.VectorSearch(ctx, vec, storageOpts)
	if err != nil {
		return nil, fmt.Errorf("retrieval.HybridSearch: vector search: %w", err)
	}

	ftsResults, err := db.FTSSearch(ctx, query, storageOpts)
	if err != nil {
		return nil, fmt.Errorf("retrieval.HybridSearch: fts search: %w", err)
	}

	merged := mergeRRF(vectorResults, ftsResults)

	if len(merged) > k {
		merged = merged[:k]
	}

	return &RetrievalResult{Chunks: merged}, nil
}

// mergeRRF merges two ranked lists using Reciprocal Rank Fusion.
//
// Formula: RRF(d) = Σ 1 / (rrfK + rank(d)) summed over all lists d appears in.
//
// A chunk appearing in both lists gets contributions from both rankings,
// naturally surfacing it above single-list results. SemanticScore and FTSScore
// from their respective lists are preserved for use by the reranker.
func mergeRRF(vectorResults, ftsResults []storage.SearchResult) []storage.SearchResult {
	type entry struct {
		result   storage.SearchResult
		rrfScore float64
	}

	byID := make(map[int]*entry, len(vectorResults)+len(ftsResults))

	for rank, r := range vectorResults {
		score := 1.0 / float64(rrfK+rank+1)
		if e, ok := byID[r.ChunkID]; ok {
			e.rrfScore += score
		} else {
			rc := r
			byID[r.ChunkID] = &entry{result: rc, rrfScore: score}
		}
	}

	for rank, r := range ftsResults {
		score := 1.0 / float64(rrfK+rank+1)
		if e, ok := byID[r.ChunkID]; ok {
			e.rrfScore += score
			e.result.FTSScore = r.FTSScore // preserve FTS score for reranker
		} else {
			rc := r
			byID[r.ChunkID] = &entry{result: rc, rrfScore: score}
		}
	}

	merged := make([]storage.SearchResult, 0, len(byID))
	for _, e := range byID {
		r := e.result
		r.FinalScore = e.rrfScore // reranker overwrites this in Phase 6
		merged = append(merged, r)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].FinalScore > merged[j].FinalScore
	})

	return merged
}