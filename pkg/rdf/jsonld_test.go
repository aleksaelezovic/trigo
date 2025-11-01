package rdf

import (
	"strings"
	"testing"
)

func TestJSONLDParser_SimpleObject(t *testing.T) {
	input := `{
  "@id": "http://example.org/alice",
  "http://example.org/name": "Alice"
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	quad := quads[0]
	if getIRI(quad.Subject) != "http://example.org/alice" {
		t.Errorf("Wrong subject: %s", getIRI(quad.Subject))
	}
	if getIRI(quad.Predicate) != "http://example.org/name" {
		t.Errorf("Wrong predicate: %s", getIRI(quad.Predicate))
	}

	literal, ok := quad.Object.(*Literal)
	if !ok {
		t.Fatalf("Expected literal object, got %T", quad.Object)
	}
	if literal.Value != "Alice" {
		t.Errorf("Expected value 'Alice', got '%s'", literal.Value)
	}
}

func TestJSONLDParser_WithContext(t *testing.T) {
	input := `{
  "@context": {
    "ex": "http://example.org/",
    "name": "ex:name",
    "age": "ex:age"
  },
  "@id": "http://example.org/alice",
  "name": "Alice",
  "age": 30
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check that context expansion worked
	predicates := make(map[string]bool)
	for _, quad := range quads {
		predicates[getIRI(quad.Predicate)] = true
	}

	if !predicates["http://example.org/name"] {
		t.Errorf("Expected predicate http://example.org/name")
	}
	if !predicates["http://example.org/age"] {
		t.Errorf("Expected predicate http://example.org/age")
	}
}

func TestJSONLDParser_BlankNode(t *testing.T) {
	input := `{
  "http://example.org/name": "Anonymous"
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	// Subject should be a blank node when @id is missing
	_, ok := quads[0].Subject.(*BlankNode)
	if !ok {
		t.Errorf("Expected blank node subject, got %T", quads[0].Subject)
	}
}

func TestJSONLDParser_ValueObject(t *testing.T) {
	input := `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "http://example.org/alice",
  "ex:name": {
    "@value": "Alice"
  }
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	literal, ok := quads[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Expected literal object, got %T", quads[0].Object)
	}
	if literal.Value != "Alice" {
		t.Errorf("Expected value 'Alice', got '%s'", literal.Value)
	}
}

func TestJSONLDParser_LanguageTag(t *testing.T) {
	input := `{
  "@id": "http://example.org/alice",
  "http://example.org/name": {
    "@value": "Alice",
    "@language": "en"
  }
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	literal, ok := quads[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Expected literal object, got %T", quads[0].Object)
	}
	if literal.Value != "Alice" {
		t.Errorf("Expected value 'Alice', got '%s'", literal.Value)
	}
	if literal.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", literal.Language)
	}
}

func TestJSONLDParser_TypedLiteral(t *testing.T) {
	input := `{
  "@context": {
    "xsd": "http://www.w3.org/2001/XMLSchema#"
  },
  "@id": "http://example.org/alice",
  "http://example.org/age": {
    "@value": "30",
    "@type": "xsd:integer"
  }
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	literal, ok := quads[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Expected literal object, got %T", quads[0].Object)
	}
	if literal.Value != "30" {
		t.Errorf("Expected value '30', got '%s'", literal.Value)
	}
	if literal.Datatype == nil {
		t.Fatalf("Expected datatype, got nil")
	}
	if literal.Datatype.IRI != "http://www.w3.org/2001/XMLSchema#integer" {
		t.Errorf("Expected datatype xsd:integer, got %s", literal.Datatype.IRI)
	}
}

func TestJSONLDParser_IDReference(t *testing.T) {
	input := `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "http://example.org/alice",
  "ex:knows": {
    "@id": "http://example.org/bob"
  }
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	// Object should be a named node (not literal)
	if getIRI(quads[0].Object) != "http://example.org/bob" {
		t.Errorf("Expected object IRI http://example.org/bob, got %s", getIRI(quads[0].Object))
	}
}

func TestJSONLDParser_Array(t *testing.T) {
	input := `[
  {
    "@id": "http://example.org/alice",
    "http://example.org/name": "Alice"
  },
  {
    "@id": "http://example.org/bob",
    "http://example.org/name": "Bob"
  }
]`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check subjects
	subjects := []string{
		getIRI(quads[0].Subject),
		getIRI(quads[1].Subject),
	}

	expectedSubjects := map[string]bool{
		"http://example.org/alice": true,
		"http://example.org/bob":   true,
	}

	for i, subj := range subjects {
		if !expectedSubjects[subj] {
			t.Errorf("Quad %d: unexpected subject: %s", i, subj)
		}
	}
}

func TestJSONLDParser_MultipleValues(t *testing.T) {
	input := `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "http://example.org/alice",
  "ex:hobby": ["reading", "hiking", "coding"]
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 3 {
		t.Fatalf("Expected 3 quads, got %d", len(quads))
	}

	// All should have same subject and predicate
	for i, quad := range quads {
		if getIRI(quad.Subject) != "http://example.org/alice" {
			t.Errorf("Quad %d: wrong subject: %s", i, getIRI(quad.Subject))
		}
		if getIRI(quad.Predicate) != "http://example.org/hobby" {
			t.Errorf("Quad %d: wrong predicate: %s", i, getIRI(quad.Predicate))
		}
	}

	// Check values
	expectedValues := map[string]bool{
		"reading": true,
		"hiking":  true,
		"coding":  true,
	}

	for i, quad := range quads {
		literal, ok := quad.Object.(*Literal)
		if !ok {
			t.Errorf("Quad %d: expected literal object, got %T", i, quad.Object)
			continue
		}
		if !expectedValues[literal.Value] {
			t.Errorf("Quad %d: unexpected value: %s", i, literal.Value)
		}
	}
}

func TestJSONLDParser_NestedObject(t *testing.T) {
	input := `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "http://example.org/alice",
  "ex:address": {
    "ex:city": "Berlin",
    "ex:country": "Germany"
  }
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 3 {
		t.Fatalf("Expected 3 quads, got %d", len(quads))
	}

	// First quad: alice ex:address _:b1
	if getIRI(quads[0].Subject) != "http://example.org/alice" {
		t.Errorf("Quad 0: wrong subject: %s", getIRI(quads[0].Subject))
	}
	if getIRI(quads[0].Predicate) != "http://example.org/address" {
		t.Errorf("Quad 0: wrong predicate: %s", getIRI(quads[0].Predicate))
	}
	blankNode, ok := quads[0].Object.(*BlankNode)
	if !ok {
		t.Fatalf("Quad 0: expected blank node object, got %T", quads[0].Object)
	}

	// Second and third quads should have the blank node as subject
	for i := 1; i < 3; i++ {
		subj, ok := quads[i].Subject.(*BlankNode)
		if !ok {
			t.Errorf("Quad %d: expected blank node subject, got %T", i, quads[i].Subject)
			continue
		}
		if subj.ID != blankNode.ID {
			t.Errorf("Quad %d: blank node ID mismatch", i)
		}
	}
}

func TestJSONLDParser_NumberAndBoolean(t *testing.T) {
	input := `{
  "@id": "http://example.org/alice",
  "http://example.org/age": 30,
  "http://example.org/active": true
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check number literal
	lit0, ok := quads[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Quad 0: expected literal object, got %T", quads[0].Object)
	}
	if lit0.Value != "30" {
		t.Errorf("Quad 0: expected value '30', got '%s'", lit0.Value)
	}

	// Check boolean literal
	lit1, ok := quads[1].Object.(*Literal)
	if !ok {
		t.Fatalf("Quad 1: expected literal object, got %T", quads[1].Object)
	}
	if lit1.Value != "true" {
		t.Errorf("Quad 1: expected value 'true', got '%s'", lit1.Value)
	}
}

func TestJSONLDParser_EmptyPrefixExpansion(t *testing.T) {
	input := `{
  "@context": {
    "ex": "http://example.org/"
  },
  "@id": "ex:alice",
  "ex:name": "Alice"
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	// Check that prefix was expanded
	if getIRI(quads[0].Subject) != "http://example.org/alice" {
		t.Errorf("Expected subject http://example.org/alice, got %s", getIRI(quads[0].Subject))
	}
	if getIRI(quads[0].Predicate) != "http://example.org/name" {
		t.Errorf("Expected predicate http://example.org/name, got %s", getIRI(quads[0].Predicate))
	}
}

func TestJSONLDParser_AllInDefaultGraph(t *testing.T) {
	input := `{
  "@id": "http://example.org/alice",
  "http://example.org/name": "Alice",
  "http://example.org/age": 30
}`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// All quads should be in default graph
	for i, quad := range quads {
		_, ok := quad.Graph.(*DefaultGraph)
		if !ok {
			t.Errorf("Quad %d: expected default graph, got %T", i, quad.Graph)
		}
	}
}

func TestJSONLDParser_ContextInArray(t *testing.T) {
	input := `[
  {
    "@context": {
      "ex": "http://example.org/"
    },
    "@id": "ex:alice",
    "ex:name": "Alice"
  }
]`

	parser := NewJSONLDParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	// Check that prefix was expanded from context in array element
	if getIRI(quads[0].Subject) != "http://example.org/alice" {
		t.Errorf("Expected subject http://example.org/alice, got %s", getIRI(quads[0].Subject))
	}
}
