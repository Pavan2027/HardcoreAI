package retrieval

import (
	"sort"
	"strings"

	"hardcoreai-rag/storage"
)

// Scoring weights for the weighted additive formula. Must sum to 1.0.
const (
	weightSemantic = 0.6
	weightFTS      = 0.2
	weightMetadata = 0.2
)

// Metadata boost values. Individual boosts are additive and capped at 1.0 total.
const (
	boostExactRegister   = 0.30
	boostPeripheral      = 0.20
	boostSectionTitle    = 0.15
	boostReferenceManual = 0.10
	boostChipFamily      = 0.05
)

// Rerank applies the weighted additive scoring formula to a hybrid search
// result list and returns it sorted by FinalScore descending.
//
// It overwrites the RRF FinalScore set by HybridSearch with:
//
//	FinalScore = SemanticScore*0.6 + FTSScore*0.2 + MetadataBoost*0.2
//
// Parameters:
//   - results:    merged hybrid results — FinalScore will be overwritten.
//   - query:      the original user query, used for metadata token matching.
//   - chipFamily: the active chip family filter (e.g. "STM32F4"), or "" if none.
//
// Rerank is a pure function: no DB access, no side effects.
func Rerank(results []storage.SearchResult, query, chipFamily string) []storage.SearchResult {
	queryTokens := tokenize(query)

	for i := range results {
		r := &results[i]
		r.MetadataBoost = computeMetadataBoost(*r, queryTokens, chipFamily)
		r.FinalScore = r.SemanticScore*weightSemantic +
			r.FTSScore*weightFTS +
			r.MetadataBoost*weightMetadata
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalScore > results[j].FinalScore
	})

	return results
}

// computeMetadataBoost returns the total metadata boost for a result.
// Individual boosts are additive; the total is capped at 1.0.
func computeMetadataBoost(r storage.SearchResult, queryTokens []string, chipFamily string) float64 {
	boost := 0.0

	// +0.30: ALL tokens of the register name appear in the query.
	// Requiring all tokens prevents "USART" alone triggering a register boost.
	if exactRegisterMatch(r.RegisterName, queryTokens) {
		boost += boostExactRegister
	}
	// +0.20: any token of the peripheral field appears in the query.
	if fieldTokenOverlap(r.Peripheral, queryTokens) {
		boost += boostPeripheral
	}
	// +0.15: any non-stop-word token of the section title appears in the query.
	if sectionTitleOverlap(r.SectionTitle, queryTokens) {
		boost += boostSectionTitle
	}
	// +0.10: reference manuals are preferred over app notes / datasheets.
	if r.DocType == "reference_manual" {
		boost += boostReferenceManual
	}
	// +0.05: chunk belongs to the chip family the user is filtering for.
	if chipFamily != "" && r.ChipFamily == chipFamily {
		boost += boostChipFamily
	}

	if boost > 1.0 {
		boost = 1.0
	}
	return boost
}

// tokenize lowercases s and splits on spaces, underscores, hyphens, and
// forward slashes — the common delimiters in STM32 identifiers and queries.
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == '/'
	})
}

// exactRegisterMatch returns true only when EVERY token of the register name
// appears in queryTokens.
//
// Example: USART_BRR → ["usart","brr"] — both must be present in the query.
// This prevents "USART configuration" from triggering a +0.30 register boost.
func exactRegisterMatch(registerName string, queryTokens []string) bool {
	if registerName == "" {
		return false
	}
	querySet := makeSet(queryTokens)
	for _, rt := range tokenize(registerName) {
		if !querySet[rt] {
			return false
		}
	}
	return true
}

// fieldTokenOverlap returns true if any token of field appears in queryTokens.
// Used for peripheral matching where a single-token match is sufficient.
func fieldTokenOverlap(field string, queryTokens []string) bool {
	if field == "" {
		return false
	}
	querySet := makeSet(queryTokens)
	for _, ft := range tokenize(field) {
		if querySet[ft] {
			return true
		}
	}
	return false
}

// sectionTitleOverlap returns true if any non-stop-word token from sectionTitle
// appears in queryTokens.
func sectionTitleOverlap(sectionTitle string, queryTokens []string) bool {
	if sectionTitle == "" {
		return false
	}
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"in": true, "of": true, "to": true, "for": true, "with": true,
		"is": true, "its": true, "by": true, "on": true, "at": true,
	}
	querySet := makeSet(queryTokens)
	for _, tt := range tokenize(sectionTitle) {
		if !stopWords[tt] && querySet[tt] {
			return true
		}
	}
	return false
}

// makeSet converts a token slice into an O(1) lookup map.
func makeSet(tokens []string) map[string]bool {
	s := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		s[t] = true
	}
	return s
}