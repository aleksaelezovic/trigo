# CLAUDE.md - Trigo Project Context

**Trigo** is a SPARQL 1.1 query engine and RDF triple store written in Go. Provides full SPARQL query support, multiple RDF format parsers, and in-memory triple store with efficient indexing.

## Project Structure

```
trigo/
├── cmd/
│   ├── server/          # HTTP API server
│   └── test-runner/     # W3C test suite runner
├── pkg/
│   ├── rdf/            # RDF parsers (turtle.go, nquads.go, trig.go, rdfxml.go, jsonld.go)
│   ├── sparql/         # SPARQL parser, optimizer, executor
│   ├── store/          # Triple store API
│   └── server/         # HTTP handlers and result serialization
├── internal/
│   ├── storage/        # In-memory storage with SPO/POS/OSP indexes
│   ├── encoding/       # RDF term encoding for storage
│   └── testsuite/      # W3C test suite infrastructure
└── testdata/
    └── rdf-tests/      # W3C test suites (git submodule)
```

## CRITICAL: Development Workflow

**IMPORTANT: You MUST follow this workflow for EVERY code change. Do NOT skip any step.**

### 1. Format Code (ALWAYS FIRST)
```bash
go fmt ./...
```

### 2. Build
```bash
go build ./...
```

### 3. Run Unit Tests
```bash
go test ./...
```

### 4. Quality Checks (ALL THREE MUST PASS)
```bash
go vet ./...
staticcheck ./...
gosec -quiet ./...
```

**YOU MUST NOT commit if any quality check fails.**

### 5. Run W3C Test Suite (for RDF/SPARQL changes)
```bash
go build -o test-runner ./cmd/test-runner

# Run appropriate test suite based on changes:
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle        # Turtle changes
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml          # RDF/XML changes
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-trig         # TriG changes
./test-runner testdata/rdf-tests/sparql/sparql11/bind       # SPARQL changes
```

### 6. Update Documentation
- **ALWAYS** update README.md test results when they change
- **ALWAYS** update docs/testing.html when test compliance improves
- **ALWAYS** update docs/index.html test results badge section
- Document test result changes in commit message

**NOTE:** Test results appear in THREE locations:
1. README.md (lines ~92-96) - Main test results section
2. docs/testing.html (lines ~263, ~305) - Detailed compliance info
3. docs/index.html (lines ~167-174) - Homepage test badges

### 7. Commit with Proper Format
```bash
git add <specific-files>  # NEVER use "git add ."
git commit -m "..."       # Use format below
git push origin main
```

## Commit Message Format (REQUIRED)

```
type(scope): Brief description

- Detail 1
- Detail 2

Test results: X% → Y% (+Z pp, +N tests)  # If applicable

All quality checks pass: go vet, staticcheck, gosec

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types:** feat, fix, refactor, test, docs, chore
**Scopes:** rdf, sparql, storage, server

## Code Style

- Use `gofmt` for all formatting
- Wrap errors: `fmt.Errorf("failed to X: %w", err)`
- Add comments for exported functions
- Use descriptive variable names
- Check errors immediately after they occur

## Testing

### W3C Test Suites

#### RDF 1.1 (Near-Complete Compliance)
```bash
# RDF Parsers
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-n-triples   # 100% ✅ (70/70)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-n-quads     # 100% ✅ (87/87)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle      # 100% ✅ (313/313)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml         # 100% ✅ (166/166)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-trig        # 99.7% (355/356, 1 edge case)

# SPARQL 1.1
./test-runner testdata/rdf-tests/sparql/sparql11/syntax-query  # 69.1% (65/94)
./test-runner testdata/rdf-tests/sparql/sparql11/bind          # 70.0% (7/10)
./test-runner testdata/rdf-tests/sparql/sparql11/csv-tsv-res   # 83.3% (5/6)
```

#### RDF 1.2 (Partial Support)
RDF 1.2 introduces quoted triples/reification (`<<s p o>>`), triple terms, and directionality annotations. These features require significant parser changes and are not yet implemented. Current test pass rates:

```bash
# RDF 1.2 Parsers (Excluding New Features)
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-n-triples   # Requires triple terms support
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-n-quads     # 70.9% (112/158) - Requires triple terms
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-turtle      # Requires quoted triples support
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-xml         # Requires directionality support
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-trig        # Requires quoted triples support
```

**Note:** RDF 1.1 compliance is near-complete (99.8% overall). RDF 1.2 features are planned for future implementation.
```

## Common Commands

```bash
# Build
go build ./...
go build -o server ./cmd/server
go build -o test-runner ./cmd/test-runner

# Test
go test ./... -v                  # Run unit tests with verbose output
go test ./pkg/rdf -run TestX      # Run specific test

# Quality
go vet ./...                      # Check for suspicious code
staticcheck ./...                 # Advanced static analysis
gosec -quiet ./...                # Security vulnerability scan

# Git
git status                        # Check working tree
git diff                          # See changes
git log --oneline -10             # Recent commits
git add <specific-files>          # Stage specific files
git commit -m "..."               # Commit with message
git push origin main              # Push to remote
```

## CRITICAL RULES

**YOU MUST:**
- ✅ Run `go fmt ./...` before every commit
- ✅ Pass all three quality checks (vet, staticcheck, gosec)
- ✅ Stage specific files, NEVER use `git add .`
- ✅ Document test result changes in commits
- ✅ Update README.md and docs/ when test results improve
- ✅ Use the commit format above

**YOU MUST NOT:**
- ❌ Commit code that doesn't compile
- ❌ Commit without running quality checks
- ❌ Skip formatting
- ❌ Use `git push --force` to main
- ❌ Commit files with secrets (.env, credentials.json)

**Suppressions (use sparingly):**
- `//lint:ignore U1000 Explanation` - for staticcheck
- `#nosec G304 Justification` - for gosec

## RDF Parsers

### Turtle Parser (pkg/rdf/turtle.go)
- **Dual mode:** Strict N-Triples vs lenient Turtle (controlled by `strictNTriples` flag)
- **Strict N-Triples rules:** No PREFIX/BASE, no abbreviations, absolute IRIs only, limited escape sequences
- **Turtle features:** PREFIX/BASE, property lists (`;`), object lists (`,`), `a` keyword, anonymous blank nodes `[]`, collections `()`, numeric/boolean literals

### N-Quads Parser (pkg/rdf/nquads.go)
- **Strict validation:** IRI character checking, Unicode escapes, language tag validation
- **Graph component:** Optional fourth component (default graph if omitted)

### TriG Parser (pkg/rdf/trig.go)
- **Extends Turtle** with named graph blocks
- **Syntax:** `{ triples }`, `<g> { triples }`, `GRAPH <g> { triples }`, `_:g { triples }`

### RDF/XML Parser (pkg/rdf/rdfxml.go)
- **Features:** rdf:Description, property elements, rdf:about/resource/ID, containers (Bag/Seq/Alt), xml:base, rdf:parseType="Resource"
- **Base URI:** xml:base takes precedence over document base

### Graph Isomorphism (pkg/rdf/isomorphism.go)
- **VF2-inspired** backtracking algorithm for blank node matching
- **Handles:** Both triples and quads, degree-based optimization, early pruning

## SPARQL

### Parser (pkg/sparql/parser/)
- **Query types:** SELECT, CONSTRUCT, ASK, DESCRIBE
- **Patterns:** BGP, OPTIONAL, UNION, MINUS, FILTER, BIND, GRAPH
- **Functions:** 20+ operators and functions (logical, comparison, arithmetic, string, type checking, numeric)

### Optimizer (pkg/sparql/optimizer/)
- **Join ordering** based on pattern specificity
- **Filter push-down**
- **Selectivity scoring:** 0 vars (most specific) to 3 vars (least specific)

### Executor (pkg/sparql/executor/)
- **Stream-based** processing with solution iterators
- **Hash joins** for OPTIONAL
- **Set difference** for MINUS

## Known Limitations

- **Collections with items:** Partial support due to parser architecture
- **Blank node property lists:** Partial support
- **Reification:** Not implemented (rdf:ID on property elements)
- **SPARQL subqueries:** Detected but not parsed
- **VALUES clause:** Not implemented
- **EXISTS/NOT EXISTS:** Parsed but not evaluated
- **Property paths:** Parsed but limited execution

## Useful References

- **W3C SPARQL 1.1:** https://www.w3.org/TR/sparql11-query/
- **W3C RDF 1.1:** https://www.w3.org/TR/rdf11-primer/
- **W3C Test Suites:** https://github.com/w3c/rdf-tests
- **Project Docs:** https://trigodb.com/

## Architecture Notes

- **Storage:** In-memory with three indexes (SPO, POS, OSP)
- **Encoding:** Terms encoded to uint64 for efficient storage
- **Concurrency:** sync.RWMutex for reader/writer locking
- **Test runner:** Converts file paths to W3C canonical URIs

---

**Last Updated:** 2025-01-05 (Phase 3 - Graph Isomorphism & CSV/TSV)

For detailed implementation notes, see code comments and git history.
