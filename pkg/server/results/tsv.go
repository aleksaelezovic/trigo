package results

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

// SPARQL TSV Results Format
// https://www.w3.org/TR/sparql11-results-csv-tsv/

// FormatSelectResultsTSV converts a SELECT result to SPARQL TSV format
func FormatSelectResultsTSV(result *executor.SelectResult) ([]byte, error) {
	var builder strings.Builder

	// Create blank node canonicalization mapping
	bnodeMap := createBlankNodeMappingTSV(result)

	// Extract variable names
	var varNames []string
	if result.Variables == nil {
		// SELECT * without variables list (shouldn't happen with new executor)
		// Fallback: collect all variables from bindings (alphabetically sorted for consistency)
		varSet := make(map[string]bool)
		for _, binding := range result.Bindings {
			for varName := range binding.Vars {
				if !varSet[varName] {
					varSet[varName] = true
					varNames = append(varNames, varName)
				}
			}
		}
		sort.Strings(varNames)
	} else {
		// Use variables from query (preserves query order)
		for _, v := range result.Variables {
			varNames = append(varNames, v.Name)
		}
	}

	// Write header row with ? prefix
	for i, varName := range varNames {
		if i > 0 {
			builder.WriteString("\t")
		}
		builder.WriteString("?")
		builder.WriteString(varName)
	}
	builder.WriteString("\n")

	// Write data rows
	for _, binding := range result.Bindings {
		for i, varName := range varNames {
			if i > 0 {
				builder.WriteString("\t")
			}
			if term, ok := binding.Vars[varName]; ok {
				builder.WriteString(termToTSVValue(term, bnodeMap))
			}
			// If variable is not bound, leave empty
		}
		builder.WriteString("\n")
	}

	return []byte(builder.String()), nil
}

// FormatAskResultTSV converts an ASK result to SPARQL TSV format
func FormatAskResultTSV(result *executor.AskResult) ([]byte, error) {
	var builder strings.Builder

	// Write header
	builder.WriteString("?result\n")

	// Write boolean value
	if result.Result {
		builder.WriteString("true")
	} else {
		builder.WriteString("false")
	}
	builder.WriteString("\n")

	return []byte(builder.String()), nil
}

// createBlankNodeMappingTSV creates a canonical mapping for blank nodes
// Maps internal blank node IDs to canonical labels (b0, b1, b2, ...)
func createBlankNodeMappingTSV(result *executor.SelectResult) map[string]string {
	bnodeMap := make(map[string]string)
	counter := 0

	// Collect all blank nodes in order of first appearance
	for _, binding := range result.Bindings {
		for _, term := range binding.Vars {
			if bn, ok := term.(*rdf.BlankNode); ok {
				if _, exists := bnodeMap[bn.ID]; !exists {
					// Use b0, b1, b2... for TSV (as per W3C test expectations)
					bnodeMap[bn.ID] = fmt.Sprintf("b%d", counter)
					counter++
				}
			}
		}
	}

	return bnodeMap
}

// termToTSVValue converts an RDF term to a TSV value string
// According to SPARQL TSV spec:
// - IRIs are enclosed in angle brackets: <iri>
// - Simple literals are enclosed in double quotes: "value"
// - Numeric literals (integer, decimal, double) without quotes: 4, 5.5
// - Language-tagged literals: "value"@language
// - Typed literals: "value"^^<datatype> (except for standard numeric types)
// - Blank nodes: _:label (canonicalized)
// - Special characters in literals must be escaped
func termToTSVValue(term rdf.Term, bnodeMap map[string]string) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return "<" + t.IRI + ">"

	case *rdf.BlankNode:
		if canonical, ok := bnodeMap[t.ID]; ok {
			return "_:" + canonical
		}
		return "_:" + t.ID

	case *rdf.Literal:
		if t.Language != "" {
			escaped := escapeTSVString(t.Value)
			return "\"" + escaped + "\"@" + t.Language
		} else if t.Datatype != nil {
			datatypeIRI := t.Datatype.IRI

			// For the three basic numeric types (integer, decimal, double), output without quotes or datatype
			// according to SPARQL 1.1 TSV spec examples
			if datatypeIRI == "http://www.w3.org/2001/XMLSchema#integer" ||
				datatypeIRI == "http://www.w3.org/2001/XMLSchema#decimal" ||
				datatypeIRI == "http://www.w3.org/2001/XMLSchema#double" {
				// Format doubles with uppercase E notation
				if datatypeIRI == "http://www.w3.org/2001/XMLSchema#double" {
					return formatDoubleTSV(t.Value)
				}
				// Output numeric value without quotes or datatype
				return t.Value
			}

			// For other typed literals (including derived numeric types like negativeInteger),
			// include the datatype
			escaped := escapeTSVString(t.Value)
			return "\"" + escaped + "\"^^<" + datatypeIRI + ">"
		}
		// Plain literal
		escaped := escapeTSVString(t.Value)
		return "\"" + escaped + "\""

	default:
		return term.String()
	}
}

// formatDoubleTSV formats a double value with lowercase e notation and decimal point
func formatDoubleTSV(value string) string {
	// Normalize to lowercase 'e' and remove '+' sign
	value = strings.ReplaceAll(value, "E+", "e")
	value = strings.ReplaceAll(value, "E-", "e-")
	value = strings.ReplaceAll(value, "E", "e")
	value = strings.ReplaceAll(value, "e+", "e")

	// Ensure there's a decimal point before the e if there isn't one
	// and remove leading zeros from exponent
	// e.g., "1e06" should become "1.0e6"
	if strings.Contains(value, "e") {
		parts := strings.Split(value, "e")
		if len(parts) == 2 {
			mantissa := parts[0]
			exponent := parts[1]

			// Add decimal point if missing
			if !strings.Contains(mantissa, ".") {
				mantissa += ".0"
			}

			// Remove leading zeros from exponent, but preserve sign
			isNegative := strings.HasPrefix(exponent, "-")
			if isNegative {
				exponent = exponent[1:] // Remove minus sign temporarily
			}
			// Remove leading zeros
			exponent = strings.TrimLeft(exponent, "0")
			if exponent == "" {
				exponent = "0"
			}
			if isNegative {
				exponent = "-" + exponent
			}

			value = mantissa + "e" + exponent
		}
	}

	return value
}

// escapeTSVString escapes special characters in TSV strings
// According to the spec, tabs, newlines, carriage returns, quotes, and backslashes must be escaped
func escapeTSVString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
