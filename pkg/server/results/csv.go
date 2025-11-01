package results

import (
	"encoding/csv"
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
				row[i] = termToCSVValue(term)
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

// termToCSVValue converts an RDF term to a CSV value string
// According to SPARQL spec:
// - IRIs are written without angle brackets
// - Literals are written without quotes (the CSV writer handles escaping)
// - Language-tagged literals: value@language
// - Typed literals: value (without datatype IRI for simplicity, or can include)
// - Blank nodes: _:label
func termToCSVValue(term rdf.Term) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return t.IRI

	case *rdf.BlankNode:
		return "_:" + t.ID

	case *rdf.Literal:
		if t.Language != "" {
			return t.Value + "@" + t.Language
		}
		// For typed literals, just return the value
		// The spec doesn't require the datatype IRI in CSV output
		return t.Value

	default:
		return term.String()
	}
}
