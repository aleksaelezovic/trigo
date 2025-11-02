<div align="center">
  <img src="assets/trigo.svg" alt="Trigo Logo" width="200"/>

  # Trigo

  **A high-performance RDF triplestore and SPARQL 1.1 query engine written in Go**

  [![Go Report Card](https://goreportcard.com/badge/github.com/aleksaelezovic/trigo)](https://goreportcard.com/report/github.com/aleksaelezovic/trigo)
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
  [![Documentation](https://img.shields.io/badge/docs-github.io-blue)](https://aleksaelezovic.github.io/trigo/)

  [Documentation](https://aleksaelezovic.github.io/trigo/) | [Quick Start](https://aleksaelezovic.github.io/trigo/quickstart.html) | [Architecture](https://aleksaelezovic.github.io/trigo/architecture.html) | [HTTP API](https://aleksaelezovic.github.io/trigo/http-endpoint.html)
</div>

## Overview

Trigo is a modern RDF triplestore inspired by [Oxigraph](https://github.com/oxigraph/oxigraph), implementing efficient storage and querying of RDF data using SPARQL. Built in Go, it provides a simple, maintainable codebase with excellent performance characteristics.

## Key Features

- **Full SPARQL 1.1 Support** - SELECT, CONSTRUCT, ASK, DESCRIBE queries with advanced patterns (OPTIONAL, UNION, MINUS, GRAPH, BIND)
- **Multiple RDF Formats** - Turtle, N-Triples, N-Quads, TriG, RDF/XML, JSON-LD parsers
- **Efficient 11-Index Architecture** - BadgerDB backend with optimal index selection
- **HTTP SPARQL Endpoint** - W3C SPARQL 1.1 Protocol compliant with interactive web UI
- **Named Graphs Support** - Full quad store with graph-level operations
- **High Performance** - xxHash3 encoding, query optimization, lazy evaluation

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/aleksaelezovic/trigo.git
cd trigo

# Build the CLI
go build -o trigo ./cmd/trigo
```

### Start SPARQL Endpoint

```bash
# Start the server
./trigo serve

# Visit http://localhost:8080/ for the interactive web UI
```

### Query via HTTP

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+json' \
  -d 'SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10'
```

### Use as a Library

```go
import (
    "github.com/aleksaelezovic/trigo/pkg/store"
    "github.com/aleksaelezovic/trigo/pkg/rdf"
)

storage, _ := storage.NewBadgerStorage("./data")
ts := store.NewTripleStore(storage)

triple := rdf.NewTriple(
    rdf.NewNamedNode("http://example.org/alice"),
    rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
    rdf.NewLiteral("Alice"),
)
ts.InsertTriple(triple)
```

## Documentation

📚 **[Full Documentation](https://aleksaelezovic.github.io/trigo/)** - Complete guides and API reference

- **[Quick Start Guide](https://aleksaelezovic.github.io/trigo/quickstart.html)** - Get started in minutes
- **[Architecture](https://aleksaelezovic.github.io/trigo/architecture.html)** - Deep dive into design and implementation
- **[HTTP API Reference](https://aleksaelezovic.github.io/trigo/http-endpoint.html)** - REST API documentation
- **[Testing & Compliance](https://aleksaelezovic.github.io/trigo/testing.html)** - W3C test suite results

## Test Results

Validated against official W3C test suites:

- **RDF N-Triples:** 100% (70/70 tests) ✅
- **RDF N-Quads:** 100% (87/87 tests) ✅
- **RDF Turtle:** 62.2% (184/296 tests)
- **SPARQL Syntax:** 69.1% (65/94 tests)
- **SPARQL BIND:** 70.0% (7/10 tests)

## Project Structure

```
trigo/
├── cmd/           # CLI applications
├── internal/      # Internal packages (encoding, storage, testing)
├── pkg/           # Public API (rdf, store, sparql, server)
└── docs/          # Documentation site
```

See the [Architecture Guide](https://aleksaelezovic.github.io/trigo/architecture.html) for details.

## Contributing

Contributions are welcome! Please:
- Check existing issues or create a new one
- Follow the existing code style
- Run tests and quality checks before submitting
- Update documentation as needed

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details

## Acknowledgments

Inspired by [Oxigraph](https://github.com/oxigraph/oxigraph) architecture. Built with [BadgerDB](https://github.com/dgraph-io/badger) and [xxHash3](https://github.com/zeebo/xxh3).

---

<div align="center">
  <a href="https://aleksaelezovic.github.io/trigo/">Documentation</a> •
  <a href="https://github.com/aleksaelezovic/trigo/issues">Issues</a> •
  <a href="https://github.com/aleksaelezovic/trigo/blob/main/CLAUDE.md">AI Assistant Context</a>
</div>
