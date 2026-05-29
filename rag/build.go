package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("🚀 Starting OS-independent build of rag-cli...")

	// 1. Force CGO enabled and set the FTS5 compiler flag
	os.Setenv("CGO_ENABLED", "1")
	os.Setenv("CGO_CFLAGS", "-DSQLITE_ENABLE_FTS5")

	// 2. Prepare the build command
	// We build it into the integration folder as rag-cli.exe (or rag-cli on Linux/Mac)
	outPath := filepath.Join("integration", "rag-cli.exe")
	if os.PathSeparator == '/' {
		outPath = filepath.Join("integration", "rag-cli")
	}

	cmd := exec.Command("go", "build", "-tags", "sqlite_fts5", "-o", outPath, filepath.Join("cmd", "rag-cli", "main.go"))
	
	// Stream output to standard out
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 3. Execute
	fmt.Printf("🏃 Running: %s\n", cmd.String())
	err := cmd.Run()
	if err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Build successful! Binary saved to: %s\n", outPath)
}
