package rdf

import (
	"testing"
)

// Helper function to get IRI from a term
func getIRI(t Term) string {
	if nn, ok := t.(*NamedNode); ok {
		return nn.IRI
	}
	return ""
}

func TestTurtleParser_PropertyListWithComma(t *testing.T) {
	input := `@prefix : <http://www.example.org/> .
:s :p :o1, :o2, :o3 .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 3 {
		t.Fatalf("Expected 3 triples, got %d", len(triples))
	}

	// Check that all have same subject and predicate
	for _, triple := range triples {
		if subj, ok := triple.Subject.(*NamedNode); ok {
			if subj.IRI != "http://www.example.org/s" {
				t.Errorf("Wrong subject: %s", subj.IRI)
			}
		}
		if pred, ok := triple.Predicate.(*NamedNode); ok {
			if pred.IRI != "http://www.example.org/p" {
				t.Errorf("Wrong predicate: %s", pred.IRI)
			}
		}
	}

	// Check objects
	expectedObjects := []string{
		"http://www.example.org/o1",
		"http://www.example.org/o2",
		"http://www.example.org/o3",
	}
	for i, triple := range triples {
		if obj, ok := triple.Object.(*NamedNode); ok {
			if obj.IRI != expectedObjects[i] {
				t.Errorf("Triple %d: expected object %s, got %s", i, expectedObjects[i], obj.IRI)
			}
		}
	}
}

func TestTurtleParser_PropertyListWithSemicolon(t *testing.T) {
	input := `@prefix : <http://www.example.org/> .
:s :p1 :o1 ; :p2 :o2 .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("Expected 2 triples, got %d", len(triples))
	}

	// Check first triple
	if getIRI(triples[0].Subject) != "http://www.example.org/s" {
		t.Errorf("Triple 0: wrong subject: %s", getIRI(triples[0].Subject))
	}
	if getIRI(triples[0].Predicate) != "http://www.example.org/p1" {
		t.Errorf("Triple 0: wrong predicate: %s", getIRI(triples[0].Predicate))
	}
	if getIRI(triples[0].Object) != "http://www.example.org/o1" {
		t.Errorf("Triple 0: wrong object: %s", getIRI(triples[0].Object))
	}

	// Check second triple
	if getIRI(triples[1].Subject) != "http://www.example.org/s" {
		t.Errorf("Triple 1: wrong subject: %s", getIRI(triples[1].Subject))
	}
	if getIRI(triples[1].Predicate) != "http://www.example.org/p2" {
		t.Errorf("Triple 1: wrong predicate: %s", getIRI(triples[1].Predicate))
	}
	if getIRI(triples[1].Object) != "http://www.example.org/o2" {
		t.Errorf("Triple 1: wrong object: %s", getIRI(triples[1].Object))
	}
}

func TestTurtleParser_PropertyListCombined(t *testing.T) {
	input := `@prefix : <http://www.example.org/> .
:s :p1 :o1, :o2 ; :p2 :o3, :o4 .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 4 {
		t.Fatalf("Expected 4 triples, got %d", len(triples))
	}

	expected := []struct {
		predicate string
		object    string
	}{
		{"http://www.example.org/p1", "http://www.example.org/o1"},
		{"http://www.example.org/p1", "http://www.example.org/o2"},
		{"http://www.example.org/p2", "http://www.example.org/o3"},
		{"http://www.example.org/p2", "http://www.example.org/o4"},
	}

	for i, triple := range triples {
		if getIRI(triple.Predicate) != expected[i].predicate {
			t.Errorf("Triple %d: expected predicate %s, got %s", i, expected[i].predicate, getIRI(triple.Predicate))
		}
		if getIRI(triple.Object) != expected[i].object {
			t.Errorf("Triple %d: expected object %s, got %s", i, expected[i].object, getIRI(triple.Object))
		}
	}
}

func TestTurtleParser_KeywordA(t *testing.T) {
	input := `@prefix ex: <http://www.w3.org/2009/sparql/docs/tests/data-sparql11/negation#> .
ex:lifeForm1 a ex:Mammal, ex:Animal .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("Expected 2 triples, got %d", len(triples))
	}

	// Both triples should have rdf:type as predicate
	rdfType := "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	for i, triple := range triples {
		if getIRI(triple.Predicate) != rdfType {
			t.Errorf("Triple %d: expected predicate rdf:type, got %s", i, getIRI(triple.Predicate))
		}
	}

	// Check objects
	expectedObjects := []string{
		"http://www.w3.org/2009/sparql/docs/tests/data-sparql11/negation#Mammal",
		"http://www.w3.org/2009/sparql/docs/tests/data-sparql11/negation#Animal",
	}
	for i, triple := range triples {
		if getIRI(triple.Object) != expectedObjects[i] {
			t.Errorf("Triple %d: expected object %s, got %s", i, expectedObjects[i], getIRI(triple.Object))
		}
	}
}

func TestTurtleParser_DatatypeWithPrefixedName(t *testing.T) {
	input := `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/> .
ex:s ex:date "2010-01-10"^^xsd:date .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d", len(triples))
	}

	triple := triples[0]
	literal, ok := triple.Object.(*Literal)
	if !ok {
		t.Fatalf("Object is not a literal")
	}

	if literal.Value != "2010-01-10" {
		t.Errorf("Expected literal value '2010-01-10', got '%s'", literal.Value)
	}

	if literal.Datatype == nil {
		t.Fatalf("Literal datatype is nil")
	}

	expectedDatatype := "http://www.w3.org/2001/XMLSchema#date"
	if literal.Datatype.IRI != expectedDatatype {
		t.Errorf("Expected datatype %s, got %s", expectedDatatype, literal.Datatype.IRI)
	}
}

func TestTurtleParser_TrailingSemicolon(t *testing.T) {
	input := `@prefix : <http://example.org/> .
:s :p :o ; .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple (trailing semicolon should be ignored), got %d", len(triples))
	}
}

func TestTurtleParser_PrefixedNameWithColons(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
ex:s ex:p ex:foo:bar .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d", len(triples))
	}

	expectedObject := "http://example.org/foo:bar"
	if getIRI(triples[0].Object) != expectedObject {
		t.Errorf("Expected object %s, got %s", expectedObject, getIRI(triples[0].Object))
	}
}

func TestTurtleParser_EmptyPrefix(t *testing.T) {
	input := `@prefix : <http://example.org/> .
:s :p :o .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d", len(triples))
	}

	expectedSubject := "http://example.org/s"
	if getIRI(triples[0].Subject) != expectedSubject {
		t.Errorf("Expected subject %s, got %s", expectedSubject, getIRI(triples[0].Subject))
	}
}

func TestTurtleParser_Comments(t *testing.T) {
	input := `# This is a comment
@prefix : <http://example.org/> .
# Another comment
:s :p :o . # Inline comment`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d", len(triples))
	}
}

func TestTurtleParser_LanguageTag(t *testing.T) {
	input := `@prefix : <http://example.org/> .
:s :p "hello"@en .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d", len(triples))
	}

	literal, ok := triples[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Object is not a literal")
	}

	if literal.Value != "hello" {
		t.Errorf("Expected literal value 'hello', got '%s'", literal.Value)
	}

	if literal.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", literal.Language)
	}
}

func TestTurtleParser_NumericLiterals(t *testing.T) {
	input := `@prefix : <http://example.org/> .
:s :intProp 42 .
:s :doubleProp 3.14 .
:s :negativeProp -10 .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 3 {
		t.Fatalf("Expected 3 triples, got %d", len(triples))
	}

	// Check integer literal
	intLiteral, ok := triples[0].Object.(*Literal)
	if !ok {
		t.Fatalf("First object is not a literal")
	}
	if intLiteral.Value != "42" {
		t.Errorf("Expected integer value '42', got '%s'", intLiteral.Value)
	}

	// Check double literal
	doubleLiteral, ok := triples[1].Object.(*Literal)
	if !ok {
		t.Fatalf("Second object is not a literal")
	}
	if doubleLiteral.Value != "3.14" {
		t.Errorf("Expected double value '3.14', got '%s'", doubleLiteral.Value)
	}

	// Check negative literal
	negativeLiteral, ok := triples[2].Object.(*Literal)
	if !ok {
		t.Fatalf("Third object is not a literal")
	}
	if negativeLiteral.Value != "-10" {
		t.Errorf("Expected negative value '-10', got '%s'", negativeLiteral.Value)
	}
}

func TestTurtleParser_BlankNodes(t *testing.T) {
	input := `@prefix : <http://example.org/> .
_:b1 :p :o .
:s :p _:b2 .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("Expected 2 triples, got %d", len(triples))
	}

	// Check first triple has blank node subject
	_, ok := triples[0].Subject.(*BlankNode)
	if !ok {
		t.Errorf("First triple subject is not a blank node")
	}

	// Check second triple has blank node object
	_, ok = triples[1].Object.(*BlankNode)
	if !ok {
		t.Errorf("Second triple object is not a blank node")
	}
}

func TestTurtleParser_EscapeSequences(t *testing.T) {
	input := `@prefix : <http://example.org/> .
:s :p "line1\nline2\ttabbed\"quoted\"" .`

	parser := NewTurtleParser(input)
	triples, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("Expected 1 triple, got %d", len(triples))
	}

	literal, ok := triples[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Object is not a literal")
	}

	expected := "line1\nline2\ttabbed\"quoted\""
	if literal.Value != expected {
		t.Errorf("Expected literal value %q, got %q", expected, literal.Value)
	}
}
