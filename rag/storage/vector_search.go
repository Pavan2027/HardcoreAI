package storage

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// SerializeEmbedding converts a []float64 into a little-endian float32 byte
// blob for storage in the chunks.embedding column.
// Exported so the indexing pipeline uses the same format.
func SerializeEmbedding(vec []float64) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		bits := math.Float32bits(float32(v))
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}

// deserializeEmbedding converts a little-endian float32 BLOB back to []float32.
// Returns nil if the blob length is not a multiple of 4.
func deserializeEmbedding(blob []byte) []float32 {
	if len(blob) == 0 || len(blob)%4 != 0 {
		return nil
	}
	vec := make([]float32, len(blob)/4)
	for i := range vec {
		bits := binary.LittleEndian.Uint32(blob[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}

// cosineSimilarity computes the cosine similarity between a query vector
// ([]float64) and a stored embedding ([]float32).
// Returns 0 for mismatched lengths or zero-magnitude vectors.
func cosineSimilarity(query []float64, stored []float32) float64 {
	if len(query) != len(stored) {
		return 0
	}
	var dot, normQ, normS float64
	for i := range query {
		q := query[i]
		s := float64(stored[i])
		dot += q * s
		normQ += q * q
		normS += s * s
	}
	if normQ == 0 || normS == 0 {
		return 0
	}
	return dot / (math.Sqrt(normQ) * math.Sqrt(normS))
}

// VectorSearch performs cosine similarity search against the embedding BLOB
// stored in the chunks table.
//
// It reads all chunks matching the filter, deserializes each embedding, and
// computes cosine similarity in Go. Results are sorted by score descending
// and trimmed to opts.K.
//
// This approach trades query-time CPU for simplicity and zero native
// dependencies. For corpora larger than ~100k chunks, consider an ANN index.
func (db *DB) VectorSearch(ctx context.Context, embedding []float64, opts SearchOptions) ([]SearchResult, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("storage.VectorSearch: embedding must not be empty")
	}

	filterClause, filterArgs := BuildFilterSQL(opts)

	// Fetch all chunks that have a non-NULL embedding. Filter is applied at
	// the SQL level so we only deserialize candidates we'd actually return.
	const baseQuery = `
		SELECT
			c.id, c.document_id, c.chunk_text, c.section_title,
			c.peripheral, c.register_name, c.page_number, c.chunk_index,
			d.filename, d.doc_type, d.chip_family, d.chip_model,
			c.embedding
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.embedding IS NOT NULL
	`

	query := baseQuery
	if filterClause != "" {
		query += "AND " + filterClause + "\n"
	}

	rows, err := db.QueryContext(ctx, query, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("storage.VectorSearch: query failed: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		result SearchResult
		score  float64
	}

	var candidates []candidate

	for rows.Next() {
		var r SearchResult
		var blob []byte
		if err := rows.Scan(
			&r.ChunkID, &r.DocumentID, &r.ChunkText, &r.SectionTitle,
			&r.Peripheral, &r.RegisterName, &r.PageNumber, &r.ChunkIndex,
			&r.Filename, &r.DocType, &r.ChipFamily, &r.ChipModel,
			&blob,
		); err != nil {
			return nil, fmt.Errorf("storage.VectorSearch: scan row: %w", err)
		}

		stored := deserializeEmbedding(blob)
		if stored == nil {
			continue // skip malformed or empty embeddings
		}

		score := cosineSimilarity(embedding, stored)
		candidates = append(candidates, candidate{result: r, score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.VectorSearch: rows error: %w", err)
	}

	// Sort by cosine similarity descending (highest score first).
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	k := opts.K
	if k <= 0 {
		k = 10
	}
	if len(candidates) > k {
		candidates = candidates[:k]
	}

	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = c.result
		results[i].SemanticScore = c.score
	}

	return results, nil
}