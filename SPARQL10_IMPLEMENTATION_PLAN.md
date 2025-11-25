# SPARQL 1.0 Implementation Plan

**Current Status:** 432/471 tests passing (91.7%)
**Remaining:** 39 failing tests (8.3%)
**Last Updated:** 2025-11-25 (Session 11 Part 3 - Equality/Comparison Investigation)

## Progress Summary

### Completed (Session 11 Part 3 - 2025-11-25 - Equality/Comparison Semantics)
- 🔍 Investigated SPARQL equality and comparison operator semantics
- ✅ Added validation for invalid numeric literals (e.g., "xyz"^^xsd:integer)
- ✅ Added RDF term equality check for identical invalid literals
- ✅ Added isNumericDatatype() helper function
- ✅ Prevented comparison (<, >, etc.) of non-literal terms
- ✅ Added error on unknown datatype comparisons
- ✅ Implemented lexical comparison for invalid numeric literals with same datatype
- **Improvements achieved:**
  - open-eq-10: 59 → 51 bindings (target: 52, need 1 more) ✨
  - open-eq-11: 59 → 51 bindings (target: 52, need 1 more) ✨
  - open-eq-08: 48 → 40 bindings (target: 42, need 2 more)
- **Remaining issues (complex semantics, require deeper investigation):**
  - open-eq tests: Off by 1-2 bindings each - subtle equality edge cases remain
  - open-cmp tests: 10x and 20x expected bindings - blank node property list Cartesian product
  - date-2: Date/time comparison issue
  - open-eq-12: OPTIONAL with FILTER interaction
- **Test status:** 432/471 (91.7%) - no regressions
- **🎯 Key findings:**
  - SPARQL != operator errors on incompatible types (per spec), which filters out bindings
  - Invalid literal handling is implementation extension point per SPARQL spec
  - Adopted Neptune-style approach: lexical comparison for invalid literals
  - Comparison tests suggest Cartesian product in blank node property list execution
- **Commits:**
  - `refactor(sparql): Improve equality and comparison operator semantics`
  - `refactor(sparql): Improve invalid numeric literal equality handling`
- **Next steps:** Blank node property list execution, RDF collection matching, dataset loading

### Completed (Session 11 Part 2 - 2025-11-25 - Dataset and GRAPH Parsing)
- ✅ Extended FROM and FROM NAMED clauses to accept prefixed names
- ✅ Extended GRAPH patterns to accept prefixed names
- ✅ Added parseIRIOrPrefixedName() helper for flexible IRI parsing
- ✅ FROM/FROM NAMED now accept both <http://...> and :prefixedName
- ✅ GRAPH patterns now accept IRI, prefixed name, or variable
- ✅ All 3 dataset/GRAPH syntax tests now passing:
  - syntax-dataset-03.rq (multiple FROM NAMED with prefixes) ✅
  - syntax-dataset-04.rq (mixed FROM and FROM NAMED) ✅
  - syntax-graph-02.rq (GRAPH with prefixed name) ✅
- **Test improvement:** 429→432 (+3 tests, +0.6pp)
- **🎯 Impact:** Proper SPARQL dataset and GRAPH parsing per specification
- **Tests remaining to 100%:** 39 tests (8.3%)
- **Note:** Dataset evaluation tests still failing (require executor implementation)

### Completed (Session 11 Part 1 - 2025-11-25 - Blank Node Scope Validation)
- ✅ Implemented blank node label scope validation in SPARQL parser
- ✅ Blank node labels cannot cross basic graph pattern boundaries
- ✅ Added currentBGPBlankNodes and allBlankNodes tracking maps
- ✅ OPTIONAL, UNION, GRAPH, MINUS, nested {} create scope boundaries
- ✅ Each nested pattern gets isolated scope with save/restore mechanism
- ✅ parseBlankNode() validates against cross-scope label usage
- ✅ All 8 blank node scope tests now passing:
  - syn-blabel-cross-graph-bad ✅
  - syn-blabel-cross-optional-bad ✅
  - syn-blabel-cross-union-bad ✅
  - syn-bad-34 (nested group) ✅
  - syn-bad-35 ✅
  - syn-bad-36 (UNION branches) ✅
  - syn-bad-37 ✅
  - syn-bad-GRAPH-breaks-BGP ✅
- **Test improvement:** 421→429 (+8 tests, +1.7pp)
- **🎯 Impact:** Proper SPARQL blank node scoping per specification
- **Tests remaining to 100%:** 42 tests (8.9%)

### Completed (Session 10 Continued Part 5 - Short-Circuit Evaluation for OR/AND)
- ✅ Implemented short-circuit evaluation for logical OR operator
- ✅ Implemented short-circuit evaluation for logical AND operator
- ✅ OR now skips right operand if left is true (critical for !bound(?x) || ?x = 5)
- ✅ AND now skips right operand if left is false
- ✅ Fixed BOUND function interaction with OPTIONAL patterns
- ✅ "OPTIONAL - Outer FILTER with BOUND" test now passes
- ✅ optional-filter suite: 3/5 → 4/5 (60.0% → 80.0%)
- **Test improvement:** 374→375 (+1 test, +0.2pp)
- **🎯 Impact:** Correct SPARQL logical operator semantics with proper short-circuiting

### Completed (Session 10 Continued Part 4 - Nested Group FILTER Scoping)
- ✅ Fixed FILTER variable scoping in nested basic graph patterns
- ✅ Added EmptyPlan type for patterns with only FILTERs (no triples)
- ✅ Empty patterns now correctly produce single empty binding per SPARQL spec
- ✅ FILTERs in nested `{ }` groups can no longer see outer variables
- ✅ Filter-nested - 2 test now passes
- ✅ algebra suite: 10/13 → 11/13 (84.6%)
- **Test improvement:** 373→374 (+1 test, +0.2pp)
- **🎯 Impact:** Correct SPARQL nested group semantics for FILTER evaluation

### Completed (Session 10 Continued Part 3 - FILTER Scoping Fix)
- ✅ Fixed FILTER scoping to apply to entire basic graph pattern
- ✅ FILTERs now collected and applied after triples/BINDs per SPARQL algebra
- ✅ FILTER position before patterns no longer causes failures
- ✅ algebra suite: 8/13 → 10/13 (76.9%)
- **Test improvement:** 371→373 (+2 tests, +0.5pp)
- **🎯 Impact:** Correct SPARQL FILTER scoping within basic graph patterns

### Completed (Session 10 Continued Part 2 - OPTIONAL/FILTER Ordering)
- ✅ Fixed FILTER placement after OPTIONAL patterns
- ✅ Extended PatternElement to include GraphPattern field
- ✅ Parser now adds OPTIONAL/UNION/MINUS/GRAPH to Elements for correct ordering
- ✅ Optimizer processes patterns in textual order
- ✅ "OPTIONAL - Outer FILTER" test now passes
- **Test improvement:** Maintains 371/450 but fixes critical semantics issue
- **🎯 Impact:** Correct SPARQL FILTER/OPTIONAL execution order per spec

### Completed (Session 10 Continued - UNION Optimizer Fix)
- ✅ Fixed UNION pattern optimization for multi-way UNIONs
- ✅ Added optimizeUnionPattern() to build binary UnionPlan trees
- ✅ UNION patterns now properly processed as alternation, not join
- ✅ Distinct suite: 9/10 (90.0%) - was 8/10
- **Test improvement:** 369→371 (+2 tests, +0.4pp)
- **🎉 Fixed:** "SELECT DISTINCT *" with UNION now working!

### Completed (Session 10 - UNION Chaining)
- ✅ Fixed chained UNION operations (A UNION B UNION C)
- ✅ UNION now supports multiple consecutive operations per SPARQL grammar
- ✅ Syntax-sparql1 suite: 81/81 (100.0%) ⭐ **COMPLETE!**
- **Test improvement:** 366→369 (+3 tests, +0.7pp)
- **🎉 Milestone:** 100% syntax-sparql1 compliance achieved!

### Completed (Session 9 Continued - String Literals & Prefixed Names)
- ✅ Added dot support in prefixed names (x.y:, :a.b)
- ✅ Implemented escape sequences in triple-quoted strings (\""")
- ✅ Trailing semicolons before graph pattern keywords (OPTIONAL, UNION, etc.)
- ✅ Syntax-sparql1 suite: 79/81 (97.5%) - was 77/81
- **Test improvement:** 363→366 (+3 tests, +0.6pp)
- **Session 9 total:** 355→366 (+11 tests, +2.6pp)

### Completed (Session 9 - Parser Fixes: Function Calls & Blank Nodes)
- ✅ Fixed prefixed function names in FILTER expressions (:myFunc)
- ✅ Implemented standalone blank node property lists ([:p :q])
- ✅ Added support for nested collections (( ( ?z ) ))
- ✅ Syntax-sparql1 suite: 76/81 (93.8%) - was 70/81
- **Test improvement:** 355→363 (+8 tests, +1.8pp)
- **🎉 Milestone:** Passed 80% compliance threshold!

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
✅ syntax-sparql1:   81/81  (100.0%) ⭐
✅ expr-builtin:     24/24  (100.0%)
✅ regex:            All passing
✅ type-promotion:   4/4    (100.0%)
✅ sort:             14/15  (93.3%)
⚠️  distinct:        9/10   (90.0%) ⬆ was 8/10
⚠️  open-eq:         11/12  (91.7%)
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
**Complexity:** MEDIUM-HIGH

**Problem:**
- Collection syntax `(1)`, `(?v)`, `(?v ?w)` in SPARQL queries not matching data
- Both Turtle and SPARQL parsers correctly expand collections to rdf:first/rdf:rest
- Issue is with blank node unification during pattern matching

**Investigation (Session 10):**
- ✅ Turtle parser correctly expands `:x :list1 ("1"^^xsd:integer)` to rdf:first/rdf:rest triples
- ✅ SPARQL parser correctly expands `:x ?p (?v)` to pattern with rdf:first/rdf:rest
- ❌ Matching fails - returns 0 bindings instead of expected results
- Root cause: Blank nodes from collections not unifying correctly during join

**Tests Failing:**
- Basic - List 2: `:x ?p (1)` - constant in collection
- Basic - List 3: `:x ?p (?v)` - variable in collection (should bind ?v to list element)
- Basic - List 4: `:x ?p (?v ?w)` - two variables in collection

**Implementation Steps:**
1. Debug blank node matching in join operations
2. Ensure blank nodes from extraTriples can unify with data blank nodes
3. May need to implement blank node renaming/canonicalization
4. Test with variables in collections

**Files to Investigate:**
- `pkg/sparql/executor/executor.go` - Join/matching logic
- `pkg/store/store.go` - Triple matching with blank nodes

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

### 6. DISTINCT with UNION
**Status:** ✅ **MOSTLY COMPLETED** - 9/10 tests passing (90%)

**Fixed:**
- ✅ UNION patterns now properly optimized as alternation (not join)
- ✅ Added `optimizeUnionPattern()` to build binary UnionPlan trees
- ✅ "SELECT DISTINCT *" now working correctly

**Remaining Issue:**
- ❌ "Strings: Distinct" - Result serialization issue with plain literals
  - Expected: `""`, `"ABC"`
  - Actual: `""^^<http://www.w3.org/2001/XMLSchema#string>`, `"ABC"^^<http://www.w3.org/2001/XMLSchema#string>`
  - This is a result format issue, not a DISTINCT/UNION logic issue

**Files Modified:**
- `pkg/sparql/optimizer/optimizer.go` - Added optimizeUnionPattern() (lines ~331-374)

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

### 9. Syntax Tests
**Status:** ✅ **COMPLETED** - 100% compliance (81/81)

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
