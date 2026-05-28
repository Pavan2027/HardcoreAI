package indexing

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"

	_ "github.com/mattn/go-sqlite3"
)

// Indexer generates embeddings and writes them to the chunks.embedding column.
type Indexer struct {
	db       *sql.DB
	embedder *Embedder
}

// NewIndexer opens the database and returns a ready Indexer.
func NewIndexer(dbPath string, embedder *Embedder) (*Indexer, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Println("✅ Indexer ready")
	return &Indexer{db: db, embedder: embedder}, nil
}

// IndexChunks generates embeddings for chunkTexts and writes each embedding
// to the chunks.embedding column for the corresponding chunkID.
//
// chunkIDs and chunkTexts must be the same length and in the same order.
func (idx *Indexer) IndexChunks(chunkIDs []int, chunkTexts []string) error {
	fmt.Printf("🔢 Generating embeddings for %d chunks...\n", len(chunkTexts))

	embeddings, err := idx.embedder.EmbedBatch(chunkTexts)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Write each embedding blob directly to the chunks table.
	// The blob format is little-endian float32, matching storage.SerializeEmbedding.
	stmt, err := tx.Prepare(`UPDATE chunks SET embedding = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, embedding := range embeddings {
		blob := float32SliceToBytes(embedding)
		if _, err := stmt.Exec(blob, chunkIDs[i]); err != nil {
			return fmt.Errorf("failed to store embedding for chunk %d: %w", chunkIDs[i], err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Printf("✅ Stored %d embeddings in chunks.embedding\n", len(embeddings))
	return nil
}

// Close closes the underlying database connection.
func (idx *Indexer) Close() error {
	return idx.db.Close()
}

// float32SliceToBytes serializes a float32 slice to little-endian bytes.
// This is the same format as storage.SerializeEmbedding — both must stay in sync.
func float32SliceToBytes(floats []float32) []byte {
	b := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(b[i*4:], bits)
	}
	return b
}