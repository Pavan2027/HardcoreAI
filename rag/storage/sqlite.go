package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // registers the "sqlite3" driver on import
)

// DB wraps the standard sql.DB with project-specific helpers.
type DB struct {
	*sql.DB
}

// Open opens a SQLite database at dbPath using the plain sqlite3 driver.
//
// vecExtPath is accepted for backward compatibility but is no longer used —
// sqlite-vec was removed in favour of in-Go cosine similarity. Pass "" or
// any string; it is silently ignored.
func Open(dbPath string, vecExtPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage.Open: failed to open db at %q: %w", dbPath, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("storage.Open: ping failed: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("storage.Open: failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("storage.Open: failed to enable foreign keys: %w", err)
	}

	return &DB{db}, nil
}

// NewDB opens (or creates) a database at dbPath and applies the full production schema.
func NewDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := Open(dbPath, "")
	if err != nil {
		return nil, fmt.Errorf("NewDB failed: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mongo_id TEXT UNIQUE,
		filename TEXT NOT NULL,
		local_path TEXT,
		source_url TEXT,
		doc_type TEXT,
		chip_family TEXT,
		chip_model TEXT,
		version TEXT,
		processing_status TEXT DEFAULT 'pending',
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		section_title TEXT,
		subsection_title TEXT,
		peripheral TEXT,
		register_name TEXT,
		page_number INTEGER,
		token_count INTEGER,
		chunk_index INTEGER,
		metadata TEXT,
		embedding BLOB,
		FOREIGN KEY(document_id) REFERENCES documents(id)
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS chunk_fts USING fts5(
		chunk_text,
		section_title,
		peripheral,
		register_name,
		content='chunks',
		content_rowid='id'
	);

	CREATE INDEX IF NOT EXISTS idx_documents_chip_family ON documents(chip_family);
	CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(processing_status);
	CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
	CREATE INDEX IF NOT EXISTS idx_chunks_peripheral ON chunks(peripheral);
	CREATE INDEX IF NOT EXISTS idx_chunks_register ON chunks(register_name);
	`

	if err := db.ApplySchema(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("NewDB: failed to apply schema: %w", err)
	}

	fmt.Println("✅ Database connected and tables ready")
	return db, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}

// ApplySchema executes schemaSQL against the database.
func (db *DB) ApplySchema(schemaSQL string) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("storage.ApplySchema: %w", err)
	}
	return nil
}

// InsertDocument inserts a new document record and returns its auto-generated ID.
func (db *DB) InsertDocument(doc Document) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO documents
			(mongo_id, filename, local_path, source_url, doc_type, chip_family, chip_model, version, processing_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.MongoID, doc.Filename, doc.LocalPath, doc.SourceURL,
		doc.DocType, doc.ChipFamily, doc.ChipModel, doc.Version, "processing",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert document: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	fmt.Printf("📄 Inserted document: %s (id=%d)\n", doc.Filename, id)
	return id, nil
}

// InsertChunk inserts a single chunk and its FTS row in one transaction.
func (db *DB) InsertChunk(chunk Chunk) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("InsertChunk: begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO chunks
			(document_id, chunk_text, section_title, subsection_title, peripheral, register_name, page_number, token_count, chunk_index, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		chunk.DocumentID, chunk.ChunkText, chunk.SectionTitle, chunk.SubsectionTitle,
		chunk.Peripheral, chunk.RegisterName, chunk.PageNumber, chunk.TokenCount,
		chunk.ChunkIndex, chunk.Metadata,
	)
	if err != nil {
		return 0, fmt.Errorf("InsertChunk: insert chunk: %w", err)
	}
	id, _ := result.LastInsertId()

	if _, err := tx.Exec(
		`INSERT INTO chunk_fts (rowid, chunk_text, section_title, peripheral, register_name) VALUES (?, ?, ?, ?, ?)`,
		id, chunk.ChunkText, chunk.SectionTitle, chunk.Peripheral, chunk.RegisterName,
	); err != nil {
		return 0, fmt.Errorf("InsertChunk: insert chunk_fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("InsertChunk: commit: %w", err)
	}
	return id, nil
}

// InsertChunks inserts a batch of chunks and their FTS rows.
func (db *DB) InsertChunks(chunks []Chunk) error {
	_, err := db.insertChunkBatch(chunks)
	return err
}

// InsertChunksAndReturnIDs inserts a batch of chunks, populates chunk_fts,
// and returns the auto-generated IDs in insertion order.
func (db *DB) InsertChunksAndReturnIDs(chunks []Chunk) ([]int, error) {
	return db.insertChunkBatch(chunks)
}

func (db *DB) insertChunkBatch(chunks []Chunk) ([]int, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("insertChunkBatch: begin tx: %w", err)
	}
	defer tx.Rollback()

	chunkStmt, err := tx.Prepare(`
		INSERT INTO chunks
			(document_id, chunk_text, section_title, subsection_title, peripheral, register_name, page_number, token_count, chunk_index, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("insertChunkBatch: prepare chunk stmt: %w", err)
	}
	defer chunkStmt.Close()

	ftsStmt, err := tx.Prepare(
		`INSERT INTO chunk_fts (rowid, chunk_text, section_title, peripheral, register_name) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("insertChunkBatch: prepare fts stmt: %w", err)
	}
	defer ftsStmt.Close()

	ids := make([]int, len(chunks))
	for i, chunk := range chunks {
		result, err := chunkStmt.Exec(
			chunk.DocumentID, chunk.ChunkText, chunk.SectionTitle, chunk.SubsectionTitle,
			chunk.Peripheral, chunk.RegisterName, chunk.PageNumber, chunk.TokenCount,
			chunk.ChunkIndex, chunk.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("insertChunkBatch: insert chunk[%d]: %w", i, err)
		}
		id, _ := result.LastInsertId()
		ids[i] = int(id)

		if _, err := ftsStmt.Exec(id, chunk.ChunkText, chunk.SectionTitle, chunk.Peripheral, chunk.RegisterName); err != nil {
			return nil, fmt.Errorf("insertChunkBatch: insert chunk_fts[%d]: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("insertChunkBatch: commit: %w", err)
	}

	fmt.Printf("✅ Inserted %d chunks (+ FTS rows) into database\n", len(chunks))
	return ids, nil
}

// UpdateDocumentStatus updates processing_status and error_message for a document.
func (db *DB) UpdateDocumentStatus(id int64, status, errMsg string) error {
	_, err := db.Exec(
		`UPDATE documents SET processing_status = ?, error_message = ? WHERE id = ?`,
		status, errMsg, id,
	)
	return err
}

// GetChunksByPeripheral retrieves all chunks tagged with a specific peripheral.
func (db *DB) GetChunksByPeripheral(peripheral string) ([]Chunk, error) {
	rows, err := db.Query(`
		SELECT id, document_id, chunk_text, section_title, peripheral, register_name, page_number, token_count, chunk_index
		FROM chunks WHERE peripheral = ?`, peripheral)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(
			&c.ID, &c.DocumentID, &c.ChunkText, &c.SectionTitle,
			&c.Peripheral, &c.RegisterName, &c.PageNumber, &c.TokenCount, &c.ChunkIndex,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}