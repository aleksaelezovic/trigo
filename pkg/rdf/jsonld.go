package rdf

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSONLDParser parses JSON-LD format
// Note: This is a simplified parser that handles common JSON-LD patterns.
// It supports:
// - Simple @id and @type
// - Properties with string values
// - Properties with @value objects
// - Properties with @id references
// - @language and @type in value objects
// - Basic @context expansion (prefix mapping)
//
// Not yet supported:
// - Full @context processing (remote contexts, nested contexts)
// - @graph
// - @list and @set
// - @reverse
// - Framing
// - Compaction/Expansion algorithms
type JSONLDParser struct{}

// NewJSONLDParser creates a new JSON-LD parser
func NewJSONLDParser() *JSONLDParser {
	return &JSONLDParser{}
}

// Parse parses JSON-LD and returns quads (all in default graph)
func (p *JSONLDParser) Parse(reader io.Reader) ([]*Quad, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading JSON-LD: %w", err)
	}

	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	var context map[string]interface{}
	var quads []*Quad
	blankNodeCounter := 0

	// Handle both single object and array of objects
	switch v := doc.(type) {
	case map[string]interface{}:
		// Extract @context if present
		if ctx, ok := v["@context"].(map[string]interface{}); ok {
			context = ctx
		}
		// Parse single object
		objQuads, err := p.parseObject(v, context, &blankNodeCounter)
		if err != nil {
			return nil, err
		}
		quads = append(quads, objQuads...)

	case []interface{}:
		// Parse array of objects
		for _, item := range v {
			if obj, ok := item.(map[string]interface{}); ok {
				// Extract @context from each object if present
				if ctx, ok := obj["@context"].(map[string]interface{}); ok {
					context = ctx
				}
				objQuads, err := p.parseObject(obj, context, &blankNodeCounter)
				if err != nil {
					return nil, err
				}
				quads = append(quads, objQuads...)
			}
		}

	default:
		return nil, fmt.Errorf("unexpected JSON-LD structure: %T", doc)
	}

	return quads, nil
}

// parseObjectWithSubject parses a JSON-LD object and returns quads and the subject
func (p *JSONLDParser) parseObjectWithSubject(obj map[string]interface{}, context map[string]interface{}, blankNodeCounter *int) ([]*Quad, Term, error) {
	var quads []*Quad

	// Get subject
	var subject Term
	if idVal, ok := obj["@id"]; ok {
		if idStr, ok := idVal.(string); ok {
			subject = NewNamedNode(p.expandIRI(idStr, context))
		}
	}
	if subject == nil {
		// Generate blank node
		*blankNodeCounter++
		subject = NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))
	}

	// Process properties
	for key, value := range obj {
		// Skip JSON-LD keywords
		if strings.HasPrefix(key, "@") {
			continue
		}

		// Expand property IRI
		predicate := NewNamedNode(p.expandIRI(key, context))

		// Parse value(s)
		switch v := value.(type) {
		case string:
			// Simple string literal
			object := NewLiteral(v)
			quad := NewQuad(subject, predicate, object, NewDefaultGraph())
			quads = append(quads, quad)

		case float64, bool:
			// Number or boolean literal
			object := NewLiteral(fmt.Sprintf("%v", v))
			quad := NewQuad(subject, predicate, object, NewDefaultGraph())
			quads = append(quads, quad)

		case map[string]interface{}:
			// Value object or nested object
			objQuads, err := p.parseValue(subject, predicate, v, context, blankNodeCounter)
			if err != nil {
				return nil, nil, err
			}
			quads = append(quads, objQuads...)

		case []interface{}:
			// Array of values
			for _, item := range v {
				switch itemVal := item.(type) {
				case string:
					object := NewLiteral(itemVal)
					quad := NewQuad(subject, predicate, object, NewDefaultGraph())
					quads = append(quads, quad)

				case map[string]interface{}:
					objQuads, err := p.parseValue(subject, predicate, itemVal, context, blankNodeCounter)
					if err != nil {
						return nil, nil, err
					}
					quads = append(quads, objQuads...)
				}
			}
		}
	}

	return quads, subject, nil
}

// parseObject parses a JSON-LD object and returns quads
func (p *JSONLDParser) parseObject(obj map[string]interface{}, context map[string]interface{}, blankNodeCounter *int) ([]*Quad, error) {
	quads, _, err := p.parseObjectWithSubject(obj, context, blankNodeCounter)
	return quads, err
}

// parseValue parses a value object (with @value, @id, @type, @language, etc.)
func (p *JSONLDParser) parseValue(subject, predicate Term, value map[string]interface{}, context map[string]interface{}, blankNodeCounter *int) ([]*Quad, error) {
	var quads []*Quad

	// Check for @id (reference to another resource)
	if idVal, ok := value["@id"]; ok {
		if idStr, ok := idVal.(string); ok {
			object := NewNamedNode(p.expandIRI(idStr, context))
			quad := NewQuad(subject, predicate, object, NewDefaultGraph())
			quads = append(quads, quad)
			return quads, nil
		}
	}

	// Check for @value (literal value)
	if val, ok := value["@value"]; ok {
		var object Term

		// Check for @language
		if lang, ok := value["@language"].(string); ok {
			object = &Literal{
				Value:    fmt.Sprintf("%v", val),
				Language: lang,
			}
		} else if typeVal, ok := value["@type"].(string); ok {
			// Check for @type (datatype)
			object = &Literal{
				Value:    fmt.Sprintf("%v", val),
				Datatype: NewNamedNode(p.expandIRI(typeVal, context)),
			}
		} else {
			// Plain literal
			object = NewLiteral(fmt.Sprintf("%v", val))
		}

		quad := NewQuad(subject, predicate, object, NewDefaultGraph())
		quads = append(quads, quad)
		return quads, nil
	}

	// Otherwise, treat as nested object (blank node)
	// Parse nested object first to get its subject (the blank node)
	nestedQuads, nestedSubject, err := p.parseObjectWithSubject(value, context, blankNodeCounter)
	if err != nil {
		return nil, err
	}

	// Create triple linking to the nested object
	quad := NewQuad(subject, predicate, nestedSubject, NewDefaultGraph())
	quads = append(quads, quad)
	quads = append(quads, nestedQuads...)

	return quads, nil
}

// expandIRI expands a compact IRI using the context
func (p *JSONLDParser) expandIRI(iri string, context map[string]interface{}) string {
	// Already a full IRI
	if strings.Contains(iri, "://") {
		return iri
	}

	// Check if it's a term defined in context first
	if context != nil {
		if expanded, ok := context[iri].(string); ok {
			// Recursively expand the result
			return p.expandIRI(expanded, context)
		}
	}

	// Check for prefix:localName pattern
	parts := strings.SplitN(iri, ":", 2)
	if len(parts) == 2 && context != nil {
		prefix := parts[0]
		localName := parts[1]

		// Look up prefix in context
		if ns, ok := context[prefix].(string); ok {
			return ns + localName
		}
	}

	// Return as-is (might be relative IRI)
	return iri
}
