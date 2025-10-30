package rdfio

import (
	"fmt"
	"io"
	"strings"

	"github.com/aleksaelezovic/trigo/internal/nquads"
	"github.com/aleksaelezovic/trigo/internal/turtle"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// RDFParser is the interface for parsing RDF data in various formats
type RDFParser interface {
	// Parse parses RDF data from a reader and returns quads
	Parse(reader io.Reader) ([]*rdf.Quad, error)

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
		return &NTriplesParser{}, nil
	case "application/n-quads":
		return &NQuadsParser{}, nil
	case "text/turtle", "application/x-turtle":
		return &TurtleParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// NTriplesParser parses N-Triples format (triples only, default graph)
type NTriplesParser struct{}

func (p *NTriplesParser) ContentType() string {
	return "application/n-triples"
}

func (p *NTriplesParser) Parse(reader io.Reader) ([]*rdf.Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use turtle parser (which handles N-Triples as a subset)
	turtleParser := turtle.NewParser(string(data))
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing N-Triples: %w", err)
	}

	// Convert triples to quads (default graph)
	quads := make([]*rdf.Quad, len(triples))
	for i, triple := range triples {
		quads[i] = rdf.NewQuad(
			triple.Subject,
			triple.Predicate,
			triple.Object,
			rdf.NewDefaultGraph(),
		)
	}

	return quads, nil
}

// NQuadsParser parses N-Quads format (quads with optional graph)
type NQuadsParser struct{}

func (p *NQuadsParser) ContentType() string {
	return "application/n-quads"
}

func (p *NQuadsParser) Parse(reader io.Reader) ([]*rdf.Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use N-Quads parser
	nquadsParser := nquads.NewParser(string(data))
	quads, err := nquadsParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing N-Quads: %w", err)
	}

	return quads, nil
}

// TurtleParser parses Turtle format (triples with prefixes, default graph)
type TurtleParser struct{}

func (p *TurtleParser) ContentType() string {
	return "text/turtle"
}

func (p *TurtleParser) Parse(reader io.Reader) ([]*rdf.Quad, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	// Use turtle parser
	turtleParser := turtle.NewParser(string(data))
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("error parsing Turtle: %w", err)
	}

	// Convert triples to quads (default graph)
	quads := make([]*rdf.Quad, len(triples))
	for i, triple := range triples {
		quads[i] = rdf.NewQuad(
			triple.Subject,
			triple.Predicate,
			triple.Object,
			rdf.NewDefaultGraph(),
		)
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
		"text/plain", // Alias for N-Triples
	}
}
