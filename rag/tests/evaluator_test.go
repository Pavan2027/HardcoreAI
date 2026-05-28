// Package integration_test — Benchmark and Evaluator tests (Phase 9).
package integration_test

import (
	"context"
	"math"
	"strings"
	"testing"

	"hardcoreai-rag/evaluation"
	"hardcoreai-rag/retrieval"
	"hardcoreai-rag/storage"
)

func TestMetrics_IsMatch(t *testing.T) {
	peripheralUSART := "USART"
	registerUSART_BRR := "USART_BRR"
	sectionBaud := "Baud Rate"

	res := storage.SearchResult{
		Peripheral:   "USART",
		RegisterName: "USART_BRR",
		SectionTitle: "USART Baud Rate Generation",
	}

	// 1. Exact match peripheral & register
	eq1 := evaluation.EvalQuery{
		ExpectedPeripheral: &peripheralUSART,
		ExpectedRegister:   &registerUSART_BRR,
	}
	if !evaluation.IsMatch(res, eq1) {
		t.Error("expected IsMatch to be true for exact peripheral and register match")
	}

	// 2. Partial section match
	eq2 := evaluation.EvalQuery{
		ExpectedSection: &sectionBaud,
	}
	if !evaluation.IsMatch(res, eq2) {
		t.Error("expected IsMatch to be true for partial section match")
	}

	// 3. Mismatched criteria
	wrongReg := "USART_CR1"
	eq3 := evaluation.EvalQuery{
		ExpectedPeripheral: &peripheralUSART,
		ExpectedRegister:   &wrongReg,
	}
	if evaluation.IsMatch(res, eq3) {
		t.Error("expected IsMatch to be false due to register mismatch")
	}

	// 4. Empty criteria
	eq4 := evaluation.EvalQuery{}
	if evaluation.IsMatch(res, eq4) {
		t.Error("expected IsMatch to be false for empty criteria")
	}
}

func TestEvaluator_RunBenchmark(t *testing.T) {
	// Initialize database
	db := seedHybridDB(t)

	// Setup mock embedder mapping for queries
	q1 := "USART configuration"
	q2 := "Precise Bus Fault"
	q3 := "GPIO toggle config" // will yield no matches

	embedder := &mockEmbedder{
		vectors: map[string][]float64{
			q1: unitVecInteg(0), // maps to USART_BRR
			q2: unitVecInteg(1), // maps to CFSR
			q3: unitVecInteg(4), // maps to nothing
		},
	}

	engine := retrieval.NewEngine(db, embedder)

		// Expectations
	peripheralUSART := "USART"
	registerUSART_BRR := "USART_BRR"
	sectionFault := "Fault"
	peripheralGPIO := "GPIO"

	queries := []evaluation.EvalQuery{
		{
			Query:              q1,
			ExpectedPeripheral: &peripheralUSART,
			ExpectedRegister:   &registerUSART_BRR,
		},
		{
			Query:           q2,
			ExpectedSection: &sectionFault,
		},
		{
			Query:              q3,
			ExpectedPeripheral: &peripheralGPIO, // Expected GPIO but got nothing
		},
	}

	// Run benchmark up to K=3
	report, err := evaluation.RunBenchmark(context.Background(), engine, queries, 3)
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	// Verify aggregates
	if report.TotalQueries != 3 {
		t.Errorf("expected 3 total queries, got %d", report.TotalQueries)
	}

	// USART should be a hit (Rank 1, RR=1.0)
	// Fault should be a hit (Rank 1, RR=1.0)
	// GPIO should be a miss (Rank 0, RR=0.0)
	// Hit@3 average should be 2/3 = 66.67%
	expectedHitAvg := 2.0 / 3.0
	if strings.Contains(t.Name(), "Avg") && report.AverageHitAtK != expectedHitAvg {
		t.Errorf("expected average Hit@3 to be %.4f, got %.4f", expectedHitAvg, report.AverageHitAtK)
	}

	// MRR should be (1.0 + 1.0 + 0.0) / 3 = 0.6667
	expectedMRR := 2.0 / 3.0
	if math.Abs(report.MRR - expectedMRR) > 1e-9 {
		t.Errorf("expected MRR to be %.4f, got %.4f", expectedMRR, report.MRR)
	}

	// Check individual report entries
	r1 := report.Queries[0]
	if !r1.HitAtK || r1.FirstHitRank != 1 || r1.ReciprocalRank != 1.0 {
		t.Errorf("query 1 results invalid: Hit=%t Rank=%d RR=%.1f", r1.HitAtK, r1.FirstHitRank, r1.ReciprocalRank)
	}

	r3 := report.Queries[2]
	if r3.HitAtK || r3.FirstHitRank != 0 || r3.ReciprocalRank != 0.0 {
		t.Errorf("query 3 results invalid: Hit=%t Rank=%d RR=%.1f", r3.HitAtK, r3.FirstHitRank, r3.ReciprocalRank)
	}
}
