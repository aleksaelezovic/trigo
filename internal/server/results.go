package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// SPARQL JSON Results Format
// https://www.w3.org/TR/sparql11-results-json/

// SPARQLResultsJSON represents the JSON format for SPARQL query results
type SPARQLResultsJSON struct {
	Head    ResultHead     `json:"head"`
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

// SPARQL XML Results Format
// https://www.w3.org/TR/rdf-sparql-XMLres/

// FormatSelectResultsXML converts a SELECT result to SPARQL XML format
func FormatSelectResultsXML(result *executor.SelectResult) ([]byte, error) {
	// Extract variable names
	var varNames []string
	if result.Variables == nil {
		varSet := make(map[string]bool)
		for _, binding := range result.Bindings {
			for varName := range binding.Vars {
				if !varSet[varName] {
					varSet[varName] = true
					varNames = append(varNames, varName)
				}
			}
		}
	} else {
		for _, v := range result.Variables {
			varNames = append(varNames, v.Name)
		}
	}

	xml := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head>
`

	for _, varName := range varNames {
		xml += "    <variable name=\"" + varName + "\"/>\n"
	}

	xml += `  </head>
  <results>
`

	for _, binding := range result.Bindings {
		xml += "    <result>\n"
		for varName, term := range binding.Vars {
			xml += "      <binding name=\"" + varName + "\">\n"
			xml += termToXML(term, "        ")
			xml += "      </binding>\n"
		}
		xml += "    </result>\n"
	}

	xml += `  </results>
</sparql>
`

	return []byte(xml), nil
}

// FormatAskResultXML converts an ASK result to SPARQL XML format
func FormatAskResultXML(result *executor.AskResult) ([]byte, error) {
	boolStr := "false"
	if result.Result {
		boolStr = "true"
	}

	xml := `<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head/>
  <boolean>` + boolStr + `</boolean>
</sparql>
`

	return []byte(xml), nil
}

func termToXML(term rdf.Term, indent string) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return indent + "<uri>" + xmlEscape(t.IRI) + "</uri>\n"

	case *rdf.BlankNode:
		return indent + "<bnode>" + xmlEscape(t.ID) + "</bnode>\n"

	case *rdf.Literal:
		if t.Language != "" {
			return indent + "<literal xml:lang=\"" + t.Language + "\">" + xmlEscape(t.Value) + "</literal>\n"
		} else if t.Datatype != nil {
			return indent + "<literal datatype=\"" + xmlEscape(t.Datatype.IRI) + "\">" + xmlEscape(t.Value) + "</literal>\n"
		}
		return indent + "<literal>" + xmlEscape(t.Value) + "</literal>\n"

	default:
		return indent + "<literal>" + xmlEscape(term.String()) + "</literal>\n"
	}
}

func xmlEscape(s string) string {
	// Simple XML escaping
	s = replaceAll(s, "&", "&amp;")
	s = replaceAll(s, "<", "&lt;")
	s = replaceAll(s, ">", "&gt;")
	s = replaceAll(s, "\"", "&quot;")
	s = replaceAll(s, "'", "&apos;")
	return s
}

func replaceAll(s, old, new string) string {
	result := ""
	for _, ch := range s {
		if string(ch) == old {
			result += new
		} else {
			result += string(ch)
		}
	}
	return result
}

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
