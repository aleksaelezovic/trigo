# Trigo Architecture

## Overview

Trigo is an RDF triplestore inspired by Oxigraph's architecture, implemented in Go. It provides efficient storage and querying of RDF data with SPARQL support.

## Core Components

### 1. RDF Data Model (`pkg/rdf`)

Defines the fundamental RDF types:

- **Term**: Interface for all RDF terms
  - `NamedNode`: IRIs
  - `BlankNode`: Anonymous nodes
  - `Literal`: String and typed literals
  - `DefaultGraph`: Represents the default graph

- **Triple**: (Subject, Predicate, Object)
- **Quad**: (Subject, Predicate, Object, Graph)

### 2. Encoding Layer (`internal/encoding`)

Handles efficient encoding and decoding of RDF terms using xxHash3 128-bit hashing.

#### Encoding Strategy

Each term is encoded as:
- 1 byte: term type
- 16 bytes: hash or inline data

**Term Types:**
- Named nodes: Always hashed
- Blank nodes: Numeric IDs inline, others hashed
- String literals: ≤16 bytes inline, others hashed
- Typed literals: Direct binary encoding (integers, floats, dates, etc.)

**Hash Function:**
- xxHash3 128-bit variant (zeebo/xxh3)
- Significantly faster than SipHash while maintaining quality

#### Key Encoding
- All keys use big-endian encoding for correct lexicographic ordering
- Enables efficient range scans

### 3. Storage Layer (`internal/storage`)

Provides an abstraction over key-value stores with BadgerDB implementation.

#### Storage Interface
```go
type Storage interface {
    Begin(writable bool) (Transaction, error)
    Close() error
    Sync() error
}
```

#### BadgerDB Implementation
- LSM-tree based storage (similar to RocksDB)
- ACID transactions with snapshot isolation
- Pure Go implementation (no CGo dependencies)
- Optimized for SSDs

#### Table Structure
11 logical tables (column families):

**Metadata:**
- `id2str`: Hash → String lookup table

**Default Graph (3 indexes):**
- `spo`: Subject → Predicate → Object
- `pos`: Predicate → Object → Subject
- `osp`: Object → Subject → Predicate

**Named Graphs (6 indexes):**
- `spog`: Subject → Predicate → Object → Graph
- `posg`: Predicate → Object → Subject → Graph
- `ospg`: Object → Subject → Predicate → Graph
- `gspo`: Graph → Subject → Predicate → Object
- `gpos`: Graph → Predicate → Object → Subject
- `gosp`: Graph → Object → Subject → Predicate

**Graph Metadata:**
- `graphs`: Named graph tracking

### 4. Store Layer (`pkg/store`)

Manages the triplestore with automatic index maintenance.

#### Operations
- `InsertQuad/InsertTriple`: Adds to all relevant indexes
- `DeleteQuad/DeleteTriple`: Removes from all indexes
- `ContainsQuad`: Checks existence
- `Query`: Pattern matching with automatic index selection

#### Index Selection Algorithm

Selects the optimal index based on bound positions in the pattern:

```
Pattern: (?s, bound_p, bound_o, ?g)
Selected: POS index (predicate and object are bound)
```

Priority:
1. Most bound terms
2. Subject/Predicate bindings preferred over Object
3. Graph-specific indexes when graph is bound

### 5. SPARQL Parser (`pkg/sparql/parser`)

Converts SPARQL query text to an Abstract Syntax Tree (AST).

#### Supported Query Types
- SELECT (with *, DISTINCT, LIMIT, OFFSET, ORDER BY)
- ASK

#### AST Structure
- `Query`: Top-level query representation
- `GraphPattern`: WHERE clause patterns
- `TriplePattern`: Triple patterns with variables
- `Filter`: Filter expressions (parsed, evaluation TODO)
- `Expression`: Binary/unary operations, functions

#### Parser Design
- Recursive descent parser
- Hand-written for better error messages and control
- Case-insensitive keyword matching
- Support for IRIs, literals, blank nodes, variables

### 6. Query Optimizer (`pkg/sparql/optimizer`)

Optimizes query execution plans using heuristic-based optimization.

#### Optimization Techniques

**1. Join Reordering (Greedy)**
- Estimates selectivity of each triple pattern
- Bound terms = more selective (fewer results)
- Orders patterns from most to least selective

**Selectivity Heuristics:**
```
- Bound subject:   selectivity × 0.01
- Bound predicate: selectivity × 0.1
- Bound object:    selectivity × 0.1
```

**2. Filter Push-Down**
- Applies filters as soon as all required variables are bound
- Reduces intermediate result sizes

**3. Join Type Selection**
- Currently: Nested loop join (simple, effective for small joins)
- Future: Hash join, merge join based on cardinality estimates

#### Query Plans

Operators (following Volcano model):
- `ScanPlan`: Scan a triple pattern
- `JoinPlan`: Join two subplans
- `FilterPlan`: Apply filter predicate
- `ProjectionPlan`: Select specific variables
- `LimitPlan`: Limit results
- `OffsetPlan`: Skip results
- `DistinctPlan`: Remove duplicates
- `OrderByPlan`: Sort results

### 7. Query Executor (`pkg/sparql/executor`)

Executes optimized query plans using the Volcano iterator model.

#### Volcano Iterator Model

Each operator is an iterator with:
- `Next()`: Advance to next result
- `Binding()`: Get current variable bindings
- `Close()`: Release resources

**Lazy Evaluation:**
- Results pulled on-demand
- Low memory footprint
- Enables pipelining

#### Iterators

**ScanIterator:**
- Wraps store's QuadIterator
- Binds variables from matched quads

**NestedLoopJoinIterator:**
- For each left binding, iterate right side
- Merges compatible bindings
- Backtracks on incompatibility

**FilterIterator:**
- Passes through bindings that satisfy filter

**ProjectionIterator:**
- Projects only selected variables

**LimitIterator:**
- Stops after N results

**OffsetIterator:**
- Skips first N results

**DistinctIterator:**
- Uses hash table to track seen bindings

## Query Execution Flow

```
SPARQL Query Text
       ↓
   [Parser]
       ↓
      AST
       ↓
  [Optimizer]
       ↓
   Query Plan
       ↓
  [Executor]
       ↓
   Iterators
       ↓
    Results
```

### Example Query Execution

Query:
```sparql
SELECT ?person ?name
WHERE {
  ?person foaf:name ?name .
  ?person foaf:age ?age .
}
LIMIT 10
```

Execution Plan:
```
LimitPlan(10)
  └─ ProjectionPlan([?person, ?name])
      └─ JoinPlan(NestedLoop)
          ├─ ScanPlan(?person foaf:age ?age)    [more selective]
          └─ ScanPlan(?person foaf:name ?name)
```

Why this order?
- `age` triple likely more selective (fewer people, specific ages)
- Join on `?person` variable
- Project only requested variables
- Limit applied last

## Transaction Model

### Snapshot Isolation
- Read transactions see a consistent snapshot
- Write transactions use atomic batch commits
- No dirty reads, non-repeatable reads, or phantom reads

### No Garbage Collection
- Deleted strings remain in `id2str` table
- Trade-off: Simpler implementation, may be referenced elsewhere
- Future: Reference counting for cleanup

## Performance Characteristics

### Strengths
- **Index Selection**: Always uses optimal index for pattern
- **Lazy Evaluation**: Memory-efficient streaming
- **Join Ordering**: Reduces intermediate result sizes
- **Big-Endian Keys**: Efficient range scans

### Current Limitations
- **Single-threaded**: No parallel query execution
- **Nested Loop Joins Only**: Hash/merge joins TODO
- **No Statistics**: Selectivity based on heuristics only
- **Limited Filter Evaluation**: Expression evaluation incomplete

## Future Enhancements

### Short-term
1. Complete filter expression evaluation
2. Implement hash join and merge join
3. Add ORDER BY execution
4. Support OPTIONAL and UNION patterns

### Medium-term
1. Collect statistics for better selectivity estimates
2. Parallel query execution
3. SPARQL UPDATE (INSERT/DELETE DATA)
4. RDF data format parsers (Turtle, N-Triples)

### Long-term
1. RDF-star support (quoted triples)
2. Property paths
3. Aggregation functions
4. Full-text search integration
5. Federated query support

## Comparison with Oxigraph

### Similarities
- 11-index architecture
- Big-endian key encoding
- Term type + hash/data encoding
- Volcano iterator model
- Snapshot isolation

### Differences
- **Hash Function**: xxHash3 128-bit vs SipHash-2-4
- **Storage**: BadgerDB vs RocksDB
- **Language**: Go vs Rust
- **Maturity**: Proof-of-concept vs production-ready

## Testing Strategy

### Unit Tests
- RDF term encoding/decoding
- Storage operations
- SPARQL parser correctness
- Query optimizer decisions

### Integration Tests
- End-to-end query execution
- Transaction isolation
- Index consistency

### W3C SPARQL Test Suite
- Official conformance tests
- Located at: https://www.w3.org/2009/sparql/docs/tests/
- TODO: Implement test runner

## References

- [Oxigraph Architecture Wiki](https://github.com/oxigraph/oxigraph/wiki/Architecture)
- [SPARQL 1.1 Query Language](https://www.w3.org/TR/sparql11-query/)
- [RDF 1.1 Concepts](https://www.w3.org/TR/rdf11-concepts/)
- [Volcano Iterator Model](https://paperhub.s3.amazonaws.com/dace52a42c07f7f8348b08dc2b186061.pdf)
- [BadgerDB Paper](https://dgraph.io/blog/post/badger/)
- [xxHash](https://github.com/Cyan4973/xxHash)
