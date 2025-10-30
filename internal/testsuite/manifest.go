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
	// Syntax tests
	TestTypePositiveSyntax       TestType = "PositiveSyntaxTest"
	TestTypePositiveSyntax11     TestType = "PositiveSyntaxTest11"
	TestTypeNegativeSyntax       TestType = "NegativeSyntaxTest"
	TestTypeNegativeSyntax11     TestType = "NegativeSyntaxTest11"

	// Evaluation tests
	TestTypeQueryEvaluation      TestType = "QueryEvaluationTest"

	// Update tests
	TestTypePositiveUpdateSyntax TestType = "PositiveUpdateSyntaxTest11"
	TestTypeNegativeUpdateSyntax TestType = "NegativeUpdateSyntaxTest11"
	TestTypeUpdateEvaluation     TestType = "UpdateEvaluationTest"
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

		// Start of new test
		if strings.Contains(line, "mf:name") {
			if currentTest != nil {
				manifest.Tests = append(manifest.Tests, *currentTest)
			}
			currentTest = &TestCase{}
			inTest = true

			// Extract test name
			if parts := strings.Split(line, `"`); len(parts) >= 2 {
				currentTest.Name = parts[1]
			}
		}

		if !inTest || currentTest == nil {
			continue
		}

		// Parse test type
		if strings.Contains(line, "rdf:type") && strings.Contains(line, "mf:") {
			if strings.Contains(line, "PositiveSyntaxTest11") {
				currentTest.Type = TestTypePositiveSyntax11
			} else if strings.Contains(line, "PositiveSyntaxTest") {
				currentTest.Type = TestTypePositiveSyntax
			} else if strings.Contains(line, "NegativeSyntaxTest11") {
				currentTest.Type = TestTypeNegativeSyntax11
			} else if strings.Contains(line, "NegativeSyntaxTest") {
				currentTest.Type = TestTypeNegativeSyntax
			} else if strings.Contains(line, "QueryEvaluationTest") {
				currentTest.Type = TestTypeQueryEvaluation
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

	return manifest, nil
}

// ResolveFile resolves a relative file path against the manifest base URI
func (m *TestManifest) ResolveFile(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(m.BaseURI, relPath)
}
