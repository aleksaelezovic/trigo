package testsuite

import (
	"fmt"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// ResultSet represents a SPARQL result set parsed from Turtle format
type ResultSet struct {
	Variables []string
	Solutions []map[string]rdf.Term
}

// ParseTurtleResults parses SPARQL results in Turtle format (rs:ResultSet vocabulary)
// Format:
//
//	[] rdf:type rs:ResultSet ;
//	   rs:resultVariable "var1" ;
//	   rs:solution [ rs:binding [ rs:variable "var1" ; rs:value <value> ] ] .
func ParseTurtleResults(data string) ([]map[string]rdf.Term, error) {
	return ParseTurtleResultsWithBase(data, "")
}

// ParseTurtleResultsWithBase parses SPARQL results in Turtle format with a base URI
func ParseTurtleResultsWithBase(data string, baseURI string) ([]map[string]rdf.Term, error) {
	// Parse the Turtle data
	parser := rdf.NewTurtleParser(data)
	if baseURI != "" {
		parser.SetBaseURI(baseURI)
	}
	triples, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Turtle: %w", err)
	}

	return parseResultSetFromTriples(triples)
}

// parseResultSetFromTriples converts a set of triples using rs:ResultSet vocabulary to a ResultSet
func parseResultSetFromTriples(triples []*rdf.Triple) ([]map[string]rdf.Term, error) {
	// Build a simple triple index by subject
	triplesBySubject := make(map[rdf.Term][]*rdf.Triple)
	for _, triple := range triples {
		triplesBySubject[triple.Subject] = append(triplesBySubject[triple.Subject], triple)
	}

	rsNS := "http://www.w3.org/2001/sw/DataAccess/tests/result-set#"
	rdfNS := "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

	// Find the ResultSet node (has rdf:type rs:ResultSet)
	var resultSetNode rdf.Term
	for _, triple := range triples {
		if pred, ok := triple.Predicate.(*rdf.NamedNode); ok {
			if pred.IRI == rdfNS+"type" {
				if obj, ok := triple.Object.(*rdf.NamedNode); ok {
					if obj.IRI == rsNS+"ResultSet" {
						resultSetNode = triple.Subject
						break
					}
				}
			}
		}
	}

	if resultSetNode == nil {
		return nil, fmt.Errorf("no rs:ResultSet found in data")
	}

	var solutions []map[string]rdf.Term

	// Extract variables and solutions from the ResultSet node
	for _, triple := range triplesBySubject[resultSetNode] {
		pred, ok := triple.Predicate.(*rdf.NamedNode)
		if !ok {
			continue
		}

		switch pred.IRI {
		case rsNS + "resultVariable":
			// Skip variables - we don't need them for comparison
			continue

		case rsNS + "solution":
			// Parse solution (blank node with rs:binding children)
			solution := make(map[string]rdf.Term)
			solutionNode := triple.Object

			// Find all bindings in this solution
			for _, bindingTriple := range triplesBySubject[solutionNode] {
				if bindingPred, ok := bindingTriple.Predicate.(*rdf.NamedNode); ok {
					if bindingPred.IRI == rsNS+"binding" {
						// Parse binding (blank node with rs:variable and rs:value)
						bindingNode := bindingTriple.Object
						var varName string
						var value rdf.Term

						for _, bt := range triplesBySubject[bindingNode] {
							if bp, ok := bt.Predicate.(*rdf.NamedNode); ok {
								switch bp.IRI {
								case rsNS + "variable":
									if lit, ok := bt.Object.(*rdf.Literal); ok {
										varName = lit.Value
									}
								case rsNS + "value":
									value = bt.Object
								}
							}
						}

						if varName != "" && value != nil {
							solution[varName] = value
						}
					}
				}
			}

			if len(solution) > 0 {
				solutions = append(solutions, solution)
			}
		}
	}

	return solutions, nil
}

// ToBindings converts the result set to the internal binding format
func (rs *ResultSet) ToBindings() ([]map[string]rdf.Term, error) {
	return rs.Solutions, nil
}
