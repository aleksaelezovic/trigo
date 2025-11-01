package rdf

import (
	"testing"
	"time"
)

// ===== NamedNode Tests =====

func TestNamedNode_Type(t *testing.T) {
	node := NewNamedNode("http://example.org/resource")
	if node.Type() != TermTypeNamedNode {
		t.Errorf("Expected TermTypeNamedNode, got %v", node.Type())
	}
}

func TestNamedNode_String(t *testing.T) {
	node := NewNamedNode("http://example.org/resource")
	expected := "<http://example.org/resource>"
	if node.String() != expected {
		t.Errorf("Expected %s, got %s", expected, node.String())
	}
}

func TestNamedNode_Equals(t *testing.T) {
	node1 := NewNamedNode("http://example.org/resource")
	node2 := NewNamedNode("http://example.org/resource")
	node3 := NewNamedNode("http://example.org/different")

	if !node1.Equals(node2) {
		t.Error("Expected equal NamedNodes to be equal")
	}

	if node1.Equals(node3) {
		t.Error("Expected different NamedNodes to not be equal")
	}

	// Test with different term type
	literal := NewLiteral("test")
	if node1.Equals(literal) {
		t.Error("NamedNode should not equal Literal")
	}
}

// ===== BlankNode Tests =====

func TestBlankNode_Type(t *testing.T) {
	node := NewBlankNode("b1")
	if node.Type() != TermTypeBlankNode {
		t.Errorf("Expected TermTypeBlankNode, got %v", node.Type())
	}
}

func TestBlankNode_String(t *testing.T) {
	node := NewBlankNode("b1")
	expected := "_:b1"
	if node.String() != expected {
		t.Errorf("Expected %s, got %s", expected, node.String())
	}
}

func TestBlankNode_Equals(t *testing.T) {
	node1 := NewBlankNode("b1")
	node2 := NewBlankNode("b1")
	node3 := NewBlankNode("b2")

	if !node1.Equals(node2) {
		t.Error("Expected equal BlankNodes to be equal")
	}

	if node1.Equals(node3) {
		t.Error("Expected different BlankNodes to not be equal")
	}

	// Test with different term type
	namedNode := NewNamedNode("http://example.org/resource")
	if node1.Equals(namedNode) {
		t.Error("BlankNode should not equal NamedNode")
	}
}

// ===== Literal Tests =====

func TestLiteral_Type(t *testing.T) {
	literal := NewLiteral("test")
	if literal.Type() != TermTypeLiteral {
		t.Errorf("Expected TermTypeLiteral, got %v", literal.Type())
	}
}

func TestLiteral_String(t *testing.T) {
	tests := []struct {
		name     string
		literal  *Literal
		expected string
	}{
		{
			name:     "plain literal",
			literal:  NewLiteral("hello"),
			expected: "\"hello\"",
		},
		{
			name:     "literal with language",
			literal:  NewLiteralWithLanguage("hello", "en"),
			expected: "\"hello\"@en",
		},
		{
			name:     "literal with datatype",
			literal:  NewLiteralWithDatatype("42", NewNamedNode("http://www.w3.org/2001/XMLSchema#integer")),
			expected: "\"42\"^^<http://www.w3.org/2001/XMLSchema#integer>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.literal.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestLiteral_Equals(t *testing.T) {
	lit1 := NewLiteral("hello")
	lit2 := NewLiteral("hello")
	lit3 := NewLiteral("world")

	if !lit1.Equals(lit2) {
		t.Error("Expected equal plain literals to be equal")
	}

	if lit1.Equals(lit3) {
		t.Error("Expected different plain literals to not be equal")
	}

	// Language-tagged literals
	litLang1 := NewLiteralWithLanguage("hello", "en")
	litLang2 := NewLiteralWithLanguage("hello", "en")
	litLang3 := NewLiteralWithLanguage("hello", "fr")

	if !litLang1.Equals(litLang2) {
		t.Error("Expected equal language-tagged literals to be equal")
	}

	if litLang1.Equals(litLang3) {
		t.Error("Expected literals with different languages to not be equal")
	}

	if litLang1.Equals(lit1) {
		t.Error("Language-tagged literal should not equal plain literal")
	}

	// Typed literals
	litType1 := NewLiteralWithDatatype("42", XSDInteger)
	litType2 := NewLiteralWithDatatype("42", XSDInteger)
	litType3 := NewLiteralWithDatatype("42", XSDString)

	if !litType1.Equals(litType2) {
		t.Error("Expected equal typed literals to be equal")
	}

	if litType1.Equals(litType3) {
		t.Error("Expected literals with different datatypes to not be equal")
	}

	// Test with different term type
	namedNode := NewNamedNode("http://example.org/resource")
	if lit1.Equals(namedNode) {
		t.Error("Literal should not equal NamedNode")
	}
}

// ===== DefaultGraph Tests =====

func TestDefaultGraph_Type(t *testing.T) {
	graph := NewDefaultGraph()
	if graph.Type() != TermTypeDefaultGraph {
		t.Errorf("Expected TermTypeDefaultGraph, got %v", graph.Type())
	}
}

func TestDefaultGraph_String(t *testing.T) {
	graph := NewDefaultGraph()
	expected := "DEFAULT"
	if graph.String() != expected {
		t.Errorf("Expected %s, got %s", expected, graph.String())
	}
}

func TestDefaultGraph_Equals(t *testing.T) {
	graph1 := NewDefaultGraph()
	graph2 := NewDefaultGraph()

	if !graph1.Equals(graph2) {
		t.Error("Expected all DefaultGraph instances to be equal")
	}

	// Test with different term type
	namedNode := NewNamedNode("http://example.org/graph")
	if graph1.Equals(namedNode) {
		t.Error("DefaultGraph should not equal NamedNode")
	}
}

// ===== Triple Tests =====

func TestTriple_String(t *testing.T) {
	subject := NewNamedNode("http://example.org/subject")
	predicate := NewNamedNode("http://example.org/predicate")
	object := NewLiteral("value")

	triple := NewTriple(subject, predicate, object)
	expected := "<http://example.org/subject> <http://example.org/predicate> \"value\" ."

	if triple.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, triple.String())
	}
}

// ===== Quad Tests =====

func TestQuad_String(t *testing.T) {
	subject := NewNamedNode("http://example.org/subject")
	predicate := NewNamedNode("http://example.org/predicate")
	object := NewLiteral("value")
	graph := NewNamedNode("http://example.org/graph")

	quad := NewQuad(subject, predicate, object, graph)
	expected := "<http://example.org/subject> <http://example.org/predicate> \"value\" <http://example.org/graph> ."

	if quad.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, quad.String())
	}
}

func TestQuad_DefaultGraph(t *testing.T) {
	subject := NewNamedNode("http://example.org/subject")
	predicate := NewNamedNode("http://example.org/predicate")
	object := NewLiteral("value")
	defaultGraph := NewDefaultGraph()

	quad := NewQuad(subject, predicate, object, defaultGraph)
	expected := "<http://example.org/subject> <http://example.org/predicate> \"value\" DEFAULT ."

	if quad.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, quad.String())
	}
}

// ===== Typed Literal Constructor Tests =====

func TestNewIntegerLiteral(t *testing.T) {
	lit := NewIntegerLiteral(42)

	if lit.Value != "42" {
		t.Errorf("Expected value '42', got '%s'", lit.Value)
	}

	if lit.Datatype == nil || lit.Datatype.IRI != XSDInteger.IRI {
		t.Errorf("Expected datatype %s", XSDInteger.IRI)
	}
}

func TestNewDoubleLiteral(t *testing.T) {
	lit := NewDoubleLiteral(3.14)

	if lit.Value != "3.14" {
		t.Errorf("Expected value '3.14', got '%s'", lit.Value)
	}

	if lit.Datatype == nil || lit.Datatype.IRI != XSDDouble.IRI {
		t.Errorf("Expected datatype %s", XSDDouble.IRI)
	}
}

func TestNewBooleanLiteral(t *testing.T) {
	litTrue := NewBooleanLiteral(true)
	litFalse := NewBooleanLiteral(false)

	if litTrue.Value != "true" {
		t.Errorf("Expected value 'true', got '%s'", litTrue.Value)
	}

	if litFalse.Value != "false" {
		t.Errorf("Expected value 'false', got '%s'", litFalse.Value)
	}

	if litTrue.Datatype == nil || litTrue.Datatype.IRI != XSDBoolean.IRI {
		t.Errorf("Expected datatype %s", XSDBoolean.IRI)
	}
}

func TestNewDateTimeLiteral(t *testing.T) {
	testTime, _ := time.Parse(time.RFC3339, "2025-01-01T12:00:00Z")
	lit := NewDateTimeLiteral(testTime)

	if lit.Value != "2025-01-01T12:00:00Z" {
		t.Errorf("Expected value '2025-01-01T12:00:00Z', got '%s'", lit.Value)
	}

	if lit.Datatype == nil || lit.Datatype.IRI != XSDDateTime.IRI {
		t.Errorf("Expected datatype %s", XSDDateTime.IRI)
	}
}

// ===== Binary Encoding Tests =====

func TestEncodeDecodeInt64(t *testing.T) {
	testCases := []int64{
		0,
		1,
		-1,
		42,
		-42,
		9223372036854775807,  // Max int64
		-9223372036854775808, // Min int64
	}

	for _, val := range testCases {
		encoded := EncodeInt64BigEndian(val)

		if len(encoded) != 8 {
			t.Errorf("Expected 8 bytes, got %d", len(encoded))
		}

		decoded := DecodeInt64BigEndian(encoded)
		if decoded != val {
			t.Errorf("Expected %d, got %d", val, decoded)
		}
	}
}

func TestEncodeDecodeFloat64(t *testing.T) {
	testCases := []float64{
		0.0,
		1.0,
		-1.0,
		3.14,
		-3.14,
		1.7976931348623157e+308,  // Max float64
		2.2250738585072014e-308,  // Min positive float64
	}

	for _, val := range testCases {
		encoded := EncodeFloat64BigEndian(val)

		if len(encoded) != 8 {
			t.Errorf("Expected 8 bytes, got %d", len(encoded))
		}

		decoded := DecodeFloat64BigEndian(encoded)
		if decoded != val {
			t.Errorf("Expected %f, got %f", val, decoded)
		}
	}
}

// ===== XSD Datatype Constants Tests =====

func TestXSDConstants(t *testing.T) {
	constants := map[string]*NamedNode{
		"XSDString":   XSDString,
		"XSDInteger":  XSDInteger,
		"XSDDecimal":  XSDDecimal,
		"XSDDouble":   XSDDouble,
		"XSDBoolean":  XSDBoolean,
		"XSDDateTime": XSDDateTime,
		"XSDDate":     XSDDate,
		"XSDTime":     XSDTime,
		"XSDDuration": XSDDuration,
	}

	xsdNamespace := "http://www.w3.org/2001/XMLSchema#"

	for name, constant := range constants {
		if constant == nil {
			t.Errorf("%s constant is nil", name)
			continue
		}
		if constant.IRI == "" {
			t.Errorf("%s constant IRI is empty", name)
		}
		if len(constant.IRI) < len(xsdNamespace) || constant.IRI[:len(xsdNamespace)] != xsdNamespace {
			t.Errorf("%s constant doesn't start with XSD namespace: %s", name, constant.IRI)
		}
	}
}

// ===== Edge Case Tests =====

func TestLiteral_EmptyString(t *testing.T) {
	lit := NewLiteral("")
	if lit.Value != "" {
		t.Errorf("Expected empty string, got '%s'", lit.Value)
	}
	if lit.String() != "\"\"" {
		t.Errorf("Expected \"\", got %s", lit.String())
	}
}

func TestBlankNode_EmptyLabel(t *testing.T) {
	node := NewBlankNode("")
	if node.ID != "" {
		t.Errorf("Expected empty ID, got '%s'", node.ID)
	}
	if node.String() != "_:" {
		t.Errorf("Expected _:, got %s", node.String())
	}
}

func TestNamedNode_EmptyIRI(t *testing.T) {
	node := NewNamedNode("")
	if node.IRI != "" {
		t.Errorf("Expected empty IRI, got '%s'", node.IRI)
	}
	if node.String() != "<>" {
		t.Errorf("Expected <>, got %s", node.String())
	}
}
