package retrieval

import "hardcoreai-rag/storage"

// StorageOptions converts a RetrievalOptions into a storage.SearchOptions
// ready to pass to VectorSearch or FTSSearch.
//
// k is passed separately because callers often inflate it for over-fetching
// (e.g. k*2 so RRF has a wider candidate pool before trimming to K).
func StorageOptions(opts RetrievalOptions, k int) storage.SearchOptions {
	return storage.SearchOptions{
		K:          k,
		ChipFamily: opts.ChipFamily,
		DocTypes:   opts.DocTypes,
	}
}