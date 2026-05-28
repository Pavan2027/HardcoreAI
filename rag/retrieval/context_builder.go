// Package retrieval contains modules for hybrid search, reranking, filtering, and context building.
package retrieval

import (
	"fmt"
	"strings"

	"hardcoreai-rag/storage"
	"hardcoreai-rag/utils"
)

// ContextBuilderResult holds the output of the BuildContext function.
type ContextBuilderResult struct {
	Context       string
	ChunksUsed    int
	ChunksDropped int
}

// BuildContext converts the final reranked chunk list into a structured, LLM-ready context string.
// It respects the token budget, dropping lower-scoring chunks when the limit is exceeded.
func BuildContext(chunks []storage.SearchResult, maxTokens int) (ContextBuilderResult, error) {
	if maxTokens <= 0 {
		maxTokens = 3000 // default budget from handoff
	}

	var usedChunks []storage.SearchResult
	var droppedCount int
	currentTokenCount := 0

	// 1. Build context chunks list under token budget
	for i, chunk := range chunks {
		formatted := formatChunk(chunk)
		chunkTokens := utils.CountTokens(formatted)

		// Check if adding this chunk exceeds the token budget
		if currentTokenCount+chunkTokens > maxTokens {
			droppedCount = len(chunks) - i
			break
		}

		usedChunks = append(usedChunks, chunk)
		currentTokenCount += chunkTokens
	}

	// 2. Assemble the final context string from the selected chunks
	var sb strings.Builder
	for i, chunk := range usedChunks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatChunk(chunk))
	}

	return ContextBuilderResult{
		Context:       sb.String(),
		ChunksUsed:    len(usedChunks),
		ChunksDropped: droppedCount,
	}, nil
}

// formatChunk returns the standard formatted representation of a chunk for LLM context.
func formatChunk(chunk storage.SearchResult) string {
	section := chunk.SectionTitle
	if section == "" {
		section = "N/A"
	}
	register := chunk.RegisterName
	if register == "" {
		register = "N/A"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Source: %s | doc_type: %s]\n", chunk.Filename, chunk.DocType))
	sb.WriteString(fmt.Sprintf("Section: %s\n", section))
	sb.WriteString(fmt.Sprintf("Register: %s | Page: %d\n\n", register, chunk.PageNumber))
	sb.WriteString(chunk.ChunkText)
	sb.WriteString("\n\n---\n")
	return sb.String()
}
