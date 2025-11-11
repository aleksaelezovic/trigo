# RDF 1.2 Implementation Plan for Trigo

**Created**: 2025-01-07
**Target**: 100% W3C RDF 1.2 Test Compliance
**Current Status**: 75.6% (1,028/1,360 tests passing)

---

## Executive Summary

RDF 1.2 introduces three major features that are causing test failures:

1. **Quoted Triples (Triple Terms)**: Triples that can be used as subjects or objects using `<< s p o >>` syntax
2. **Annotation Syntax**: Syntactic sugar for metadata using `{| ... |}`
3. **Language Direction Tags**: Bidirectional text support using `@lang--dir` syntax

### Current Test Results

| Parser | Current | Target | Tests Passing |
|--------|---------|--------|---------------|
| N-Triples | 60.0% | 95%+ | 87/145 → 138+/145 |
| N-Quads | 61.6% | 95%+ | 101/164 → 156+/164 |
| Turtle | 79.5% | 98%+ | 329/414 → 405+/414 |
| TriG | 80.6% | 98%+ | 348/432 → 423+/432 |
| RDF/XML | 79.5% | 95%+ | 163/205 → 195+/205 |
| **TOTAL** | **75.6%** | **97%+** | **1,028/1,360 → 1,317+/1,360** |

### Estimated Effort

- **Total Hours**: 130-172 hours
- **Timeline**: 4-6 weeks (single developer)
- **Complexity**: High (new term types, recursive parsing, storage changes)

---

## 1. RDF Data Model Changes

### 1.1 New Term Type: QuotedTriple

**File**: `pkg/rdf/term.go`

**Implementation**:
```go
// Add new TermType constant
const (
    TermTypeIRI TermType = iota
    TermTypeBlankNode
    TermTypeLiteral
    TermTypeQuotedTriple  // NEW
)

// New type for quoted triples
type QuotedTriple struct {
    Subject   Term
    Predicate Term
    Object    Term
}

func NewQuotedTriple(subject, predicate, object Term) *QuotedTriple {
    // Validation rules:
    // - Subject: IRI, BlankNode, or QuotedTriple
    // - Predicate: Must be IRI (no QuotedTriple)
    // - Object: IRI, BlankNode, Literal, or QuotedTriple
    // - No cycles allowed
}

func (q *QuotedTriple) Type() TermType { return TermTypeQuotedTriple }
func (q *QuotedTriple) String() string { return fmt.Sprintf("<<%s %s %s>>", q.Subject, q.Predicate, q.Object) }
func (q *QuotedTriple) Equals(other Term) bool { /* component-wise equality */ }
```

**Complexity**: Medium (3-4 hours)
**Dependencies**: None

### 1.2 Language Direction Support

**File**: `pkg/rdf/term.go`

**Implementation**:
```go
// Extend Literal struct
type Literal struct {
    Value     string
    Language  string
    Direction string      // NEW: "ltr", "rtl", or ""
    Datatype  *NamedNode
}

// Add helper for rdf:dirLangString datatype
var RDFDirLangString = NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#dirLangString")

func NewLiteralWithLanguageAndDirection(value, language, direction string) *Literal {
    return &Literal{
        Value:     value,
        Language:  language,
        Direction: direction,
        Datatype:  RDFDirLangString,
    }
}

func (l *Literal) String() string {
    if l.Language != "" {
        result := fmt.Sprintf("\"%s\"@%s", l.Value, l.Language)
        if l.Direction != "" {
            result += "--" + l.Direction
        }
        return result
    }
    // ... existing logic
}
```

**Complexity**: Low (1-2 hours)
**Dependencies**: None

---

## 2. Storage Layer Changes

### 2.1 Term Encoding

**File**: `internal/encoding/encoder.go`

**QuotedTriple Encoding Strategy**:
```go
func (e *TermEncoder) encodeQuotedTriple(qt *rdf.QuotedTriple) (store.EncodedTerm, *string, error) {
    var encoded store.EncodedTerm
    encoded[0] = byte(rdf.TermTypeQuotedTriple)

    // Serialize the triple to canonical string form
    serialized := fmt.Sprintf("<<%s %s %s>>", qt.Subject, qt.Predicate, qt.Object)

    // Hash the serialized form (FNV-1a 128-bit)
    hash := e.Hash128(serialized)
    copy(encoded[1:], hash[:])

    // Store in id2str table for reconstruction
    return encoded, &serialized, nil
}
```

**Design Rationale**:
- Quoted triples are hashed and stored as strings
- 17-byte encoding: 1 byte type + 16 bytes hash
- Actual triple structure reconstructed on decode by parsing stored string
- Alternative (inline storage) would require 3×17=51 bytes, exceeds current limit

**Language Direction Encoding**:
```go
func (e *TermEncoder) encodeLangStringLiteral(lit *rdf.Literal) (store.EncodedTerm, *string, error) {
    // Modify to include direction in combined string
    combined := lit.Value + "@" + lit.Language
    if lit.Direction != "" {
        combined += "--" + lit.Direction
    }
    // Hash and store as before
    // ...
}
```

**Complexity**: Medium (4-6 hours)
**Dependencies**: Data Model changes

### 2.2 Term Decoding

**File**: `internal/encoding/decoder.go`

```go
func (d *TermDecoder) DecodeQuotedTriple(encoded store.EncodedTerm) (*rdf.QuotedTriple, error) {
    // 1. Retrieve serialized form from id2str table using hash
    serialized, err := d.lookupString(encoded)
    if err != nil {
        return nil, err
    }

    // 2. Parse the string representation
    // Remove << and >> delimiters
    // Split into subject, predicate, object
    // Recursively decode each component

    // 3. Construct QuotedTriple
    return rdf.NewQuotedTriple(subject, predicate, object)
}
```

**Complexity**: Medium (4-6 hours)
**Dependencies**: Encoding implementation

### 2.3 Storage Implications

**Current Storage**:
- 11 indexes: SPO, POS, OSP, SPOG, POSG, OSPG, GSPO, GPOS, GOSP, ID2Str, Graphs
- Each term encoded to 17 bytes (1 type + 16 hash/data)

**Impact**:
- ✅ No new indexes needed
- ✅ Quoted triples treated as atomic terms in indexes
- ⚠️ Query patterns on quoted triple components require decoding (slower)
- 💡 Could add specialized indexes later if performance issues arise

**Estimated Overhead**:
- Dataset with 10% quoted triples: ~5-8% storage increase
- Query performance: 0-15% slower for quoted triple queries
- Parse performance: 0-5% slower overall

**Complexity**: Low (already handled by existing design)

---

## 3. Parser Changes

### 3.1 N-Triples Parser

**File**: `pkg/rdf/nquads.go`

#### A. Language Direction Parsing

**Location**: Lines ~423-442 (language tag parsing in `parseLiteral()`)

**Grammar**:
```
LANGTAG ::= '@' [a-zA-Z]+ ('-' [a-zA-Z0-9]+)* ('--' ('ltr' | 'rtl'))?
```

**Implementation**:
```go
func (p *NQuadsParser) parseLiteral() (rdf.Term, error) {
    // ... existing literal parsing ...

    if p.peek() == '@' {
        p.consume() // '@'

        // Parse language tag
        language := p.parseLangTag()

        // Check for direction suffix
        if p.peek() == '-' && p.peekN(2) == '-' {
            p.consume() // first '-'
            p.consume() // second '-'

            direction := p.parseIdentifier()
            if direction != "ltr" && direction != "rtl" {
                return nil, fmt.Errorf("invalid direction: %s (must be 'ltr' or 'rtl')", direction)
            }

            return rdf.NewLiteralWithLanguageAndDirection(value, language, direction), nil
        }

        return rdf.NewLiteralWithLanguage(value, language), nil
    }
    // ... rest of literal parsing ...
}
```

**Test Examples**:
```ntriples
<http://example/s> <http://example/p> "Hello"@en--ltr .
<http://example/s> <http://example/p> "مرحبا"@ar--rtl .
```

#### B. Quoted Triple Parsing

**Location**: Add new method, call from `parseTerm()` when encountering `<<`

**Grammar**:
```
quotedTriple ::= '<<(' subject predicate object ')>>'
```

**Implementation**:
```go
func (p *NQuadsParser) parseQuotedTriple() (*rdf.QuotedTriple, error) {
    // Expect '<<('
    if !p.expect("<<(") {
        return nil, p.error("expected '<<('")
    }

    p.skipWhitespace()

    // Parse subject (can be IRI, blank node, or nested quoted triple)
    subject, err := p.parseTerm()
    if err != nil {
        return nil, fmt.Errorf("failed to parse subject: %w", err)
    }

    // Validate: subject cannot be literal
    if _, ok := subject.(*rdf.Literal); ok {
        return nil, p.error("quoted triple subject cannot be a literal")
    }

    p.skipWhitespace()

    // Parse predicate (must be IRI)
    predicate, err := p.parseTerm()
    if err != nil {
        return nil, fmt.Errorf("failed to parse predicate: %w", err)
    }

    // Validate: predicate must be IRI
    if _, ok := predicate.(*rdf.NamedNode); !ok {
        return nil, p.error("quoted triple predicate must be an IRI")
    }

    p.skipWhitespace()

    // Parse object (can be any term including quoted triple)
    object, err := p.parseTerm()
    if err != nil {
        return nil, fmt.Errorf("failed to parse object: %w", err)
    }

    p.skipWhitespace()

    // Expect ')>>'
    if !p.expect(")>>") {
        return nil, p.error("expected ')>>' to close quoted triple")
    }

    // Check for cycles (prevent infinite recursion)
    if p.detectCycle(subject, predicate, object) {
        return nil, p.error("circular reference in quoted triple")
    }

    return rdf.NewQuotedTriple(subject, predicate, object), nil
}

func (p *NQuadsParser) parseTerm() (rdf.Term, error) {
    p.skipWhitespace()

    ch := p.peek()
    switch {
    case ch == '<':
        // Could be IRI or start of quoted triple
        if p.peekN(2) == '<' {
            return p.parseQuotedTriple()
        }
        return p.parseIRI()
    case ch == '_':
        return p.parseBlankNode()
    case ch == '"':
        return p.parseLiteral()
    default:
        return nil, p.error("unexpected character")
    }
}
```

**Test Examples**:
```ntriples
# Quoted triple as object
<http://example/s> <http://example/p> <<( <http://example/s1> <http://example/p1> <http://example/o1> )>> .

# Quoted triple as subject
<<( <http://example/s1> <http://example/p1> <http://example/o1> )>> <http://example/p2> <http://example/o2> .

# Nested quoted triples
<http://example/s> <http://example/p> <<( <<( <http://example/s1> <http://example/p1> <http://example/o1> )>> <http://example/p2> <http://example/o2> )>> .

# Blank node in quoted triple
<<( _:b1 <http://example/p> <http://example/o> )>> <http://example/confidence> "0.9"^^<http://www.w3.org/2001/XMLSchema#decimal> .
```

**Complexity**: Medium-High (12-16 hours)
**Dependencies**: Data Model, Encoding

### 3.2 N-Quads Parser

**File**: `pkg/rdf/nquads.go`

**Changes**:
- Extend all N-Triples changes to N-Quads
- Graph context support for quoted triples (4th component)
- Quoted triples can appear in any graph

**Test Examples**:
```nquads
<<( <s> <p> <o> )>> <p2> <o2> <graph> .
<s> <p> <<( <s1> <p1> <o1> )>> <graph> .
<s> <p> "text"@en--ltr <graph> .
```

**Complexity**: Medium (8-10 hours)
**Dependencies**: N-Triples implementation

### 3.3 Turtle Parser

**File**: `pkg/rdf/turtle.go`

#### A. Language Direction

**Location**:
- Lines ~1100-1110 (short literals in `parseLiteral()`)
- Lines ~1192-1202 (long literals in `parseLongLiteral()`)

**Implementation**: Same approach as N-Triples, adapted for Turtle's literal parsing

#### B. Quoted Triple Syntax

**Grammar**:
```
quotedTriple ::= '<<' subject predicate object ( '~' reifier )? '>>'
```

**Implementation**:
```go
func (p *TurtleParser) parseQuotedTriple() (*rdf.QuotedTriple, error) {
    // Expect '<<'
    if !p.expect("<<") {
        return nil, p.error("expected '<<'")
    }

    p.skipWhitespaceAndComments()

    // Parse triple components
    subject, err := p.parseTerm()
    if err != nil {
        return nil, err
    }

    p.skipWhitespaceAndComments()

    predicate, err := p.parseTerm()
    if err != nil {
        return nil, err
    }

    p.skipWhitespaceAndComments()

    object, err := p.parseTerm()
    if err != nil {
        return nil, err
    }

    p.skipWhitespaceAndComments()

    // Check for reifier (RDF 1.2 extension)
    var reifier rdf.Term
    if p.peek() == '~' {
        p.consume() // '~'
        p.skipWhitespaceAndComments()

        reifier, err = p.parseTerm()
        if err != nil {
            return nil, fmt.Errorf("failed to parse reifier: %w", err)
        }

        // Reifier must be IRI or blank node
        switch reifier.(type) {
        case *rdf.NamedNode, *rdf.BlankNode:
            // Valid
        default:
            return nil, p.error("reifier must be an IRI or blank node")
        }
    }

    // Expect '>>'
    if !p.expect(">>") {
        return nil, p.error("expected '>>'")
    }

    qt := rdf.NewQuotedTriple(subject, predicate, object)

    // If reifier was specified, store it (for annotation desugaring)
    if reifier != nil {
        qt.Reifier = reifier
    }

    return qt, nil
}

func (p *TurtleParser) parseTerm() (rdf.Term, error) {
    p.skipWhitespaceAndComments()

    ch := p.peek()
    switch {
    case ch == '<':
        // Could be IRI or quoted triple
        if p.peekN(2) == '<' {
            return p.parseQuotedTriple()
        }
        return p.parseIRI()
    case ch == '[':
        return p.parseBlankNodePropertyList()
    case ch == '(':
        return p.parseCollection()
    // ... rest of term parsing
    }
}
```

**Must handle quoted triples in**:
- Collections: `( << s p o >> )`
- Blank node property lists: `[ :p << s p o >> ]`
- Property object lists: `:s :p << s p o >> , << s2 p2 o2 >>`

**Test Examples**:
```turtle
PREFIX : <http://example/>

# Quoted triple as object
:s :p << :s1 :p1 :o1 >> .

# Quoted triple as subject
<< :s1 :p1 :o1 >> :p2 :o2 .

# Nested quoted triples
<< << :s1 :p1 :o1 >> :p2 :o2 >> :p3 :o3 .

# Quoted triple with reifier
:s :p << :s1 :p1 :o1 ~ :reifier >> .

# In collections
:s :p ( << :s1 :p1 :o1 >> << :s2 :p2 :o2 >> ) .

# In blank node property lists
:s :p [ :q << :s1 :p1 :o1 >> ] .

# Language direction
:s :p "Hello"@en--ltr .
:s :p "مرحبا"@ar--rtl .
```

#### C. Annotation Syntax

**Grammar**:
```
annotation ::= '{|' predicateObjectList '|}'
```

**Desugaring**:
```turtle
# Input:
:s :p :o {| :r :z |} .

# Output (after desugaring):
:s :p :o .
_:reifier rdf:reifies << :s :p :o >> .
_:reifier :r :z .
```

**Implementation**:
```go
func (p *TurtleParser) parseTriples() ([]*rdf.Triple, error) {
    // Parse subject
    subject, err := p.parseTerm()
    if err != nil {
        return nil, err
    }

    // Parse predicate-object list
    triples, err := p.parsePredicateObjectList(subject)
    if err != nil {
        return nil, err
    }

    // Check for annotation block
    p.skipWhitespaceAndComments()
    if p.peek() == '{' && p.peekN(2) == '|' {
        // Parse annotation and desugar
        annotationTriples, err := p.parseAnnotationBlock(triples[len(triples)-1])
        if err != nil {
            return nil, err
        }
        triples = append(triples, annotationTriples...)
    }

    return triples, nil
}

func (p *TurtleParser) parseAnnotationBlock(annotatedTriple *rdf.Triple) ([]*rdf.Triple, error) {
    // Expect '{|'
    if !p.expect("{|") {
        return nil, p.error("expected '{|'")
    }

    p.skipWhitespaceAndComments()

    // Generate or use explicit reifier
    var reifier rdf.Term
    if annotatedTriple.Reifier != nil {
        reifier = annotatedTriple.Reifier
    } else {
        // Generate blank node
        reifier = p.generateBlankNode()
    }

    // Create quoted triple from annotated triple
    quotedTriple := rdf.NewQuotedTriple(
        annotatedTriple.Subject,
        annotatedTriple.Predicate,
        annotatedTriple.Object,
    )

    // Create rdf:reifies triple
    reifiesTriple := &rdf.Triple{
        Subject:   reifier,
        Predicate: rdf.RDFReifies,
        Object:    quotedTriple,
    }

    result := []*rdf.Triple{reifiesTriple}

    // Parse predicate-object list for reifier
    annotationTriples, err := p.parsePredicateObjectList(reifier)
    if err != nil {
        return nil, err
    }
    result = append(result, annotationTriples...)

    p.skipWhitespaceAndComments()

    // Expect '|}'
    if !p.expect("|}") {
        return nil, p.error("expected '|}'")
    }

    return result, nil
}
```

**Test Examples**:
```turtle
PREFIX : <http://example/>

# Simple annotation
:s :p :o {| :confidence 0.95 |} .

# Annotation with multiple properties
:s :p :o {| :source :wikipedia ; :date "2024-01-01"^^xsd:date |} .

# Annotation with explicit reifier
:s :p :o ~ :ann1 {| :source :wikipedia |} .

# Multiple annotations on same statement
:s :p :o {| :r1 :z1 |} {| :r2 :z2 |} .

# Annotation followed by more triples
:s :p :o {| :confidence 0.95 |} ;
   :p2 :o2 .
```

**Complexity**: High (16-20 hours for core + 12-16 hours for annotations = 28-36 hours total)
**Dependencies**: Data Model, N-Triples implementation

### 3.4 TriG Parser

**File**: `pkg/rdf/trig.go`

**Changes**:
- TriG delegates to Turtle parser for triple parsing (lines ~288-300)
- Ensure prefixes and blank node counters are properly shared
- Annotation syntax must work inside graph blocks
- Quoted triples in graph contexts

**Test Examples**:
```trig
PREFIX : <http://example/>

:graph1 {
    :s :p << :s1 :p1 :o1 >> .
    :s :p :o {| :confidence 0.95 |} .
    :s :q "text"@en--ltr .
}

<< :s :p :o >> :metadata "value" :graph2 .
```

**Complexity**: Medium (8-12 hours)
**Dependencies**: Turtle implementation

### 3.5 RDF/XML Parser

**File**: `pkg/rdf/rdfxml.go`

#### A. Language Direction

**Implementation**:
- Look for `xml:dir` attribute with `ltr` or `rtl` values
- Combine with `xml:lang` attribute
- May use `direction` attribute in some contexts

**XML Examples**:
```xml
<!-- Language with direction -->
<rdf:Description rdf:about="http://example/s">
  <ex:p xml:lang="ar" xml:dir="rtl">مرحبا</ex:p>
</rdf:Description>

<!-- Alternative syntax -->
<rdf:Description rdf:about="http://example/s">
  <ex:p xml:lang="en" direction="ltr">Hello</ex:p>
</rdf:Description>
```

#### B. Reification and Quoted Triples

**Implementation**:
- Parse `<rdf:reifies>` elements
- Handle nested triple structures
- Support `rdf:Triple` elements

**XML Examples**:
```xml
<!-- Simple reification -->
<rdf:Description rdf:about="http://example/reifier1">
  <rdf:reifies>
    <rdf:Triple>
      <rdf:subject rdf:resource="http://example/s"/>
      <rdf:predicate rdf:resource="http://example/p"/>
      <rdf:object rdf:resource="http://example/o"/>
    </rdf:Triple>
  </rdf:reifies>
  <ex:confidence>0.95</ex:confidence>
</rdf:Description>

<!-- Quoted triple as object -->
<rdf:Description rdf:about="http://example/s">
  <ex:p>
    <rdf:Triple>
      <rdf:subject rdf:resource="http://example/s1"/>
      <rdf:predicate rdf:resource="http://example/p1"/>
      <rdf:object rdf:resource="http://example/o1"/>
    </rdf:Triple>
  </ex:p>
</rdf:Description>
```

**Complexity**: Medium-High (12-16 hours)
**Dependencies**: Data Model, Encoding

### 3.6 JSON-LD Parser

**File**: `pkg/rdf/jsonld.go`

**Changes**:
- JSON-LD RDF-star syntax uses special `@id` structure
- `@direction` keyword for language direction
- Triple terms as JSON objects

**JSON Examples**:
```json
{
  "@context": {
    "ex": "http://example/"
  },
  "@id": "ex:s",
  "ex:p": {
    "@annotation": {
      "@id": {
        "@id": "ex:s1",
        "ex:p1": {"@id": "ex:o1"}
      }
    }
  }
}
```

**Complexity**: Medium (10-14 hours)
**Dependencies**: Data Model, Encoding

---

## 4. SPARQL Query Support (Optional - Can Defer)

### 4.1 SPARQL Parser Changes

**File**: `pkg/sparql/parser/ast.go` and `parser.go`

**AST Extensions**:
```go
// Extend TermOrVariable to support quoted triples
type TermOrVariable struct {
    Term         rdf.Term
    Variable     *Variable
    QuotedTriple *QuotedTriplePattern  // NEW
}

// Pattern matching on quoted triple components
type QuotedTriplePattern struct {
    Subject   TermOrVariable
    Predicate TermOrVariable
    Object    TermOrVariable
}
```

**Triple Pattern Syntax**:
```sparql
# Match quoted triples as objects
?s ?p << ?s1 ?p1 ?o1 >> .

# Match quoted triples as subjects
<< ?s1 ?p1 ?o1 >> ?p2 ?o2 .

# Match with variables in nested components
<< :alice :knows ?who >> :confidence ?conf .

# BIND with triple terms
BIND(<< :s :p :o >> AS ?triple)
```

**Complexity**: High (16-20 hours)
**Dependencies**: Data Model

### 4.2 SPARQL Executor Changes

**File**: `pkg/sparql/executor/executor.go`

**Pattern Matching**:
- When matching against quoted triple patterns, decode the stored term and match components
- Variable bindings for quoted triples must preserve structure
- Join optimization needs to consider quoted triple patterns

**Built-in Functions**:
```sparql
# Triple accessor functions (SPARQL-star)
TRIPLE(?s, ?p, ?o)    # Construct quoted triple
SUBJECT(?triple)      # Extract subject
PREDICATE(?triple)    # Extract predicate
OBJECT(?triple)       # Extract object
isTRIPLE(?term)       # Test if term is quoted triple
```

**Complexity**: High (20-24 hours)
**Dependencies**: SPARQL Parser

**NOTE**: SPARQL-star support is not required for RDF 1.2 parsing compliance. This can be deferred to a later phase.

---

## 5. Implementation Phases

### Phase 1: Foundation (Week 1) - CRITICAL

**Duration**: 5-7 days
**Effort**: 20-28 hours

1. **Data Model** (Days 1-2)
   - [ ] Add QuotedTriple type to term.go
   - [ ] Add Direction field to Literal
   - [ ] Implement validation logic
   - [ ] Write unit tests for new types
   - [ ] Test cycle detection

2. **Encoding/Decoding** (Days 3-5)
   - [ ] Implement QuotedTriple encoding (hash-based)
   - [ ] Implement QuotedTriple decoding (string parsing)
   - [ ] Implement direction support in literal encoding
   - [ ] Update encoder/decoder tests
   - [ ] Performance benchmarks

**Deliverables**:
- ✅ QuotedTriple and Direction support in data model
- ✅ Storage layer can encode/decode new term types
- ✅ All unit tests passing
- ✅ No regression in existing functionality

### Phase 2: N-Triples/N-Quads (Week 2) - HIGH PRIORITY

**Duration**: 5-7 days
**Effort**: 24-30 hours

3. **N-Triples Parser** (Days 1-4)
   - [ ] Implement language direction parsing
   - [ ] Implement quoted triple parsing
   - [ ] Handle nested quoted triples
   - [ ] Add cycle detection
   - [ ] Update parser tests
   - [ ] Run W3C N-Triples 1.2 tests
   - [ ] Target: 95%+ compliance

4. **N-Quads Parser** (Days 4-5)
   - [ ] Extend N-Triples changes to N-Quads
   - [ ] Test graph context support
   - [ ] Run W3C N-Quads 1.2 tests
   - [ ] Target: 95%+ compliance

**Deliverables**:
- ✅ N-Triples: 60% → 95%+ (87/145 → 138+/145)
- ✅ N-Quads: 62% → 95%+ (101/164 → 156+/164)
- ✅ All quality checks passing

### Phase 3: Turtle Core (Week 3) - HIGH PRIORITY

**Duration**: 6-8 days
**Effort**: 28-36 hours

5. **Turtle Parser - Core** (Days 1-5)
   - [ ] Implement quoted triple syntax (`<<` and `>>`)
   - [ ] Implement reifier syntax (`~ reifier`)
   - [ ] Handle language direction in literals
   - [ ] Support quoted triples in collections
   - [ ] Support quoted triples in blank node property lists
   - [ ] Integration testing
   - [ ] Run W3C Turtle 1.2 tests (syntax subset)
   - [ ] Target: 85%+ on syntax tests

**Deliverables**:
- ✅ Quoted triple parsing in Turtle
- ✅ Language direction support
- ✅ Integration with existing Turtle features

### Phase 4: Turtle Annotations (Week 4) - HIGH PRIORITY

**Duration**: 5-7 days
**Effort**: 24-32 hours

6. **Turtle Parser - Annotations** (Days 1-5)
   - [ ] Implement annotation block parsing `{| |}`
   - [ ] Implement desugaring to rdf:reifies triples
   - [ ] Generate blank nodes for implicit reifiers
   - [ ] Handle multiple annotation blocks
   - [ ] Handle annotations with explicit reifiers
   - [ ] Run full W3C Turtle 1.2 tests
   - [ ] Target: 98%+ compliance

**Deliverables**:
- ✅ Turtle: 79.5% → 98%+ (329/414 → 405+/414)
- ✅ Annotation syntax fully supported

### Phase 5: TriG (Week 4-5) - HIGH PRIORITY

**Duration**: 4-5 days
**Effort**: 16-24 hours

7. **TriG Parser** (Days 1-4)
   - [ ] Ensure Turtle changes propagate correctly
   - [ ] Test graph-scoped annotations
   - [ ] Test quoted triples in named graphs
   - [ ] Handle prefix sharing between graphs
   - [ ] Run W3C TriG 1.2 tests
   - [ ] Target: 98%+ compliance

**Deliverables**:
- ✅ TriG: 80.6% → 98%+ (348/432 → 423+/432)
- ✅ All Turtle features work in TriG

### Phase 6: RDF/XML (Week 5-6) - MEDIUM PRIORITY

**Duration**: 6-8 days
**Effort**: 24-32 hours

8. **RDF/XML Parser** (Days 1-6)
   - [ ] Implement language direction attributes
   - [ ] Implement rdf:reifies elements
   - [ ] Implement rdf:Triple structures
   - [ ] Handle nested triple terms
   - [ ] Run W3C RDF/XML 1.2 tests
   - [ ] Target: 95%+ compliance

**Deliverables**:
- ✅ RDF/XML: 79.5% → 95%+ (163/205 → 195+/205)
- ✅ XML-specific RDF-star syntax supported

### Phase 7: JSON-LD (Optional - Can Defer)

**Duration**: 5-7 days
**Effort**: 20-28 hours

9. **JSON-LD Parser** (If needed)
   - [ ] RDF-star JSON syntax
   - [ ] Direction support
   - [ ] Test against examples

### Phase 8: SPARQL-star (Optional - Can Defer)

**Duration**: 10-12 days
**Effort**: 48-60 hours

10. **SPARQL Parser & Executor**
    - [ ] Quoted triple pattern syntax
    - [ ] AST extensions
    - [ ] Pattern matching on quoted triples
    - [ ] Built-in functions
    - [ ] Query optimization

**NOTE**: SPARQL-star is not required for RDF 1.2 compliance and can be implemented in a separate project phase.

---

## 6. Testing Strategy

### 6.1 Unit Tests

**New Test Files**:
```
pkg/rdf/term_test.go                    # QuotedTriple tests
pkg/rdf/turtle_quoted_test.go           # Quoted triple parsing
pkg/rdf/turtle_annotation_test.go       # Annotation syntax
pkg/rdf/turtle_langdir_test.go          # Language direction
internal/encoding/encoder_quoted_test.go # Encoding tests
internal/encoding/decoder_quoted_test.go # Decoding tests
```

**Test Coverage**:
- QuotedTriple creation and validation
- Nested quoted triples
- Cycle detection
- Language direction parsing
- Annotation desugaring
- Encoding/decoding round-trips
- Edge cases and error handling

### 6.2 W3C Test Suite Validation

**Commands**:
```bash
# Run all RDF 1.2 test suites
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-n-triples
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-n-quads
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-turtle
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-trig
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-xml
```

**Expected Results After Implementation**:
| Parser | Current | Target | Status |
|--------|---------|--------|--------|
| N-Triples | 60.0% (87/145) | 95%+ (138+/145) | ⬆️ +35pp |
| N-Quads | 61.6% (101/164) | 95%+ (156+/164) | ⬆️ +34pp |
| Turtle | 79.5% (329/414) | 98%+ (405+/414) | ⬆️ +18pp |
| TriG | 80.6% (348/432) | 98%+ (423+/432) | ⬆️ +17pp |
| RDF/XML | 79.5% (163/205) | 95%+ (195+/205) | ⬆️ +16pp |
| **TOTAL** | **75.6%** | **97%+** | **⬆️ +21pp** |

### 6.3 Regression Testing

**Ensure RDF 1.1 compliance remains 100%**:
```bash
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-n-triples
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-n-quads
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-trig
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml
```

**All must remain at 100% pass rate.**

### 6.4 Performance Benchmarks

**Metrics to Track**:
```go
// Benchmarks to add
BenchmarkParseQuotedTriple
BenchmarkParseAnnotation
BenchmarkEncodeDecode_QuotedTriple
BenchmarkQuery_QuotedTriple
```

**Acceptance Criteria**:
- Parse performance: <10% slower than RDF 1.1 for documents without RDF-star
- Storage overhead: <10% for typical datasets
- Query performance: TBD (may be slower initially, optimize later)

---

## 7. Quality Assurance

### 7.1 Code Quality Checks

**Run before every commit**:
```bash
go fmt ./...
go build ./...
go test ./...
go vet ./...
staticcheck ./...
gosec -quiet ./...
```

**All must pass with zero errors/warnings.**

### 7.2 Documentation Requirements

**Files to Update**:
1. `README.md`
   - Update test results table
   - Add RDF 1.2 support announcement
   - Update feature list

2. `CLAUDE.md`
   - Update parser descriptions
   - Add RDF 1.2 features section
   - Update limitations section

3. `docs/testing.html`
   - Add RDF 1.2 compliance section
   - Update test result tables

4. `docs/index.html`
   - Update feature badges
   - Add RDF-star mention

**New Documentation**:
1. `docs/rdf12-features.md` - User guide for RDF 1.2 features
2. `docs/quoted-triples.md` - Technical implementation details
3. API documentation for new types

### 7.3 Commit Message Format

```
type(scope): Brief description

- Detail 1
- Detail 2

Test results: Old% → New% (+Xpp, +N tests)

All quality checks pass: go vet, staticcheck, gosec

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: feat, fix, refactor, test, docs
**Scopes**: rdf, sparql, storage, test

---

## 8. Risk Management

### 8.1 High-Risk Areas

**1. Annotation Syntax Desugaring**
- **Risk**: Complex blank node generation and reifier management
- **Mitigation**:
  - Comprehensive unit tests for all annotation patterns
  - Reference implementation study
  - Incremental development with frequent testing

**2. Recursive Quoted Triple Parsing**
- **Risk**: Stack overflow, circular references, performance
- **Mitigation**:
  - Depth limit on nesting (e.g., max 100 levels)
  - Cycle detection during parsing
  - Performance benchmarks at each level

**3. Storage Performance**
- **Risk**: String-based storage may be slow for deep queries
- **Mitigation**:
  - Profile early and often
  - Optimize encoding if needed
  - Consider specialized indexes in Phase 2

### 8.2 Medium-Risk Areas

**1. W3C Test Suite Edge Cases**
- **Risk**: Obscure test cases may reveal spec ambiguities
- **Mitigation**:
  - Study failing tests carefully
  - Reference other implementations (Apache Jena, RDFLib)
  - Consult W3C specs and community

**2. SPARQL-star Semantics**
- **Risk**: Query semantics for quoted triples may be complex
- **Mitigation**:
  - Defer SPARQL-star to later phase
  - Focus on parsing first

**3. Backward Compatibility**
- **Risk**: Changes might break existing code
- **Mitigation**:
  - Maintain 100% RDF 1.1 test compliance
  - Version API if needed
  - Feature flags for RDF 1.2 mode (optional)

### 8.3 Contingency Plans

**If Phase 1 Takes Longer Than Expected**:
- Reduce scope of Phase 2 (skip N-Quads initially)
- Focus on Turtle first (highest value)

**If Annotation Syntax Proves Too Complex**:
- Implement basic quoted triples first
- Defer annotations to Phase 2

**If Performance Issues Arise**:
- Document performance characteristics
- Plan optimization phase after basic implementation

---

## 9. Success Criteria

### 9.1 Minimum Viable Product (MVP)

**Must Have**:
- [x] Parse all RDF 1.2 quoted triple syntax
- [x] Parse language direction tags
- [x] Store and retrieve quoted triples
- [x] Pass ≥95% of RDF 1.2 W3C tests
- [x] No regression in RDF 1.1 tests (maintain 100%)
- [x] All quality checks pass (vet, staticcheck, gosec)

**Test Targets**:
- N-Triples: ≥95% (138+/145)
- N-Quads: ≥95% (156+/164)
- Turtle: ≥98% (405+/414)
- TriG: ≥98% (423+/432)
- RDF/XML: ≥95% (195+/205)

### 9.2 Stretch Goals

**Nice to Have**:
- [ ] SPARQL-star query support
- [ ] Optimized quoted triple storage
- [ ] 100% W3C test compliance (1,360/1,360)
- [ ] Performance benchmarks published
- [ ] Comprehensive user documentation

### 9.3 Definition of Done

A phase is complete when:
1. All planned features are implemented
2. Unit tests written and passing
3. W3C test suite compliance targets met
4. Code review completed (if applicable)
5. Documentation updated
6. All quality checks pass
7. Commit made with proper message

---

## 10. Resources & References

### 10.1 Specifications

**RDF 1.2**:
- RDF 1.2 Concepts: https://www.w3.org/TR/rdf12-concepts/
- RDF 1.2 Turtle: https://w3c.github.io/rdf-turtle/spec/
- RDF 1.2 N-Triples: https://w3c.github.io/rdf-n-triples/spec/
- RDF 1.2 TriG: https://w3c.github.io/rdf-trig/spec/
- RDF 1.2 N-Quads: https://w3c.github.io/rdf-n-quads/spec/
- RDF 1.2 XML Syntax: https://w3c.github.io/rdf-xml/spec/

**RDF-star**:
- RDF-star and SPARQL-star: https://w3c.github.io/rdf-star/cg-spec/editors_draft.html
- Use Cases: https://w3c.github.io/rdf-star/UCR/rdf-star-ucr.html

### 10.2 Test Suites

**Local Paths**:
- N-Triples: `testdata/rdf-tests/rdf/rdf12/rdf-n-triples/`
- N-Quads: `testdata/rdf-tests/rdf/rdf12/rdf-n-quads/`
- Turtle: `testdata/rdf-tests/rdf/rdf12/rdf-turtle/`
- TriG: `testdata/rdf-tests/rdf/rdf12/rdf-trig/`
- RDF/XML: `testdata/rdf-tests/rdf/rdf12/rdf-xml/`

**Remote**:
- W3C RDF Tests: https://github.com/w3c/rdf-tests
- Test Results: https://w3c.github.io/rdf-tests/

### 10.3 Reference Implementations

**Study These**:
- Apache Jena: https://jena.apache.org/ (Java, RDF-star support)
- RDFLib: https://github.com/RDFLib/rdflib (Python, RDF-star support)
- Oxigraph: https://github.com/oxigraph/oxigraph (Rust, RDF-star support)
- N3.js: https://github.com/rdfjs/N3.js (JavaScript, RDF-star support)

### 10.4 Community Resources

- W3C RDF-DEV Community Group: https://www.w3.org/community/rdf-dev/
- RDF-star discussions: https://github.com/w3c/rdf-star/discussions
- SPARQL-star issues: https://github.com/w3c/sparql-dev/issues

---

## 11. Implementation Checklist

### Phase 1: Foundation ✅
- [ ] Create QuotedTriple type in term.go
- [ ] Add Direction field to Literal
- [ ] Implement validation logic
- [ ] Add QuotedTriple encoding
- [ ] Add QuotedTriple decoding
- [ ] Add language direction encoding
- [ ] Write unit tests
- [ ] All tests passing

### Phase 2: N-Triples/N-Quads ✅
- [ ] Implement language direction parsing (N-Triples)
- [ ] Implement quoted triple parsing (N-Triples)
- [ ] Add cycle detection
- [ ] N-Triples W3C tests ≥95%
- [ ] Extend to N-Quads
- [ ] N-Quads W3C tests ≥95%

### Phase 3: Turtle Core ✅
- [ ] Implement quoted triple syntax in Turtle
- [ ] Implement reifier syntax
- [ ] Support in collections
- [ ] Support in blank node property lists
- [ ] Language direction in Turtle
- [ ] Partial W3C tests ≥85%

### Phase 4: Turtle Annotations ✅
- [ ] Implement annotation block parsing
- [ ] Implement desugaring logic
- [ ] Generate blank nodes
- [ ] Handle multiple annotations
- [ ] Turtle W3C tests ≥98%

### Phase 5: TriG ✅
- [ ] Propagate Turtle changes to TriG
- [ ] Test graph contexts
- [ ] TriG W3C tests ≥98%

### Phase 6: RDF/XML ✅
- [ ] Implement language direction attributes
- [ ] Implement rdf:reifies elements
- [ ] Implement rdf:Triple structures
- [ ] RDF/XML W3C tests ≥95%

### Phase 7: Documentation ✅
- [ ] Update README.md
- [ ] Update CLAUDE.md
- [ ] Update docs/testing.html
- [ ] Update docs/index.html
- [ ] Create docs/rdf12-features.md
- [ ] Create docs/quoted-triples.md

### Phase 8: Final Validation ✅
- [ ] All RDF 1.2 tests ≥97%
- [ ] All RDF 1.1 tests still 100%
- [ ] All quality checks pass
- [ ] Performance acceptable
- [ ] Documentation complete

---

## 12. Notes & Open Questions

### Questions to Resolve

1. **Blank Node Generation Strategy for Annotations**
   - Should we use sequential (`_:ann1`, `_:ann2`) or hash-based?
   - **Recommendation**: Sequential (simpler, matches test expectations)

2. **Maximum Nesting Depth**
   - Should we limit quoted triple nesting?
   - **Recommendation**: Yes, 100 levels (prevent stack overflow)

3. **SPARQL-star Priority**
   - Implement now or defer?
   - **Recommendation**: Defer (parsing is more critical)

4. **Performance Optimization Timing**
   - Optimize encoding now or later?
   - **Recommendation**: Later (working solution first)

### Implementation Notes

- Start with data model and encoding (foundation)
- Move to simpler parsers (N-Triples) before complex ones (Turtle)
- Test frequently against W3C suite
- Don't optimize prematurely
- Focus on correctness first, performance second

### Future Enhancements

- Specialized indexes for quoted triple components
- Inline encoding for simple quoted triples
- SPARQL-star full support
- Query optimization for quoted triple patterns
- Streaming parser for large RDF-star datasets

---

## Appendix A: Syntax Examples

### A.1 N-Triples RDF 1.2

```ntriples
# Language direction
<http://example/s> <http://example/p> "Hello"@en--ltr .
<http://example/s> <http://example/p> "مرحبا"@ar--rtl .

# Quoted triple as object
<http://example/s> <http://example/p> <<( <http://example/s1> <http://example/p1> <http://example/o1> )>> .

# Quoted triple as subject
<<( <http://example/s1> <http://example/p1> <http://example/o1> )>> <http://example/p2> <http://example/o2> .

# Nested quoted triples
<http://example/s> <http://example/p> <<( <<( <http://example/s1> <http://example/p1> <http://example/o1> )>> <http://example/p2> <http://example/o2> )>> .

# Blank node in quoted triple
<<( _:b1 <http://example/p> <http://example/o> )>> <http://example/confidence> "0.9"^^<http://www.w3.org/2001/XMLSchema#decimal> .
```

### A.2 Turtle RDF 1.2

```turtle
PREFIX : <http://example/>
PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>

# Language direction
:s :p "Hello"@en--ltr .
:s :p "مرحبا"@ar--rtl .

# Quoted triple as object
:s :p << :s1 :p1 :o1 >> .

# Quoted triple as subject
<< :s1 :p1 :o1 >> :p2 :o2 .

# Nested quoted triples
<< << :s1 :p1 :o1 >> :p2 :o2 >> :p3 :o3 .

# Quoted triple with reifier
:s :p << :s1 :p1 :o1 ~ :reifier >> .

# Annotation syntax (simple)
:s :p :o {| :confidence "0.95"^^xsd:decimal |} .

# Annotation syntax (complex)
:s :p :o {|
    :source :wikipedia ;
    :date "2024-01-01"^^xsd:date ;
    :confidence "0.95"^^xsd:decimal
|} .

# Annotation with explicit reifier
:s :p :o ~ :ann1 {| :source :wikipedia |} .

# Multiple annotations
:s :p :o {| :r1 :z1 |} {| :r2 :z2 |} .

# In collections
:s :p ( << :s1 :p1 :o1 >> << :s2 :p2 :o2 >> ) .

# In blank node property lists
:s :p [ :q << :s1 :p1 :o1 >> ] .
```

### A.3 TriG RDF 1.2

```trig
PREFIX : <http://example/>

:graph1 {
    :s :p << :s1 :p1 :o1 >> .
    :s :p :o {| :confidence "0.95"^^xsd:decimal |} .
    :s :q "text"@en--ltr .
}

:graph2 {
    << :s :p :o >> :metadata "value" .
}
```

### A.4 RDF/XML RDF 1.2

```xml
<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example/">

  <!-- Language with direction -->
  <rdf:Description rdf:about="http://example/s">
    <ex:p xml:lang="ar" xml:dir="rtl">مرحبا</ex:p>
  </rdf:Description>

  <!-- Reification -->
  <rdf:Description rdf:about="http://example/reifier1">
    <rdf:reifies>
      <rdf:Triple>
        <rdf:subject rdf:resource="http://example/s"/>
        <rdf:predicate rdf:resource="http://example/p"/>
        <rdf:object rdf:resource="http://example/o"/>
      </rdf:Triple>
    </rdf:reifies>
    <ex:confidence rdf:datatype="http://www.w3.org/2001/XMLSchema#decimal">0.95</ex:confidence>
  </rdf:Description>

</rdf:RDF>
```

---

## Appendix B: Testing Commands

### B.1 Build and Test Commands

```bash
# Format code
go fmt ./...

# Build all packages
go build ./...

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests verbose
go test -v ./...

# Run specific test
go test -v ./pkg/rdf -run TestQuotedTriple

# Quality checks
go vet ./...
staticcheck ./...
gosec -quiet ./...
```

### B.2 W3C Test Suite Commands

```bash
# Build test runner
go build -o test-runner ./cmd/test-runner

# Run RDF 1.1 tests (regression check)
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-n-triples
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-n-quads
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-turtle
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-trig
./test-runner testdata/rdf-tests/rdf/rdf11/rdf-xml

# Run RDF 1.2 tests
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-n-triples
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-n-quads
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-turtle
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-trig
./test-runner testdata/rdf-tests/rdf/rdf12/rdf-xml

# Run all RDF tests
for dir in testdata/rdf-tests/rdf/rdf11/rdf-*/ testdata/rdf-tests/rdf/rdf12/rdf-*/; do
    echo "Testing: $dir"
    ./test-runner "$dir"
done
```

### B.3 Performance Benchmarks

```bash
# Run benchmarks
go test -bench=. ./pkg/rdf
go test -bench=. ./internal/encoding

# Run benchmarks with memory profiling
go test -bench=. -benchmem ./pkg/rdf

# Profile CPU
go test -cpuprofile=cpu.prof -bench=. ./pkg/rdf
go tool pprof cpu.prof

# Profile memory
go test -memprofile=mem.prof -bench=. ./pkg/rdf
go tool pprof mem.prof
```

---

## Appendix C: Troubleshooting

### C.1 Common Issues

**Issue**: Quoted triple parsing fails with "invalid character"
- **Cause**: Parser not recognizing `<<` delimiter
- **Solution**: Check `parseTerm()` logic, ensure `<<` handled before single `<`

**Issue**: Annotation syntax generates incorrect reifier
- **Cause**: Blank node counter not incremented
- **Solution**: Verify blank node generation logic

**Issue**: Language direction not preserved
- **Cause**: Encoding not handling Direction field
- **Solution**: Update `encodeLangStringLiteral()` to include direction

**Issue**: Cycle detection too strict
- **Cause**: False positives in cycle detection
- **Solution**: Only detect actual cycles, not all repeated terms

**Issue**: W3C tests fail with "file not found"
- **Cause**: Test file paths not resolved correctly
- **Solution**: Check manifest include resolution logic

### C.2 Debugging Tips

1. **Enable verbose logging**: Add debug prints to parser
2. **Use minimal test cases**: Create simple `.ttl` files to isolate issues
3. **Compare with reference implementations**: Check Apache Jena behavior
4. **Study W3C test expectations**: Read `.nq` result files
5. **Use git bisect**: Find regression commit if tests break

---

**END OF IMPLEMENTATION PLAN**

---

**Next Steps**:
1. Review this plan
2. Clarify any questions
3. Start with Phase 1: Foundation
4. Test early and often
5. Track progress against checklist

**Good luck! 🚀**
