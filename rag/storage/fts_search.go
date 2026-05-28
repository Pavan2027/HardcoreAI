package storage

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"
)

var stopWords = map[string]bool{
	"a": true, "about": true, "above": true, "after": true, "again": true,
	"against": true, "all": true, "am": true, "an": true, "and": true,
	"any": true, "are": true, "as": true, "at": true, "be": true,
	"because": true, "been": true, "before": true, "being": true, "below": true,
	"between": true, "both": true, "but": true, "by": true, "can": true,
	"did": true, "do": true, "does": true, "doing": true, "down": true,
	"during": true, "each": true, "few": true, "for": true, "from": true,
	"further": true, "had": true, "has": true, "have": true, "having": true,
	"he": true, "her": true, "here": true, "hers": true, "him": true,
	"his": true, "how": true, "i": true, "if": true, "in": true,
	"into": true, "is": true, "it": true, "its": true, "me": true,
	"more": true, "most": true, "my": true, "no": true, "nor": true,
	"not": true, "of": true, "off": true, "on": true, "once": true,
	"only": true, "or": true, "other": true, "our": true, "ours": true,
	"out": true, "over": true, "own": true, "same": true, "she": true,
	"should": true, "so": true, "some": true, "such": true, "than": true,
	"that": true, "the": true, "their": true, "theirs": true, "them": true,
	"then": true, "there": true, "these": true, "they": true, "this": true,
	"those": true, "through": true, "to": true, "too": true, "under": true,
	"until": true, "up": true, "very": true, "was": true, "we": true,
	"were": true, "what": true, "when": true, "where": true, "which": true,
	"while": true, "who": true, "whom": true, "why": true, "with": true,
	"you": true, "your": true, "yours": true, "causes": true, "cause": true,
}

// sanitizeFTSQuery strips characters that FTS5 treats as syntax operators
// while preserving tokens common in STM32 docs: letters, digits, underscores
// (USART_BRR), hyphens (chip names), and spaces. It splits terms, removes
// English stop words, and joins them using 'OR' for high-recall BM25 matching.
func sanitizeFTSQuery(query string) string {
	var b strings.Builder
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == ' ' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	
	words := strings.Fields(b.String())
	var keywords []string
	for _, w := range words {
		lowerW := strings.ToLower(w)
		if !stopWords[lowerW] && len(lowerW) > 1 {
			keywords = append(keywords, w)
		}
	}

	if len(keywords) == 0 {
		if len(words) == 0 {
			return ""
		}
		return strings.Join(words, " OR ")
	}

	return strings.Join(keywords, " OR ")
}

// FTSSearch performs BM25-ranked full-text search against chunk_fts.
//
// Filters in opts are ANDed into the WHERE clause at the SQL level.
// When filters are active the query over-fetches by 3× before applying LIMIT,
// since filter conditions narrow the FTS candidates after the MATCH.
//
// FTSScore on each result is normalised to [0,1]: best BM25 rank → 1.0.
func (db *DB) FTSSearch(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("storage.FTSSearch: query must not be empty")
	}

	clean := sanitizeFTSQuery(query)
	if clean == "" {
		return []SearchResult{}, nil
	}

	fetchK := opts.K
	if fetchK <= 0 {
		fetchK = 10
	}

	filterClause, filterArgs := BuildFilterSQL(opts)
	if filterClause != "" {
		fetchK *= 3
	}

	// Base query: FTS MATCH, JOINs to chunks and documents.
	// Filter clause is ANDed after MATCH. ORDER BY bm25 and LIMIT are appended last.
	const baseQuery = `
		SELECT
			c.id, c.document_id, c.chunk_text, c.section_title,
			c.peripheral, c.register_name, c.page_number, c.chunk_index,
			d.filename, d.doc_type, d.chip_family, d.chip_model,
			bm25(chunk_fts) AS bm25_score
		FROM chunk_fts
		JOIN chunks c ON c.id = chunk_fts.rowid
		JOIN documents d ON d.id = c.document_id
		WHERE chunk_fts MATCH ?
	`

	q := baseQuery
	if filterClause != "" {
		q += "AND " + filterClause + "\n"
	}
	q += "ORDER BY bm25_score\nLIMIT ?"

	// Arg order: MATCH string, filter args (if any), then LIMIT.
	args := make([]interface{}, 0, 2+len(filterArgs))
	args = append(args, clean)
	args = append(args, filterArgs...)
	args = append(args, fetchK)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("storage.FTSSearch: query failed: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	var rawScores []float64

	for rows.Next() {
		var r SearchResult
		var bm25 float64
		if err := rows.Scan(
			&r.ChunkID, &r.DocumentID, &r.ChunkText, &r.SectionTitle,
			&r.Peripheral, &r.RegisterName, &r.PageNumber, &r.ChunkIndex,
			&r.Filename, &r.DocType, &r.ChipFamily, &r.ChipModel,
			&bm25,
		); err != nil {
			return nil, fmt.Errorf("storage.FTSSearch: scan row: %w", err)
		}
		results = append(results, r)
		rawScores = append(rawScores, bm25)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.FTSSearch: rows error: %w", err)
	}

	if len(results) == 0 {
		return []SearchResult{}, nil
	}

	// BM25 from FTS5 is negative: most negative = best match.
	// Normalise: best (minScore) → 1.0, worst (maxScore) → 0.0.
	minScore, maxScore := rawScores[0], rawScores[0]
	for _, s := range rawScores[1:] {
		if s < minScore {
			minScore = s
		}
		if s > maxScore {
			maxScore = s
		}
	}

	spread := maxScore - minScore
	for i := range results {
		if math.Abs(spread) < 1e-9 {
			results[i].FTSScore = 1.0
		} else {
			results[i].FTSScore = (maxScore - rawScores[i]) / spread
		}
	}

	if opts.K > 0 && len(results) > opts.K {
		results = results[:opts.K]
	}

	return results, nil
}