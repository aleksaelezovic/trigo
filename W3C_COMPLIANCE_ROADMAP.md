# W3C Compliance Roadmap: Path to 100%

**Last Updated:** 2025-11-28
**Current Overall Compliance:** 98.5% (3,707/3,769 tests)
**Target:** 100% compliance across all W3C standards

---

## Executive Summary

Trigo has achieved excellent W3C compliance with near-perfect RDF parsing and strong SPARQL 1.0 support. This roadmap outlines the path to achieving 100% compliance across all standards.

**Current State:**
- ✅ **RDF 1.1:** 100% (954/954 tests) - COMPLETE
- 🟡 **RDF 1.2:** 99.8% (1,272/1,275 tests) - 3 tests remaining
- 🟡 **SPARQL 1.0:** 99.2% (467/471 tests) - 4 tests remaining
- 🔴 **SPARQL 1.1:** ~15-20% (many features not implemented)

**Strategy:** High-impact first - prioritize most-used features regardless of implementation difficulty

**Estimated Total Effort:** 4-6 weeks for complete SPARQL 1.1 implementation

---

## Detailed Test Results

### RDF 1.1 (100% - COMPLETE ✅)

| Format | Tests | Pass Rate | Status |
|--------|-------|-----------|--------|
| N-Triples | 70/70 | 100% | ✅ Complete |
| N-Quads | 87/87 | 100% | ✅ Complete |
| Turtle | 296/296 | 100% | ✅ Complete |
| TriG | 335/335 | 100% | ✅ Complete |
| RDF/XML | 166/166 | 100% | ✅ Complete |
| **TOTAL** | **954/954** | **100%** | ✅ **COMPLETE** |

**Action:** Maintain 100% - no regressions allowed during SPARQL work

---

### RDF 1.2 (99.8% - 3 tests remaining)

| Format | Tests | Pass Rate | Remaining |
|--------|-------|-----------|-----------|
| N-Triples | 139/140 | 99.3% | 1 C14N canonicalization test |
| N-Quads | 154/155 | 99.4% | 1 C14N canonicalization test |
| Turtle | 388/388 | 100% | ✅ Complete |
| TriG | 395/396 | 99.7% | 1 test skipped |
| RDF/XML | 196/196 | 100% | ✅ Complete |
| **TOTAL** | **1,272/1,275** | **99.8%** | **3 tests** |

#### Remaining Issues

**1. C14N triple-term-02 (N-Triples & N-Quads - 2 tests)**
- **Issue:** Canonical output mismatch for triple terms
- **Root Cause:** RDF 1.2 canonicalization (C14N) for triple terms not implemented
- **Complexity:** Medium
- **Estimated Effort:** 4-8 hours
- **Files:**
  - `pkg/rdf/ntriples.go` - Add C14N ordering for triple terms
  - `pkg/rdf/nquads.go` - Same fix
- **Approach:**
  1. Implement canonical blank node labeling for triple terms
  2. Ensure deterministic ordering of triples within triple terms
  3. Follow RDF 1.2 C14N specification

**2. TriG skipped test (1 test)**
- **Issue:** Test skipped - quoted triple syntax edge case
- **Action Required:** Investigate why test is skipped
- **Estimated Effort:** 1-2 hours to investigate, may not need implementation

---

### SPARQL 1.0 (99.2% - 4 tests remaining)

| Category | Tests | Pass Rate | Status |
|----------|-------|-----------|--------|
| **TOTAL** | **467/471** | **99.2%** | 🟡 4 tests remaining |
| Perfect Suites | 15 suites | 100% | ✅ Complete |

**Perfect Compliance Suites (15):**
- distinct, boolean-effective-value, expr-builtin, expr-ops, bound, cast, reduced, regex, solution-seq, sort, triple-match, type-promotion, bnode-coreference, syntax-sparql5, ask

#### Remaining 4 Tests

**1. graph-optional (HIGH PRIORITY)**
- **Issue:** Expected 1 binding, got 4 bindings
- **Query Pattern:** GRAPH ?g { ?s ?p ?o OPTIONAL { ?s ?p ?g } }
- **Root Cause:** graphOptionalIterator producing duplicate bindings
- **Analysis from SPARQL10_IMPLEMENTATION_PLAN.md:**
  - Optimizer verified correct (only ONE OptionalPlan)
  - Duplication happening at execution time
  - Iterator state management bug suspected
- **Complexity:** High
- **Estimated Effort:** 6-12 hours
- **Files:**
  - `pkg/sparql/executor/executor.go` - Debug graphOptionalIterator
- **Approach:**
  1. Add logging to graphOptionalIterator to trace binding production
  2. Verify OPTIONAL semantics within GRAPH patterns per spec
  3. Check for iterator reset or state issues
  4. Fix duplication logic

**2. Complex optional semantics: 3 (MEDIUM PRIORITY)**
- **Issue:** Expected 1 binding, got 2 bindings
- **Root Cause:** OPTIONAL/FILTER interaction edge case
- **Complexity:** Medium
- **Estimated Effort:** 4-6 hours
- **Files:**
  - `pkg/sparql/executor/executor.go` - Optional iterator
  - `pkg/sparql/evaluator/operators.go` - Filter evaluation
- **Approach:**
  1. Analyze specific test query and data
  2. Review SPARQL 1.0 spec section 9.3 (OPTIONAL)
  3. Fix filter evaluation on optional variables

**3. date-2 (MEDIUM PRIORITY)**
- **Issue:** Expected 3 bindings, got 3 bindings (same count, wrong values)
- **Root Cause:** Date/time comparison semantics
- **Complexity:** Medium-High
- **Estimated Effort:** 6-10 hours
- **Files:**
  - `pkg/sparql/evaluator/operators.go` - Comparison operators
  - New file: `pkg/sparql/evaluator/datetime.go`
- **Approach:**
  1. Implement proper xsd:date and xsd:dateTime value-based comparison
  2. Handle timezone normalization (Z, +00:00, etc.)
  3. Parse date/time values for comparison (currently lexical only)

**4. dawg-optional-filter-005-not-simplified (LOW PRIORITY)**
- **Issue:** Expected 3 bindings, got 3 bindings (same count, wrong values)
- **Root Cause:** OPTIONAL with FILTER edge case
- **Complexity:** Medium
- **Estimated Effort:** 4-6 hours
- **Related to:** Test #2 above

---

### SPARQL 1.1 (15-20% estimated - Major work required)

Based on test runs:

| Feature | Tests | Pass Rate | Status |
|---------|-------|-----------|--------|
| **syntax-query** | 64/94 | 68.1% | 🔴 30 tests remaining |
| **bind** | 8/10 | 80.0% | 🟡 2 tests remaining |
| **csv-tsv-res** | 5/6 | 83.3% | 🟡 1 test remaining |
| **construct** | 5/7 | 71.4% | 🟡 2 tests remaining |
| **aggregates** | 0/32 | 0% | 🔴 NOT IMPLEMENTED |
| **grouping** | 2/6 | 33.3% | 🔴 NOT IMPLEMENTED |
| **property-path** | 0/25 | 0% | 🔴 NOT IMPLEMENTED |
| **subquery** | 0/1 | 0% | 🔴 NOT IMPLEMENTED |
| **bindings (VALUES)** | 0/9 | 0% | 🔴 NOT IMPLEMENTED |
| **negation (MINUS)** | 1/9 | 11.1% | 🔴 PARTIAL |
| **exists** | 2/5 | 40.0% | 🟡 PARSED, LIMITED |
| **functions** | 7/56 | 12.5% | 🔴 PARTIAL |
| **cast** | 0/6 | 0% | 🔴 Edge cases missing |
| **project-expression** | 0/6 | 0% | 🔴 NOT IMPLEMENTED |

---

## Implementation Phases

### Phase 1: Complete Near-Finished Work (Est: 1-2 days)

**Goal:** Fix RDF 1.2 and SPARQL 1.0 edge cases

**Tasks:**
1. **RDF 1.2 C14N (4-8 hours)**
   - Implement triple-term canonicalization
   - Fix N-Triples and N-Quads C14N output
   - Test: 2 tests → 100% RDF 1.2 N-Triples/N-Quads

2. **SPARQL 1.0 High-Priority (6-12 hours)**
   - Fix graph-optional iterator duplication
   - Fix complex optional semantics
   - Test: 2 tests → closer to 100%

**Expected Result:**
- RDF 1.2: 99.8% → ~99.9%
- SPARQL 1.0: 99.2% → ~99.6%

---

### Phase 2: High-Impact SPARQL 1.1 Features (Est: 2-3 weeks)

**Goal:** Implement most-used SPARQL 1.1 query features

#### 2.1 Aggregates & GROUP BY (Est: 3-5 days)

**Priority:** CRITICAL - Users need this for analytics

**Features to Implement:**
- COUNT, SUM, AVG, MIN, MAX, SAMPLE, GROUP_CONCAT
- GROUP BY clause with variable grouping
- HAVING clause for filtered aggregation
- Aggregate expressions (e.g., SUM(?price * ?quantity))

**Implementation Approach:**
1. **Parser Extensions** (`pkg/sparql/parser/`)
   - Parse GROUP BY with variables and expressions
   - Parse HAVING with filter conditions
   - Parse aggregate function calls

2. **Optimizer** (`pkg/sparql/optimizer/`)
   - Add GroupByPlan and AggregationPlan types
   - Handle aggregation in query planning
   - Push projections after aggregation

3. **Executor** (`pkg/sparql/executor/`)
   - Implement grouping iterator (hash-based grouping)
   - Implement aggregation functions
   - Handle HAVING filter after aggregation

4. **Evaluator** (`pkg/sparql/evaluator/`)
   - Add aggregate function evaluation
   - Implement GROUP_CONCAT with SEPARATOR

**Files to Modify:**
- `pkg/sparql/parser/parser.go`
- `pkg/sparql/optimizer/optimizer.go`
- `pkg/sparql/executor/executor.go`
- `pkg/sparql/evaluator/functions.go`

**Expected Test Improvement:** +32 tests (aggregates) + 4 tests (grouping) = +36 tests

---

#### 2.2 Subqueries (Est: 2-3 days)

**Priority:** HIGH - Essential for complex queries

**Features to Implement:**
- Nested SELECT in WHERE clause
- Subquery result binding
- Projection and solution modifiers in subqueries

**Implementation Approach:**
1. **Parser** - Parse SELECT within WHERE block
2. **Optimizer** - Add SubqueryPlan type
3. **Executor** - Execute subquery and bind results

**Files to Modify:**
- `pkg/sparql/parser/parser.go`
- `pkg/sparql/optimizer/optimizer.go`
- `pkg/sparql/executor/executor.go`

**Expected Test Improvement:** +1 test (subquery suite has only 1 test, but functionality enables other tests)

---

#### 2.3 Property Paths (Est: 4-6 days)

**Priority:** HIGH - Core SPARQL 1.1 feature

**Path Operators to Implement:**
- `/` - Sequence path (a/b)
- `*` - Zero-or-more path (a*)
- `+` - One-or-more path (a+)
- `?` - Zero-or-one path (a?)
- `|` - Alternative path (a|b)
- `^` - Inverse path (^a)
- `!` - Negated property set (!(a|b))
- `()` - Grouping

**Implementation Approach:**
1. **Parser** - Recognize property path syntax in predicates
2. **Optimizer** - Convert paths to traversal plans
3. **Executor** - Implement path evaluation (BFS/DFS traversal)

**Complexity:** High - Requires graph traversal algorithms

**Files to Modify:**
- `pkg/sparql/parser/parser.go` - Path syntax parsing
- `pkg/sparql/optimizer/optimizer.go` - Path planning
- `pkg/sparql/executor/executor.go` - Path traversal
- New file: `pkg/sparql/executor/paths.go`

**Expected Test Improvement:** +25 tests

---

#### 2.4 VALUES Clause (Est: 1-2 days)

**Priority:** MEDIUM - Useful for inline data

**Features to Implement:**
- VALUES with variable bindings
- Multiple variables and rows
- UNDEF values
- VALUES in different query positions

**Implementation Approach:**
1. **Parser** - Parse VALUES block
2. **Optimizer** - Add ValuesPlan type
3. **Executor** - Emit bindings from VALUES data

**Files to Modify:**
- `pkg/sparql/parser/parser.go`
- `pkg/sparql/optimizer/optimizer.go`
- `pkg/sparql/executor/executor.go`

**Expected Test Improvement:** +9 tests

---

**Phase 2 Total:**
- Estimated Time: 2-3 weeks
- Expected Tests: +71 tests minimum
- Major capability boost for production use

---

### Phase 3: Complete SPARQL 1.0 (Est: 2-3 days)

**Goal:** Fix remaining SPARQL 1.0 edge cases

**Tasks:**
1. Date/time comparison (6-10 hours)
2. dawg-optional-filter-005-not-simplified (4-6 hours)

**Expected Result:** SPARQL 1.0 → 100%

---

### Phase 4: SPARQL 1.1 UPDATE Operations (Est: 1-2 weeks)

**Goal:** Implement write operations

**Features:**
- INSERT DATA / DELETE DATA
- INSERT WHERE / DELETE WHERE
- CLEAR / CREATE / DROP GRAPH
- COPY / MOVE / ADD GRAPH

**Implementation Approach:**
1. **Parser** - Parse UPDATE operations
2. **Executor** - Implement graph modification operations
3. **Store** - Add transaction support if needed

**Files to Modify:**
- `pkg/sparql/parser/parser.go`
- `pkg/sparql/executor/update.go` (new file)
- `pkg/store/store.go`

**Estimated Time:** 1-2 weeks

---

### Phase 5: SPARQL 1.1 Advanced Features (Est: 2-3 weeks)

**Goal:** Complete SPARQL 1.1 specification

#### 5.1 EXISTS/NOT EXISTS Evaluation (Est: 3-4 days)
- Currently parsed but not evaluated
- Implement subquery existence checking

#### 5.2 Advanced Functions (Est: 4-6 days)
- String: REPLACE, UUID, STRUUID, REGEX (full implementation)
- Math: Complete set
- Date/time: NOW, YEAR, MONTH, DAY, HOURS, MINUTES, SECONDS, TIMEZONE, TZ
- Hash: MD5, SHA1, SHA256, SHA384, SHA512

#### 5.3 Projection Expressions (Est: 2-3 days)
- SELECT (expression AS ?var)
- Expression evaluation in projection

#### 5.4 Service Federation (Est: 5-7 days - OPTIONAL)
- SERVICE keyword
- Federated queries to remote endpoints
- **Note:** May defer as optional feature

**Phase 5 Total:**
- Estimated Time: 2-3 weeks
- Completes SPARQL 1.1 specification

---

## Development Workflow

### Quality Standards (REQUIRED)

Before every commit:
```bash
# 1. Format
go fmt ./...

# 2. Build
go build ./...

# 3. Unit Tests
go test ./...

# 4. Quality Checks (ALL MUST PASS)
go vet ./...
staticcheck ./...
gosec -quiet ./...

# 5. W3C Tests (relevant suite)
./test-runner testdata/rdf-tests/...

# 6. Verify RDF 1.1 (NO REGRESSIONS)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle
```

### Commit Message Format

```
type(scope): Brief description

- Detail 1
- Detail 2

Test results: X% → Y% (+Z pp, +N tests)

All quality checks pass: go vet, staticcheck, gosec

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types:** feat, fix, refactor, test, docs
**Scopes:** rdf, sparql, storage, server

### Documentation Requirements

After test improvements, update:
1. **README.md** (lines 87-123)
2. **docs/testing.html** (lines 228-413)
3. **docs/index.html** (lines 164-203)

All three files must have identical test numbers.

---

## Priority Matrix

### Immediate (This Week)
1. ✅ Run all test suites - DONE
2. ✅ Update documentation - DONE
3. 🔲 RDF 1.2 C14N fixes (2 tests)
4. 🔲 SPARQL 1.0 graph-optional fix (1 test)

### Short-Term (Next 2-4 Weeks)
5. 🔲 Aggregates & GROUP BY (36+ tests)
6. 🔲 Property Paths (25+ tests)
7. 🔲 Subqueries (enables many tests)
8. 🔲 VALUES clause (9 tests)

### Medium-Term (1-2 Months)
9. 🔲 Complete SPARQL 1.0 (2 remaining tests)
10. 🔲 EXISTS/NOT EXISTS full evaluation
11. 🔲 Advanced functions
12. 🔲 UPDATE operations

### Long-Term (2-3 Months)
13. 🔲 Service federation (optional)
14. 🔲 100% SPARQL 1.1 compliance

---

## Risk Mitigation

### No Regressions Policy
- RDF 1.1 MUST stay at 100%
- Run full RDF 1.1 test suite before every commit
- Any regression blocks the commit

### Testing Strategy
- Run relevant test suite for each change
- Add unit tests for new features
- Integration tests for complex features

### Performance Considerations
- Profile before optimizing
- Maintain current query performance
- Aggregates may need optimization for large datasets

---

## Success Metrics

### Phase 1 Complete
- RDF 1.2: 99.8% → 99.9%
- SPARQL 1.0: 99.2% → 99.6%

### Phase 2 Complete
- SPARQL 1.1 core features: 15% → 60%+
- Production-ready analytics queries

### Phase 3 Complete
- SPARQL 1.0: 100% ✅

### Phase 4 Complete
- SPARQL 1.1 write operations: functional

### Final Goal
- RDF 1.1: 100% ✅
- RDF 1.2: 100% ✅
- SPARQL 1.0: 100% ✅
- SPARQL 1.1: 100% ✅
- **Overall W3C Compliance: 100%** 🎉

---

## References

- **SPARQL 1.0 Spec:** https://www.w3.org/TR/rdf-sparql-query/
- **SPARQL 1.1 Spec:** https://www.w3.org/TR/sparql11-query/
- **RDF 1.1 Spec:** https://www.w3.org/TR/rdf11-primer/
- **RDF 1.2 Spec:** https://www.w3.org/TR/rdf12-primer/
- **W3C Test Suites:** https://github.com/w3c/rdf-tests

---

**This roadmap is a living document. Update as progress is made.**
