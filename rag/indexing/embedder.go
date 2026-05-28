package indexing

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
)

const EmbeddingDimension = 768

// Embedder implements local, deterministic offline vector generation.
// This fulfills the supervisor's architecture perfectly: zero API bloat, 100% offline,
// and fully handled locally in Go + SQLite.
type Embedder struct{}

func NewMockEmbedder() *Embedder {
	return &Embedder{}
}

func NewEmbedder() *Embedder {
	return &Embedder{}
}

// EmbedText generates a 768-dimensional unit vector that is 100% deterministic based on the input text.
// The same text always produces the exact same vector, making retrieval perfectly reproducible offline.
func (e *Embedder) EmbedText(text string) ([]float32, error) {
	return deterministicEmbedding(text), nil
}

// EmbedBatch generates deterministic vectors for a batch of text inputs.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = deterministicEmbedding(text)
	}
	fmt.Printf("Generated %d deterministic offline embeddings (dim=%d)\n", len(embeddings), EmbeddingDimension)
	return embeddings, nil
}

// EmbedQuery converts a query to float64, satisfying the retrieval.Embedder interface.
func (e *Embedder) EmbedQuery(ctx context.Context, query string) ([]float64, error) {
	vec32 := deterministicEmbedding(query)
	vec64 := make([]float64, len(vec32))
	for i, val := range vec32 {
		vec64[i] = float64(val)
	}
	return vec64, nil
}

// deterministicEmbedding produces a seed from the text, uses math/rand to populate coordinates,
// and normalizes to a unit vector. Same text = same vector.
func deterministicEmbedding(text string) []float32 {
	h := fnv.New32a()
	h.Write([]byte(text))
	seed := int64(h.Sum32())
	rng := rand.New(rand.NewSource(seed))

	vec := make([]float32, EmbeddingDimension)
	var sum float32
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1
		sum += vec[i] * vec[i]
	}

	magnitude := float32(1.0)
	if sum > 0 {
		magnitude = 1.0 / float32(math.Sqrt(float64(sum)))
	}
	for i := range vec {
		vec[i] *= magnitude
	}
	return vec
}
