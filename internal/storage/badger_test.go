package storage

import (
	"testing"

	"github.com/aleksaelezovic/trigo/internal/encoding"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

func TestBatchInsertAndQuery(t *testing.T) {
	// Create temporary storage
	tmpDir := t.TempDir()
	storage, err := NewBadgerStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	tripleStore := store.NewTripleStore(storage, encoding.NewTermEncoder(), encoding.NewTermDecoder())

	// Create test quads
	quads := []*rdf.Quad{
		rdf.NewQuad(
			rdf.NewNamedNode("http://example.org/alice"),
			rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
			rdf.NewLiteral("Alice"),
			rdf.NewDefaultGraph(),
		),
		rdf.NewQuad(
			rdf.NewNamedNode("http://example.org/bob"),
			rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
			rdf.NewLiteral("Bob"),
			rdf.NewDefaultGraph(),
		),
		rdf.NewQuad(
			rdf.NewNamedNode("http://example.org/charlie"),
			rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
			rdf.NewLiteral("Charlie"),
			rdf.NewNamedNode("http://example.org/graph1"),
		),
	}

	// Batch insert
	err = tripleStore.InsertQuadsBatch(quads)
	if err != nil {
		t.Fatalf("failed to batch insert: %v", err)
	}

	// Query: Check count
	count, err := tripleStore.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}

	// Query: Get all triples from default graph
	pattern := &store.Pattern{
		Subject:   &store.Variable{Name: "s"},
		Predicate: &store.Variable{Name: "p"},
		Object:    &store.Variable{Name: "o"},
		Graph:     rdf.NewDefaultGraph(),
	}

	iter, err := tripleStore.Query(pattern)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer iter.Close()

	defaultGraphCount := 0
	for iter.Next() {
		quad, err := iter.Quad()
		if err != nil {
			t.Fatalf("failed to get quad: %v", err)
		}
		if quad == nil {
			t.Fatal("got nil quad")
		}
		defaultGraphCount++

		// Verify it's in default graph
		if quad.Graph.Type() != rdf.TermTypeDefaultGraph {
			t.Errorf("expected default graph, got type %d", quad.Graph.Type())
		}
	}

	if defaultGraphCount != 2 {
		t.Errorf("expected 2 quads in default graph, got %d", defaultGraphCount)
	}

	// Query: Get triples from named graph
	namedGraphPattern := &store.Pattern{
		Subject:   &store.Variable{Name: "s"},
		Predicate: &store.Variable{Name: "p"},
		Object:    &store.Variable{Name: "o"},
		Graph:     rdf.NewNamedNode("http://example.org/graph1"),
	}

	iter2, err := tripleStore.Query(namedGraphPattern)
	if err != nil {
		t.Fatalf("failed to query named graph: %v", err)
	}
	defer iter2.Close()

	namedGraphCount := 0
	for iter2.Next() {
		quad, err := iter2.Quad()
		if err != nil {
			t.Fatalf("failed to get quad from named graph: %v", err)
		}
		if quad == nil {
			t.Fatal("got nil quad from named graph")
		}
		namedGraphCount++

		// Verify subject is charlie
		if quad.Subject.Type() != rdf.TermTypeNamedNode {
			t.Errorf("expected named node subject, got type %d", quad.Subject.Type())
		}
		subjectNode, ok := quad.Subject.(*rdf.NamedNode)
		if !ok {
			t.Error("failed to cast subject to NamedNode")
		} else if subjectNode.IRI != "http://example.org/charlie" {
			t.Errorf("expected charlie, got %s", subjectNode.IRI)
		}
	}

	if namedGraphCount != 1 {
		t.Errorf("expected 1 quad in named graph, got %d", namedGraphCount)
	}
}

func TestBatchInsertAndQuerySpecificValues(t *testing.T) {
	// Create temporary storage
	tmpDir := t.TempDir()
	storage, err := NewBadgerStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	tripleStore := store.NewTripleStore(storage, encoding.NewTermEncoder(), encoding.NewTermDecoder())

	// Create test data with specific values we'll query
	aliceNode := rdf.NewNamedNode("http://example.org/alice")
	nameProperty := rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name")
	aliceLiteral := rdf.NewLiteral("Alice")

	quads := []*rdf.Quad{
		rdf.NewQuad(
			aliceNode,
			nameProperty,
			aliceLiteral,
			rdf.NewDefaultGraph(),
		),
		rdf.NewQuad(
			rdf.NewNamedNode("http://example.org/alice"),
			rdf.NewNamedNode("http://xmlns.com/foaf/0.1/age"),
			rdf.NewLiteralWithDatatype("30", rdf.XSDInteger),
			rdf.NewDefaultGraph(),
		),
	}

	// Batch insert
	err = tripleStore.InsertQuadsBatch(quads)
	if err != nil {
		t.Fatalf("failed to batch insert: %v", err)
	}

	// Query: Find alice's name (subject and predicate bound)
	pattern := &store.Pattern{
		Subject:   aliceNode,
		Predicate: nameProperty,
		Object:    &store.Variable{Name: "o"},
		Graph:     rdf.NewDefaultGraph(),
	}

	iter, err := tripleStore.Query(pattern)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer iter.Close()

	found := false
	for iter.Next() {
		quad, err := iter.Quad()
		if err != nil {
			t.Fatalf("failed to get quad: %v", err)
		}

		// Verify the object is "Alice"
		if quad.Object.Type() != rdf.TermTypeLiteral {
			t.Errorf("expected literal object, got type %d", quad.Object.Type())
		}
		literal, ok := quad.Object.(*rdf.Literal)
		if !ok {
			t.Error("failed to cast object to Literal")
		} else if literal.Value != "Alice" {
			t.Errorf("expected 'Alice', got '%s'", literal.Value)
		} else {
			found = true
		}
	}

	if !found {
		t.Error("did not find alice's name")
	}
}

func TestBatchDeleteAndQuery(t *testing.T) {
	// Create temporary storage
	tmpDir := t.TempDir()
	storage, err := NewBadgerStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	tripleStore := store.NewTripleStore(storage, encoding.NewTermEncoder(), encoding.NewTermDecoder())

	// Create and insert test quads
	quads := []*rdf.Quad{
		rdf.NewQuad(
			rdf.NewNamedNode("http://example.org/alice"),
			rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
			rdf.NewLiteral("Alice"),
			rdf.NewDefaultGraph(),
		),
		rdf.NewQuad(
			rdf.NewNamedNode("http://example.org/bob"),
			rdf.NewNamedNode("http://xmlns.com/foaf/0.1/name"),
			rdf.NewLiteral("Bob"),
			rdf.NewDefaultGraph(),
		),
	}

	err = tripleStore.InsertQuadsBatch(quads)
	if err != nil {
		t.Fatalf("failed to batch insert: %v", err)
	}

	// Verify count before delete
	count, err := tripleStore.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2 before delete, got %d", count)
	}

	// Batch delete one quad
	err = tripleStore.DeleteQuadsBatch([]*rdf.Quad{quads[0]})
	if err != nil {
		t.Fatalf("failed to batch delete: %v", err)
	}

	// Verify count after delete
	count, err = tripleStore.Count()
	if err != nil {
		t.Fatalf("failed to count after delete: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 after delete, got %d", count)
	}

	// Query to verify only Bob remains
	pattern := &store.Pattern{
		Subject:   &store.Variable{Name: "s"},
		Predicate: &store.Variable{Name: "p"},
		Object:    &store.Variable{Name: "o"},
		Graph:     rdf.NewDefaultGraph(),
	}

	iter, err := tripleStore.Query(pattern)
	if err != nil {
		t.Fatalf("failed to query after delete: %v", err)
	}
	defer iter.Close()

	foundBob := false
	foundAlice := false
	for iter.Next() {
		quad, err := iter.Quad()
		if err != nil {
			t.Fatalf("failed to get quad: %v", err)
		}

		subject, ok := quad.Subject.(*rdf.NamedNode)
		if !ok {
			t.Error("expected NamedNode subject")
			continue
		}

		if subject.IRI == "http://example.org/bob" {
			foundBob = true
		}
		if subject.IRI == "http://example.org/alice" {
			foundAlice = true
		}
	}

	if !foundBob {
		t.Error("Bob should still be present after delete")
	}
	if foundAlice {
		t.Error("Alice should be deleted")
	}
}
