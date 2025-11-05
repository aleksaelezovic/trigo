package results

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

// SPARQL CSV Results Format
// https://www.w3.org/TR/sparql11-results-csv-tsv/

// FormatSelectResultsCSV converts a SELECT result to SPARQL CSV format
func FormatSelectResultsCSV(result *executor.SelectResult) ([]byte, error) {
	var builder strings.Builder
	w := csv.NewWriter(&builder)

	// Create blank node canonicalization mapping
	bnodeMap := createBlankNodeMapping(result)

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

	// Write header row
	if err := w.Write(varNames); err != nil {
		return nil, err
	}

	// Write data rows
	for _, binding := range result.Bindings {
		row := make([]string, len(varNames))
		for i, varName := range varNames {
			if term, ok := binding.Vars[varName]; ok {
				row[i] = termToCSVValue(term, bnodeMap)
			}
			// If variable is not bound, leave empty string
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return []byte(builder.String()), nil
}

// FormatAskResultCSV converts an ASK result to SPARQL CSV format
func FormatAskResultCSV(result *executor.AskResult) ([]byte, error) {
	var builder strings.Builder
	w := csv.NewWriter(&builder)

	// Write header
	if err := w.Write([]string{"result"}); err != nil {
		return nil, err
	}

	// Write boolean value
	value := "false"
	if result.Result {
		value = "true"
	}
	if err := w.Write([]string{value}); err != nil {
		return nil, err
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return []byte(builder.String()), nil
}

// createBlankNodeMapping creates a canonical mapping for blank nodes
// Maps internal blank node IDs to canonical labels (a, b, c, ... or b0, b1, b2 after z)
func createBlankNodeMapping(result *executor.SelectResult) map[string]string {
	bnodeMap := make(map[string]string)
	counter := 0

	// Collect all blank nodes in order of first appearance
	for _, binding := range result.Bindings {
		for _, term := range binding.Vars {
			if bn, ok := term.(*rdf.BlankNode); ok {
				if _, exists := bnodeMap[bn.ID]; !exists {
					// Use single letters a-z, then fall back to b0, b1, b2...
					var label string
					if counter < 26 {
						label = string(rune('a' + counter))
					} else {
						label = fmt.Sprintf("b%d", counter-26)
					}
					bnodeMap[bn.ID] = label
					counter++
				}
			}
		}
	}

	return bnodeMap
}

// termToCSVValue converts an RDF term to a CSV value string
// According to SPARQL spec:
// - IRIs are written without angle brackets
// - Literals are written without quotes (the CSV writer handles escaping)
// - Language-tagged literals: value@language
// - Typed literals: value (without datatype IRI for simplicity, or can include)
// - Blank nodes: _:label (canonicalized)
func termToCSVValue(term rdf.Term, bnodeMap map[string]string) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return t.IRI

	case *rdf.BlankNode:
		if canonical, ok := bnodeMap[t.ID]; ok {
			return "_:" + canonical
		}
		return "_:" + t.ID

	case *rdf.Literal:
		if t.Language != "" {
			return t.Value + "@" + t.Language
		}
		// For typed literals, format numeric values properly
		if t.Datatype != nil {
			datatypeIRI := t.Datatype.IRI
			// Format doubles with uppercase E notation
			if datatypeIRI == "http://www.w3.org/2001/XMLSchema#double" {
				return formatDouble(t.Value)
			}
		}
		// For other typed literals, just return the value
		// The spec doesn't require the datatype IRI in CSV output
		return t.Value

	default:
		return term.String()
	}
}

// formatDouble formats a double value with uppercase E notation and decimal point
func formatDouble(value string) string {
	// Replace lowercase 'e' with uppercase 'E' and remove '+' sign
	value = strings.ReplaceAll(value, "e+", "E")
	value = strings.ReplaceAll(value, "e-", "E-")
	value = strings.ReplaceAll(value, "e", "E")

	// Ensure there's a decimal point before the E if there isn't one
	// and remove leading zeros from exponent
	// e.g., "1E06" should become "1.0E6"
	if strings.Contains(value, "E") {
		parts := strings.Split(value, "E")
		if len(parts) == 2 {
			mantissa := parts[0]
			exponent := parts[1]

			// Add decimal point if missing
			if !strings.Contains(mantissa, ".") {
				mantissa += ".0"
			}

			// Remove leading zeros/plus from exponent, but preserve sign
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

			value = mantissa + "E" + exponent
		}
	}

	return value
}
