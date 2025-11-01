package rdf

import (
	"strings"
	"testing"
)

func TestRDFXMLParser_SimpleDescription(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:name>Alice</ex:name>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
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

func TestRDFXMLParser_MultipleProperties(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:name>Alice</ex:name>
    <ex:age>30</ex:age>
    <ex:city>Berlin</ex:city>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 3 {
		t.Fatalf("Expected 3 quads, got %d", len(quads))
	}

	// All should have the same subject
	expectedSubject := "http://example.org/alice"
	for i, quad := range quads {
		if getIRI(quad.Subject) != expectedSubject {
			t.Errorf("Quad %d: wrong subject: %s", i, getIRI(quad.Subject))
		}
	}

	// Check predicates and values
	expectedData := map[string]string{
		"http://example.org/name": "Alice",
		"http://example.org/age":  "30",
		"http://example.org/city": "Berlin",
	}

	for i, quad := range quads {
		predicate := getIRI(quad.Predicate)
		expectedValue, ok := expectedData[predicate]
		if !ok {
			t.Errorf("Quad %d: unexpected predicate: %s", i, predicate)
			continue
		}

		literal, ok := quad.Object.(*Literal)
		if !ok {
			t.Errorf("Quad %d: expected literal object, got %T", i, quad.Object)
			continue
		}
		if literal.Value != expectedValue {
			t.Errorf("Quad %d: expected value '%s', got '%s'", i, expectedValue, literal.Value)
		}
	}
}

func TestRDFXMLParser_ResourceAttribute(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:knows rdf:resource="http://example.org/bob"/>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	quad := quads[0]
	if getIRI(quad.Object) != "http://example.org/bob" {
		t.Errorf("Expected object IRI 'http://example.org/bob', got '%s'", getIRI(quad.Object))
	}
}

func TestRDFXMLParser_TypedLiteral(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xmlns:xsd="http://www.w3.org/2001/XMLSchema#">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:age rdf:datatype="http://www.w3.org/2001/XMLSchema#integer">30</ex:age>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
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

func TestRDFXMLParser_LanguageTag(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:name xml:lang="en">Alice</ex:name>
    <ex:name xml:lang="de">Alicia</ex:name>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check first literal
	lit0, ok := quads[0].Object.(*Literal)
	if !ok {
		t.Fatalf("Quad 0: expected literal object, got %T", quads[0].Object)
	}
	if lit0.Value != "Alice" {
		t.Errorf("Quad 0: expected value 'Alice', got '%s'", lit0.Value)
	}
	if lit0.Language != "en" {
		t.Errorf("Quad 0: expected language 'en', got '%s'", lit0.Language)
	}

	// Check second literal
	lit1, ok := quads[1].Object.(*Literal)
	if !ok {
		t.Fatalf("Quad 1: expected literal object, got %T", quads[1].Object)
	}
	if lit1.Value != "Alicia" {
		t.Errorf("Quad 1: expected value 'Alicia', got '%s'", lit1.Value)
	}
	if lit1.Language != "de" {
		t.Errorf("Quad 1: expected language 'de', got '%s'", lit1.Language)
	}
}

func TestRDFXMLParser_BlankNode(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description>
    <ex:name>Anonymous</ex:name>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("Expected 1 quad, got %d", len(quads))
	}

	// Subject should be a blank node
	_, ok := quads[0].Subject.(*BlankNode)
	if !ok {
		t.Errorf("Expected blank node subject, got %T", quads[0].Subject)
	}
}

func TestRDFXMLParser_NestedBlankNode(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:address>
      <rdf:Description>
        <ex:city>Berlin</ex:city>
        <ex:country>Germany</ex:country>
      </rdf:Description>
    </ex:address>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
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

	// Second quad: _:b1 ex:city "Berlin"
	subj1, ok := quads[1].Subject.(*BlankNode)
	if !ok {
		t.Fatalf("Quad 1: expected blank node subject, got %T", quads[1].Subject)
	}
	if subj1.ID != blankNode.ID {
		t.Errorf("Quad 1: blank node ID mismatch")
	}

	// Third quad: _:b1 ex:country "Germany"
	subj2, ok := quads[2].Subject.(*BlankNode)
	if !ok {
		t.Fatalf("Quad 2: expected blank node subject, got %T", quads[2].Subject)
	}
	if subj2.ID != blankNode.ID {
		t.Errorf("Quad 2: blank node ID mismatch")
	}
}

func TestRDFXMLParser_MultipleDescriptions(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:name>Alice</ex:name>
  </rdf:Description>
  <rdf:Description rdf:about="http://example.org/bob">
    <ex:name>Bob</ex:name>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
	quads, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check subjects are different
	subj0 := getIRI(quads[0].Subject)
	subj1 := getIRI(quads[1].Subject)

	if subj0 != "http://example.org/alice" {
		t.Errorf("Quad 0: expected subject alice, got %s", subj0)
	}
	if subj1 != "http://example.org/bob" {
		t.Errorf("Quad 1: expected subject bob, got %s", subj1)
	}
}

func TestRDFXMLParser_EmptyProperties(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:nickname></ex:nickname>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
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
	if literal.Value != "" {
		t.Errorf("Expected empty string, got '%s'", literal.Value)
	}
}

func TestRDFXMLParser_WhitespaceInLiterals(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:bio>  Alice Smith  </ex:bio>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
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

	// Whitespace should be preserved
	if literal.Value != "  Alice Smith  " {
		t.Errorf("Expected '  Alice Smith  ', got '%s'", literal.Value)
	}
}

func TestRDFXMLParser_AllInDefaultGraph(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/alice">
    <ex:name>Alice</ex:name>
    <ex:age>30</ex:age>
  </rdf:Description>
</rdf:RDF>`

	parser := NewRDFXMLParser()
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
