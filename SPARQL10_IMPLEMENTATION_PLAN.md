# SPARQL 1.0 Implementation Plan

**Current Status:** 354/450 tests passing (78.7%)
**Remaining:** 96 failing tests (21.3%)
**Last Updated:** 2025-01-20

## Progress Summary

### Completed (Session 8 - Manifest Parser Fix)
- ✅ Fixed manifest parser test detection logic
- ✅ Tests were being assigned wrong action files due to multi-line parsing bug
- ✅ Parser now correctly identifies test definitions by looking for "mf:name"
- ✅ Syntax-sparql1 suite: 69/81 (85.2%) - was 68/80 with wrong files
- **Note:** Test counts changed (469→450) because old parser was miscounting/duplicating tests
- **Test improvement:** More accurate test detection, +1 real test fix (syntax-order-01)

### Completed (Session 7 - OPTIONAL/FILTER Context Binding)
- ✅ Implemented context binding for OPTIONAL/FILTER patterns
- ✅ Filters inside OPTIONAL can now access outer scope variables
- ✅ Added preBindingIterator for variable injection at scan level
- ✅ Created createIteratorWithContext() for context propagation
- ✅ Exported ExtractNumeric() for future value-based comparison
- ✅ Fixed xsd:string normalization (plain literals = xsd:string)
- ✅ Optional-filter suite: 7/9 (77.8%) - was 5/9
- **Test improvement:** 362→364 (+2 tests, +0.4pp)

### Completed (Session 6)
- ✅ DATATYPE() function returns rdf:langString for language-tagged literals
- ✅ LANG() function errors on non-literals (not empty string)
- ✅ Language tag normalization in test result comparison (case-insensitive)
- ✅ Numeric/non-numeric type comparison errors
- ✅ Unknown datatype comparison semantics
- ✅ Type promotion for all 16 XSD numeric types
- ✅ Hash-based encoding for numeric literals (preserves lexical forms)
- ✅ expr-builtin suite: 24/24 (100%)
- ✅ open-eq suite: 11/12 (91.7%)

### Test Suite Status
```
✅ expr-builtin:     24/24  (100.0%)
✅ regex:            All passing
✅ sort:             14/15  (93.3%)
✅ type-promotion:   4/4    (100.0%)
⚠️  open-eq:         11/12  (91.7%)
⚠️  distinct:        7/10   (70.0%)
⚠️  construct:       5/8    (62.5%)
⚠️  optional-filter: 3/6    (50.0%)
⚠️  basic:           35/38  (92.1%)
⚠️  optional:        ~60%
⚠️  graph:           ~40%
```

## Remaining Issues by Category

### 1. OPTIONAL/FILTER Semantics (~25 tests)
**Priority:** HIGH
**Complexity:** MEDIUM-HIGH

**Problem:**
- FILTER outside OPTIONAL should handle unbound variables
- `FILTER(?x < 5)` where ?x is unbound should exclude the binding (error in filter)
- Current implementation may not properly handle filter errors for optional variables

**Tests Failing:**
- OPTIONAL - Outer FILTER
- OPTIONAL - Outer FILTER with BOUND
- dawg-optional-filter-005-simplified
- Complex optional semantics: 1-4
- Optional-filter - 1, 2
- Filter-placement - 2, 3
- Filter-nested - 2

**Implementation Steps:**
1. Review SPARQL 1.0 spec section on OPTIONAL semantics (9.3)
2. Identify where filters are evaluated in our executor
3. Ensure filters on unbound variables from OPTIONAL cause errors
4. Error in filter → binding excluded from results
5. Add test cases for edge cases

**Files to Modify:**
- `pkg/sparql/executor/executor.go` - Optional iterator logic
- `pkg/sparql/executor/executor.go` - Filter iterator logic

### 2. GRAPH Operations (~20 tests)
**Priority:** MEDIUM
**Complexity:** HIGH

**Problem:**
- Named graph operations not fully implemented
- GRAPH keyword queries
- Dataset operations (FROM NAMED)

**Tests Failing:**
- graph-03, graph-04, graph-06, graph-07, graph-08
- Join operator with Graph and Union
- Various dataset tests

**Implementation Steps:**
1. Review current GRAPH implementation in parser and executor
2. Implement proper named graph context tracking
3. Handle FROM NAMED in dataset construction
4. Implement GRAPH pattern matching
5. Test with multi-graph datasets

**Files to Modify:**
- `pkg/sparql/parser/parser.go` - GRAPH pattern parsing
- `pkg/sparql/executor/executor.go` - Graph iterator
- `pkg/store/store.go` - Named graph support

### 3. RDF Collections/Lists (3 tests)
**Priority:** LOW-MEDIUM
**Complexity:** MEDIUM

**Problem:**
- RDF collection syntax `(item1 item2 item3)` not fully supported
- Collection expansion to rdf:first/rdf:rest triples incomplete

**Tests Failing:**
- Basic - List 2
- Basic - List 3
- Basic - List 4

**Implementation Steps:**
1. Review Turtle parser collection handling
2. Identify where collection expansion fails
3. Complete rdf:first/rdf:rest/rdf:nil expansion
4. Test with nested collections

**Files to Modify:**
- `pkg/rdf/turtle.go` - Collection parsing (lines ~1400-1500)

### 4. Date/Time Comparisons (2+ tests)
**Priority:** LOW
**Complexity:** HIGH

**Problem:**
- XSD date/dateTime value-based comparison not implemented
- Currently using lexical comparison only
- Need to parse and normalize dates for comparison

**Tests Failing:**
- date-2
- Possibly others in type-promotion

**Implementation Steps:**
1. Add date/time parsing library or implement parser
2. Implement value-based comparison for xsd:date, xsd:dateTime
3. Handle timezone normalization (Z, +00:00, etc.)
4. Add to extractNumeric() or create separate comparison path

**Files to Modify:**
- `pkg/sparql/evaluator/operators.go` - Add date comparison
- Potentially add new file `pkg/sparql/evaluator/datetime.go`

### 5. CONSTRUCT Edge Cases (3 tests)
**Priority:** LOW
**Complexity:** LOW-MEDIUM

**Problem:**
- Some CONSTRUCT tests failing, likely result format issues
- Reification tests definitely need reification support (skip for now)

**Tests Failing:**
- dawg-construct-optional (investigate format issue)
- dawg-construct-reification-1 (skip - reification not required)
- dawg-construct-reification-2 (skip - reification not required)

**Implementation Steps:**
1. Debug dawg-construct-optional test specifically
2. Check if result format is correct (Turtle output)
3. Verify blank node handling in CONSTRUCT
4. Skip reification tests (not SPARQL 1.0 requirement)

**Files to Modify:**
- `pkg/sparql/executor/executor.go` - CONSTRUCT result handling
- `pkg/server/results/` - Result serialization

### 6. DISTINCT with UNION (3 tests)
**Priority:** MEDIUM
**Complexity:** LOW-MEDIUM

**Problem:**
- DISTINCT not properly deduplicating across UNION branches
- May be related to how we build binding keys

**Tests Failing:**
- SELECT DISTINCT *
- Strings: Distinct
- All: Distinct

**Implementation Steps:**
1. Review UNION implementation in executor
2. Check if DISTINCT is applied before or after UNION merge
3. Verify binding key generation includes all variables
4. Test with SELECT * to ensure all variables included

**Files to Modify:**
- `pkg/sparql/executor/executor.go` - DISTINCT iterator (lines ~990-1020)
- `pkg/sparql/executor/executor.go` - UNION iterator

### 7. Remaining open-eq/open-cmp Tests (5 tests)
**Priority:** LOW
**Complexity:** MEDIUM

**Problem:**
- Edge cases in type comparison
- Possibly related to numeric type promotion or equality

**Tests Failing:**
- open-eq-08, open-eq-10, open-eq-11, open-eq-12
- open-cmp-01, open-cmp-02

**Implementation Steps:**
1. Analyze each failing test individually
2. Likely related to complex type interactions
3. May need refinements to type promotion logic

**Files to Modify:**
- `pkg/sparql/evaluator/operators.go` - Comparison operators

### 8. BOUND Function Edge Cases (2 tests)
**Priority:** LOW
**Complexity:** LOW

**Problem:**
- BOUND function implementation exists but some tests fail
- Likely related to OPTIONAL/FILTER interaction

**Tests Failing:**
- dawg-bound-query-001
- OPTIONAL - Outer FILTER with BOUND

**Implementation Steps:**
1. Debug specific test cases
2. Verify BOUND works with OPTIONAL correctly
3. Check if filter evaluation order matters

**Files to Modify:**
- `pkg/sparql/evaluator/functions.go` - BOUND function (lines ~86-99)

### 9. Syntax Tests (1 test)
**Priority:** VERY LOW
**Complexity:** LOW

**Problem:**
- syntax-order-01.rq fails but should parse
- Likely test runner issue, not parser issue

**Tests Failing:**
- syntax-order-01.rq

**Implementation Steps:**
1. Check if parser actually accepts the query
2. May be test runner expecting different behavior
3. Low priority - doesn't affect actual functionality

## Implementation Priority Order

### Phase 1: High-Value, Medium-Complexity (Target: +15-20 tests)
1. **DISTINCT with UNION** - Quick win, 3 tests
2. **OPTIONAL/FILTER semantics** - Big impact, ~25 tests
3. **BOUND edge cases** - Related to OPTIONAL, 2 tests

**Expected Result:** ~385/469 (82%)

### Phase 2: Medium-Value, Medium-Complexity (Target: +10-15 tests)
4. **Remaining open-eq/open-cmp** - 5 tests
5. **RDF Collections** - 3 tests
6. **CONSTRUCT edge cases** - 1 test (skip reification)

**Expected Result:** ~395/469 (84%)

### Phase 3: High-Complexity or Low-Priority (Target: +5-10 tests)
7. **Date/Time comparisons** - 2+ tests
8. **GRAPH operations** - ~20 tests (complex, may not all pass)
9. **Syntax test** - 1 test

**Expected Result:** ~410/469 (87%)

## Notes

- **RDF 1.1 tests must remain at 100%** throughout all changes
- Run full quality checks (go vet, staticcheck, gosec) before each commit
- Update README.md and docs/ when test results improve significantly
- Focus on fixes that benefit multiple tests
- GRAPH operations are complex - may defer to future

## Development Workflow Reminder

```bash
# 1. Format & Build
go fmt ./...
go build ./...

# 2. Unit Tests
go test ./...

# 3. Quality Checks (ALL must pass)
go vet ./...
staticcheck ./...
gosec -quiet ./...

# 4. SPARQL 1.0 Tests
go build -o test-runner ./cmd/test-runner
./test-runner testdata/rdf-tests/sparql/sparql10

# 5. Verify RDF 1.1 (must stay 100%)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle

# 6. Commit
git add <specific-files>
git commit -m "..."
git push origin main
```

## Resources

- **SPARQL 1.0 Spec:** https://www.w3.org/TR/rdf-sparql-query/
- **Test Suite:** https://github.com/w3c/rdf-tests
- **Key Spec Sections:**
  - 9.3: OPTIONAL pattern matching
  - 11.4: Filter constraints
  - 12: Formal semantics
  - 17.4: Functions and operators

---

**Goal:** Reach 90%+ compliance (420+ tests passing) while maintaining RDF 1.1 at 100%
