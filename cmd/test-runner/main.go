package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aleksaelezovic/trigo/internal/testsuite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test-runner <manifest-file-or-directory>")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  test-runner testdata/rdf-tests/sparql/sparql11/syntax-query/manifest.ttl")
		fmt.Println("  test-runner testdata/rdf-tests/sparql/sparql11/syntax-query")
		os.Exit(1)
	}

	path := os.Args[1]

	// Create temporary test database
	dbPath := "./test_db_temp"
	defer os.RemoveAll(dbPath)

	runner, err := testsuite.NewTestRunner(dbPath)
	if err != nil {
		log.Fatalf("Failed to create test runner: %v", err)
	}
	defer runner.Close()

	// Check if path is a directory or file
	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("Failed to access path: %v", err)
	}

	if info.IsDir() {
		// Run all manifest files in directory
		manifestPath := filepath.Join(path, "manifest.ttl")
		if _, err := os.Stat(manifestPath); err == nil {
			if err := runner.RunManifest(manifestPath); err != nil {
				log.Fatalf("Failed to run manifest: %v", err)
			}
		} else {
			log.Fatalf("No manifest.ttl found in directory: %s", path)
		}
	} else {
		// Run single manifest file
		if err := runner.RunManifest(path); err != nil {
			log.Fatalf("Failed to run manifest: %v", err)
		}
	}

	// Exit with appropriate code
	stats := runner.GetStats()
	if stats.Failed > 0 {
		os.Exit(1)
	}
}
