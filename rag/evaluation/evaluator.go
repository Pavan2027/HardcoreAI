// Package evaluation provides benchmarking, retrieval quality analysis, and regression testing utilities.
package evaluation

import (
	"context"
	"fmt"

	"hardcoreai-rag/retrieval"
)

// RunBenchmark runs a test query suite against the retrieval Engine and computes evaluation metrics.
// K controls the evaluation limit (e.g. evaluating Hit@K, Precision@K).
func RunBenchmark(
	ctx context.Context,
	engine retrieval.RetrievalEngine,
	queries []EvalQuery,
	k int,
) (*BenchmarkReport, error) {
	if len(queries) == 0 {
		return nil, fmt.Errorf("RunBenchmark: query list must not be empty")
	}
	if k <= 0 {
		k = 10 // default evaluation limit
	}

	var queryReports []QueryReport
	totalHits := 0
	sumRR := 0.0
	sumPrecision := 0.0

	for _, eq := range queries {
		// Run full end-to-end engine retrieval
		opts := retrieval.RetrievalOptions{
			K:         k,
			MaxTokens: 4000, // generous budget for evaluation
		}
		
		res, err := engine.Retrieve(ctx, eq.Query, opts)
		if err != nil {
			return nil, fmt.Errorf("RunBenchmark: query %q failed: %w", eq.Query, err)
		}

		// Compute metrics for this query
		matchCount := 0
		firstHitRank := 0
		hitAtK := false
		reciprocalRank := 0.0

		// Check the retrieved chunks up to K limit
		limit := len(res.Chunks)
		if limit > k {
			limit = k
		}

		for idx := 0; idx < limit; idx++ {
			chunk := res.Chunks[idx]
			if IsMatch(chunk, eq) {
				matchCount++
				if !hitAtK {
					hitAtK = true
					firstHitRank = idx + 1
					reciprocalRank = 1.0 / float64(firstHitRank)
				}
			}
		}

		precision := float64(matchCount) / float64(k)

		// Accumulate aggregates
		if hitAtK {
			totalHits++
		}
		sumRR += reciprocalRank
		sumPrecision += precision

		// Construct strings for expected fields for report formatting
		expPeripheral := ""
		if eq.ExpectedPeripheral != nil {
			expPeripheral = *eq.ExpectedPeripheral
		}
		expRegister := ""
		if eq.ExpectedRegister != nil {
			expRegister = *eq.ExpectedRegister
		}
		expSection := ""
		if eq.ExpectedSection != nil {
			expSection = *eq.ExpectedSection
		}

		queryReports = append(queryReports, QueryReport{
			Query:              eq.Query,
			ExpectedPeripheral: expPeripheral,
			ExpectedRegister:   expRegister,
			ExpectedSection:    expSection,
			HitAtK:             hitAtK,
			ReciprocalRank:     reciprocalRank,
			PrecisionAtK:       precision,
			FirstHitRank:       firstHitRank,
		})
	}

	numQueries := len(queries)
	return &BenchmarkReport{
		TotalQueries:  numQueries,
		AverageHitAtK: float64(totalHits) / float64(numQueries),
		MRR:           sumRR / float64(numQueries),
		MeanPrecision: sumPrecision / float64(numQueries),
		Queries:       queryReports,
	}, nil
}
