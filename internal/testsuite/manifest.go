package testsuite

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TestManifest represents a SPARQL test manifest
type TestManifest struct {
	BaseURI string
	Tests   []TestCase
}

// TestCase represents a single SPARQL test
type TestCase struct {
	Name        string
	Type        TestType
	Action      string      // Query file
	Data        []string    // Data files
	GraphData   []GraphData // Named graph data
	Result      string      // Expected result file
	Approved    bool
	Description string
}

// GraphData represents a named graph in a test
type GraphData struct {
	Name string
	File string
}

// TestType represents the type of test
type TestType string

const (
	// SPARQL Syntax tests
	TestTypePositiveSyntax   TestType = "PositiveSyntaxTest"
	TestTypePositiveSyntax11 TestType = "PositiveSyntaxTest11"
	TestTypeNegativeSyntax   TestType = "NegativeSyntaxTest"
	TestTypeNegativeSyntax11 TestType = "NegativeSyntaxTest11"

	// SPARQL Evaluation tests
	TestTypeQueryEvaluation TestType = "QueryEvaluationTest"

	// SPARQL Result format tests
	TestTypeCSVResultFormat  TestType = "CSVResultFormatTest"
	TestTypeTSVResultFormat  TestType = "TSVResultFormatTest"
	TestTypeJSONResultFormat TestType = "JSONResultFormatTest"

	// SPARQL Update tests
	TestTypePositiveUpdateSyntax TestType = "PositiveUpdateSyntaxTest11"
	TestTypeNegativeUpdateSyntax TestType = "NegativeUpdateSyntaxTest11"
	TestTypeUpdateEvaluation     TestType = "UpdateEvaluationTest"

	// RDF Turtle tests
	TestTypeTurtleEval           TestType = "TestTurtleEval"
	TestTypeTurtlePositiveSyntax TestType = "TestTurtlePositiveSyntax"
	TestTypeTurtleNegativeSyntax TestType = "TestTurtleNegativeSyntax"
	TestTypeTurtleNegativeEval   TestType = "TestTurtleNegativeEval"

	// RDF N-Triples tests
	TestTypeNTriplesPositiveSyntax TestType = "TestNTriplesPositiveSyntax"
	TestTypeNTriplesNegativeSyntax TestType = "TestNTriplesNegativeSyntax"

	// RDF N-Quads tests
	TestTypeNQuadsPositiveSyntax TestType = "TestNQuadsPositiveSyntax"
	TestTypeNQuadsNegativeSyntax TestType = "TestNQuadsNegativeSyntax"

	// RDF TriG tests
	TestTypeTrigEval           TestType = "TestTrigEval"
	TestTypeTrigPositiveSyntax TestType = "TestTrigPositiveSyntax"
	TestTypeTrigNegativeSyntax TestType = "TestTrigNegativeSyntax"
	TestTypeTrigNegativeEval   TestType = "TestTrigNegativeEval"

	// RDF/XML tests
	TestTypeXMLEval           TestType = "TestXMLEval"
	TestTypeXMLNegativeSyntax TestType = "TestXMLNegativeSyntax"

	// JSON-LD tests (if needed in future)
	TestTypeJSONLDEval           TestType = "TestJSONLDEval"
	TestTypeJSONLDNegativeSyntax TestType = "TestJSONLDNegativeSyntax"
)

// ParseManifest parses a Turtle manifest file (simplified parser)
// This is a basic implementation - a full parser would use a proper Turtle library
func ParseManifest(path string) (*TestManifest, error) {
	file, err := os.Open(path) // #nosec G304 - test suite legitimately reads test manifest files
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest: %w", err)
	}
	defer file.Close()

	manifest := &TestManifest{
		BaseURI: filepath.Dir(path),
	}

	scanner := bufio.NewScanner(file)
	var currentTest *TestCase
	var inTest bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Start of new test (test definition can be <#testname> or :testname)
		// Handles both "rdf:type" and shorthand "a rdft:" or "a mf:"
		isTestStart := (strings.HasPrefix(line, "<#") || strings.HasPrefix(line, ":")) &&
			(strings.Contains(line, "rdf:type") || strings.Contains(line, " a "))

		if isTestStart {
			if currentTest != nil {
				manifest.Tests = append(manifest.Tests, *currentTest)
			}
			currentTest = &TestCase{}
			inTest = true
			// Parse the rdf:type on this line immediately
		}

		if !inTest || currentTest == nil {
			continue
		}

		// Extract test name
		if strings.Contains(line, "mf:name") {
			if parts := strings.Split(line, `"`); len(parts) >= 2 {
				currentTest.Name = parts[1]
			}
		}

		// Parse test type
		if strings.Contains(line, "rdf:type") || strings.Contains(line, " a mf:") || strings.Contains(line, "a rdft:") {
			// SPARQL tests
			if strings.Contains(line, "PositiveSyntaxTest11") {
				currentTest.Type = TestTypePositiveSyntax11
			} else if strings.Contains(line, "PositiveSyntaxTest") {
				currentTest.Type = TestTypePositiveSyntax
			} else if strings.Contains(line, "NegativeSyntaxTest11") {
				currentTest.Type = TestTypeNegativeSyntax11
			} else if strings.Contains(line, "NegativeSyntaxTest") {
				currentTest.Type = TestTypeNegativeSyntax
			} else if strings.Contains(line, "CSVResultFormatTest") {
				currentTest.Type = TestTypeCSVResultFormat
			} else if strings.Contains(line, "JSONResultFormatTest") {
				currentTest.Type = TestTypeJSONResultFormat
			} else if strings.Contains(line, "QueryEvaluationTest") {
				currentTest.Type = TestTypeQueryEvaluation
				// RDF Turtle tests
			} else if strings.Contains(line, "TestTurtleNegativeEval") {
				currentTest.Type = TestTypeTurtleNegativeEval
			} else if strings.Contains(line, "TestTurtleEval") {
				currentTest.Type = TestTypeTurtleEval
			} else if strings.Contains(line, "TestTurtlePositiveSyntax") {
				currentTest.Type = TestTypeTurtlePositiveSyntax
			} else if strings.Contains(line, "TestTurtleNegativeSyntax") {
				currentTest.Type = TestTypeTurtleNegativeSyntax
				// RDF N-Triples tests
			} else if strings.Contains(line, "TestNTriplesPositiveSyntax") {
				currentTest.Type = TestTypeNTriplesPositiveSyntax
			} else if strings.Contains(line, "TestNTriplesNegativeSyntax") {
				currentTest.Type = TestTypeNTriplesNegativeSyntax
				// RDF N-Quads tests
			} else if strings.Contains(line, "TestNQuadsPositiveSyntax") {
				currentTest.Type = TestTypeNQuadsPositiveSyntax
			} else if strings.Contains(line, "TestNQuadsNegativeSyntax") {
				currentTest.Type = TestTypeNQuadsNegativeSyntax
				// RDF TriG tests
			} else if strings.Contains(line, "TestTrigNegativeEval") {
				currentTest.Type = TestTypeTrigNegativeEval
			} else if strings.Contains(line, "TestTrigEval") {
				currentTest.Type = TestTypeTrigEval
			} else if strings.Contains(line, "TestTrigPositiveSyntax") {
				currentTest.Type = TestTypeTrigPositiveSyntax
			} else if strings.Contains(line, "TestTrigNegativeSyntax") {
				currentTest.Type = TestTypeTrigNegativeSyntax
				// RDF/XML tests
			} else if strings.Contains(line, "TestXMLEval") {
				currentTest.Type = TestTypeXMLEval
			} else if strings.Contains(line, "TestXMLNegativeSyntax") {
				currentTest.Type = TestTypeXMLNegativeSyntax
				// JSON-LD tests
			} else if strings.Contains(line, "TestJSONLDEval") {
				currentTest.Type = TestTypeJSONLDEval
			} else if strings.Contains(line, "TestJSONLDNegativeSyntax") {
				currentTest.Type = TestTypeJSONLDNegativeSyntax
			}
		}

		// Parse action (query file)
		if strings.Contains(line, "mf:action") || strings.Contains(line, "qt:query") {
			if parts := strings.Split(line, "<"); len(parts) >= 2 {
				if parts2 := strings.Split(parts[1], ">"); len(parts2) >= 1 {
					currentTest.Action = parts2[0]
				}
			}
		}

		// Parse data files
		if strings.Contains(line, "qt:data") {
			if parts := strings.Split(line, "<"); len(parts) >= 2 {
				if parts2 := strings.Split(parts[1], ">"); len(parts2) >= 1 {
					currentTest.Data = append(currentTest.Data, parts2[0])
				}
			}
		}

		// Parse result file
		if strings.Contains(line, "mf:result") {
			if parts := strings.Split(line, "<"); len(parts) >= 2 {
				if parts2 := strings.Split(parts[1], ">"); len(parts2) >= 1 {
					currentTest.Result = parts2[0]
				}
			}
		}

		// Parse approval status
		if strings.Contains(line, "mf:approval") && strings.Contains(line, "Approved") {
			currentTest.Approved = true
		}

		// Parse description/comment
		if strings.Contains(line, "rdfs:comment") {
			if parts := strings.Split(line, `"`); len(parts) >= 2 {
				currentTest.Description = parts[1]
			}
		}
	}

	// Add last test
	if currentTest != nil {
		manifest.Tests = append(manifest.Tests, *currentTest)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	// Post-process tests to detect TSV result format tests
	// TSV tests are marked as QueryEvaluationTest but have .tsv result files
	for i := range manifest.Tests {
		if manifest.Tests[i].Type == TestTypeQueryEvaluation &&
			strings.HasSuffix(manifest.Tests[i].Result, ".tsv") {
			manifest.Tests[i].Type = TestTypeTSVResultFormat
		}
	}

	return manifest, nil
}

// ResolveFile resolves a relative file path against the manifest base URI
func (m *TestManifest) ResolveFile(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(m.BaseURI, relPath)
}
