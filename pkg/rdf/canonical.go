package rdf

import (
	"fmt"
	"strings"
)

// SerializeTriplesCanonical serializes triples to canonical N-Triples format (C14N)
// Implements RDF 1.2 canonicalization rules (escape sequences, whitespace)
// Note: Canonical form specifies representation, NOT ordering. Input order is preserved.
func SerializeTriplesCanonical(triples []*Triple) string {
	if len(triples) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, triple := range triples {
		builder.WriteString(serializeTermCanonical(triple.Subject))
		builder.WriteString(" ")
		builder.WriteString(serializeTermCanonical(triple.Predicate))
		builder.WriteString(" ")
		builder.WriteString(serializeTermCanonical(triple.Object))
		builder.WriteString(" .\n")
	}

	return builder.String()
}

// SerializeQuadsCanonical serializes quads to canonical N-Quads format (C14N)
// Implements RDF 1.2 canonicalization rules (escape sequences, whitespace)
// Note: Canonical form specifies representation, NOT ordering. Input order is preserved.
func SerializeQuadsCanonical(quads []*Quad) string {
	if len(quads) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, quad := range quads {
		builder.WriteString(serializeTermCanonical(quad.Subject))
		builder.WriteString(" ")
		builder.WriteString(serializeTermCanonical(quad.Predicate))
		builder.WriteString(" ")
		builder.WriteString(serializeTermCanonical(quad.Object))

		// Add graph if not default graph
		if quad.Graph != nil {
			if _, isDefault := quad.Graph.(*DefaultGraph); !isDefault {
				builder.WriteString(" ")
				builder.WriteString(serializeTermCanonical(quad.Graph))
			}
		}

		builder.WriteString(" .\n")
	}

	return builder.String()
}

// serializeTermCanonical serializes a single RDF term in canonical format
func serializeTermCanonical(term Term) string {
	switch t := term.(type) {
	case *NamedNode:
		return fmt.Sprintf("<%s>", escapeIRICanonical(t.IRI))
	case *BlankNode:
		return fmt.Sprintf("_:%s", t.ID)
	case *Literal:
		return serializeLiteralCanonical(t)
	case *TripleTerm:
		return serializeTripleTermCanonical(t)
	default:
		return ""
	}
}

// serializeLiteralCanonical serializes a literal in canonical format
func serializeLiteralCanonical(lit *Literal) string {
	escaped := escapeStringCanonical(lit.Value)

	// Language tag with optional directionality
	if lit.Language != "" {
		// Normalize language tag to lowercase
		langTag := strings.ToLower(lit.Language)

		// Handle directionality (e.g., @en--ltr)
		if lit.Direction != "" {
			// Normalize direction to lowercase
			direction := strings.ToLower(lit.Direction)
			return fmt.Sprintf(`"%s"@%s--%s`, escaped, langTag, direction)
		}
		return fmt.Sprintf(`"%s"@%s`, escaped, langTag)
	}

	// Datatype
	if lit.Datatype != nil {
		// Omit xsd:string datatype in canonical format (it's the default)
		if lit.Datatype.IRI != "http://www.w3.org/2001/XMLSchema#string" {
			return fmt.Sprintf(`"%s"^^<%s>`, escaped, lit.Datatype.IRI)
		}
	}

	// Plain literal (xsd:string is implicit)
	return fmt.Sprintf(`"%s"`, escaped)
}

// serializeTripleTermCanonical serializes a triple term (quoted triple) in canonical format
// Format: <<( <s> <p> <o> )>> with mandatory spaces
func serializeTripleTermCanonical(tt *TripleTerm) string {
	return fmt.Sprintf("<<( %s %s %s )>>",
		serializeTermCanonical(tt.Subject),
		serializeTermCanonical(tt.Predicate),
		serializeTermCanonical(tt.Object))
}

// escapeStringCanonical escapes a string value for canonical N-Triples/N-Quads output
// Implements RDF 1.2 escape rules:
// - Special named escapes: \t \b \n \r \f \" \\
// - Unicode: \uXXXX for U+0000 to U+FFFF, \UXXXXXXXX for higher
func escapeStringCanonical(s string) string {
	var builder strings.Builder
	builder.Grow(len(s))

	for _, r := range s {
		switch r {
		case '\t': // 0x09 TAB
			builder.WriteString(`\t`)
		case '\b': // 0x08 BACKSPACE
			builder.WriteString(`\b`)
		case '\n': // 0x0A LINE FEED
			builder.WriteString(`\n`)
		case '\r': // 0x0D CARRIAGE RETURN
			builder.WriteString(`\r`)
		case '\f': // 0x0C FORM FEED
			builder.WriteString(`\f`)
		case '"': // 0x22 QUOTATION MARK
			builder.WriteString(`\"`)
		case '\\': // 0x5C REVERSE SOLIDUS
			builder.WriteString(`\\`)
		default:
			// In canonical N-Triples, control characters and noncharacters must be escaped
			if r < 0x20 {
				// Control characters (0x00-0x1F): Use \uXXXX
				builder.WriteString(fmt.Sprintf(`\u%04X`, r))
			} else if r == 0x7F {
				// DEL character: Use \uXXXX
				builder.WriteString(fmt.Sprintf(`\u%04X`, r))
			} else if r >= 0xFFFE && r <= 0xFFFF {
				// Noncharacters U+FFFE and U+FFFF: Use \uXXXX
				builder.WriteString(fmt.Sprintf(`\u%04X`, r))
			} else {
				// All other characters (printable ASCII and valid UTF-8)
				builder.WriteRune(r)
			}
		}
	}

	return builder.String()
}

// escapeIRICanonical escapes an IRI for canonical output
// IRIs should already be valid, so this mainly ensures consistency
func escapeIRICanonical(iri string) string {
	// For canonical output, IRIs are expected to be already properly escaped
	// Just return as-is since the parser validates IRI syntax
	return iri
}
