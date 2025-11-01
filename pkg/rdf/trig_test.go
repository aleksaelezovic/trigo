package rdf

import (
	"testing"
)

func TestTriGParser_SimpleDefaultGraph(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
ex:alice ex:name "Alice" .`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
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

	// Check it's in default graph
	_, ok := quad.Graph.(*DefaultGraph)
	if !ok {
		t.Errorf("Expected default graph, got %T", quad.Graph)
	}
}

func TestTriGParser_NamedGraph(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

GRAPH ex:graph1 {
  ex:bob ex:name "Bob" .
  ex:bob ex:age 30 .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Both should be in the named graph
	for i, quad := range quads {
		graphNode, ok := quad.Graph.(*NamedNode)
		if !ok {
			t.Errorf("Quad %d: expected named graph, got %T", i, quad.Graph)
			continue
		}
		if graphNode.IRI != "http://example.org/graph1" {
			t.Errorf("Quad %d: wrong graph IRI: %s", i, graphNode.IRI)
		}
	}
}

func TestTriGParser_MixedDefaultAndNamed(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

ex:alice ex:name "Alice" .

GRAPH ex:graph1 {
  ex:bob ex:name "Bob" .
}

ex:charlie ex:name "Charlie" .`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 3 {
		t.Fatalf("Expected 3 quads, got %d", len(quads))
	}

	// First quad - default graph
	_, ok := quads[0].Graph.(*DefaultGraph)
	if !ok {
		t.Errorf("Quad 0: expected default graph, got %T", quads[0].Graph)
	}

	// Second quad - named graph
	graphNode, ok := quads[1].Graph.(*NamedNode)
	if !ok {
		t.Errorf("Quad 1: expected named graph, got %T", quads[1].Graph)
	} else if graphNode.IRI != "http://example.org/graph1" {
		t.Errorf("Quad 1: wrong graph IRI: %s", graphNode.IRI)
	}

	// Third quad - default graph again
	_, ok = quads[2].Graph.(*DefaultGraph)
	if !ok {
		t.Errorf("Quad 2: expected default graph, got %T", quads[2].Graph)
	}
}

func TestTriGParser_EmptyPrefix(t *testing.T) {
	input := `PREFIX : <http://example.org/>

:s :p :o .

GRAPH :g {
  :s2 :p2 :o2 .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check first quad (default graph)
	if getIRI(quads[0].Subject) != "http://example.org/s" {
		t.Errorf("Quad 0: wrong subject: %s", getIRI(quads[0].Subject))
	}

	// Check second quad (named graph)
	if getIRI(quads[1].Subject) != "http://example.org/s2" {
		t.Errorf("Quad 1: wrong subject: %s", getIRI(quads[1].Subject))
	}
	graphNode, ok := quads[1].Graph.(*NamedNode)
	if !ok {
		t.Errorf("Quad 1: expected named graph, got %T", quads[1].Graph)
	} else if graphNode.IRI != "http://example.org/g" {
		t.Errorf("Quad 1: wrong graph IRI: %s", graphNode.IRI)
	}
}

func TestTriGParser_MultipleGraphs(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

GRAPH ex:graph1 {
  ex:alice ex:name "Alice" .
}

GRAPH ex:graph2 {
  ex:bob ex:name "Bob" .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check graphs
	graphs := []string{
		"http://example.org/graph1",
		"http://example.org/graph2",
	}

	for i, quad := range quads {
		graphNode, ok := quad.Graph.(*NamedNode)
		if !ok {
			t.Errorf("Quad %d: expected named graph, got %T", i, quad.Graph)
			continue
		}
		if graphNode.IRI != graphs[i] {
			t.Errorf("Quad %d: expected graph %s, got %s", i, graphs[i], graphNode.IRI)
		}
	}
}

func TestTriGParser_BlankNodesInGraph(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

GRAPH ex:graph1 {
  _:b1 ex:name "Anonymous" .
  ex:alice ex:knows _:b1 .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// First quad should have blank node subject
	_, ok := quads[0].Subject.(*BlankNode)
	if !ok {
		t.Errorf("Quad 0: expected blank node subject, got %T", quads[0].Subject)
	}

	// Second quad should have blank node object
	_, ok = quads[1].Object.(*BlankNode)
	if !ok {
		t.Errorf("Quad 1: expected blank node object, got %T", quads[1].Object)
	}
}

func TestTriGParser_LiteralsInGraph(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

GRAPH ex:graph1 {
  ex:alice ex:name "Alice" .
  ex:alice ex:age "30"^^xsd:integer .
  ex:alice ex:nickname "Ali"@en .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 3 {
		t.Fatalf("Expected 3 quads, got %d", len(quads))
	}

	// Check plain literal
	lit0, ok := quads[0].Object.(*Literal)
	if !ok {
		t.Errorf("Quad 0: expected literal object, got %T", quads[0].Object)
	} else if lit0.Value != "Alice" {
		t.Errorf("Quad 0: expected value 'Alice', got '%s'", lit0.Value)
	}

	// Check typed literal
	lit1, ok := quads[1].Object.(*Literal)
	if !ok {
		t.Errorf("Quad 1: expected literal object, got %T", quads[1].Object)
	} else {
		if lit1.Value != "30" {
			t.Errorf("Quad 1: expected value '30', got '%s'", lit1.Value)
		}
		if lit1.Datatype == nil || lit1.Datatype.IRI != "http://www.w3.org/2001/XMLSchema#integer" {
			t.Errorf("Quad 1: wrong datatype")
		}
	}

	// Check language-tagged literal
	lit2, ok := quads[2].Object.(*Literal)
	if !ok {
		t.Errorf("Quad 2: expected literal object, got %T", quads[2].Object)
	} else {
		if lit2.Value != "Ali" {
			t.Errorf("Quad 2: expected value 'Ali', got '%s'", lit2.Value)
		}
		if lit2.Language != "en" {
			t.Errorf("Quad 2: expected language 'en', got '%s'", lit2.Language)
		}
	}
}

func TestTriGParser_Comments(t *testing.T) {
	input := `# This is a comment
@prefix ex: <http://example.org/> .

# Default graph triple
ex:alice ex:name "Alice" .

# Named graph
GRAPH ex:graph1 {
  # Triple in named graph
  ex:bob ex:name "Bob" . # Inline comment
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}
}

func TestTriGParser_BaseDeclaration(t *testing.T) {
	input := `BASE <http://example.org/>

<alice> <name> "Alice" .

GRAPH <graph1> {
  <bob> <name> "Bob" .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(quads) != 2 {
		t.Fatalf("Expected 2 quads, got %d", len(quads))
	}

	// Check that base was applied
	if getIRI(quads[0].Subject) != "http://example.org/alice" {
		t.Errorf("Quad 0: base not applied to subject: %s", getIRI(quads[0].Subject))
	}

	graphNode, ok := quads[1].Graph.(*NamedNode)
	if !ok {
		t.Errorf("Quad 1: expected named graph, got %T", quads[1].Graph)
	} else if graphNode.IRI != "http://example.org/graph1" {
		t.Errorf("Quad 1: base not applied to graph: %s", graphNode.IRI)
	}
}

func TestTriGParser_EscapeSequences(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

GRAPH ex:graph1 {
  ex:alice ex:bio "line1\nline2\ttabbed\"quoted\"" .
}`

	parser := NewTriGParser(input)
	quads, err := parser.Parse()
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

	expected := "line1\nline2\ttabbed\"quoted\""
	if literal.Value != expected {
		t.Errorf("Expected literal value %q, got %q", expected, literal.Value)
	}
}
