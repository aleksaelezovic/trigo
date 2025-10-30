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

Trigo implements a subset of SPARQL 1.1 Query, inspired by [Oxigraph](https://github.com/oxigraph/oxigraph)'s architecture. The query engine uses a Volcano iterator model with query optimization.

### Query Types

| Feature | Status | Notes |
|---------|--------|-------|
| **SELECT** | ✅ Implemented | Full support with projection, variables, and `*` |
| **ASK** | ✅ Implemented | Boolean queries working |
| **CONSTRUCT** | ✅ Implemented | Template instantiation with N-Triples output |
| **DESCRIBE** | 🚧 Parsed only | AST support, execution TODO |

### Query Modifiers

| Feature | Status | Notes |
|---------|--------|-------|
| **DISTINCT** | ✅ Implemented | Hash-based deduplication |
| **LIMIT** | ✅ Implemented | Result limiting |
| **OFFSET** | ✅ Implemented | Result skipping |
| **ORDER BY** | 🚧 Parsed only | Sorting expressions parsed, execution TODO |

### Graph Patterns

| Feature | Status | Notes |
|---------|--------|-------|
| **Basic Graph Patterns** | ✅ Implemented | Triple patterns with variables |
| **Joins** | ✅ Implemented | Nested loop joins with optimization |
| **FILTER** | 🚧 Parsed only | Expression parsing done, evaluation TODO |
| **OPTIONAL** | 🚧 Parsed only | Left joins planned |
| **UNION** | 🚧 Parsed only | Alternation planned |
| **GRAPH** | ✅ Implemented | Named graph queries with filtering |
| **MINUS** | 🚧 Parsed only | Negation planned |

### Operators & Functions

**Parsed (evaluation TODO):**
- **Logical:** `&&`, `||`, `!`
- **Comparison:** `=`, `!=`, `<`, `<=`, `>`, `>=`
- **Arithmetic:** `+`, `-`, `*`, `/`
- **String Functions:** `REGEX`, `STR`, `LANG`, `DATATYPE`
- **Numeric Functions:** `isNumeric`, `ABS`, `CEIL`, `FLOOR`, `ROUND`

**Planned:**
- Built-in functions: `BOUND`, `sameTerm`, `isIRI`, `isBlank`, `isLiteral`
- String functions: `STRLEN`, `SUBSTR`, `UCASE`, `LCASE`, `CONTAINS`, `STRSTARTS`, `STRENDS`
- Date/time functions: `NOW`, `YEAR`, `MONTH`, `DAY`, `HOURS`, `MINUTES`, `SECONDS`
- Hash functions: `MD5`, `SHA1`, `SHA256`, `SHA512`
- Aggregates: `COUNT`, `SUM`, `AVG`, `MIN`, `MAX`, `GROUP_CONCAT`, `SAMPLE`

### Advanced Features (Not Yet Implemented)

- ❌ **Subqueries** - Nested SELECT queries
- ❌ **Property Paths** - Transitive property queries
- ❌ **Aggregation** - GROUP BY, HAVING, aggregate functions
- ❌ **BIND** - Variable assignment in patterns
- ❌ **VALUES** - Inline data
- ❌ **SERVICE** - Federated queries
- ❌ **SPARQL UPDATE** - INSERT, DELETE, LOAD, CLEAR operations
- ❌ **Blank Node Property Lists** - `[ foaf:name "Alice" ]` syntax
- ❌ **Collection Syntax** - `( item1 item2 )` for RDF lists

### RDF Serialization Formats

**Query Results:**
- ✅ **SPARQL JSON** - application/sparql-results+json (SELECT, ASK)
- ✅ **SPARQL XML** - application/sparql-results+xml (SELECT, ASK)
- ✅ **N-Triples** - application/n-triples (CONSTRUCT)

**RDF Data (Planned):**
- ❌ **Turtle** - text/turtle
- ❌ **TriG** - application/trig (with named graphs)
- ❌ **N-Quads** - application/n-quads
- ❌ **RDF/XML** - application/rdf+xml
- ❌ **JSON-LD** - application/ld+json

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

**Runtime:**
- [BadgerDB](https://github.com/dgraph-io/badger) v4.8.0 - Fast LSM-tree based key-value store
- [xxh3](https://github.com/zeebo/xxh3) v1.0.2 - Fast xxHash3 implementation for Go

**Development Tools:**
- [staticcheck](https://staticcheck.io/) - Go static analyzer
- [gosec](https://github.com/securego/gosec) - Go security checker

## Roadmap

### Near-term (Query Execution)
- [ ] **FILTER expression evaluation** - Complete evaluator for all parsed operators
- [ ] **ORDER BY execution** - Implement result sorting
- [ ] **DESCRIBE** - Execute resource description queries
- [ ] **OPTIONAL patterns** - Left join implementation
- [ ] **UNION patterns** - Alternation support

### Medium-term (Advanced SPARQL)
- [ ] **Aggregation** - GROUP BY, HAVING, COUNT, SUM, AVG, MIN, MAX
- [ ] **Subqueries** - Nested SELECT support
- [ ] **BIND** - Variable assignment in patterns
- [ ] **VALUES** - Inline data blocks
- [ ] **Property paths** - Transitive/recursive queries (`*`, `+`, `?`, `/`, `|`)
- [ ] **Built-in functions** - Complete SPARQL 1.1 function library

### Long-term (Ecosystem)
- [ ] **SPARQL UPDATE** - INSERT DATA, DELETE DATA, INSERT/DELETE WHERE, LOAD, CLEAR
- [ ] **RDF-star** - Quoted triples support (following RDF-star spec)
- [ ] **Federated queries** - SERVICE keyword for remote endpoints
- [ ] **Full-text search** - Integrate text indexing
- [ ] **Bulk loading** - Efficient import of Turtle, N-Triples, N-Quads, RDF/XML
- [ ] **Benchmarking** - Performance comparisons with Oxigraph, Blazegraph, Jena
- [ ] **Query optimization** - Statistics-based join ordering, cost-based optimization

### Completed ✅
- [x] **HTTP SPARQL endpoint** - W3C SPARQL 1.1 Protocol compliance
- [x] **W3C test suite integration** - Automated testing infrastructure
- [x] **Code quality tools** - staticcheck, gosec, comprehensive linting
- [x] **CONSTRUCT queries** - Template-based RDF graph construction with N-Triples serialization
- [x] **GRAPH patterns** - Named graph queries with proper filtering and index optimization

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
