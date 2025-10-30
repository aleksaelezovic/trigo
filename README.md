# Trigo - RDF Triplestore in Go

Trigo is an RDF triplestore implementation in Go, inspired by [Oxigraph](https://github.com/oxigraph/oxigraph). It provides efficient storage and querying of RDF data using SPARQL.

## Features

- **Efficient Storage**: Uses BadgerDB (LSM-tree based) for persistent storage
- **11 Indexes**: Implements all 11 index permutations (SPO, POS, OSP, SPOG, POSG, OSPG, GSPO, GPOS, GOSP, plus id2str and graphs metadata)
- **xxHash3 128-bit**: Fast hashing using xxhash3 128-bit variant for string identifiers
- **SPARQL Support**: Parser for SPARQL SELECT and ASK queries
- **HTTP SPARQL Endpoint**: W3C SPARQL 1.1 Protocol compliant REST API
- **Query Optimization**: Greedy join reordering based on selectivity and filter push-down
- **Volcano Iterator Model**: Efficient query execution using the iterator model
- **RDF Data Types**: Support for IRIs, blank nodes, literals (strings, integers, doubles, booleans, dates)
- **Named Graphs**: Full support for quads and named graphs
- **Multiple Result Formats**: SPARQL JSON and XML results

## Architecture

Trigo follows Oxigraph's architecture principles:

### Storage Layer
- **BadgerDB**: LSM-tree based key-value store with ACID transactions
- **11 Indexes**: Multiple permutations of (Subject, Predicate, Object, Graph) for efficient query patterns
- **Term Encoding**: Each RDF term is encoded as type byte + 16 bytes (hash or inline data)
- **Big-Endian Keys**: All keys use big-endian encoding for correct lexicographic ordering

### Encoding
- **Inline Strings**: Strings ≤16 bytes stored inline
- **Hashed Strings**: Longer strings stored as xxhash3 128-bit hash with lookup in id2str table
- **Numeric Types**: Direct binary encoding for integers, decimals, and dates
- **IRIs and Blank Nodes**: Always hashed (128-bit)

### SPARQL Processing
1. **Parser**: Converts SPARQL text to Abstract Syntax Tree (AST)
2. **Optimizer**: Reorders triple patterns by selectivity, pushes down filters
3. **Executor**: Volcano iterator model with operators (Scan, Join, Filter, Project, Limit, etc.)

## Installation

```bash
go get github.com/aleksaelezovic/trigo
```

## Usage

### Basic Example

```go
package main

import (
    "github.com/aleksaelezovic/trigo/internal/storage"
    "github.com/aleksaelezovic/trigo/internal/store"
    "github.com/aleksaelezovic/trigo/pkg/rdf"
)

func main() {
    // Create storage
    storage, _ := storage.NewBadgerStorage("./data")
    defer storage.Close()

    // Create triplestore
    store := store.NewTripleStore(storage)

    // Insert a triple
    triple := rdf.NewTriple(
        rdf.NewNamedNode("http://example.org/alice"),
        rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
        rdf.NewLiteral("Alice"),
    )
    store.InsertTriple(triple)
}
```

### CLI Demo

```bash
# Build the CLI
go build -o trigo ./cmd/trigo

# Run the demo
./trigo demo

# Execute a custom query
./trigo query "SELECT ?s ?p ?o WHERE { ?s ?p ?o }"

# Start HTTP SPARQL endpoint
./trigo serve
# Then visit http://localhost:8080/sparql
```

## Project Structure

```
trigo/
├── cmd/
│   └── trigo/           # CLI application
├── internal/
│   ├── encoding/        # Term encoding with xxhash3
│   ├── storage/         # Storage abstraction and BadgerDB implementation
│   ├── store/           # Triplestore with 11 indexes
│   ├── server/          # HTTP SPARQL endpoint
│   └── sparql/
│       ├── parser/      # SPARQL parser
│       ├── optimizer/   # Query optimizer
│       └── executor/    # Query executor (Volcano model)
└── pkg/
    └── rdf/             # RDF data model (terms, triples, quads)
```

## SPARQL Support

### Supported Query Types
- ✅ SELECT (with variables and *)
- ✅ ASK
- 🚧 CONSTRUCT (planned)
- 🚧 DESCRIBE (planned)

### Supported Features
- ✅ Triple patterns with variables
- ✅ Multiple triple patterns (joins)
- ✅ DISTINCT
- ✅ LIMIT
- ✅ OFFSET
- ✅ ORDER BY (parsed, execution TODO)
- 🚧 FILTER (parsed, evaluation TODO)
- 🚧 OPTIONAL
- 🚧 UNION
- 🚧 Named graphs in queries

## HTTP SPARQL Endpoint

Trigo includes a W3C SPARQL 1.1 Protocol compliant HTTP endpoint:

```bash
# Start the server
./trigo serve

# Query via HTTP
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+json' \
  -d 'SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10'
```

**Supported Features:**
- GET and POST methods
- SPARQL JSON Results format
- SPARQL XML Results format
- Content negotiation
- CORS support
- Web UI with documentation

📖 **See [HTTP_ENDPOINT.md](HTTP_ENDPOINT.md) for complete documentation**

## Testing with W3C SPARQL Test Suite

Trigo includes the official W3C SPARQL 1.1 test suite:

```bash
# Clone with test suite (submodule)
git clone --recursive https://github.com/aleksaelezovic/trigo.git

# Build and run test runner
go build -o test-runner ./cmd/test-runner
./test-runner testdata/rdf-tests/sparql/sparql11/syntax-query

# Current results: 30.9% pass rate on syntax tests
# (Missing features: aggregates, subqueries, BIND, VALUES, etc.)
```

📖 **See [TESTING.md](TESTING.md) for complete testing documentation**

## Performance Considerations

1. **Index Selection**: Queries automatically select the best index based on bound positions
2. **Join Ordering**: Triple patterns are reordered by selectivity (most selective first)
3. **Filter Push-Down**: Filters are applied as early as possible
4. **Lazy Evaluation**: Iterator model enables streaming results without materializing intermediate results
5. **Transaction Isolation**: Snapshot isolation for consistent reads

## Limitations

Current limitations that match Oxigraph's acknowledged trade-offs:

- No automatic garbage collection for id2str table
- Single-threaded query execution
- No full-text search support
- Limited FILTER expression evaluation

## Dependencies

- [BadgerDB](https://github.com/dgraph-io/badger) - Fast LSM-tree based key-value store
- [xxh3](https://github.com/zeebo/xxh3) - Fast xxHash3 implementation for Go

## Roadmap

- [ ] Complete FILTER expression evaluation
- [ ] Implement OPTIONAL and UNION
- [ ] Add CONSTRUCT and DESCRIBE query support
- [ ] Implement ORDER BY execution
- [ ] Add support for RDF-star (quoted triples)
- [ ] Property paths in SPARQL
- [ ] Aggregation functions (COUNT, SUM, AVG, etc.)
- [ ] SPARQL UPDATE support (INSERT, DELETE)
- [ ] Implement W3C SPARQL test suite runner
- [ ] Benchmarking against other triplestores
- [x] **HTTP SPARQL endpoint** ✅
- [ ] Bulk data loading (Turtle, N-Triples, RDF/XML)

## References

- [Oxigraph Architecture](https://github.com/oxigraph/oxigraph/wiki/Architecture)
- [W3C SPARQL 1.1 Specification](https://www.w3.org/TR/sparql11-query/)
- [W3C RDF 1.1 Concepts](https://www.w3.org/TR/rdf11-concepts/)
- [BadgerDB Documentation](https://dgraph.io/docs/badger/)
- [xxHash](https://github.com/Cyan4973/xxHash)

## License

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.
