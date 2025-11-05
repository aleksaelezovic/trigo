# CLAUDE.md - AI Assistant Context for Trigo

This document provides comprehensive context for AI assistants working on the Trigo codebase.

## Project Overview

**Trigo** is a SPARQL 1.1 query engine and RDF triple store written in Go. It provides:
- Full SPARQL 1.1 query support (SELECT, CONSTRUCT, ASK, DESCRIBE)
- Multiple RDF format parsers (Turtle, N-Triples, N-Quads, TriG, RDF/XML, JSON-LD)
- In-memory triple store with efficient indexing (SPO, POS, OSP)
- HTTP API server for SPARQL queries
- W3C test suite compliance validation

## Architecture

### Core Components

```
trigo/
├── cmd/
│   ├── server/          # HTTP API server
│   └── test-runner/     # W3C test suite runner
├── pkg/
│   ├── rdf/            # RDF parsers and term definitions
│   │   ├── turtle.go   # Turtle/N-Triples parser (shared, strict mode)
│   │   ├── nquads.go   # N-Quads parser with strict validation
│   │   ├── trig.go     # TriG parser (Turtle + named graphs)
│   │   ├── rdfxml.go   # RDF/XML parser
│   │   ├── jsonld.go   # JSON-LD parser (basic)
│   │   └── term.go     # RDF terms (NamedNode, BlankNode, Literal)
│   ├── sparql/
│   │   ├── parser/     # SPARQL query parser
│   │   ├── optimizer/  # Query optimization (join ordering)
│   │   └── executor/   # Query execution engine
│   ├── store/          # Triple store API
│   └── server/         # HTTP handlers and result serialization
├── internal/
│   ├── storage/        # In-memory storage with indexing
│   ├── encoding/       # RDF term encoding for storage
│   └── testsuite/      # W3C test suite infrastructure
└── testdata/
    └── rdf-tests/      # W3C test suites (git submodule)
```

### Key Design Patterns

**RDF Term Representation:**
```go
type Term interface {
    Type() TermType
    String() string
    Equals(Term) bool
}

// Concrete types: NamedNode, BlankNode, Literal, DefaultGraph
```

**Parser Architecture:**
- Most parsers return `[]*Triple` or `[]*Quad`
- Parsers are stateful (maintain position, counters, context)
- Strict vs lenient modes (N-Triples/N-Quads are strict)

**Storage Indexing:**
- Three indexes: SPO, POS, OSP (all permutations)
- Terms are encoded to uint64 for efficient storage
- Uses sync.RWMutex for concurrent access

## RDF Parser Implementation Details

### Turtle Parser (`pkg/rdf/turtle.go`)

**Dual Mode Operation:**
```go
type TurtleParser struct {
    strictNTriples bool  // When true, enforce strict N-Triples syntax
    // ...
}

// Factory functions:
NewTurtleParser(input string) *TurtleParser     // Lenient Turtle mode
NewNTriplesParser(input string) *TurtleParser   // Strict N-Triples mode
```

**Strict N-Triples Rules:**
- No PREFIX/BASE directives
- No abbreviations (`,`, `;`, `a` keyword)
- No bare numeric/boolean literals (must be quoted with datatype)
- Only specific escape sequences: `\t`, `\b`, `\n`, `\r`, `\f`, `\\`, `\"`, `\uXXXX`, `\UXXXXXXXX`
- Absolute IRIs only (no relative URIs)

**Features Implemented:**
- ✅ PREFIX/BASE (Turtle mode)
- ✅ Property list shorthand (`;`)
- ✅ Object list shorthand (`,`)
- ✅ Anonymous blank nodes `[]`
- ✅ Empty collections `()`
- ✅ Triple-quoted literals `"""..."""` and `'''...'''`
- ✅ Numeric literals (integers, decimals, doubles)
- ✅ Boolean literals (`true`, `false`)
- ✅ Unicode escape sequences
- ⚠️ Collections with items (partial support)
- ⚠️ Blank node property lists (partial support)

### N-Quads Parser (`pkg/rdf/nquads.go`)

**Strict Validation:**
```go
type NQuadsParser struct {
    strictMode bool  // Always true - N-Quads uses strict syntax
    // ...
}
```

**IRI Validation:**
- Rejects invalid characters: space, `<`, `>`, `"`, `{`, `}`, `|`, `^`, `` ` ``, control chars
- Supports Unicode escape sequences in IRIs: `\uXXXX`, `\UXXXXXXXX`
- Validates language tags (must start with letter per BCP 47)

**Graph Component:**
- Fourth component is optional (default graph if omitted)
- Same strict IRI rules apply to graph names

### TriG Parser (`pkg/rdf/trig.go`)

**Graph Block Syntax:**
```turtle
# Anonymous graph block
{ <s> <p> <o> . }

# Named graph block (shorthand)
<http://example.org/graph> { <s> <p> <o> . }

# Named graph block (GRAPH keyword)
GRAPH <http://example.org/graph> { <s> <p> <o> . }

# Blank node graph
_:g1 { <s> <p> <o> . }
```

**Implementation Strategy:**
- Extends Turtle parser
- Uses lookahead to distinguish graph blocks from regular triples
- Checks for `{` after first term to identify graph blocks
- Anonymous blocks use generated blank node as graph name

### RDF/XML Parser (`pkg/rdf/rdfxml.go`)

**Features Implemented:**
- ✅ `rdf:Description` elements
- ✅ Property elements (child elements as properties)
- ✅ `rdf:about`, `rdf:resource`, `rdf:ID` attributes
- ✅ `rdf:datatype`, `xml:lang` attributes
- ✅ RDF containers: `rdf:Bag`, `rdf:Seq`, `rdf:Alt`
- ✅ Auto-numbered `rdf:li` elements (→ `rdf:_1`, `rdf:_2`, ...)
- ✅ Explicit `rdf:_N` properties
- ✅ `xml:base` tracking and resolution
- ✅ `rdf:parseType="Resource"` for blank node objects
- ✅ Property attributes on `rdf:Description` elements
- ✅ Property attributes on property elements (structured values)
- ✅ Typed nodes (elements with implicit `rdf:type`)
- ✅ Document base URI support via `SetBaseURI()`

**Base URI Resolution:**
```go
type RDFXMLParser struct {
    baseURIStack []string // xml:base stack
    documentBase string   // Document base URI (file location)
}

// xml:base takes precedence over document base
func (p *RDFXMLParser) getCurrentBase() string {
    if len(p.baseURIStack) > 0 {
        return p.baseURIStack[len(p.baseURIStack)-1]
    }
    return p.documentBase
}
```

**Property Attributes Pattern:**
When a property element has non-RDF namespace attributes:
```xml
<eg:Creator eg:named="Dürst"/>
```
Generates:
```turtle
subject eg:Creator _:blank .
_:blank eg:named "Dürst" .
```

**Not Yet Implemented:**
- ⚠️ `rdf:parseType="Collection"`
- ⚠️ Reification (rdf:ID on property elements)
- ⚠️ `rdf:nodeID` attribute
- ⚠️ Negative test validation (error handling)

## SPARQL Implementation Details

### Parser (`pkg/sparql/parser/`)

**Query Types Supported:**
- SELECT (with variables and expressions)
- CONSTRUCT (template-based graph construction)
- ASK (boolean queries)
- DESCRIBE (not fully executed)

**Pattern Support:**
- ✅ Basic Graph Patterns (BGP)
- ✅ OPTIONAL (left outer join)
- ✅ UNION (pattern alternation)
- ✅ MINUS (set difference)
- ✅ FILTER (20+ operators and functions)
- ✅ BIND (variable binding)
- ✅ GRAPH (named graph patterns)
- ✅ Property paths (parsed but limited execution)
- ⚠️ EXISTS/NOT EXISTS (parsed, not evaluated)
- ⚠️ Subqueries (detected but not parsed)
- ❌ VALUES clause
- ❌ Service federation

**FILTER Functions:**
- Logical: `&&`, `||`, `!`
- Comparison: `=`, `!=`, `<`, `>`, `<=`, `>=`
- Arithmetic: `+`, `-`, `*`, `/`
- String: STRLEN, SUBSTR, UCASE, LCASE, CONCAT, CONTAINS, STRSTARTS, STRENDS
- Type checking: BOUND, isIRI, isBlank, isLiteral, isNumeric
- Numeric: ABS, CEIL, FLOOR, ROUND
- IN/NOT IN operators

### Optimizer (`pkg/sparql/optimizer/`)

**Optimization Strategies:**
1. Join ordering based on pattern specificity
2. Filter push-down (evaluate filters early)
3. Pattern selectivity estimation

**Selectivity Scoring:**
- 3 variables (?, ?, ?): score 3 (least specific)
- 2 variables (s, ?, ?): score 2
- 1 variable (s, p, ?): score 1
- 0 variables (s, p, o): score 0 (most specific)

### Executor (`pkg/sparql/executor/`)

**Execution Model:**
- Stream-based processing with Solution iterators
- Hash joins for OPTIONAL
- Union concatenation
- Set difference for MINUS

**Important Classes:**
```go
type Solution map[string]rdf.Term  // Variable bindings

type Plan interface {
    Execute(ctx context.Context) (SolutionIterator, error)
}

// Concrete plans: ScanPlan, FilterPlan, JoinPlan, UnionPlan, etc.
```

## W3C Test Suite Infrastructure

### Test Runner (`cmd/test-runner/`)

**Usage:**
```bash
go build -o test-runner ./cmd/test-runner

# Run specific test suite
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle
./test-runner testdata/rdf-tests/sparql/sparql11/syntax-query

# Run specific manifest
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml/manifest.ttl
```

### Test Types

**RDF Parser Tests:**
- `PositiveSyntaxTest` - Must parse successfully
- `NegativeSyntaxTest` - Must fail to parse
- `EvalTest` - Parse and compare triples with expected N-Triples output

**SPARQL Tests:**
- `PositiveSyntaxTest11` - Query must parse
- `NegativeSyntaxTest11` - Query must fail to parse
- `QueryEvaluationTest` - Execute query and compare results

### Manifest Parser (`internal/testsuite/manifest.go`)

Manifests are Turtle files describing tests:
```turtle
@prefix mf: <http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#> .

<> a mf:Manifest ;
    mf:entries ( :test1 :test2 ) .

:test1 a mf:PositiveSyntaxTest11 ;
    mf:name "Test Name" ;
    mf:action <query.rq> .
```

### Document Base URI Handling

Test runner converts file paths to W3C canonical URIs:
```go
func (r *TestRunner) filePathToURI(filePath string) string {
    // testdata/rdf-tests/rdf/rdf11/rdf-xml/test.rdf
    // → https://w3c.github.io/rdf-tests/rdf/rdf11/rdf-xml/test.rdf
}
```

This is critical for RDF/XML `rdf:ID` resolution.

## Current Test Results

### RDF 1.1 Parser Compliance

| Format | Pass Rate | Status |
|--------|-----------|--------|
| N-Triples | 100.0% (70/70) | ✅ PERFECT |
| N-Quads | 100.0% (87/87) | ✅ PERFECT |
| Turtle | 66.2% (196/296) | 🟨 Good |
| TriG | 47.2% (158/335) | 🟨 Moderate |
| RDF/XML | 47.0% (78/166) | 🟨 Good |
| JSON-LD | Not measured | ⚠️ Basic |

### SPARQL Compliance

**Syntax Tests:** 69.1% (65/94)
- ✅ All aggregate syntax tests
- ✅ SELECT expressions
- ✅ Property list shorthand
- ⚠️ Some advanced features not parsed

**Execution Tests:**
- BIND: 70.0% (7/10)
- CONSTRUCT: 28.6% (2/7)
- Other suites not fully measured

## Known Limitations

### Collections and Property Lists

**Issue:** Parser architecture limitation where `parseTerm()` returns a single term, but collections and blank node property lists generate multiple triples.

**Current Workaround:** Limited support with special cases.

### Reification

**Missing:** RDF/XML `rdf:ID` on property elements should generate reification triples:
```xml
<eg:prop rdf:ID="stmt">value</eg:prop>
```
Should generate:
```turtle
subject eg:prop "value" .
#stmt rdf:type rdf:Statement .
#stmt rdf:subject subject .
#stmt rdf:predicate eg:prop .
#stmt rdf:object "value" .
```

## Development Conventions

### Code Style

**Go Conventions:**
- Use `gofmt` for formatting
- Follow standard Go project layout
- Use descriptive variable names
- Add comments for exported functions

**Error Handling:**
```go
// Always wrap errors with context
return nil, fmt.Errorf("failed to parse IRI: %w", err)

// Check errors immediately
if err != nil {
    return nil, err
}
```

### Quality Checks

**Must Pass Before Commit:**
```bash
go vet ./...          # Check for suspicious code
staticcheck ./...     # Advanced static analysis
gosec -quiet ./...    # Security checker
go test ./...         # Unit tests
```

### Git Commit Conventions

**Commit Message Format:**
```
feat(rdf): Add RDF/XML container support

- Implement rdf:Bag, rdf:Seq, rdf:Alt
- Add auto-numbered rdf:li elements
- Test results: 20.6% → 34.5%

All quality checks pass: go vet, staticcheck, gosec

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Commit Types:**
- `feat:` New feature
- `fix:` Bug fix
- `refactor:` Code restructuring
- `test:` Test additions/modifications
- `docs:` Documentation updates
- `chore:` Build/tooling changes

**Scopes:**
- `rdf` - RDF parsers
- `sparql` - SPARQL parser/executor
- `storage` - Triple store
- `server` - HTTP API

## Development Workflow

**CRITICAL: This workflow MUST be followed for every code change.**

### Standard Development Process

When implementing features or fixing bugs, follow these steps **in order**:

#### 1. Format Code (ALWAYS)
```bash
go fmt ./...
```
**Do this first and after any code changes.** This ensures consistent formatting across the codebase.

#### 2. Build and Check Compilation
```bash
go build ./...
```
Ensures all packages compile without errors. Fix any compilation errors before proceeding.

#### 3. Run Unit Tests
```bash
go test ./...
```
Or for verbose output:
```bash
go test ./... -v
```
**All existing tests must pass.** If you break existing tests, fix your code or update tests appropriately.

#### 4. Run Quality Checks (MANDATORY)

**Run ALL three quality checks:**

```bash
# Check for suspicious constructs
go vet ./...

# Advanced static analysis
staticcheck ./...

# Security vulnerability scanning
gosec -quiet ./...
```

**All three must pass with no errors before committing.** This is non-negotiable.

**Common Issues:**
- `go vet`: Catches unreachable code, printf issues, struct tags
- `staticcheck`: Finds unused code, ineffective assignments, simplifications (use `//lint:ignore` if needed)
- `gosec`: Security issues like file inclusion, weak crypto (use `#nosec` with justification if needed)

#### 5. Run W3C Test Suite (For RDF/SPARQL Changes)

If you modified RDF parsers or SPARQL components:

```bash
# Build test runner
go build -o test-runner ./cmd/test-runner

# Run relevant test suite
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle        # For Turtle changes
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml          # For RDF/XML changes
./test-runner testdata/rdf-tests/sparql/sparql11/bind       # For SPARQL changes
```

**Document the results** in your commit message:
- Before: X% (N/M tests)
- After: Y% (K/M tests)
- Change: +Z percentage points

#### 6. Update Documentation (If Needed)

**Update these files when appropriate:**

- `TESTING.md` - Always update for test result changes
- `README.md` - Update for new features or API changes
- `CLAUDE.md` - Update for architectural changes or new patterns

**Test results in TESTING.md format:**
```markdown
- **rdf11/rdf-xml**: 38.8% pass rate (64/165 tests) ✅ **IMPROVED from 34.5%**
```

#### 7. Stage and Commit Changes

```bash
# Check what changed
git status
git diff

# Stage specific files (prefer specific files over '.')
git add pkg/rdf/rdfxml.go internal/testsuite/runner.go TESTING.md

# Commit with proper message
git commit -m "feat(rdf): Add property attributes support

Detailed description here...

RDF/XML compliance: 34.5% → 38.8% (+4.3pp, +7 tests)

All quality checks pass: go vet, staticcheck, gosec

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

#### 8. Push to Remote (Optional)

```bash
# Push to main branch
git push origin main

# Or push to feature branch
git push origin feature-branch-name
```

### Quick Workflow Checklist

Use this checklist for every change:

- [ ] `go fmt ./...` - Format code
- [ ] `go build ./...` - Verify compilation
- [ ] `go test ./...` - Run unit tests
- [ ] `go vet ./...` - Check for issues
- [ ] `staticcheck ./...` - Static analysis
- [ ] `gosec -quiet ./...` - Security scan
- [ ] Run W3C test suite (if applicable)
- [ ] Update `TESTING.md` (if test results changed)
- [ ] Update `README.md` (if features/API changed)
- [ ] `git add <files>` - Stage specific files
- [ ] `git commit -m "..."` - Commit with proper message
- [ ] Document test results in commit message
- [ ] `git push` (if ready)

### When to Commit

**Commit frequency guidelines:**

✅ **DO commit when:**
- A feature is complete and all checks pass
- A bug is fixed and verified
- Tests pass rate improves significantly
- A logical unit of work is done

❌ **DON'T commit when:**
- Code doesn't compile
- Tests are failing (unless intentionally adding failing tests)
- Quality checks have errors
- Work is incomplete and would break others

**Prefer smaller, focused commits over large monolithic ones.**

### Example: Complete Development Session

```bash
# 1. Make code changes in your editor

# 2. Format
go fmt ./...

# 3. Build
go build ./...

# 4. Test
go test ./...

# 5. Quality checks
go vet ./... && staticcheck ./... && gosec -quiet ./...

# 6. Run W3C tests (if applicable)
go build -o test-runner ./cmd/test-runner
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml

# 7. Update docs
# Edit TESTING.md with new test results

# 8. Commit
git add pkg/rdf/rdfxml.go TESTING.md
git commit -m "feat(rdf): Implement feature X

- Detail 1
- Detail 2

Test results: 30% → 35% (+5pp, +8 tests)

All quality checks pass: go vet, staticcheck, gosec

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# 9. Push
git push origin main
```

### Emergency Fixes

If you need to commit a quick fix:

**Minimum requirements:**
1. `go fmt ./...`
2. `go build ./...`
3. `go vet ./...`
4. Commit

Even for emergency fixes, **never skip formatting and vet checks.**

### Working with Feature Branches

```bash
# Create feature branch
git checkout -b feature/new-parser

# Make changes, follow full workflow

# Commit to feature branch
git add .
git commit -m "feat: ..."
git push origin feature/new-parser

# When ready to merge
git checkout main
git merge feature/new-parser
git push origin main
```

### Handling Quality Check Failures

**If staticcheck complains about unused code:**
```go
// Option 1: Remove unused code
// Option 2: Use lint directive with explanation
//lint:ignore U1000 This will be used in future feature X
func unusedFunction() { }
```

**If gosec flags false positives:**
```go
// Use #nosec with clear justification
data, err := os.ReadFile(path) // #nosec G304 - test suite reads test files
```

**Never suppress warnings without understanding why they exist.**

### Building and Running

**Build Everything:**
```bash
go build ./...
```

**Build Specific Components:**
```bash
go build -o server ./cmd/server
go build -o test-runner ./cmd/test-runner
```

**Run Tests:**
```bash
# Unit tests
go test ./...

# W3C test suite
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle

# Run server
./server
```

**Run with Docker:**
```bash
docker build -t trigo .
docker run -p 8080:8080 trigo
```

## Important Implementation Notes

### RDF Term Encoding

Terms are encoded to uint64 for storage efficiency:
```go
// internal/encoding/encoding.go
type Encoder struct {
    termToID  map[string]uint64
    idToTerm  map[uint64]rdf.Term
    nextID    uint64
}
```

This allows:
- Fast term comparison (uint64 vs string)
- Efficient indexing
- Memory-efficient storage

### Concurrent Access

Storage uses read-write locks:
```go
type TripleStore struct {
    mu sync.RWMutex
    // Multiple readers OR single writer
}
```

Always use:
- `RLock()`/`RUnlock()` for reads
- `Lock()`/`Unlock()` for writes

### Parser State Management

Most parsers maintain position:
```go
type TurtleParser struct {
    input  string
    pos    int
    length int
    // ...
}
```

**Important:**
- Always validate `pos < length` before accessing `input[pos]`
- Use `skipWhitespaceAndComments()` liberally
- Maintain blank node counters per document

### Testing Infrastructure

**Test Runner Expectations:**
1. Reads manifest.ttl files
2. For each test, reads action file (input)
3. For eval tests, reads result file (expected output)
4. Compares triples (order-independent)
5. Reports pass/fail with detailed errors

**Triple Comparison:**
- Order-independent (set comparison)
- Blank node aware (but not isomorphic matching)
- Uses string representation for matching

## Recent Development History

### Phase 1: Turtle Parser Improvements (44.4% → 62.2%)
- Added Unicode escape sequences
- Implemented anonymous blank nodes and empty collections
- Added @base directive with relative IRI resolution
- Implemented numeric and boolean literals
- Fixed manifest parser for N-Quads detection

### Phase 2a: TriG and Basic RDF/XML (32.2% → 46.0% TriG, 20.6% → 34.5% RDF/XML)
- Implemented TriG graph blocks (anonymous, named, GRAPH keyword)
- Added RDF/XML containers (Bag, Seq, Alt)
- Implemented rdf:li auto-numbering
- Added xml:base tracking
- Property attributes on rdf:Description

### Phase 2b: Advanced RDF/XML (34.5% → 38.8%)
- Implemented rdf:parseType="Resource"
- Added document base URI support
- Property attributes on property elements
- Proper xml: namespace filtering
- W3C canonical URI resolution in test runner

### Phase 3: Graph Isomorphism & SPARQL Result Formats (Latest)
- Implemented graph isomorphism for blank node matching
  - VF2-inspired backtracking algorithm
  - Handles both triples and quads
  - Fixed 30 tests across RDF formats
  - Turtle: 62.2% → 66.2% (+12 tests)
  - RDF/XML: 38.8% → 47.0% (+14 tests)
  - TriG: 46.0% → 47.2% (+4 tests)
- Implemented W3C-compliant CSV/TSV result serialization
  - Canonical blank node labeling
  - Proper double formatting (1.0E6 for CSV, 1.0e6 for TSV)
  - Datatype handling for TSV format
  - CSV/TSV tests: 0% → 83.3% (5/6 tests)

## Working with This Codebase

### Adding a New RDF Format

1. Create parser file in `pkg/rdf/`
2. Implement `Parse()` method returning `[]*Triple` or `[]*Quad`
3. Add case in `internal/testsuite/runner.go` `parseRDFData()`
4. Add tests in `pkg/rdf/`
5. Update `TESTING.md` with results
6. Commit with test results

### Adding SPARQL Features

1. Update `pkg/sparql/parser/` to parse new syntax
2. Add AST nodes if needed
3. Update `pkg/sparql/optimizer/` for new patterns
4. Implement execution in `pkg/sparql/executor/`
5. Run W3C test suite to validate
6. Update `TESTING.md`

### Debugging Test Failures

**For RDF Parser Tests:**
```bash
# Run single test directory
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml/rdfms-empty-property-elements

# Examine specific test files
cat testdata/rdf-tests/rdf/rdf11/rdf-xml/test.rdf    # Input
cat testdata/rdf-tests/rdf/rdf11/rdf-xml/test.nt    # Expected output
```

**For SPARQL Tests:**
```bash
# Check query syntax
cat testdata/rdf-tests/sparql/sparql11/bind/bind01.rq

# Check test data
cat testdata/rdf-tests/sparql/sparql11/bind/data01.ttl

# Check expected results
cat testdata/rdf-tests/sparql/sparql11/bind/bind01.srx
```

### Security Notes

**File Operations:**
Test runner uses `#nosec G304` because it legitimately reads test files:
```go
data, err := os.ReadFile(path) // #nosec G304 - test suite reads test files
```

**Input Validation:**
- Always validate user input in HTTP handlers
- Use parameter binding for queries (though not SQL)
- Sanitize error messages (don't leak paths)

## Future Improvements Needed

### High Priority
1. **Graph Isomorphism** - Blank node matching for tests
2. **Reification Support** - RDF/XML rdf:ID on properties
3. **Negative Test Validation** - Error handling for invalid inputs

### Medium Priority
4. **VALUES Clause** - SPARQL 1.1 inline data
5. **Subqueries** - Nested SELECT queries
6. **EXISTS/NOT EXISTS** - Filter evaluation
7. **rdf:parseType="Collection"** - RDF/XML collections

### Low Priority
8. **Property Paths** - Full execution support
9. **Service Federation** - SPARQL 1.1 SERVICE
10. **UPDATE Operations** - SPARQL 1.1 Update
11. **RDF 1.2 Support** - RDF-star features

## Useful Commands Reference

```bash
# Quality checks (run before commit)
go vet ./... && staticcheck ./... && gosec -quiet ./...

# Build everything
go build ./...

# Run all unit tests
go test ./... -v

# Build and run test runner
go build -o test-runner ./cmd/test-runner
./test-runner testdata/rdf-tests/rdf/rdf11/

# Build and run server
go build -o server ./cmd/server
./server

# Check submodule status
git submodule status

# Update submodules
git submodule update --init --recursive

# View recent commits
git log --oneline -20

# Stage and commit
git add <files>
git commit -m "feat(scope): description"

# Push to remote
git push origin main
```

## Contact and Resources

- **GitHub Repository:** https://github.com/aleksaelezovic/trigo
- **W3C SPARQL 1.1:** https://www.w3.org/TR/sparql11-query/
- **W3C RDF 1.1:** https://www.w3.org/TR/rdf11-primer/
- **W3C Test Suites:** https://github.com/w3c/rdf-tests

---

**Last Updated:** 2025-01-02 (based on commits through Phase 2b)
**Maintainer:** Aleksa Elezovic
**AI Assistant Context Version:** 1.0
