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

### ✅ Implemented

- Positive syntax tests (basic SELECT, ASK)
- Negative syntax tests
- Basic manifest parsing

### 🚧 Partial Support

- SELECT queries (basic patterns only)
- ASK queries
- Triple patterns
- LIMIT, OFFSET
- DISTINCT

### ❌ Not Yet Implemented

- Aggregates (COUNT, SUM, AVG, etc.)
- GROUP BY, HAVING
- BIND clause
- VALUES clause
- Subqueries
- OPTIONAL, UNION, MINUS
- Property paths
- EXISTS, NOT EXISTS
- FILTER expressions (partial)
- CONSTRUCT queries
- UPDATE operations

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
├─ Evaluation Tests → Parser + Optimizer + Executor
└─ Update Tests → (TODO)
```

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
