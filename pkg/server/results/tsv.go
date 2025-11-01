package results

import (
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
				builder.WriteString(termToTSVValue(term))
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

// termToTSVValue converts an RDF term to a TSV value string
// According to SPARQL TSV spec:
// - IRIs are enclosed in angle brackets: <iri>
// - Simple literals are enclosed in double quotes: "value"
// - Numeric literals (integer, decimal, double) without quotes: 4, 5.5
// - Language-tagged literals: "value"@language
// - Typed literals: "value"^^<datatype> (except for standard numeric types)
// - Blank nodes: _:label
// - Special characters in literals must be escaped
func termToTSVValue(term rdf.Term) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return "<" + t.IRI + ">"

	case *rdf.BlankNode:
		return "_:" + t.ID

	case *rdf.Literal:
		if t.Language != "" {
			escaped := escapeTSVString(t.Value)
			return "\"" + escaped + "\"@" + t.Language
		} else if t.Datatype != nil {
			// For numeric types (integer, decimal, double), output without quotes or datatype
			// according to SPARQL 1.1 TSV spec examples
			datatypeIRI := t.Datatype.IRI
			if datatypeIRI == "http://www.w3.org/2001/XMLSchema#integer" ||
				datatypeIRI == "http://www.w3.org/2001/XMLSchema#decimal" ||
				datatypeIRI == "http://www.w3.org/2001/XMLSchema#double" {
				// Output numeric value without quotes or datatype
				return t.Value
			}
			// For other typed literals, include the datatype
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
