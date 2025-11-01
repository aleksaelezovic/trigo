package results

import (
	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

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
