// Package storage_test — BuildFilterSQL unit tests (Phase 7).
// No DB or vec0 extension required.
//
// HOW TO RUN:
//
//	go test -tags "fts5" ./storage/ -run TestBuildFilterSQL -v
package storage_test

import (
	"strings"
	"testing"

	"hardcoreai-rag/storage"
)

// TestBuildFilterSQL_Empty verifies empty opts returns no clause or args.
func TestBuildFilterSQL_Empty(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{})
	if clause != "" {
		t.Errorf("expected empty clause, got %q", clause)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
	t.Log("✓ Empty opts → empty clause")
}

// TestBuildFilterSQL_ChipFamily verifies chip family produces correct SQL and arg.
func TestBuildFilterSQL_ChipFamily(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{ChipFamily: "STM32F4"})
	if !strings.Contains(clause, "d.chip_family") {
		t.Errorf("expected chip_family condition in clause, got %q", clause)
	}
	if len(args) != 1 || args[0] != "STM32F4" {
		t.Errorf("expected args [STM32F4], got %v", args)
	}
	t.Logf("✓ ChipFamily clause: %q args: %v", clause, args)
}

// TestBuildFilterSQL_ChipModel verifies chip model filter.
func TestBuildFilterSQL_ChipModel(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{ChipModel: "STM32F429"})
	if !strings.Contains(clause, "d.chip_model") {
		t.Errorf("expected chip_model in clause, got %q", clause)
	}
	if len(args) != 1 || args[0] != "STM32F429" {
		t.Errorf("expected args [STM32F429], got %v", args)
	}
	t.Logf("✓ ChipModel clause: %q args: %v", clause, args)
}

// TestBuildFilterSQL_Peripheral verifies peripheral filter.
func TestBuildFilterSQL_Peripheral(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{Peripheral: "USART"})
	if !strings.Contains(clause, "c.peripheral") {
		t.Errorf("expected c.peripheral in clause, got %q", clause)
	}
	if len(args) != 1 || args[0] != "USART" {
		t.Errorf("expected args [USART], got %v", args)
	}
	t.Logf("✓ Peripheral clause: %q args: %v", clause, args)
}

// TestBuildFilterSQL_DocTypes_Single verifies single doc type produces IN(?) clause.
func TestBuildFilterSQL_DocTypes_Single(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{
		DocTypes: []string{"reference_manual"},
	})
	if !strings.Contains(clause, "d.doc_type IN") {
		t.Errorf("expected IN clause for doc_type, got %q", clause)
	}
	if len(args) != 1 || args[0] != "reference_manual" {
		t.Errorf("expected args [reference_manual], got %v", args)
	}
	t.Logf("✓ Single DocType clause: %q", clause)
}

// TestBuildFilterSQL_DocTypes_Multiple verifies multiple doc types produce IN(?,?) clause.
func TestBuildFilterSQL_DocTypes_Multiple(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{
		DocTypes: []string{"reference_manual", "datasheet"},
	})
	if !strings.Contains(clause, "IN (?,?)") {
		t.Errorf("expected IN (?,?) for two doc types, got %q", clause)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d: %v", len(args), args)
	}
	t.Logf("✓ Multi DocType clause: %q args: %v", clause, args)
}

// TestBuildFilterSQL_Combined verifies multiple filters are AND-joined.
func TestBuildFilterSQL_Combined(t *testing.T) {
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{
		ChipFamily: "STM32F4",
		Peripheral: "USART",
		DocTypes:   []string{"reference_manual"},
	})

	// All three conditions should appear.
	for _, expected := range []string{"d.chip_family", "c.peripheral", "d.doc_type IN"} {
		if !strings.Contains(clause, expected) {
			t.Errorf("expected %q in combined clause, got %q", expected, clause)
		}
	}
	// Conditions should be AND-joined.
	if !strings.Contains(clause, " AND ") {
		t.Errorf("expected AND between conditions, got %q", clause)
	}
	// 3 args: STM32F4, USART, reference_manual.
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(args), args)
	}
	t.Logf("✓ Combined clause: %q args: %v", clause, args)
}

// TestBuildFilterSQL_KFieldIgnored verifies that K is not part of the filter clause.
func TestBuildFilterSQL_KFieldIgnored(t *testing.T) {
	// K=10 with no other filters → empty clause.
	clause, args := storage.BuildFilterSQL(storage.SearchOptions{K: 10})
	if clause != "" || len(args) != 0 {
		t.Errorf("K alone should produce no filter clause, got clause=%q args=%v", clause, args)
	}
	t.Log("✓ K alone → empty clause (K is not a filter field)")
}