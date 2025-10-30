## W3C SPARQL Test Suite

Trigo includes infrastructure for running the official W3C SPARQL 1.1 test suite to validate compliance with SPARQL standards.

## Setup

The W3C test suite is included as a git submodule:

```bash
# Clone with submodules
git clone --recursive https://github.com/aleksaelezovic/trigo.git

# Or initialize submodules after cloning
git submodule update --init --recursive
```

## Test Suite Structure

The test suite is located in `testdata/rdf-tests/` and includes:

- **sparql10/** - SPARQL 1.0 tests
- **sparql11/** - SPARQL 1.1 Query and Update tests
- **sparql12/** - SPARQL 1.2 tests

### Test Categories

Tests are organized by feature:

```
sparql11/
├── syntax-query/        # Query syntax (positive/negative)
├── syntax-update/       # Update syntax tests
├── aggregates/          # COUNT, SUM, AVG, etc.
├── bind/                # BIND clause
├── bindings/            # VALUES clause
├── construct/           # CONSTRUCT queries
├── exists/              # EXISTS and NOT EXISTS
├── functions/           # Built-in functions
├── grouping/            # GROUP BY
├── negation/            # MINUS, FILTER NOT EXISTS
├── property-path/       # Property paths
├── subquery/            # Subqueries
└── ...
```

## Running Tests

### Build the Test Runner

```bash
go build -o test-runner ./cmd/test-runner
```

### Run Specific Test Suite

```bash
# Run syntax tests
./test-runner testdata/rdf-tests/sparql/sparql11/syntax-query

# Run a specific manifest
./test-runner testdata/rdf-tests/sparql/sparql11/syntax-query/manifest.ttl
```

### Example Output

```
📋 Running manifest: testdata/rdf-tests/sparql/sparql11/syntax-query/manifest.ttl
   Found 94 tests

  ✅ PASS: syntax-construct-where-02.rq
  ❌ FAIL: syntax-aggregate-01.rq
  ⏭️  SKIP: syntax-update-01.ru

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 TEST SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Total:   94
Passed:  29 (30.9%)
Failed:  64
Skipped: 1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Test Types

### Positive Syntax Tests

Verify that valid SPARQL queries parse successfully:

```turtle
:test1 rdf:type mf:PositiveSyntaxTest11 ;
    mf:name "Basic SELECT" ;
    mf:action <query.rq> .
```

### Negative Syntax Tests

Verify that invalid queries are rejected:

```turtle
:test2 rdf:type mf:NegativeSyntaxTest11 ;
    mf:name "Invalid WHERE" ;
    mf:action <bad-query.rq> .
```

### Query Evaluation Tests

Execute queries against data and compare results:

```turtle
:test3 rdf:type mf:QueryEvaluationTest ;
    mf:name "Basic BGP" ;
    mf:action [
        qt:query <query.rq> ;
        qt:data <data.ttl>
    ] ;
    mf:result <result.srx> .
```

## Current Test Support

### Syntax Tests (Parser Validation)
- **Pass Rate: 69.1%** (65/94 tests in syntax-query suite)
- ✅ All SELECT expression tests (5/5)
- ✅ All aggregate syntax tests (15/15)
- ✅ All IN/NOT IN tests (3/3)
- ✅ EXISTS/NOT EXISTS parsing
- ✅ Property list shorthand (semicolon/comma)
- ✅ Boolean literals (true/false)

### Execution Tests (End-to-End Validation)

#### ✅ Implemented and Validated
- **BIND expressions**: 70.0% pass rate (7/10 tests)
  - ✅ Basic BIND with expressions
  - ✅ BIND variables in subsequent patterns
  - ✅ FILTER on BIND variables
  - ⚠️ Known limitations: UNION scoping, forward references
- **CONSTRUCT queries**: 28.6% pass rate (2/7 tests)
  - ✅ Template instantiation
  - ✅ CONSTRUCT WHERE shorthand
  - ✅ N-Triples output
- **Basic graph patterns**: Full support
  - ✅ Triple patterns with variables
  - ✅ Nested loop joins
  - ✅ Join ordering optimization
- **Query modifiers**: Full support
  - ✅ DISTINCT (hash-based deduplication)
  - ✅ LIMIT and OFFSET
  - ✅ ORDER BY (ASC/DESC)
- **Complex patterns**: Full support
  - ✅ OPTIONAL (left outer join)
  - ✅ UNION (pattern alternation)
  - ✅ MINUS (set difference)
  - ✅ GRAPH (named graph queries)
- **FILTER expressions**: 20+ operators and functions
  - ✅ Logical operators (&&, ||, !)
  - ✅ Comparison operators (=, !=, <, >, <=, >=)
  - ✅ Arithmetic operators (+, -, *, /)
  - ✅ IN/NOT IN operators
  - ✅ String functions (STRLEN, SUBSTR, UCASE, LCASE, CONCAT, etc.)
  - ✅ Type checking (BOUND, isIRI, isBlank, isLiteral, isNumeric)
  - ✅ Numeric functions (ABS, CEIL, FLOOR, ROUND)

#### 🚧 Parsed But Not Executed
- **EXISTS/NOT EXISTS**: Parser complete, evaluation TODO
- **Aggregates**: Syntax supported (COUNT, SUM, AVG, MIN, MAX, GROUP_CONCAT, SAMPLE)
- **GROUP BY**: Parsed with variables and expressions
- **HAVING**: Parsed with filter conditions
- **DESCRIBE**: Parser complete, execution TODO

#### ❌ Not Yet Implemented
- VALUES clause
- Subqueries (detected but not parsed)
- Property paths
- REGEX function
- Date/time functions
- Hash functions (MD5, SHA1, etc.)
- UPDATE operations
- Service federation

## Implementation Details

### Test Runner Architecture

```
Test Runner
    ↓
Manifest Parser (.ttl files)
    ↓
Test Evaluator
    ↓
├─ Syntax Tests → SPARQL Parser
├─ Evaluation Tests → Full Pipeline:
│   ├─ Turtle Parser (load test data)
│   ├─ SPARQL Parser (parse query)
│   ├─ Optimizer (build execution plan)
│   ├─ Executor (run query)
│   ├─ SPARQL XML Parser (parse expected results)
│   └─ Result Comparator (validate correctness)
└─ Update Tests → (TODO)
```

### Key Components

**Turtle Parser** (`internal/turtle/parser.go`):
- Loads RDF test data from `.ttl` files
- Supports PREFIX/BASE declarations
- Handles IRIs, blank nodes, literals
- Supports datatypes and language tags
- Sufficient for W3C test data files

**SPARQL XML Parser** (`internal/sparqlxml/parser.go`):
- Parses expected results from `.srx` files
- Converts to RDF term bindings
- Supports all RDF term types
- Enables order-independent result comparison

**Query Evaluation** (`internal/testsuite/runner.go`):
1. Clear store between tests
2. Load test data using Turtle parser
3. Parse SPARQL query
4. Optimize query plan
5. Execute query with full pipeline
6. Parse expected results
7. Compare actual vs expected (order-independent)

### Manifest Format

Manifests are Turtle files describing tests:

```turtle
@prefix mf: <http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#> .
@prefix qt: <http://www.w3.org/2001/sw/DataAccess/tests/test-query#> .

<> rdf:type mf:Manifest ;
    mf:entries (
        :test1
        :test2
    ) .

:test1 a mf:PositiveSyntaxTest11 ;
    mf:name "Test Name" ;
    mf:action <query-file.rq> .
```

## Adding Test Support

To add support for a new test type:

1. **Update Manifest Parser** (`internal/testsuite/manifest.go`)
   - Add new `TestType` constant
   - Update parser to recognize new test type

2. **Implement Test Evaluator** (`internal/testsuite/runner.go`)
   - Add case in `runTest()` switch
   - Implement evaluation function

3. **Add Required Features** (if needed)
   - Extend parser for new syntax
   - Add optimizer support
   - Implement executor operators

## Testing Philosophy

The W3C SPARQL test suite serves multiple purposes:

1. **Compliance Validation** - Ensure standard conformance
2. **Regression Testing** - Catch breaking changes
3. **Feature Tracking** - Identify gaps in implementation
4. **Documentation** - Tests demonstrate expected behavior

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: SPARQL Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          submodules: recursive

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Build Test Runner
        run: go build -o test-runner ./cmd/test-runner

      - name: Run Syntax Tests
        run: ./test-runner testdata/rdf-tests/sparql/sparql11/syntax-query
```

## References

- [W3C SPARQL 1.1 Test Cases](https://www.w3.org/2009/sparql/docs/tests/)
- [RDF Test Suite Repository](https://github.com/w3c/rdf-tests)
- [SPARQL 1.1 Query Specification](https://www.w3.org/TR/sparql11-query/)
- [Test Manifest Vocabulary](https://www.w3.org/2001/sw/DataAccess/tests/test-manifest)

## Future Improvements

- [ ] Complete evaluation test support
- [ ] Add UPDATE test support
- [ ] Implement result comparison (XML, JSON, CSV)
- [ ] Add graph isomorphism checking
- [ ] Support federated query tests
- [ ] Generate HTML test reports
- [ ] Track compliance percentage over time
- [ ] Add benchmarking for performance tests

## Contributing

When adding new SPARQL features:

1. Run relevant test suite section
2. Document pass rate in commit message
3. Create issues for failing tests
4. Update this document with current status
