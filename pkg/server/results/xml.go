package results

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

// Results represents SPARQL XML query results
type Results struct {
	Head    Head           `xml:"head"`
	Results ResultsElement `xml:"results"`
	Boolean *bool          `xml:"boolean"` // For ASK queries
}

// Head represents the head element with variable names
type Head struct {
	Variables []Variable `xml:"variable"`
}

// Variable represents a variable declaration
type Variable struct {
	Name string `xml:"name,attr"`
}

// ResultsElement contains the result bindings
type ResultsElement struct {
	Results []Result `xml:"result"`
}

// Result represents a single result binding
type Result struct {
	Bindings []Binding `xml:"binding"`
}

// Binding represents a variable binding in a result
type Binding struct {
	Name    string   `xml:"name,attr"`
	URI     *string  `xml:"uri"`
	Literal *Literal `xml:"literal"`
	BNode   *string  `xml:"bnode"`
}

// Literal represents a literal value
type Literal struct {
	Value    string `xml:",chardata"`
	Lang     string `xml:"lang,attr,omitempty"`
	Datatype string `xml:"datatype,attr,omitempty"`
}

// ParseXMLResults parses SPARQL XML results
func ParseXMLResults(r io.Reader) (*Results, error) {
	var results Results
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to parse XML results: %w", err)
	}
	return &results, nil
}

// ToBindings converts XML results to a list of bindings (maps of variable name to RDF term)
func (r *Results) ToBindings() ([]map[string]rdf.Term, error) {
	if r.Boolean != nil {
		// ASK query result
		return nil, fmt.Errorf("ASK queries not supported for binding comparison")
	}

	var bindings []map[string]rdf.Term

	for _, result := range r.Results.Results {
		binding := make(map[string]rdf.Term)

		for _, b := range result.Bindings {
			var term rdf.Term
			var err error

			if b.URI != nil {
				term = rdf.NewNamedNode(*b.URI)
			} else if b.BNode != nil {
				term = rdf.NewBlankNode(*b.BNode)
			} else if b.Literal != nil {
				if b.Literal.Lang != "" {
					term = rdf.NewLiteralWithLanguage(b.Literal.Value, b.Literal.Lang)
				} else if b.Literal.Datatype != "" {
					term = rdf.NewLiteralWithDatatype(b.Literal.Value, rdf.NewNamedNode(b.Literal.Datatype))
				} else {
					term = rdf.NewLiteral(b.Literal.Value)
				}
			} else {
				err = fmt.Errorf("binding has no value")
			}

			if err != nil {
				return nil, fmt.Errorf("failed to convert binding %s: %w", b.Name, err)
			}

			binding[b.Name] = term
		}

		bindings = append(bindings, binding)
	}

	return bindings, nil
}

// CompareResults compares two sets of bindings, ignoring order and handling blank node isomorphism
func CompareResults(expected, actual []map[string]rdf.Term) bool {
	if len(expected) != len(actual) {
		return false
	}

	// Extract all blank nodes from both result sets
	expectedBlanks := extractBlankNodesFromBindings(expected)
	actualBlanks := extractBlankNodesFromBindings(actual)

	if len(expectedBlanks) != len(actualBlanks) {
		return false
	}

	// If no blank nodes, use simple string comparison
	if len(expectedBlanks) == 0 {
		return compareResultsSimple(expected, actual)
	}

	// Try to find a valid blank node mapping that makes the result sets equal
	mapping := make(map[string]string)
	usedTargets := make(map[string]bool)
	return backtrackResultBindings(expected, actual, expectedBlanks, actualBlanks, mapping, usedTargets, 0)
}

// extractBlankNodesFromBindings extracts all unique blank node IDs from bindings
func extractBlankNodesFromBindings(bindings []map[string]rdf.Term) []string {
	blanks := make(map[string]bool)
	for _, binding := range bindings {
		for _, term := range binding {
			if bn, ok := term.(*rdf.BlankNode); ok {
				blanks[bn.ID] = true
			}
		}
	}

	result := make([]string, 0, len(blanks))
	for id := range blanks {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// backtrackResultBindings tries to find a valid blank node mapping via backtracking
func backtrackResultBindings(expected, actual []map[string]rdf.Term, expectedBlanks, actualBlanks []string,
	mapping map[string]string, usedTargets map[string]bool, index int) bool {

	if index == len(expectedBlanks) {
		return verifyResultMapping(expected, actual, mapping)
	}

	currentBlank := expectedBlanks[index]

	for _, candidateBlank := range actualBlanks {
		if usedTargets[candidateBlank] {
			continue
		}

		mapping[currentBlank] = candidateBlank
		usedTargets[candidateBlank] = true

		if backtrackResultBindings(expected, actual, expectedBlanks, actualBlanks, mapping, usedTargets, index+1) {
			return true
		}

		delete(mapping, currentBlank)
		delete(usedTargets, candidateBlank)
	}

	return false
}

// verifyResultMapping checks if the given blank node mapping makes the result sets equal
func verifyResultMapping(expected, actual []map[string]rdf.Term, mapping map[string]string) bool {
	// Apply mapping to expected bindings and convert to string set
	expectedStrs := make(map[string]bool)
	for _, binding := range expected {
		str := bindingToStringWithMapping(binding, mapping)
		expectedStrs[str] = true
	}

	// Convert actual bindings to string set
	actualStrs := make(map[string]bool)
	for _, binding := range actual {
		str := bindingToStringWithMapping(binding, nil)
		actualStrs[str] = true
	}

	// Check if sets are equal
	if len(expectedStrs) != len(actualStrs) {
		return false
	}

	for str := range expectedStrs {
		if !actualStrs[str] {
			return false
		}
	}

	return true
}

// compareResultsSimple compares result sets without blank node isomorphism
func compareResultsSimple(expected, actual []map[string]rdf.Term) bool {
	sortBindings := func(bindings []map[string]rdf.Term) []string {
		var strs []string
		for _, binding := range bindings {
			strs = append(strs, bindingToString(binding))
		}
		sort.Strings(strs)
		return strs
	}

	expectedStrs := sortBindings(expected)
	actualStrs := sortBindings(actual)

	for i := range expectedStrs {
		if expectedStrs[i] != actualStrs[i] {
			return false
		}
	}

	return true
}

// bindingToStringWithMapping converts a binding to string, applying blank node mapping
func bindingToStringWithMapping(binding map[string]rdf.Term, mapping map[string]string) string {
	var vars []string
	for v := range binding {
		vars = append(vars, v)
	}
	sort.Strings(vars)

	var str string
	for i, v := range vars {
		if i > 0 {
			str += "|"
		}
		term := binding[v]
		termStr := term.String()

		// Apply blank node mapping if provided
		if mapping != nil {
			if bn, ok := term.(*rdf.BlankNode); ok {
				if mapped, exists := mapping[bn.ID]; exists {
					termStr = "_:" + mapped
				}
			}
		}

		str += v + "=" + termStr
	}
	return str
}

// bindingToString converts a binding to a canonical string representation
func bindingToString(binding map[string]rdf.Term) string {
	// Get sorted variable names
	var vars []string
	for v := range binding {
		vars = append(vars, v)
	}
	sort.Strings(vars)

	// Build string
	var str string
	for i, v := range vars {
		if i > 0 {
			str += "|"
		}
		str += v + "=" + binding[v].String()
	}
	return str
}

// SPARQL XML Results Format (Serialization)
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
