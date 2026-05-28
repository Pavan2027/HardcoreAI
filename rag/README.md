# HardcoreAI-RAG

> A local-first, domain-aware retrieval engine for STM32 embedded systems documentation. Combines in-memory vector search with BM25 full-text retrieval to surface the most useful engineering context for an LLM — not just the most semantically similar text.

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)
![SQLite](https://img.shields.io/badge/SQLite-FTS5-003B57?style=flat-square&logo=sqlite)
![Embeddings](https://img.shields.io/badge/Embeddings-Offline_Deterministic-00ADD8?style=flat-square&logo=go)
![License](https://img.shields.io/badge/License-Proprietary-red?style=flat-square)

---

## Overview

HardcoreAI-RAG is a **single-binary, privacy-preserving retrieval engine** purpose-built for STM32 reference manuals, datasheets, and fault documentation. It ingests PDFs, chunks them into engineering-aware segments, and indexes them locally in SQLite. At query time it runs **hybrid retrieval** — cosine similarity search combined with BM25 keyword search — merges the results using Reciprocal Rank Fusion, then applies a **metadata-aware reranker** that understands the difference between a register match and a section title match before assembling a token-budgeted context string ready for any LLM.

This is not a generic chatbot backend. The entire pipeline is tuned for embedded systems documentation, where register names, acronyms, and hex values matter as much as semantic meaning.

---

## Features

### Core Modules

| Module | Description |
|---|---|
| **Ingestion Pipeline** | Dynamically scans the production `data/` directory for PDFs, parses them, splits them into semantically coherent chunks, and automatically infers metadata (type, family, model, version). |
| **Offline Embedding Engine** | Generates deterministic 768-dimensional embeddings locally — no external API dependency at index or query time. |
| **Hybrid Retrieval** | Runs in-Go cosine similarity search and SQLite FTS5 BM25 keyword search in parallel, then merges results using Reciprocal Rank Fusion (RRF, k=60). |
| **Metadata Reranker** | Applies weighted additive scoring that boosts exact register matches (+0.30), peripheral matches (+0.20), section title overlap (+0.15), reference manual preference (+0.10), and chip family alignment (+0.05). |
| **Token-Budgeted Context Builder** | Assembles a structured, traceable LLM-ready context string. Uses `cl100k_base` (tiktoken) to count tokens accurately and drops lowest-scoring chunks when the budget is exceeded. |
| **Evaluation Framework** | Benchmarks retrieval quality against a JSON test suite. Reports Hit@K, Mean Reciprocal Rank (MRR), and Precision@K per query and in aggregate. |
| **CLI** | Unified subcommands: `rag-cli ingest`, `rag-cli query`, and `rag-cli eval` for end-to-end local index management and search quality benchmarking. |

### Architecture Highlights

- **Single Binary** — The entire retrieval stack compiles to one Go binary with zero Python runtime and zero external services.
- **Local-First** — All embeddings, vectors, and FTS indexes live in a single `SQLite` file (`data/rag.db`) on disk. No cloud retrieval latency.
- **Domain-Aware Retrieval** — FTS5 is not an afterthought. STM32 docs contain register names (`USART_BRR`), macros, and hex addresses that embedding models alone handle poorly. Hybrid retrieval is load-bearing.
- **Reranking Over Raw Similarity** — The reranker is where relevance is determined. Semantic similarity is a starting signal, not the answer.
- **Evaluation-First** — `evaluation/test_queries.json` defines expected peripheral, register, and section matches. Run `rag-cli eval` to catch retrieval regressions before they reach the LLM.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.24+ |
| Database | SQLite (`mattn/go-sqlite3`) |
| Full-Text Search | SQLite FTS5 (BM25 ranking) |
| Vector Search | In-Go cosine similarity over BLOB embeddings |
| Embeddings | Deterministic offline hashing (FNV-1a + math/rand, 768-dim, no API dependency) |
| Token Counting | `pkoukk/tiktoken-go` (`cl100k_base`, GPT-4 compatible) |
| PDF Parsing | `ledongthuc/pdf` |
| CLI | Standard library `flag` |

---

## Project Structure

```
hardcoreai-rag/
├── cmd/
│   └── rag-cli/
│       └── main.go             ← Unified CLI entry point (ingest + query + eval commands)
│
├── data/                       ← Production manuals corpus and SQLite DB (local only)
│   ├── rag.db                  ← Generated local-first search database
│   └── *.pdf                   ← Ingested STM32 manuals & datasheets
│
├── evaluation/                 ← Benchmark framework
│   ├── evaluator.go            ← RunBenchmark: Hit@K, MRR, Precision@K
│   ├── metrics.go              ← IsMatch logic, EvalQuery / BenchmarkReport types
│   └── test_queries.json       ← Ground-truth query → expected result mapping
│
├── indexing/                   ← Embedding generation and DB write-back
│   ├── embedder.go             ← Offline deterministic embedding engine
│   ├── indexer.go              ← Writes embedding BLOBs to chunks.embedding
│   └── indexing_types.go
│
├── ingestion/                  ← PDF parsing and chunking
│   ├── pdf_parser.go           ← Extracts text + metadata from STM32 PDFs
│   ├── chunker.go              ← Splits into engineering-aware chunks
│   ├── downloader.go           ← Fetches PDFs from ST's public document server
│   └── ingestion_types.go
│
├── retrieval/                  ← Core retrieval pipeline (primary ownership)
│   ├── search.go               ← Engine: orchestrates all phases via Retrieve()
│   ├── hybrid_search.go        ← RRF merge of vector + FTS results
│   ├── reranker.go             ← Weighted additive metadata scoring
│   ├── filters.go              ← StorageOptions converter
│   ├── context_builder.go      ← Token-budgeted LLM context assembly
│   ├── retrieval_types.go      ← RetrievalOptions, RetrievalResult types
│   └── reranker_test.go        ← Unit tests for scoring and boost logic
│
├── scripts/
│   ├── download_stm_docs/
│   │   └── main.go             ← Bulk downloader for STM32 document corpus
│   └── verify_retrieval/
│       └── main.go             ← Smoke test: runs a live query against data/rag.db
│
├── storage/                    ← SQLite access layer (shared ownership)
│   ├── sqlite.go               ← DB open, schema apply, chunk insert helpers
│   ├── models.go               ← Document, Chunk, SearchResult, SearchOptions
│   ├── filters.go              ← BuildFilterSQL: SQL-level pre-filtering
│   ├── vector_search.go        ← Cosine similarity over chunks.embedding
│   ├── fts_search.go           ← BM25 FTS5 search with sanitization
│   ├── queries.go              ← Named query constants
│   ├── schema.sql              ← Source of truth for DB schema
│   ├── storage_test.go
│   ├── vector_search_test.go
│   ├── fts_search_test.go
│   └── filters_test.go
│
├── tests/                      ← Integration tests (cross-package)
│   ├── hybrid_search_test.go
│   ├── reranker_test.go
│   ├── context_builder_test.go
│   └── evaluator_test.go
│
├── utils/
│   └── tokenizer.go            ← cl100k_base token counter with offline fallback
│
├── go.mod
└── go.sum
```

---

## Getting Started

### Prerequisites

- Go 1.24+
- A populated `data/rag.db` (run the ingestion pipeline once first)

### 1. Download and index documents

```bash
# Download STM32 PDFs from ST's public server directly into data/
go run -tags "sqlite_fts5" ./scripts/download_stm_docs/main.go

# Run the full dynamic ingestion pipeline (scan data folder → parse → chunk → embed → index)
# This dynamically reads all PDFs in data/ and indexes them into data/rag.db
go run -tags "sqlite_fts5" ./cmd/rag-cli/main.go ingest
```

### 2. Verify retrieval is working

```bash
# Quick smoke test against the indexed production DB
go run -tags "sqlite_fts5" ./scripts/verify_retrieval/main.go
```

You should see ranked STM32 chunks with score breakdowns and a formatted LLM-ready context block.

---

## Usage

### Ingest mode

```bash
# Scans all PDFs in data/ and populates data/rag.db automatically
go run -tags "sqlite_fts5" ./cmd/rag-cli/main.go ingest
```

### Query mode

```bash
go run -tags "sqlite_fts5" ./cmd/rag-cli/main.go query \
  --db data/rag.db \
  --query "What causes a precise BusFault in STM32?" \
  --chip-family STM32F4 \
  --k 5 \
  --max-tokens 3000
```

**Example output:**

```
=== RANKED SEARCH RESULTS ===
Rank 1 [Chunk ID: 142] (Final Score: 0.8241)
  • File:       STM32F4_Reference_Manual_RM0090.pdf (Page 210)
  • Section:    Fault Status Registers
  • Peripheral: CoreDebug
  • Register:   CFSR
  • Scores:     Semantic=0.8810 | FTS=0.9100 | Boost=0.4500 | FINAL=0.8241

=== LLM-READY CONTEXT WINDOW ===
Used Chunks: 4 | Dropped Chunks: 1
[Source: STM32F4_Reference_Manual_RM0090.pdf | reference_manual]
Section: Fault Status Registers
Register: CFSR | Page: 210
...
```

### Evaluation / Benchmark mode

```bash
go run -tags "sqlite_fts5" ./cmd/rag-cli/main.go eval \
  --db data/rag.db \
  --queries evaluation/test_queries.json \
  --k 3
```

## HardcoreAI Integration

This repo now includes an [`integration/`](./integration/README.md) package for wiring `hardcoreai-rag` into another app through a small FastAPI layer.

Use it when you want:

- file upload handling outside the Go binary
- an HTTP endpoint that shells out to `rag-cli`
- a starter Svelte panel for attaching PDFs and requesting RAG context

The integration package is intentionally additive. It does not change the Go retrieval engine or its test surface.

**Example output:**

```
=== BENCHMARK SUITE SUMMARY ===
  • Total Queries Evaluated : 6
  • Average Hit@3           : 83.33%
  • Mean Reciprocal Rank MRR: 0.7778
  • Mean Precision@3        : 0.4444
```

---

## Running Tests

```bash
# Storage layer — vector search, FTS, filters
go test -tags "sqlite_fts5" ./storage/ -v

# Reranker unit tests (no DB needed)
go test -tags "sqlite_fts5" ./retrieval/ -run TestRerank -v

# Integration tests — hybrid search, context builder, evaluator
go test -tags "sqlite_fts5" ./tests/ -v

# Everything at once
go test -tags "sqlite_fts5" ./... -v
```

All tests use in-memory SQLite with synthetic seed data. No external services, API keys, or network access required.

---

## Retrieval Pipeline

```
User Query
    │
    ▼
Offline Deterministic Hashing Embedding (768-dim)
    │
    ├──────────────────────────────┐
    ▼                              ▼
Vector Search               FTS5 BM25 Search
(Cosine similarity          (Register names, macros,
 over BLOB embeddings)       acronyms, hex values)
    │                              │
    └──────────────┬───────────────┘
                   ▼
            RRF Merge (k=60)
      (Deduplication & rank fusion)
                   │
                   ▼
            Metadata Reranker
       (Weighted additive scoring)
       semantic×0.6 + fts×0.2 + boost×0.2
                   │
                   ▼
            Context Builder
       (Token budget, tiktoken)
                   │
                   ▼
        LLM-Ready Context String
```

## Reranker Boost Table

| Condition | Boost |
|---|---|
| All tokens of register name appear in query | +0.30 |
| Peripheral name token in query | +0.20 |
| Non-stop-word section title token in query | +0.15 |
| Document type is `reference_manual` | +0.10 |
| Chunk's chip family matches active filter | +0.05 |

Boosts are additive and capped at 1.0.

---

<p align="center">Built with Go + SQLite · 2026</p>
