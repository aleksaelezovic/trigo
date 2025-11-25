package dataset

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aleksaelezovic/trigo/internal/encoding"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

// Loader loads RDF datasets from FROM and FROM NAMED clauses
type Loader struct {
	defaultBaseDir string
	httpClient     *http.Client
}

// NewLoader creates a new dataset loader
func NewLoader(defaultBaseDir string) *Loader {
	return &Loader{
		defaultBaseDir: defaultBaseDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoadedDataset represents a loaded RDF dataset
type LoadedDataset struct {
	Store        *store.TripleStore
	DefaultGraph []string // FROM IRIs
	NamedGraphs  []string // FROM NAMED IRIs
}

// LoadOptions provides options for loading a dataset
type LoadOptions struct {
	BaseDir string // Optional: override default base directory for this load
}

// Load creates a temporary store with dataset from FROM and FROM NAMED clauses
func (l *Loader) Load(from []string, fromNamed []string, opts *LoadOptions) (*LoadedDataset, error) {
	// Determine base directory
	baseDir := l.defaultBaseDir
	if opts != nil && opts.BaseDir != "" {
		baseDir = opts.BaseDir
	}

	// Create temporary store for the dataset using a temporary directory
	tmpDir, err := os.MkdirTemp("", "trigo-dataset-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Note: The temporary store will be garbage collected when no longer referenced,
	// but the temp directory will remain. In production, consider cleanup strategies.
	tmpStorage, err := storage.NewBadgerStorage(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir) // Clean up on error (best effort)
		return nil, fmt.Errorf("failed to create temp storage: %w", err)
	}

	tripleStore := store.NewTripleStore(tmpStorage, encoding.NewTermEncoder(), encoding.NewTermDecoder())

	// Load FROM files into default graph
	for _, iri := range from {
		quads, err := l.loadRDFFile(iri, baseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load FROM <%s>: %w", iri, err)
		}

		// Insert into default graph
		for _, quad := range quads {
			// Override graph to default graph
			defaultQuad := rdf.NewQuad(
				quad.Subject,
				quad.Predicate,
				quad.Object,
				rdf.NewDefaultGraph(),
			)
			if err := tripleStore.InsertQuad(defaultQuad); err != nil {
				return nil, fmt.Errorf("failed to insert quad from <%s>: %w", iri, err)
			}
		}
	}

	// Load FROM NAMED files into named graphs
	for _, iri := range fromNamed {
		quads, err := l.loadRDFFile(iri, baseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load FROM NAMED <%s>: %w", iri, err)
		}

		// Resolve the IRI to get the absolute graph name
		// This ensures relative IRIs like "data-g2.ttl" become absolute URIs
		resolvedPath, isHTTP, err := l.resolveIRI(iri, baseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve graph IRI <%s>: %w", iri, err)
		}

		var graphName string
		if isHTTP {
			graphName = resolvedPath
		} else {
			// Convert file path to W3C canonical URI
			graphName = filePathToBaseURI(resolvedPath)
		}

		graph := rdf.NewNamedNode(graphName)

		// Insert into named graph
		for _, quad := range quads {
			// Override graph to specified named graph
			namedQuad := rdf.NewQuad(
				quad.Subject,
				quad.Predicate,
				quad.Object,
				graph,
			)
			if err := tripleStore.InsertQuad(namedQuad); err != nil {
				return nil, fmt.Errorf("failed to insert quad into graph <%s>: %w", graphName, err)
			}
		}
	}

	return &LoadedDataset{
		Store:        tripleStore,
		DefaultGraph: from,
		NamedGraphs:  fromNamed,
	}, nil
}

// loadRDFFile loads a single RDF file and returns quads
func (l *Loader) loadRDFFile(iri string, baseDir string) ([]*rdf.Quad, error) {
	resolvedPath, isHTTP, err := l.resolveIRI(iri, baseDir)
	if err != nil {
		return nil, err
	}

	var data []byte
	var formatExt string
	var baseURI string

	if isHTTP {
		// Load from HTTP/HTTPS
		resp, err := l.httpClient.Get(resolvedPath) // #nosec G107 - URL is from SPARQL query, not user input
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed for %s: %w", resolvedPath, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, resolvedPath)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read HTTP response: %w", err)
		}

		// Detect format from URL path or Content-Type header
		formatExt = detectFormatFromURL(resolvedPath)
		if formatExt == "" {
			formatExt = detectFormatFromContentType(resp.Header.Get("Content-Type"))
		}

		// Use URL as base URI
		baseURI = resolvedPath
	} else {
		// Load from local file
		data, err = os.ReadFile(resolvedPath) // #nosec G304 - file path resolved from SPARQL query dataset clause
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", resolvedPath, err)
		}

		formatExt = filepath.Ext(resolvedPath)

		// Convert file path to base URI (W3C test suite convention)
		baseURI = filePathToBaseURI(resolvedPath)
	}

	// Parse RDF data
	return l.parseRDFData(data, formatExt, baseURI)
}

// resolveIRI resolves an IRI to an absolute path or URL
func (l *Loader) resolveIRI(iri string, baseDir string) (path string, isHTTP bool, err error) {
	iri = strings.Trim(iri, "<>")

	// HTTP/HTTPS - use as-is
	if strings.HasPrefix(iri, "http://") || strings.HasPrefix(iri, "https://") {
		return iri, true, nil
	}

	// file:// URI - strip prefix
	if strings.HasPrefix(iri, "file://") {
		return strings.TrimPrefix(iri, "file://"), false, nil
	}

	// Relative path - resolve against baseDir
	if !filepath.IsAbs(iri) {
		return filepath.Join(baseDir, iri), false, nil
	}

	return iri, false, nil
}

// parseRDFData parses RDF data based on format
func (l *Loader) parseRDFData(data []byte, formatExt string, baseURI string) ([]*rdf.Quad, error) {
	dataStr := string(data)

	switch strings.ToLower(formatExt) {
	case ".nt", "ntriples", "n-triples":
		// N-Triples → convert triples to quads
		parser := rdf.NewNTriplesParser(dataStr)
		triples, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("N-Triples parse error: %w", err)
		}
		return triplesToQuads(triples), nil

	case ".nq", "nquads", "n-quads":
		// N-Quads
		parser := rdf.NewNQuadsParser(dataStr)
		quads, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("N-Quads parse error: %w", err)
		}
		return quads, nil

	case ".ttl", "turtle", "text/turtle":
		// Turtle → convert triples to quads
		parser := rdf.NewTurtleParser(dataStr)
		if baseURI != "" {
			parser.SetBaseURI(baseURI)
		}
		triples, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("turtle parse error: %w", err)
		}
		return triplesToQuads(triples), nil

	case ".trig", "trig":
		// TriG
		parser := rdf.NewTriGParser(dataStr)
		if baseURI != "" {
			parser.SetBaseURI(baseURI)
		}
		quads, err := parser.Parse()
		if err != nil {
			return nil, fmt.Errorf("TriG parse error: %w", err)
		}
		return quads, nil

	case ".rdf", ".xml", "rdfxml", "rdf/xml", "application/rdf+xml":
		// RDF/XML
		parser := rdf.NewRDFXMLParser()
		if baseURI != "" {
			parser.SetBaseURI(baseURI)
		}
		reader := strings.NewReader(dataStr)
		quads, err := parser.Parse(reader)
		if err != nil {
			return nil, fmt.Errorf("RDF/XML parse error: %w", err)
		}
		return quads, nil

	case ".jsonld", "jsonld", "json-ld", "application/ld+json":
		// JSON-LD
		parser := rdf.NewJSONLDParser()
		reader := strings.NewReader(dataStr)
		quads, err := parser.Parse(reader)
		if err != nil {
			return nil, fmt.Errorf("JSON-LD parse error: %w", err)
		}
		return quads, nil

	default:
		return nil, fmt.Errorf("unsupported RDF format: %s", formatExt)
	}
}

// detectFormatFromURL extracts format from URL file extension
func detectFormatFromURL(url string) string {
	// Strip query parameters
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}
	// Strip fragment
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}
	return filepath.Ext(url)
}

// detectFormatFromContentType maps HTTP Content-Type to format extension
func detectFormatFromContentType(contentType string) string {
	// Strip charset and other parameters
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch strings.ToLower(contentType) {
	case "application/n-triples", "text/plain":
		return ".nt"
	case "application/n-quads":
		return ".nq"
	case "text/turtle", "application/x-turtle":
		return ".ttl"
	case "application/trig", "application/x-trig":
		return ".trig"
	case "application/rdf+xml", "application/xml", "text/xml":
		return ".rdf"
	case "application/ld+json", "application/json":
		return ".jsonld"
	default:
		return ""
	}
}

// filePathToBaseURI converts a file path to a base URI
func filePathToBaseURI(filePath string) string {
	// W3C test files have canonical online location
	if strings.Contains(filePath, "rdf-tests/") {
		idx := strings.Index(filePath, "rdf-tests/")
		if idx != -1 {
			relativePath := filePath[idx+len("rdf-tests/"):]
			return "https://w3c.github.io/rdf-tests/" + relativePath
		}
	}

	// For other files, use file:// URI
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	return "file://" + absPath
}

// triplesToQuads converts triples to quads with default graph
func triplesToQuads(triples []*rdf.Triple) []*rdf.Quad {
	quads := make([]*rdf.Quad, len(triples))
	for i, triple := range triples {
		quads[i] = rdf.NewQuad(
			triple.Subject,
			triple.Predicate,
			triple.Object,
			rdf.NewDefaultGraph(),
		)
	}
	return quads
}
