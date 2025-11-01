package results

import (
	"fmt"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

// N-Triples Results Format
// https://www.w3.org/TR/n-triples/

// FormatConstructResultNTriples converts a CONSTRUCT result to N-Triples format
// https://www.w3.org/TR/n-triples/
func FormatConstructResultNTriples(result *executor.ConstructResult) ([]byte, error) {
	var builder strings.Builder

	for _, triple := range result.Triples {
		// Subject
		if err := formatNTriplesTerm(&builder, triple.Subject); err != nil {
			return nil, err
		}
		builder.WriteString(" ")

		// Predicate
		if err := formatNTriplesTerm(&builder, triple.Predicate); err != nil {
			return nil, err
		}
		builder.WriteString(" ")

		// Object
		if err := formatNTriplesTerm(&builder, triple.Object); err != nil {
			return nil, err
		}
		builder.WriteString(" .\n")
	}

	return []byte(builder.String()), nil
}

// formatNTriplesTerm formats a term in N-Triples format
func formatNTriplesTerm(builder *strings.Builder, term executor.Term) error {
	switch term.Type {
	case "iri":
		builder.WriteString("<")
		builder.WriteString(term.Value)
		builder.WriteString(">")
	case "blank":
		builder.WriteString("_:")
		builder.WriteString(term.Value)
	case "literal":
		// Parse literal value to check for language/datatype
		value := term.Value

		// Check for language tag (e.g., "hello"@en)
		if idx := strings.LastIndex(value, "@"); idx != -1 {
			literalValue := value[:idx]
			lang := value[idx+1:]
			builder.WriteString("\"")
			builder.WriteString(escapeNTriplesString(literalValue))
			builder.WriteString("\"@")
			builder.WriteString(lang)
		} else if idx := strings.Index(value, "^^<"); idx != -1 {
			// Check for datatype (e.g., "123"^^<http://www.w3.org/2001/XMLSchema#integer>)
			literalValue := value[:idx]
			datatype := value[idx+2:] // Skip "^^"
			builder.WriteString("\"")
			builder.WriteString(escapeNTriplesString(literalValue))
			builder.WriteString("\"^^")
			builder.WriteString(datatype) // datatype already includes <>
		} else {
			// Simple string literal
			builder.WriteString("\"")
			builder.WriteString(escapeNTriplesString(value))
			builder.WriteString("\"")
		}
	default:
		return fmt.Errorf("unknown term type: %s", term.Type)
	}
	return nil
}

// escapeNTriplesString escapes special characters in N-Triples string literals
func escapeNTriplesString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
