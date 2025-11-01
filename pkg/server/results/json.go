package results

import (
	"encoding/json"
	"sort"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

// SPARQL JSON Results Format
// https://www.w3.org/TR/sparql11-results-json/

// SPARQLResultsJSON represents the JSON format for SPARQL query results
type SPARQLResultsJSON struct {
	Head    ResultHead      `json:"head"`
	Results *ResultBindings `json:"results,omitempty"`
	Boolean *bool           `json:"boolean,omitempty"`
}

// ResultHead contains the variable names
type ResultHead struct {
	Vars []string `json:"vars"`
}

// ResultBindings contains the result bindings
type ResultBindings struct {
	Bindings []map[string]BindingValue `json:"bindings"`
}

// BindingValue represents a single bound value
type BindingValue struct {
	Type     string  `json:"type"`
	Value    string  `json:"value"`
	Datatype *string `json:"datatype,omitempty"`
	XMLLang  *string `json:"xml:lang,omitempty"`
}

// FormatSelectResultsJSON converts a SELECT result to SPARQL JSON format
func FormatSelectResultsJSON(result *executor.SelectResult) ([]byte, error) {
	// Extract variable names
	var varNames []string
	if result.Variables == nil {
		// SELECT * - collect all variables from bindings
		varSet := make(map[string]bool)
		for _, binding := range result.Bindings {
			for varName := range binding.Vars {
				if !varSet[varName] {
					varSet[varName] = true
					varNames = append(varNames, varName)
				}
			}
		}
		// Sort variables alphabetically for consistent ordering
		sort.Strings(varNames)
	} else {
		// Specific variables
		for _, v := range result.Variables {
			varNames = append(varNames, v.Name)
		}
	}

	// Convert bindings
	jsonBindings := make([]map[string]BindingValue, 0, len(result.Bindings))
	for _, binding := range result.Bindings {
		jsonBinding := make(map[string]BindingValue)
		for varName, term := range binding.Vars {
			jsonBinding[varName] = termToBindingValue(term)
		}
		jsonBindings = append(jsonBindings, jsonBinding)
	}

	sparqlResult := SPARQLResultsJSON{
		Head: ResultHead{
			Vars: varNames,
		},
		Results: &ResultBindings{
			Bindings: jsonBindings,
		},
	}

	return json.MarshalIndent(sparqlResult, "", "  ")
}

// FormatAskResultJSON converts an ASK result to SPARQL JSON format
func FormatAskResultJSON(result *executor.AskResult) ([]byte, error) {
	sparqlResult := SPARQLResultsJSON{
		Head: ResultHead{
			Vars: []string{},
		},
		Boolean: &result.Result,
	}

	return json.MarshalIndent(sparqlResult, "", "  ")
}

// termToBindingValue converts an RDF term to a SPARQL JSON binding value
func termToBindingValue(term rdf.Term) BindingValue {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return BindingValue{
			Type:  "uri",
			Value: t.IRI,
		}

	case *rdf.BlankNode:
		return BindingValue{
			Type:  "bnode",
			Value: t.ID,
		}

	case *rdf.Literal:
		bv := BindingValue{
			Type:  "literal",
			Value: t.Value,
		}

		if t.Language != "" {
			bv.XMLLang = &t.Language
		} else if t.Datatype != nil {
			datatypeIRI := t.Datatype.IRI
			bv.Datatype = &datatypeIRI
		}

		return bv

	default:
		return BindingValue{
			Type:  "literal",
			Value: term.String(),
		}
	}
}
