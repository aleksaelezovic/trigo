package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aleksaelezovic/trigo/internal/server"
	"github.com/aleksaelezovic/trigo/internal/sparql/executor"
	"github.com/aleksaelezovic/trigo/internal/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/internal/sparql/parser"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/internal/store"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: trigo <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  demo         - Run a demo with sample data")
		fmt.Println("  query <q>    - Execute a SPARQL query")
		fmt.Println("  serve [addr] - Start HTTP SPARQL endpoint (default: localhost:8080)")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "demo":
		runDemo()
	case "query":
		if len(os.Args) < 3 {
			fmt.Println("Usage: trigo query <sparql-query>")
			os.Exit(1)
		}
		runQuery(os.Args[2])
	case "serve":
		addr := "localhost:8080"
		if len(os.Args) >= 3 {
			addr = os.Args[2]
		}
		runServer(addr)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func runDemo() {
	fmt.Println("=== Trigo RDF Triplestore Demo ===\n")

	// Create storage
	dbPath := "./trigo_data"
	fmt.Printf("Opening database at: %s\n", dbPath)

	badgerStorage, err := storage.NewBadgerStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer badgerStorage.Close()

	// Create triplestore
	tripleStore := store.NewTripleStore(badgerStorage)
	fmt.Println("Triplestore initialized\n")

	// Insert sample data
	fmt.Println("Inserting sample data...")

	// Create some example triples
	alice := rdf.NewNamedNode("http://example.org/alice")
	bob := rdf.NewNamedNode("http://example.org/bob")
	carol := rdf.NewNamedNode("http://example.org/carol")

	knows := rdf.NewNamedNode("http://xmlns.com/foaf/0.1/knows")
	name := rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name")
	age := rdf.NewNamedNode("http://xmlns.com/foaf/0.1/age")

	// Insert triples
	triples := []*rdf.Triple{
		rdf.NewTriple(alice, name, rdf.NewLiteral("Alice")),
		rdf.NewTriple(alice, age, rdf.NewIntegerLiteral(30)),
		rdf.NewTriple(alice, knows, bob),

		rdf.NewTriple(bob, name, rdf.NewLiteral("Bob")),
		rdf.NewTriple(bob, age, rdf.NewIntegerLiteral(25)),
		rdf.NewTriple(bob, knows, carol),

		rdf.NewTriple(carol, name, rdf.NewLiteral("Carol")),
		rdf.NewTriple(carol, age, rdf.NewIntegerLiteral(28)),
	}

	for _, triple := range triples {
		if err := tripleStore.InsertTriple(triple); err != nil {
			log.Fatalf("Failed to insert triple: %v", err)
		}
		fmt.Printf("  ✓ %s\n", triple)
	}

	// Count triples
	count, err := tripleStore.Count()
	if err != nil {
		log.Fatalf("Failed to count triples: %v", err)
	}
	fmt.Printf("\nTotal triples stored: %d\n", count)

	// Query example
	fmt.Println("\n=== Querying Data ===\n")

	sparqlQuery := `
		SELECT ?person ?name ?age
		WHERE {
			?person <http://xmlns.com/foaf/0.1/name> ?name .
			?person <http://xmlns.com/foaf/0.1/age> ?age .
		}
	`

	fmt.Printf("Query:\n%s\n", sparqlQuery)

	// Parse query
	p := parser.NewParser(sparqlQuery)
	query, err := p.Parse()
	if err != nil {
		log.Fatalf("Failed to parse query: %v", err)
	}
	fmt.Println("✓ Query parsed successfully")

	// Optimize query
	stats := &optimizer.Statistics{TotalTriples: count}
	opt := optimizer.NewOptimizer(stats)
	optimizedQuery, err := opt.Optimize(query)
	if err != nil {
		log.Fatalf("Failed to optimize query: %v", err)
	}
	fmt.Println("✓ Query optimized successfully")

	// Execute query
	exec := executor.NewExecutor(tripleStore)
	result, err := exec.Execute(optimizedQuery)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	fmt.Println("✓ Query executed successfully\n")

	// Display results
	fmt.Println("Results:")
	if selectResult, ok := result.(*executor.SelectResult); ok {
		// Print header
		fmt.Print("| ")
		if selectResult.Variables != nil {
			for _, v := range selectResult.Variables {
				fmt.Printf("%-20s | ", v.Name)
			}
		}
		fmt.Println()
		fmt.Println("|" + "----------------------|" + "----------------------|" + "----------------------|")

		// Print rows
		for _, binding := range selectResult.Bindings {
			fmt.Print("| ")
			if selectResult.Variables != nil {
				for _, v := range selectResult.Variables {
					if term, exists := binding.Vars[v.Name]; exists {
						fmt.Printf("%-20s | ", formatTerm(term))
					} else {
						fmt.Printf("%-20s | ", "")
					}
				}
			}
			fmt.Println()
		}

		fmt.Printf("\nFound %d results\n", len(selectResult.Bindings))
	}

	fmt.Println("\n=== Demo Complete ===")
}

func runQuery(sparqlQuery string) {
	// Open existing database
	dbPath := "./trigo_data"
	badgerStorage, err := storage.NewBadgerStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to open storage: %v", err)
	}
	defer badgerStorage.Close()

	tripleStore := store.NewTripleStore(badgerStorage)

	// Parse query
	p := parser.NewParser(sparqlQuery)
	query, err := p.Parse()
	if err != nil {
		log.Fatalf("Failed to parse query: %v", err)
	}

	// Get statistics
	count, _ := tripleStore.Count()
	stats := &optimizer.Statistics{TotalTriples: count}

	// Optimize query
	opt := optimizer.NewOptimizer(stats)
	optimizedQuery, err := opt.Optimize(query)
	if err != nil {
		log.Fatalf("Failed to optimize query: %v", err)
	}

	// Execute query
	exec := executor.NewExecutor(tripleStore)
	result, err := exec.Execute(optimizedQuery)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}

	// Display results
	if selectResult, ok := result.(*executor.SelectResult); ok {
		fmt.Println("Results:")
		for _, binding := range selectResult.Bindings {
			for varName, term := range binding.Vars {
				fmt.Printf("  %s = %s\n", varName, formatTerm(term))
			}
			fmt.Println()
		}
	} else if askResult, ok := result.(*executor.AskResult); ok {
		fmt.Printf("Result: %t\n", askResult.Result)
	}
}

func runServer(addr string) {
	// Open existing database or create new one
	dbPath := "./trigo_data"
	fmt.Printf("Opening database at: %s\n", dbPath)

	badgerStorage, err := storage.NewBadgerStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to open storage: %v", err)
	}
	defer badgerStorage.Close()

	tripleStore := store.NewTripleStore(badgerStorage)

	// Get current count
	count, _ := tripleStore.Count()
	fmt.Printf("Database loaded with %d triples\n", count)

	// Create and start server
	srv := server.NewServer(tripleStore, addr)
	fmt.Printf("\n🚀 Trigo SPARQL endpoint starting...\n")
	fmt.Printf("   Endpoint: http://%s/sparql\n", addr)
	fmt.Printf("   Web UI:   http://%s/\n\n", addr)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func formatTerm(term rdf.Term) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		// Return just the local name if possible
		iri := t.IRI
		if idx := len(iri) - 1; idx >= 0 {
			for i := idx; i >= 0; i-- {
				if iri[i] == '/' || iri[i] == '#' {
					return iri[i+1:]
				}
			}
		}
		return iri
	case *rdf.Literal:
		return t.Value
	default:
		return term.String()
	}
}
