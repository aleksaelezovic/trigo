package rdf

import (
	"testing"
)

func TestAreGraphsIsomorphic_EmptyGraphs(t *testing.T) {
	expected := []*Triple{}
	actual := []*Triple{}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Empty graphs should be isomorphic")
	}
}

func TestAreGraphsIsomorphic_NoBlankNodes(t *testing.T) {
	expected := []*Triple{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &Literal{Value: "object", Datatype: XSDString},
		},
	}

	actual := []*Triple{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &Literal{Value: "object", Datatype: XSDString},
		},
	}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Identical graphs without blank nodes should be isomorphic")
	}
}

func TestAreGraphsIsomorphic_NoBlankNodes_Different(t *testing.T) {
	expected := []*Triple{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject1"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &Literal{Value: "object", Datatype: XSDString},
		},
	}

	actual := []*Triple{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject2"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &Literal{Value: "object", Datatype: XSDString},
		},
	}

	if AreGraphsIsomorphic(expected, actual) {
		t.Error("Different graphs should not be isomorphic")
	}
}

func TestAreGraphsIsomorphic_SingleBlankNode(t *testing.T) {
	// Expected: _:j0 <http://example.org/property> "value" .
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "j0"},
			Predicate: &NamedNode{IRI: "http://example.org/property"},
			Object:    &Literal{Value: "value", Datatype: XSDString},
		},
	}

	// Actual: _:b1 <http://example.org/property> "value" .
	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://example.org/property"},
			Object:    &Literal{Value: "value", Datatype: XSDString},
		},
	}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with single blank node should be isomorphic despite different labels")
	}
}

func TestAreGraphsIsomorphic_SingleBlankNode_Different(t *testing.T) {
	// Expected: _:j0 <http://example.org/property1> "value" .
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "j0"},
			Predicate: &NamedNode{IRI: "http://example.org/property1"},
			Object:    &Literal{Value: "value", Datatype: XSDString},
		},
	}

	// Actual: _:b1 <http://example.org/property2> "value" .
	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://example.org/property2"},
			Object:    &Literal{Value: "value", Datatype: XSDString},
		},
	}

	if AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with different predicates should not be isomorphic")
	}
}

func TestAreGraphsIsomorphic_MultipleBlankNodes(t *testing.T) {
	// Expected:
	// _:bag rdf:type rdf:Bag .
	// _:bag rdf:_1 <http://example.org/item1> .
	// _:bag rdf:_2 <http://example.org/item2> .
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "bag"},
			Predicate: &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"},
			Object:    &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#Bag"},
		},
		{
			Subject:   &BlankNode{ID: "bag"},
			Predicate: &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#_1"},
			Object:    &NamedNode{IRI: "http://example.org/item1"},
		},
		{
			Subject:   &BlankNode{ID: "bag"},
			Predicate: &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#_2"},
			Object:    &NamedNode{IRI: "http://example.org/item2"},
		},
	}

	// Actual:
	// _:b1 rdf:type rdf:Bag .
	// _:b1 rdf:_1 <http://example.org/item1> .
	// _:b1 rdf:_2 <http://example.org/item2> .
	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"},
			Object:    &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#Bag"},
		},
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#_1"},
			Object:    &NamedNode{IRI: "http://example.org/item1"},
		},
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns#_2"},
			Object:    &NamedNode{IRI: "http://example.org/item2"},
		},
	}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with same blank node used multiple times should be isomorphic")
	}
}

func TestAreGraphsIsomorphic_TwoDistinctBlankNodes(t *testing.T) {
	// Expected:
	// _:a <http://example.org/knows> _:b .
	// _:a <http://example.org/name> "Alice" .
	// _:b <http://example.org/name> "Bob" .
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "a"},
			Predicate: &NamedNode{IRI: "http://example.org/knows"},
			Object:    &BlankNode{ID: "b"},
		},
		{
			Subject:   &BlankNode{ID: "a"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Alice", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "b"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
	}

	// Actual:
	// _:x <http://example.org/knows> _:y .
	// _:x <http://example.org/name> "Alice" .
	// _:y <http://example.org/name> "Bob" .
	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "x"},
			Predicate: &NamedNode{IRI: "http://example.org/knows"},
			Object:    &BlankNode{ID: "y"},
		},
		{
			Subject:   &BlankNode{ID: "x"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Alice", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "y"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
	}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with two distinct blank nodes should be isomorphic")
	}
}

func TestAreGraphsIsomorphic_TwoDistinctBlankNodes_WrongMapping(t *testing.T) {
	// Expected:
	// _:a <http://example.org/knows> _:b .
	// _:a <http://example.org/name> "Alice" .
	// _:b <http://example.org/name> "Bob" .
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "a"},
			Predicate: &NamedNode{IRI: "http://example.org/knows"},
			Object:    &BlankNode{ID: "b"},
		},
		{
			Subject:   &BlankNode{ID: "a"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Alice", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "b"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
	}

	// Actual: Names are swapped - should not be isomorphic
	// _:x <http://example.org/knows> _:y .
	// _:x <http://example.org/name> "Bob" .
	// _:y <http://example.org/name> "Alice" .
	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "x"},
			Predicate: &NamedNode{IRI: "http://example.org/knows"},
			Object:    &BlankNode{ID: "y"},
		},
		{
			Subject:   &BlankNode{ID: "x"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "y"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Alice", Datatype: XSDString},
		},
	}

	if AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with swapped blank node associations should not be isomorphic")
	}
}

func TestAreGraphsIsomorphic_DifferentNumberOfTriples(t *testing.T) {
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "a"},
			Predicate: &NamedNode{IRI: "http://example.org/property"},
			Object:    &Literal{Value: "value", Datatype: XSDString},
		},
	}

	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "x"},
			Predicate: &NamedNode{IRI: "http://example.org/property"},
			Object:    &Literal{Value: "value", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "y"},
			Predicate: &NamedNode{IRI: "http://example.org/property"},
			Object:    &Literal{Value: "value2", Datatype: XSDString},
		},
	}

	if AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with different number of triples should not be isomorphic")
	}
}

func TestAreGraphsIsomorphic_ComplexGraph(t *testing.T) {
	// Expected: A more complex graph with interconnected blank nodes
	// _:person1 <http://example.org/name> "Alice" .
	// _:person1 <http://example.org/friend> _:person2 .
	// _:person1 <http://example.org/friend> _:person3 .
	// _:person2 <http://example.org/name> "Bob" .
	// _:person3 <http://example.org/name> "Charlie" .
	expected := []*Triple{
		{
			Subject:   &BlankNode{ID: "person1"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Alice", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "person1"},
			Predicate: &NamedNode{IRI: "http://example.org/friend"},
			Object:    &BlankNode{ID: "person2"},
		},
		{
			Subject:   &BlankNode{ID: "person1"},
			Predicate: &NamedNode{IRI: "http://example.org/friend"},
			Object:    &BlankNode{ID: "person3"},
		},
		{
			Subject:   &BlankNode{ID: "person2"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "person3"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Charlie", Datatype: XSDString},
		},
	}

	// Actual: Same structure with different labels
	actual := []*Triple{
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Alice", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://example.org/friend"},
			Object:    &BlankNode{ID: "b2"},
		},
		{
			Subject:   &BlankNode{ID: "b1"},
			Predicate: &NamedNode{IRI: "http://example.org/friend"},
			Object:    &BlankNode{ID: "b3"},
		},
		{
			Subject:   &BlankNode{ID: "b2"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
		{
			Subject:   &BlankNode{ID: "b3"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Charlie", Datatype: XSDString},
		},
	}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Complex graphs with multiple interconnected blank nodes should be isomorphic")
	}
}

func TestAreGraphsIsomorphic_MixedBlankAndNamed(t *testing.T) {
	// Expected: Mix of blank nodes and named nodes
	// <http://example.org/alice> <http://example.org/knows> _:b .
	// _:b <http://example.org/name> "Bob" .
	expected := []*Triple{
		{
			Subject:   &NamedNode{IRI: "http://example.org/alice"},
			Predicate: &NamedNode{IRI: "http://example.org/knows"},
			Object:    &BlankNode{ID: "b"},
		},
		{
			Subject:   &BlankNode{ID: "b"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
	}

	// Actual: Same structure, different blank node label
	actual := []*Triple{
		{
			Subject:   &NamedNode{IRI: "http://example.org/alice"},
			Predicate: &NamedNode{IRI: "http://example.org/knows"},
			Object:    &BlankNode{ID: "person1"},
		},
		{
			Subject:   &BlankNode{ID: "person1"},
			Predicate: &NamedNode{IRI: "http://example.org/name"},
			Object:    &Literal{Value: "Bob", Datatype: XSDString},
		},
	}

	if !AreGraphsIsomorphic(expected, actual) {
		t.Error("Graphs with mix of blank and named nodes should be isomorphic")
	}
}

// Quad tests

func TestAreQuadsIsomorphic_EmptyGraphs(t *testing.T) {
	expected := []*Quad{}
	actual := []*Quad{}

	if !AreQuadsIsomorphic(expected, actual) {
		t.Error("Empty quad graphs should be isomorphic")
	}
}

func TestAreQuadsIsomorphic_NoBlankNodes(t *testing.T) {
	expected := []*Quad{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &Literal{Value: "object", Datatype: XSDString},
			Graph:     &NamedNode{IRI: "http://example.org/graph"},
		},
	}

	actual := []*Quad{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &Literal{Value: "object", Datatype: XSDString},
			Graph:     &NamedNode{IRI: "http://example.org/graph"},
		},
	}

	if !AreQuadsIsomorphic(expected, actual) {
		t.Error("Identical quads without blank nodes should be isomorphic")
	}
}

func TestAreQuadsIsomorphic_BlankNodeGraph(t *testing.T) {
	// Expected: Quads with blank node as graph name
	// <s> <p> <o> _:g1 .
	expected := []*Quad{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &NamedNode{IRI: "http://example.org/object"},
			Graph:     &BlankNode{ID: "g1"},
		},
	}

	// Actual: Same quad, different blank node label for graph
	// <s> <p> <o> _:graph0 .
	actual := []*Quad{
		{
			Subject:   &NamedNode{IRI: "http://example.org/subject"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &NamedNode{IRI: "http://example.org/object"},
			Graph:     &BlankNode{ID: "graph0"},
		},
	}

	if !AreQuadsIsomorphic(expected, actual) {
		t.Error("Quads with blank node graphs should be isomorphic despite different labels")
	}
}

func TestAreQuadsIsomorphic_BlankNodesInTripleAndGraph(t *testing.T) {
	// Expected: Blank nodes in both triple and graph positions
	// _:subj <p> _:obj _:g .
	expected := []*Quad{
		{
			Subject:   &BlankNode{ID: "subj"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &BlankNode{ID: "obj"},
			Graph:     &BlankNode{ID: "g"},
		},
	}

	// Actual: Same structure, different labels
	// _:s1 <p> _:o1 _:g1 .
	actual := []*Quad{
		{
			Subject:   &BlankNode{ID: "s1"},
			Predicate: &NamedNode{IRI: "http://example.org/predicate"},
			Object:    &BlankNode{ID: "o1"},
			Graph:     &BlankNode{ID: "g1"},
		},
	}

	if !AreQuadsIsomorphic(expected, actual) {
		t.Error("Quads with blank nodes in all positions should be isomorphic")
	}
}

func TestAreQuadsIsomorphic_ComplexQuadGraph(t *testing.T) {
	// Expected: Multiple quads with shared blank nodes
	// _:s <p1> "value1" _:g .
	// _:s <p2> "value2" _:g .
	expected := []*Quad{
		{
			Subject:   &BlankNode{ID: "s"},
			Predicate: &NamedNode{IRI: "http://example.org/p1"},
			Object:    &Literal{Value: "value1", Datatype: XSDString},
			Graph:     &BlankNode{ID: "g"},
		},
		{
			Subject:   &BlankNode{ID: "s"},
			Predicate: &NamedNode{IRI: "http://example.org/p2"},
			Object:    &Literal{Value: "value2", Datatype: XSDString},
			Graph:     &BlankNode{ID: "g"},
		},
	}

	// Actual: Same structure, different labels
	actual := []*Quad{
		{
			Subject:   &BlankNode{ID: "subj1"},
			Predicate: &NamedNode{IRI: "http://example.org/p1"},
			Object:    &Literal{Value: "value1", Datatype: XSDString},
			Graph:     &BlankNode{ID: "graph1"},
		},
		{
			Subject:   &BlankNode{ID: "subj1"},
			Predicate: &NamedNode{IRI: "http://example.org/p2"},
			Object:    &Literal{Value: "value2", Datatype: XSDString},
			Graph:     &BlankNode{ID: "graph1"},
		},
	}

	if !AreQuadsIsomorphic(expected, actual) {
		t.Error("Complex quad graphs with shared blank nodes should be isomorphic")
	}
}
