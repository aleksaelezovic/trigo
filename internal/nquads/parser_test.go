package nquads

import (
	"testing"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

func TestParseNQuads(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // number of quads expected
		wantErr  bool
	}{
		{
			name: "simple triple (N-Triples format)",
			input: `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`,
			expected: 1,
			wantErr:  false,
		},
		{
			name: "quad with named graph",
			input: `<http://example.org/s> <http://example.org/p> <http://example.org/o> <http://example.org/g> .
`,
			expected: 1,
			wantErr:  false,
		},
		{
			name: "multiple quads",
			input: `<http://example.org/s1> <http://example.org/p1> "literal1" .
<http://example.org/s2> <http://example.org/p2> "literal2"^^<http://www.w3.org/2001/XMLSchema#string> <http://example.org/g> .
<http://example.org/s3> <http://example.org/p3> "hello"@en .
`,
			expected: 3,
			wantErr:  false,
		},
		{
			name: "with PREFIX",
			input: `PREFIX ex: <http://example.org/>
ex:s ex:p ex:o .
`,
			expected: 1,
			wantErr:  false,
		},
		{
			name: "blank nodes",
			input: `_:b1 <http://example.org/p> "value" .
<http://example.org/s> <http://example.org/p> _:b2 _:graph .
`,
			expected: 2,
			wantErr:  false,
		},
		{
			name: "numeric literals",
			input: `<http://example.org/s> <http://example.org/p> 42 .
<http://example.org/s2> <http://example.org/p2> 3.14 .
`,
			expected: 2,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			quads, err := parser.Parse()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(quads) != tt.expected {
				t.Errorf("expected %d quads, got %d", tt.expected, len(quads))
			}

			// Verify quads are not nil
			for i, quad := range quads {
				if quad == nil {
					t.Errorf("quad %d is nil", i)
					continue
				}
				if quad.Subject == nil {
					t.Errorf("quad %d has nil subject", i)
				}
				if quad.Predicate == nil {
					t.Errorf("quad %d has nil predicate", i)
				}
				if quad.Object == nil {
					t.Errorf("quad %d has nil object", i)
				}
				if quad.Graph == nil {
					t.Errorf("quad %d has nil graph", i)
				}
			}
		})
	}
}

func TestParseNQuadsWithGraph(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <http://example.org/o> <http://example.org/g> .
`
	parser := NewParser(input)
	quads, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("expected 1 quad, got %d", len(quads))
	}

	quad := quads[0]

	// Check that graph is not the default graph
	if quad.Graph.Type() == rdf.TermTypeDefaultGraph {
		t.Error("expected named graph, got default graph")
	}

	// Check that graph is a named node with correct IRI
	if quad.Graph.Type() != rdf.TermTypeNamedNode {
		t.Errorf("expected named node for graph, got type %d", quad.Graph.Type())
	}
}

func TestParseNTriplesAsQuads(t *testing.T) {
	// N-Triples should be parsed as quads in the default graph
	input := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`
	parser := NewParser(input)
	quads, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(quads) != 1 {
		t.Fatalf("expected 1 quad, got %d", len(quads))
	}

	quad := quads[0]

	// Check that graph is the default graph
	if quad.Graph.Type() != rdf.TermTypeDefaultGraph {
		t.Errorf("expected default graph, got type %d", quad.Graph.Type())
	}
}
