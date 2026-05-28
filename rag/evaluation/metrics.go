// Package evaluation provides benchmarking, retrieval quality analysis, and regression testing utilities.
package evaluation

import (
	"strings"

	"hardcoreai-rag/storage"
)

// EvalQuery represents a single evaluation query with expected metadata.
type EvalQuery struct {
	Query              string  `json:"query"`
	ExpectedPeripheral *string `json:"expected_peripheral"`
	ExpectedRegister   *string `json:"expected_register"`
	ExpectedSection    *string `json:"expected_section"`
}

// QueryReport holds metrics computed for an individual evaluation query.
type QueryReport struct {
	Query              string  `json:"query"`
	ExpectedPeripheral string  `json:"expected_peripheral"`
	ExpectedRegister   string  `json:"expected_register"`
	ExpectedSection    string  `json:"expected_section"`
	HitAtK             bool    `json:"hit_at_k"`
	ReciprocalRank     float64 `json:"reciprocal_rank"`
	PrecisionAtK       float64 `json:"precision_at_k"`
	FirstHitRank       int     `json:"first_hit_rank"` // 1-indexed rank of first hit, 0 if no hit
}

// BenchmarkReport holds aggregated metrics for the entire evaluation test suite.
type BenchmarkReport struct {
	TotalQueries  int           `json:"total_queries"`
	AverageHitAtK float64       `json:"average_hit_at_k"`
	MRR           float64       `json:"mrr"`
	MeanPrecision float64       `json:"mean_precision"`
	Queries       []QueryReport `json:"queries"`
}

// IsMatch checks if a SearchResult chunk matches the expectations set in EvalQuery.
// A result is considered a match if ALL specified non-nil, non-empty expected fields match.
// Section names are matched using case-insensitive partial substring match.
func IsMatch(res storage.SearchResult, eq EvalQuery) bool {
	hasCriteria := false

	if eq.ExpectedPeripheral != nil && *eq.ExpectedPeripheral != "" {
		hasCriteria = true
		if !strings.EqualFold(strings.TrimSpace(res.Peripheral), strings.TrimSpace(*eq.ExpectedPeripheral)) {
			return false
		}
	}

	if eq.ExpectedRegister != nil && *eq.ExpectedRegister != "" {
		hasCriteria = true
		if !strings.EqualFold(strings.TrimSpace(res.RegisterName), strings.TrimSpace(*eq.ExpectedRegister)) {
			return false
		}
	}

	if eq.ExpectedSection != nil && *eq.ExpectedSection != "" {
		hasCriteria = true
		expectedLower := strings.ToLower(strings.TrimSpace(*eq.ExpectedSection))
		actualLower := strings.ToLower(strings.TrimSpace(res.SectionTitle))
		if !strings.Contains(actualLower, expectedLower) {
			return false
		}
	}

	// If no expectations are provided, it cannot count as a hit
	return hasCriteria
}
