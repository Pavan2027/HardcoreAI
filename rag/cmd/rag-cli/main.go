package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"hardcoreai-rag/evaluation"
	"hardcoreai-rag/indexing"
	"hardcoreai-rag/ingestion"
	"hardcoreai-rag/retrieval"
	"hardcoreai-rag/storage"
)

func main() {

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "query":
		runQueryCommand()
	case "eval":
		runEvalCommand()
	case "ingest":
		runIngestCommand()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("❌ Unknown command: %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🛠️  STM32 RAG — Local Engineering Retrieval CLI")
	fmt.Println("==================================================")
	fmt.Println("Usage:")
	fmt.Println("  rag-cli query  --db <path> --query \"<text>\" [options]")
	fmt.Println("  rag-cli eval   --db <path> --queries <json-path> [options]")
	fmt.Println("  rag-cli ingest --db <path> --dir <dir-path> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  query        Run hybrid RAG search and build LLM context")
	fmt.Println("  eval         Run retrieval quality benchmark & calculate metrics")
	fmt.Println("  ingest       Scan directory for PDFs and index them locally")
	fmt.Println("\nUse 'rag-cli <command> --help' for details on a specific command.")
}

func runQueryCommand() {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	dbPath := fs.String("db", "", "Path to SQLite database file (required)")
	queryText := fs.String("query", "", "The search query (required)")
	chipFamily := fs.String("chip-family", "", "Optional chip family filter (e.g. STM32F4)")
	cgoPath := fs.String("cgo-path", "", "Path to vec0 shared library (defaults to env VEC0_PATH or bin/vec0)")
	k := fs.Int("k", 3, "Maximum number of results to fetch")
	maxTokens := fs.Int("max-tokens", 3000, "Token budget for context window")

	_ = fs.Parse(os.Args[2:])

	if *dbPath == "" || *queryText == "" {
		fmt.Println("❌ Error: --db and --query are required flags.")
		fs.Usage()
		os.Exit(1)
	}

	// Resolve cgo shared library path
	vecLib := resolveVecPath(*cgoPath)

	// Open DB
	db, err := storage.Open(*dbPath, vecLib)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup deterministic offline local embedder
	embedder := indexing.NewEmbedder()
	engine := retrieval.NewEngine(db, embedder)

	opts := retrieval.RetrievalOptions{
		K:          *k,
		ChipFamily: *chipFamily,
		MaxTokens:  *maxTokens,
	}

	fmt.Printf("\n🔎 Running Hybrid Search + Reranking for: %q\n", *queryText)
	fmt.Printf("📋 Options: ChipFamily=%s, K=%d, MaxTokens=%d\n\n", *chipFamily, *k, *maxTokens)

	res, err := engine.Retrieve(context.Background(), *queryText, opts)
	if err != nil {
		log.Fatalf("❌ Retrieve failed: %v", err)
	}

	if len(res.Chunks) == 0 {
		fmt.Println("⚠️  No chunks retrieved matching criteria.")
		return
	}

	// Print ranked result list with scores
	fmt.Println("📊 === RANKED SEARCH RESULTS ===")
	for i, chunk := range res.Chunks {
		fmt.Printf("Rank %d [Chunk ID: %d] (Final Score: %.4f)\n", i+1, chunk.ChunkID, chunk.FinalScore)
		fmt.Printf("  • File:       %s (Page %d)\n", chunk.Filename, chunk.PageNumber)
		fmt.Printf("  • Section:    %s\n", defaultStr(chunk.SectionTitle, "N/A"))
		fmt.Printf("  • Peripheral: %s\n", defaultStr(chunk.Peripheral, "N/A"))
		fmt.Printf("  • Register:   %s\n", defaultStr(chunk.RegisterName, "N/A"))
		fmt.Printf("  • Score Breakdown: Semantic=%.4f | FTS=%.4f | Boost=%.4f\n",
			chunk.SemanticScore, chunk.FTSScore, chunk.MetadataBoost)
		fmt.Printf("  • Text Snippet:\n    \"%s...\"\n\n", truncate(chunk.ChunkText, 120))
	}

	// Print context builder output
	fmt.Println("🤖 === LLM-READY PROMPT CONTEXT WINDOW ===")
	fmt.Printf("Used Chunks: %d | Trimmed Chunks: %d\n", res.ChunksUsed, res.ChunksDropped)
	fmt.Println("----------------------------------------------------------------------")
	fmt.Println(res.Context)
	fmt.Println("----------------------------------------------------------------------")
}

func runEvalCommand() {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	dbPath := fs.String("db", "", "Path to SQLite database file (required)")
	queriesPath := fs.String("queries", "", "Path to test queries JSON file (required)")
	cgoPath := fs.String("cgo-path", "", "Path to vec0 shared library (defaults to env VEC0_PATH or bin/vec0)")
	k := fs.Int("k", 3, "Evaluation rank limit (Hit@K, Precision@K)")

	_ = fs.Parse(os.Args[2:])

	if *dbPath == "" || *queriesPath == "" {
		fmt.Println("❌ Error: --db and --queries are required flags.")
		fs.Usage()
		os.Exit(1)
	}

	// Load expected queries from JSON
	fileData, err := os.ReadFile(*queriesPath)
	if err != nil {
		log.Fatalf("❌ Failed to read expected queries file: %v", err)
	}

	var evalQueries []evaluation.EvalQuery
	if err := json.Unmarshal(fileData, &evalQueries); err != nil {
		log.Fatalf("❌ Failed to parse test queries JSON: %v", err)
	}

	// Resolve cgo shared library path
	vecLib := resolveVecPath(*cgoPath)

	// Open DB
	db, err := storage.Open(*dbPath, vecLib)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup deterministic offline local embedder
	embedder := indexing.NewEmbedder()
	engine := retrieval.NewEngine(db, embedder)

	fmt.Printf("\n🚀 Running Benchmark Evaluation Suite (%d Queries, K=%d)...\n\n", len(evalQueries), *k)

	report, err := evaluation.RunBenchmark(context.Background(), engine, evalQueries, *k)
	if err != nil {
		log.Fatalf("❌ Benchmark failed: %v", err)
	}

	// Output results in a beautiful table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "QUERY\tHIT@K\tFIRST HIT RANK\tRECIPROCAL RANK\tPRECISION@K")
	fmt.Fprintln(w, "-----\t-----\t--------------\t---------------\t-----------")

	for _, qr := range report.Queries {
		hitStr := "❌ No"
		if qr.HitAtK {
			hitStr = "✓ Yes"
		}

		rankStr := "N/A"
		if qr.FirstHitRank > 0 {
			rankStr = fmt.Sprintf("%d", qr.FirstHitRank)
		}

		// Truncate long query text for display
		qDisp := truncate(qr.Query, 32)

		fmt.Fprintf(w, "%s\t%s\t%s\t%.4f\t%.4f\n", qDisp, hitStr, rankStr, qr.ReciprocalRank, qr.PrecisionAtK)
	}
	w.Flush()

	// Print aggregated evaluation stats
	fmt.Println("\n📊 === BENCHMARK SUITE SUMMARY ===")
	fmt.Printf("  • Total Queries Evaluated : %d\n", report.TotalQueries)
	fmt.Printf("  • Average Hit@%d           : %.2f%%\n", *k, report.AverageHitAtK*100)
	fmt.Printf("  • Mean Reciprocal Rank MRR: %.4f\n", report.MRR)
	fmt.Printf("  • Mean Precision@%d        : %.4f\n", *k, report.MeanPrecision)
	fmt.Println("==================================================")
}

func resolveVecPath(cgoPath string) string {
	if cgoPath != "" {
		return cgoPath
	}
	if envPath := os.Getenv("VEC0_PATH"); envPath != "" {
		return envPath
	}
	return "bin/vec0"
}

func defaultStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

func runIngestCommand() {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	dbPath := fs.String("db", "data/rag.db", "Path to SQLite database")
	dirPath := fs.String("dir", "data", "Directory containing STM32 PDFs to ingest")

	_ = fs.Parse(os.Args[2:])

	if *dbPath == "" || *dirPath == "" {
		fmt.Println("Error: --db and --dir are required flags.")
		fs.Usage()
		os.Exit(1)
	}

	// 1. Scan directory for PDF files
	files, err := os.ReadDir(*dirPath)
	if err != nil {
		log.Fatalf("Failed to read directory %s: %v", *dirPath, err)
	}

	var pdfFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".pdf") {
			pdfFiles = append(pdfFiles, filepath.Join(*dirPath, f.Name()))
		}
	}

	if len(pdfFiles) == 0 {
		fmt.Printf("No PDF files found in directory: %s\n", *dirPath)
		return
	}

	fmt.Printf("Starting Ingestion Pipeline for %d files in %s...\n", len(pdfFiles), *dirPath)

	// Open DB connection
	db, err := storage.NewDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Setup indexer with deterministic local embedder
	fmt.Println("Using deterministic local offline embedder")
	embedder := indexing.NewEmbedder()
	indexer, err := indexing.NewIndexer(*dbPath, embedder)
	if err != nil {
		log.Fatalf("Failed to initialize indexer: %v", err)
	}
	defer indexer.Close()

	parser := ingestion.NewPDFParser()
	chunker := ingestion.NewChunker()

	for _, localPath := range pdfFiles {
		filename := filepath.Base(localPath)
		fmt.Printf("\n=============================\n")
		fmt.Printf("Processing: %s\n", filename)
		fmt.Printf("=============================\n")

		// Dynamic metadata inference
		docType := "document"
		lowerName := strings.ToLower(filename)
		if strings.Contains(lowerName, "reference_manual") || strings.Contains(lowerName, "rm") {
			docType = "reference_manual"
		} else if strings.Contains(lowerName, "datasheet") || strings.Contains(lowerName, "ds") {
			docType = "datasheet"
		} else if strings.Contains(lowerName, "programming_manual") || strings.Contains(lowerName, "pm") {
			docType = "programming_manual"
		}

		chipFamily := "STM32"
		if strings.Contains(lowerName, "stm32f4") {
			chipFamily = "STM32F4"
		} else if strings.Contains(lowerName, "stm32f7") {
			chipFamily = "STM32F7"
		} else if strings.Contains(lowerName, "stm32h7") {
			chipFamily = "STM32H7"
		}

		chipModel := chipFamily
		// Simple chip model extraction if possible (e.g. stm32f407)
		words := strings.FieldsFunc(lowerName, func(r rune) bool {
			return r == '_' || r == '-' || r == '.' || r == ' '
		})
		for _, w := range words {
			if strings.HasPrefix(w, "stm32") && len(w) > 7 {
				chipModel = strings.ToUpper(w)
				break
			}
		}

		version := "v1.0"
		// Try to extract document number e.g. RM0090
		for _, w := range words {
			if (strings.HasPrefix(w, "rm") || strings.HasPrefix(w, "pm")) && len(w) == 6 {
				version = strings.ToUpper(w)
				break
			}
		}

		// Parse PDF
		doc, err := parser.ParsePDF(localPath)
		if err != nil {
			fmt.Printf("Failed to parse: %v\n", err)
			continue
		}

		// Insert document record
		docID, err := db.InsertDocument(storage.Document{
			MongoID:    "local_" + filename,
			Filename:   filename,
			LocalPath:  localPath,
			DocType:    docType,
			ChipFamily: chipFamily,
			ChipModel:  chipModel,
			Version:    version,
		})
		if err != nil {
			fmt.Printf("Failed to insert document: %v\n", err)
			continue
		}

		// Chunk
		ingestionChunks := chunker.ChunkDocument(doc)

		// Convert to storage chunks
		storageChunks := make([]storage.Chunk, len(ingestionChunks))
		for i, c := range ingestionChunks {
			storageChunks[i] = storage.Chunk{
				DocumentID:   int(docID),
				ChunkText:    c.ChunkText,
				SectionTitle: c.SectionTitle,
				Peripheral:   c.Peripheral,
				RegisterName: c.RegisterName,
				PageNumber:   c.PageNumber,
				TokenCount:   c.TokenCount,
				ChunkIndex:   c.ChunkIndex,
			}
		}

		// Insert chunks and get their IDs back
		chunkIDs, err := db.InsertChunksAndReturnIDs(storageChunks)
		if err != nil {
			db.UpdateDocumentStatus(docID, "failed", err.Error())
			fmt.Printf("Failed to insert chunks: %v\n", err)
			continue
		}

		// Extract texts for embedding
		chunkTexts := make([]string, len(ingestionChunks))
		for i, c := range ingestionChunks {
			chunkTexts[i] = c.ChunkText
		}

		// Generate and store embeddings
		if err := indexer.IndexChunks(chunkIDs, chunkTexts); err != nil {
			db.UpdateDocumentStatus(docID, "failed", err.Error())
			fmt.Printf("Failed to index embeddings: %v\n", err)
			continue
		}

		db.UpdateDocumentStatus(docID, "indexed", "")
		fmt.Printf("Successfully indexed: %s\n", filename)
	}

	fmt.Printf("\n=============================\n")
	fmt.Printf("INGESTION COMPLETE\n")
	fmt.Printf("=============================\n")
}
