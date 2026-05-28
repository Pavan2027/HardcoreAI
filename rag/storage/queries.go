package storage

// SQL query constants used across the storage package.
// Using named constants avoids magic strings scattered through the codebase.

const (
	// QueryAllChunks retrieves all chunks with their parent document metadata.
	QueryAllChunks = `
		SELECT
			c.id, c.document_id, c.chunk_text, c.section_title,
			c.subsection_title, c.peripheral, c.register_name,
			c.page_number, c.token_count, c.chunk_index,
			d.filename, d.doc_type, d.chip_family, d.chip_model
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
	`

	// QueryChunkByID retrieves a single chunk with document metadata.
	QueryChunkByID = `
		SELECT
			c.id, c.document_id, c.chunk_text, c.section_title,
			c.subsection_title, c.peripheral, c.register_name,
			c.page_number, c.token_count, c.chunk_index,
			d.filename, d.doc_type, d.chip_family, d.chip_model
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.id = ?
	`

	// QueryAllDocuments retrieves all documents.
	QueryAllDocuments = `
		SELECT id, mongo_id, filename, doc_type, chip_family, chip_model, version, created_at
		FROM documents
	`

	// DEPRECATED:
	// QueryVectorSearch finds top-K nearest embedding neighbours using sqlite-vec.
	// The KNN LIMIT must sit directly on the vec0 virtual table — not on an outer
	// JOIN — otherwise sqlite-vec raises "A LIMIT or 'k = ?' constraint is required".
	QueryVectorSearch = `
		SELECT
			c.id, c.document_id, c.chunk_text, c.section_title,
			c.peripheral, c.register_name, c.page_number, c.chunk_index,
			d.filename, d.doc_type, d.chip_family, d.chip_model,
			knn.distance
		FROM (
			SELECT chunk_id, distance
			FROM vec_chunks
			WHERE embedding MATCH ?
			ORDER BY distance
			LIMIT ?
		) knn
		JOIN chunks c ON c.id = knn.chunk_id
		JOIN documents d ON d.id = c.document_id
		ORDER BY knn.distance
	`

	// DEPRECATED: 
	// QueryFTSSearch performs BM25-ranked full-text search.
	// The ? parameters are: FTS query string, K limit.
	QueryFTSSearch = `
		SELECT
			c.id, c.document_id, c.chunk_text, c.section_title,
			c.peripheral, c.register_name, c.page_number, c.chunk_index,
			d.filename, d.doc_type, d.chip_family, d.chip_model,
			bm25(chunk_fts) AS bm25_score
		FROM chunk_fts
		JOIN chunks c ON c.id = chunk_fts.rowid
		JOIN documents d ON d.id = c.document_id
		WHERE chunk_fts MATCH ?
		ORDER BY bm25_score
		LIMIT ?
	`
)
