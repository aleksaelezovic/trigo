# Quick Start Guide

## Building Trigo

```bash
# Clone or navigate to the repository
cd /path/to/trigo

# Build the CLI application
go build -o trigo ./cmd/trigo

# Verify the build
./trigo
```

## Running the Demo

The demo creates a simple knowledge graph about people and their relationships:

```bash
./trigo demo
```

This will:
1. Create a BadgerDB database in `./trigo_data`
2. Insert sample triples about Alice, Bob, and Carol
3. Execute a SPARQL SELECT query
4. Display the results

## Project Structure

```
trigo/
├── cmd/
│   └── trigo/
│       └── main.go              # CLI application entry point
├── internal/
│   ├── encoding/
│   │   ├── encoder.go           # xxHash3 term encoding
│   │   └── decoder.go           # Term decoding
│   ├── storage/
│   │   ├── storage.go           # Storage interface
│   │   └── badger.go            # BadgerDB implementation
│   ├── store/
│   │   ├── store.go             # Triplestore with 11 indexes
│   │   └── query.go             # Pattern matching queries
│   └── sparql/
│       ├── parser/
│       │   ├── ast.go           # Abstract Syntax Tree
│       │   └── parser.go        # SPARQL parser
│       ├── optimizer/
│       │   └── optimizer.go     # Query optimizer
│       └── executor/
│           └── executor.go      # Volcano iterator execution
├── pkg/
│   └── rdf/
│       └── term.go              # RDF data model
├── README.md                    # Main documentation
├── ARCHITECTURE.md              # Detailed architecture
└── QUICKSTART.md               # This file
```

## Using Trigo as a Library

### Basic Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/aleksaelezovic/trigo/internal/storage"
    "github.com/aleksaelezovic/trigo/internal/store"
    "github.com/aleksaelezovic/trigo/pkg/rdf"
)

func main() {
    // Create storage
    storage, err := storage.NewBadgerStorage("./my_data")
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Close()

    // Create triplestore
    ts := store.NewTripleStore(storage)

    // Insert some triples
    alice := rdf.NewNamedNode("http://example.org/alice")
    name := rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name")

    triple := rdf.NewTriple(alice, name, rdf.NewLiteral("Alice"))

    if err := ts.InsertTriple(triple); err != nil {
        log.Fatal(err)
    }

    // Query data
    pattern := &store.Pattern{
        Subject:   store.NewVariable("s"),
        Predicate: store.NewVariable("p"),
        Object:    store.NewVariable("o"),
    }

    iter, err := ts.Query(pattern)
    if err != nil {
        log.Fatal(err)
    }
    defer iter.Close()

    // Iterate results
    for iter.Next() {
        quad, err := iter.Quad()
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println(quad)
    }
}
```

### SPARQL Query Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/aleksaelezovic/trigo/internal/sparql/executor"
    "github.com/aleksaelezovic/trigo/internal/sparql/optimizer"
    "github.com/aleksaelezovic/trigo/internal/sparql/parser"
    "github.com/aleksaelezovic/trigo/internal/storage"
    "github.com/aleksaelezovic/trigo/internal/store"
)

func main() {
    // Setup (storage, store)
    storage, _ := storage.NewBadgerStorage("./my_data")
    defer storage.Close()
    ts := store.NewTripleStore(storage)

    // Parse SPARQL query
    query := `
        SELECT ?person ?name
        WHERE {
            ?person <http://xmlns.com/foaf/0.1/name> ?name .
        }
    `

    p := parser.NewParser(query)
    ast, err := p.Parse()
    if err != nil {
        log.Fatal(err)
    }

    // Optimize
    count, _ := ts.Count()
    stats := &optimizer.Statistics{TotalTriples: count}
    opt := optimizer.NewOptimizer(stats)
    optimized, err := opt.Optimize(ast)
    if err != nil {
        log.Fatal(err)
    }

    // Execute
    exec := executor.NewExecutor(ts)
    result, err := exec.Execute(optimized)
    if err != nil {
        log.Fatal(err)
    }

    // Process results
    if selectResult, ok := result.(*executor.SelectResult); ok {
        for _, binding := range selectResult.Bindings {
            for varName, term := range binding.Vars {
                fmt.Printf("%s = %s\n", varName, term)
            }
        }
    }
}
```

## Sample SPARQL Queries

### Select All Triples

```sparql
SELECT ?s ?p ?o
WHERE {
    ?s ?p ?o .
}
```

### Find People and Their Names

```sparql
SELECT ?person ?name
WHERE {
    ?person <http://xmlns.com/foaf/0.1/name> ?name .
}
```

### Find People Alice Knows

```sparql
SELECT ?friend ?friendName
WHERE {
    <http://example.org/alice> <http://xmlns.com/foaf/0.1/knows> ?friend .
    ?friend <http://xmlns.com/foaf/0.1/name> ?friendName .
}
```

### Ask if Bob Knows Anyone

```sparql
ASK {
    <http://example.org/bob> <http://xmlns.com/foaf/0.1/knows> ?someone .
}
```

### Select with LIMIT and DISTINCT

```sparql
SELECT DISTINCT ?person
WHERE {
    ?person <http://xmlns.com/foaf/0.1/name> ?name .
}
LIMIT 5
```

## Working with Different RDF Terms

### Named Nodes (IRIs)

```go
node := rdf.NewNamedNode("http://example.org/resource")
```

### Blank Nodes

```go
blank := rdf.NewBlankNode("b1")
```

### Literals

```go
// Simple literal
lit1 := rdf.NewLiteral("Hello World")

// Language-tagged literal
lit2 := rdf.NewLiteralWithLanguage("Bonjour", "fr")

// Typed literal
lit3 := rdf.NewIntegerLiteral(42)
lit4 := rdf.NewDoubleLiteral(3.14)
lit5 := rdf.NewBooleanLiteral(true)
lit6 := rdf.NewDateTimeLiteral(time.Now())
```

### Triples and Quads

```go
// Triple (stored in default graph)
triple := rdf.NewTriple(subject, predicate, object)
ts.InsertTriple(triple)

// Quad (with explicit graph)
graph := rdf.NewNamedNode("http://example.org/graph1")
quad := rdf.NewQuad(subject, predicate, object, graph)
ts.InsertQuad(quad)
```

## Performance Tips

1. **Batch Inserts**: Use write transactions to insert multiple triples at once
2. **Index Selection**: Bind as many terms as possible in patterns for efficient index usage
3. **Query Ordering**: More selective patterns should appear first in WHERE clauses
4. **Memory**: BadgerDB caches data in memory; adjust cache size based on dataset

## Troubleshooting

### Database Already Exists

If you get "database already exists" error:

```bash
rm -rf ./trigo_data
./trigo demo
```

### Build Errors

Ensure you have Go 1.21+ installed:

```bash
go version
```

Update dependencies:

```bash
go mod tidy
```

### Query Not Returning Expected Results

- Check that data was inserted correctly
- Verify IRI spelling (case-sensitive)
- Use `SELECT * WHERE { ?s ?p ?o }` to see all triples
- Check that the graph context matches (default vs named graphs)

## Next Steps

1. Read [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation
2. Read [README.md](README.md) for feature overview and roadmap
3. Explore the source code in `internal/` and `pkg/`
4. Try implementing support for additional SPARQL features
5. Run the W3C SPARQL test suite (coming soon)

## Getting Help

- Check existing documentation in this repository
- Review the source code (it's heavily commented)
- Compare with Oxigraph architecture: https://github.com/oxigraph/oxigraph/wiki/Architecture
- Refer to SPARQL 1.1 specification: https://www.w3.org/TR/sparql11-query/
