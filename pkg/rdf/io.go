package rdf

import (
	"fmt"
	"io"
	"strings"
)

// RDFParser is the interface for parsing RDF data in various formats
type RDFParser interface {
	// Parse parses RDF data from a reader and returns quads
	Parse(reader io.Reader) ([]*Quad, error)

	// ContentType returns the MIME type this parser handles
	ContentType() string
}

// NewParser creates an RDF parser based on the content type
func NewParser(contentType string) (RDFParser, error) {
	// Normalize content type (remove parameters like charset)
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}

	switch ct {
	case "application/n-triples", "text/plain":
		return &NTriplesIOParser{}, nil
	case "application/n-quads":
		return &NQuadsIOParser{}, nil
	case "text/turtle", "application/x-turtle":
		return &TurtleIOParser{}, nil
	case "application/trig", "application/x-trig":
		return &TriGIOParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// NTriplesIOParser parses N-Triples format (triples only, default graph)
type NTriplesIOParser struct{}

func (p *NTriplesIOParser) ContentType() string {
	return "application/n-triples"
}

func (p *NTriplesIOParser) Parse(reader io.Reader) ([]*Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use turtle parser (which handles N-Triples as a subset)
	turtleParser := NewTurtleParser(string(data))
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing N-Triples: %w", err)
	}

	// Convert triples to quads (default graph)
	quads := make([]*Quad, len(triples))
	for i, triple := range triples {
		quads[i] = NewQuad(
			triple.Subject,
			triple.Predicate,
			triple.Object,
			NewDefaultGraph(),
		)
	}

	return quads, nil
}

// NQuadsIOParser parses N-Quads format (quads with optional graph)
type NQuadsIOParser struct{}

func (p *NQuadsIOParser) ContentType() string {
	return "application/n-quads"
}

func (p *NQuadsIOParser) Parse(reader io.Reader) ([]*Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use N-Quads parser
	nquadsParser := NewNQuadsParser(string(data))
	quads, err := nquadsParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing N-Quads: %w", err)
	}

	return quads, nil
}

// TurtleIOParser parses Turtle format (triples with prefixes, default graph)
type TurtleIOParser struct{}

func (p *TurtleIOParser) ContentType() string {
	return "text/turtle"
}

func (p *TurtleIOParser) Parse(reader io.Reader) ([]*Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use turtle parser
	turtleParser := NewTurtleParser(string(data))
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing Turtle: %w", err)
	}

	// Convert triples to quads (default graph)
	quads := make([]*Quad, len(triples))
	for i, triple := range triples {
		quads[i] = NewQuad(
			triple.Subject,
			triple.Predicate,
			triple.Object,
			NewDefaultGraph(),
		)
	}

	return quads, nil
}

// TriGIOParser parses TriG format (Turtle + named graphs, quads)
type TriGIOParser struct{}

func (p *TriGIOParser) ContentType() string {
	return "application/trig"
}

func (p *TriGIOParser) Parse(reader io.Reader) ([]*Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use TriG parser
	trigParser := NewTriGParser(string(data))
	quads, err := trigParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing TriG: %w", err)
	}

	return quads, nil
}

// GetSupportedContentTypes returns a list of all supported content types
func GetSupportedContentTypes() []string {
	return []string{
		"application/n-triples",
		"application/n-quads",
		"text/turtle",
		"application/x-turtle",
		"application/trig",
		"application/x-trig",
		"text/plain", // Alias for N-Triples
	}
}
