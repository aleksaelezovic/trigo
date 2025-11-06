package testsuite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aleksaelezovic/trigo/internal/encoding"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/server/results"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
	"github.com/aleksaelezovic/trigo/pkg/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

// TestRunner runs W3C SPARQL test suite tests
type TestRunner struct {
	store *store.TripleStore
	stats *TestStats
}

// TestStats tracks test execution statistics
type TestStats struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Errors  []TestError
}

// TestError represents a test failure
type TestError struct {
	TestName string
	Type     TestType
	Error    string
}

// NewTestRunner creates a new test runner
func NewTestRunner(dbPath string) (*TestRunner, error) {
	storage, err := storage.NewBadgerStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &TestRunner{
		store: store.NewTripleStore(storage, encoding.NewTermEncoder(), encoding.NewTermDecoder()),
		stats: &TestStats{},
	}, nil
}

// Close closes the test runner
func (r *TestRunner) Close() error {
	return r.store.Close()
}

// RunManifest runs all tests in a manifest file
func (r *TestRunner) RunManifest(manifestPath string) error {
	manifest, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	fmt.Printf("\n📋 Running manifest: %s\n", manifestPath)
	fmt.Printf("   Found %d tests\n\n", len(manifest.Tests))

	for _, test := range manifest.Tests {
		r.stats.Total++

		result := r.runTest(manifest, &test)

		switch result {
		case TestResultPass:
			r.stats.Passed++
			fmt.Printf("  ✅ PASS: %s\n", test.Name)
		case TestResultFail:
			r.stats.Failed++
			fmt.Printf("  ❌ FAIL: %s\n", test.Name)
		case TestResultSkip:
			r.stats.Skipped++
			fmt.Printf("  ⏭️  SKIP: %s (type: %s)\n", test.Name, test.Type)
		case TestResultError:
			r.stats.Failed++
			fmt.Printf("  💥 ERROR: %s\n", test.Name)
		}
	}

	r.printSummary()
	return nil
}

// TestResult represents the result of running a test
type TestResult int

const (
	TestResultPass TestResult = iota
	TestResultFail
	TestResultSkip
	TestResultError
)

// runTest runs a single test case
func (r *TestRunner) runTest(manifest *TestManifest, test *TestCase) TestResult {
	switch test.Type {
	// SPARQL tests
	case TestTypePositiveSyntax, TestTypePositiveSyntax11:
		return r.runPositiveSyntaxTest(manifest, test)
	case TestTypeNegativeSyntax, TestTypeNegativeSyntax11:
		return r.runNegativeSyntaxTest(manifest, test)
	case TestTypeQueryEvaluation:
		return r.runQueryEvaluationTest(manifest, test)
	case TestTypeCSVResultFormat:
		return r.runCSVFormatTest(manifest, test)
	case TestTypeTSVResultFormat:
		return r.runTSVFormatTest(manifest, test)
	case TestTypeJSONResultFormat:
		return r.runJSONFormatTest(manifest, test)
	// RDF Turtle tests
	case TestTypeTurtleEval:
		return r.runRDFEvalTest(manifest, test, "turtle")
	case TestTypeTurtlePositiveSyntax:
		return r.runRDFPositiveSyntaxTest(manifest, test, "turtle")
	case TestTypeTurtleNegativeSyntax:
		return r.runRDFNegativeSyntaxTest(manifest, test, "turtle")
	// RDF N-Triples tests
	case TestTypeNTriplesPositiveSyntax:
		return r.runRDFPositiveSyntaxTest(manifest, test, "ntriples")
	case TestTypeNTriplesNegativeSyntax:
		return r.runRDFNegativeSyntaxTest(manifest, test, "ntriples")
	// RDF N-Quads tests
	case TestTypeNQuadsPositiveSyntax:
		return r.runRDFPositiveSyntaxTest(manifest, test, "nquads")
	case TestTypeNQuadsNegativeSyntax:
		return r.runRDFNegativeSyntaxTest(manifest, test, "nquads")
	// RDF TriG tests
	case TestTypeTrigEval:
		return r.runRDFEvalTest(manifest, test, "trig")
	case TestTypeTrigPositiveSyntax:
		return r.runRDFPositiveSyntaxTest(manifest, test, "trig")
	case TestTypeTrigNegativeSyntax:
		return r.runRDFNegativeSyntaxTest(manifest, test, "trig")
	// RDF/XML tests
	case TestTypeXMLEval:
		return r.runRDFEvalTest(manifest, test, "rdfxml")
	case TestTypeXMLNegativeSyntax:
		return r.runRDFNegativeSyntaxTest(manifest, test, "rdfxml")
	// JSON-LD tests
	case TestTypeJSONLDEval:
		return r.runRDFEvalTest(manifest, test, "jsonld")
	case TestTypeJSONLDNegativeSyntax:
		return r.runRDFNegativeSyntaxTest(manifest, test, "jsonld")
	default:
		// Skip unsupported test types for now
		return TestResultSkip
	}
}

// runPositiveSyntaxTest verifies a query parses successfully
func (r *TestRunner) runPositiveSyntaxTest(manifest *TestManifest, test *TestCase) TestResult {
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	queryFile := manifest.ResolveFile(test.Action)
	queryBytes, err := os.ReadFile(queryFile) // #nosec G304 - test suite legitimately reads test query files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read query file: %v", err))
		return TestResultError
	}

	// Try to parse the query
	p := parser.NewParser(string(queryBytes))
	_, err = p.Parse()

	if err != nil {
		r.recordError(test, fmt.Sprintf("Parser error: %v", err))
		return TestResultFail
	}

	return TestResultPass
}

// runNegativeSyntaxTest verifies a query fails to parse
func (r *TestRunner) runNegativeSyntaxTest(manifest *TestManifest, test *TestCase) TestResult {
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	queryFile := manifest.ResolveFile(test.Action)
	queryBytes, err := os.ReadFile(queryFile) // #nosec G304 - test suite legitimately reads test query files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read query file: %v", err))
		return TestResultError
	}

	// Try to parse the query - it should fail
	p := parser.NewParser(string(queryBytes))
	_, err = p.Parse()

	if err == nil {
		r.recordError(test, "Query parsed successfully but should have failed")
		return TestResultFail
	}

	// Expected to fail, so this is a pass
	return TestResultPass
}

// runQueryEvaluationTest runs a query and compares results
func (r *TestRunner) runQueryEvaluationTest(manifest *TestManifest, test *TestCase) TestResult {
	// Clear store before each test
	if err := r.clearStore(); err != nil {
		r.recordError(test, fmt.Sprintf("Failed to clear store: %v", err))
		return TestResultError
	}

	// Load data files
	if err := r.loadTestData(manifest, test); err != nil {
		r.recordError(test, fmt.Sprintf("Failed to load test data: %v", err))
		return TestResultError
	}

	// Read and parse query
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	queryFile := manifest.ResolveFile(test.Action)
	queryBytes, err := os.ReadFile(queryFile) // #nosec G304 - test suite legitimately reads test query files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read query file: %v", err))
		return TestResultError
	}

	// Parse query
	p := parser.NewParser(string(queryBytes))
	query, err := p.Parse()
	if err != nil {
		r.recordError(test, fmt.Sprintf("Parser error: %v", err))
		return TestResultFail
	}

	// Optimize query
	count, _ := r.store.Count()
	stats := &optimizer.Statistics{TotalTriples: count}
	opt := optimizer.NewOptimizer(stats)
	plan, err := opt.Optimize(query)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Optimizer error: %v", err))
		return TestResultFail
	}

	// Execute query
	exec := executor.NewExecutor(r.store)
	result, err := exec.Execute(plan)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Execution error: %v", err))
		return TestResultFail
	}

	// Handle different query types
	switch res := result.(type) {
	case *executor.SelectResult, *executor.AskResult:
		// Handle SELECT/ASK queries
		selectResult, ok := result.(*executor.SelectResult)
		if !ok {
			// ASK queries return boolean, not implemented yet for comparison
			r.recordError(test, "ASK query comparison not implemented yet")
			return TestResultSkip
		}

		actualBindings, err := r.resultsToBindings(selectResult)
		if err != nil {
			r.recordError(test, fmt.Sprintf("Failed to convert results: %v", err))
			return TestResultFail
		}

		// Load expected results
		if test.Result == "" {
			r.recordError(test, "No result file specified")
			return TestResultError
		}

		expectedBindings, err := r.loadExpectedResults(manifest, test)
		if err != nil {
			r.recordError(test, fmt.Sprintf("Failed to load expected results: %v", err))
			return TestResultFail
		}

		// Compare results
		if !results.CompareResults(expectedBindings, actualBindings) {
			r.recordError(test, fmt.Sprintf("Results mismatch: expected %d bindings, got %d bindings", len(expectedBindings), len(actualBindings)))
			return TestResultFail
		}

		return TestResultPass

	case *executor.ConstructResult:
		// Handle CONSTRUCT queries
		// Convert executor.Triple to rdf.Triple
		actualTriples := make([]*rdf.Triple, len(res.Triples))
		for i, t := range res.Triples {
			subj, err := r.executorTermToRDFTerm(t.Subject)
			if err != nil {
				r.recordError(test, fmt.Sprintf("Failed to convert subject: %v", err))
				return TestResultFail
			}
			pred, err := r.executorTermToRDFTerm(t.Predicate)
			if err != nil {
				r.recordError(test, fmt.Sprintf("Failed to convert predicate: %v", err))
				return TestResultFail
			}
			obj, err := r.executorTermToRDFTerm(t.Object)
			if err != nil {
				r.recordError(test, fmt.Sprintf("Failed to convert object: %v", err))
				return TestResultFail
			}
			actualTriples[i] = rdf.NewTriple(subj, pred, obj)
		}

		// Load expected N-Triples results
		if test.Result == "" {
			r.recordError(test, "No result file specified")
			return TestResultError
		}

		expectedTriples, err := r.loadExpectedTriples(manifest, test)
		if err != nil {
			r.recordError(test, fmt.Sprintf("Failed to load expected triples: %v", err))
			return TestResultFail
		}

		// Compare triples (order-independent)
		if !r.compareTriples(expectedTriples, actualTriples) {
			r.recordError(test, fmt.Sprintf("Triples mismatch: expected %d triples, got %d triples", len(expectedTriples), len(actualTriples)))
			return TestResultFail
		}

		return TestResultPass

	default:
		r.recordError(test, fmt.Sprintf("Unsupported query result type: %T", result))
		return TestResultFail
	}
}

// clearStore removes all triples from the store
func (r *TestRunner) clearStore() error {
	// Simple approach: clear by iterating and deleting
	// For a production system, would want a more efficient Clear() method
	pattern := &store.Pattern{
		Subject:   &store.Variable{Name: "s"},
		Predicate: &store.Variable{Name: "p"},
		Object:    &store.Variable{Name: "o"},
		Graph:     &store.Variable{Name: "g"},
	}
	iter, err := r.store.Query(pattern)
	if err != nil {
		return err
	}
	defer iter.Close()

	var triples []*rdf.Triple
	for iter.Next() {
		quad, err := iter.Quad()
		if err != nil {
			return err
		}
		// Convert quad to triple (ignore graph for now)
		triple := rdf.NewTriple(quad.Subject, quad.Predicate, quad.Object)
		triples = append(triples, triple)
	}

	for _, triple := range triples {
		if err := r.store.DeleteTriple(triple); err != nil {
			return err
		}
	}

	return nil
}

// loadTestData loads test data files into the store
func (r *TestRunner) loadTestData(manifest *TestManifest, test *TestCase) error {
	for _, dataFile := range test.Data {
		dataPath := manifest.ResolveFile(dataFile)
		dataBytes, err := os.ReadFile(dataPath) // #nosec G304 - test suite legitimately reads test data files
		if err != nil {
			return fmt.Errorf("failed to read data file %s: %w", dataFile, err)
		}

		// Parse Turtle data
		turtleParser := rdf.NewTurtleParser(string(dataBytes))
		triples, err := turtleParser.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse Turtle data in %s: %w", dataFile, err)
		}

		// Insert triples
		for _, triple := range triples {
			if err := r.store.InsertTriple(triple); err != nil {
				return fmt.Errorf("failed to insert triple: %w", err)
			}
		}
	}

	return nil
}

// resultsToBindings converts query results to bindings
func (r *TestRunner) resultsToBindings(results *executor.SelectResult) ([]map[string]rdf.Term, error) {
	var bindings []map[string]rdf.Term

	for _, result := range results.Bindings {
		binding := make(map[string]rdf.Term)
		for k, v := range result.Vars {
			binding[k] = v
		}
		bindings = append(bindings, binding)
	}

	return bindings, nil
}

// loadExpectedResults loads expected results from file
func (r *TestRunner) loadExpectedResults(manifest *TestManifest, test *TestCase) ([]map[string]rdf.Term, error) {
	resultPath := manifest.ResolveFile(test.Result)
	resultFile, err := os.Open(resultPath) // #nosec G304 - test suite legitimately reads test result files
	if err != nil {
		return nil, fmt.Errorf("failed to open result file: %w", err)
	}
	defer resultFile.Close()

	// Parse SPARQL XML results
	xmlResults, err := results.ParseXMLResults(resultFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML results: %w", err)
	}

	return xmlResults.ToBindings()
}

// loadExpectedTriples loads expected N-Triples from result file
func (r *TestRunner) loadExpectedTriples(manifest *TestManifest, test *TestCase) ([]*rdf.Triple, error) {
	resultPath := manifest.ResolveFile(test.Result)
	resultBytes, err := os.ReadFile(resultPath) // #nosec G304 - test suite legitimately reads test result files
	if err != nil {
		return nil, fmt.Errorf("failed to read result file: %w", err)
	}

	// Parse N-Triples/Turtle data
	turtleParser := rdf.NewTurtleParser(string(resultBytes))
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse expected triples: %w", err)
	}

	return triples, nil
}

// filePathToURI converts a file path to a URI for use as base URI
func (r *TestRunner) filePathToURI(filePath string) string {
	// W3C test files have a canonical online location
	// Check if this is a W3C test file
	if strings.Contains(filePath, "rdf-tests/") {
		// Extract the path after "rdf-tests/"
		idx := strings.Index(filePath, "rdf-tests/")
		if idx != -1 {
			relativePath := filePath[idx+len("rdf-tests/"):]
			return "https://w3c.github.io/rdf-tests/" + relativePath
		}
	}

	// For non-W3C files, use file:// URI
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		// Fall back to original path
		absPath = filePath
	}
	return "file://" + absPath
}

// compareTriples compares two sets of triples for equality (order-independent, blank node isomorphism)
func (r *TestRunner) compareTriples(expected, actual []*rdf.Triple) bool {
	// Use graph isomorphism algorithm that handles blank node label differences
	return rdf.AreGraphsIsomorphic(expected, actual)
}

// executorTermToRDFTerm converts an executor.Term to rdf.Term
func (r *TestRunner) executorTermToRDFTerm(t executor.Term) (rdf.Term, error) {
	switch t.Type {
	case "iri":
		return rdf.NewNamedNode(t.Value), nil
	case "blank":
		return rdf.NewBlankNode(t.Value), nil
	case "literal":
		return rdf.NewLiteral(t.Value), nil
	default:
		return nil, fmt.Errorf("unknown term type: %s", t.Type)
	}
}

// recordError records a test error
func (r *TestRunner) recordError(test *TestCase, errMsg string) {
	r.stats.Errors = append(r.stats.Errors, TestError{
		TestName: test.Name,
		Type:     test.Type,
		Error:    errMsg,
	})
}

// printSummary prints test execution summary
func (r *TestRunner) printSummary() {
	fmt.Println("\n" + strings.Repeat("━", 60))
	fmt.Println("📊 TEST SUMMARY")
	fmt.Println(strings.Repeat("━", 60))
	fmt.Printf("Total:   %d\n", r.stats.Total)
	fmt.Printf("Passed:  %d (%.1f%%)\n", r.stats.Passed,
		float64(r.stats.Passed)/float64(r.stats.Total)*100)
	fmt.Printf("Failed:  %d\n", r.stats.Failed)
	fmt.Printf("Skipped: %d\n", r.stats.Skipped)

	if len(r.stats.Errors) > 0 {
		fmt.Println("\n❌ ERRORS:")
		for i, err := range r.stats.Errors {
			if i >= 10 {
				fmt.Printf("   ... and %d more\n", len(r.stats.Errors)-10)
				break
			}
			fmt.Printf("   • %s: %s\n", err.TestName, err.Error)
		}
	}

	fmt.Println(strings.Repeat("━", 60))
}

// GetStats returns the current test statistics
func (r *TestRunner) GetStats() *TestStats {
	return r.stats
}

// runCSVFormatTest runs a CSV result format test
func (r *TestRunner) runCSVFormatTest(manifest *TestManifest, test *TestCase) TestResult {
	return r.runResultFormatTest(manifest, test, "csv")
}

// runTSVFormatTest runs a TSV result format test
func (r *TestRunner) runTSVFormatTest(manifest *TestManifest, test *TestCase) TestResult {
	return r.runResultFormatTest(manifest, test, "tsv")
}

// runJSONFormatTest runs a JSON result format test
func (r *TestRunner) runJSONFormatTest(manifest *TestManifest, test *TestCase) TestResult {
	return r.runResultFormatTest(manifest, test, "json")
}

// runResultFormatTest is a generic method for testing result formats
func (r *TestRunner) runResultFormatTest(manifest *TestManifest, test *TestCase, format string) TestResult {
	// Clear store before each test
	if err := r.clearStore(); err != nil {
		r.recordError(test, fmt.Sprintf("Failed to clear store: %v", err))
		return TestResultError
	}

	// Load data files
	if err := r.loadTestData(manifest, test); err != nil {
		r.recordError(test, fmt.Sprintf("Failed to load test data: %v", err))
		return TestResultError
	}

	// Read and parse query
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	queryFile := manifest.ResolveFile(test.Action)
	queryBytes, err := os.ReadFile(queryFile) // #nosec G304 - test suite legitimately reads test query files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read query file: %v", err))
		return TestResultError
	}

	// Parse query
	p := parser.NewParser(string(queryBytes))
	query, err := p.Parse()
	if err != nil {
		r.recordError(test, fmt.Sprintf("Parser error: %v", err))
		return TestResultFail
	}

	// Optimize query
	count, _ := r.store.Count()
	stats := &optimizer.Statistics{TotalTriples: count}
	opt := optimizer.NewOptimizer(stats)
	plan, err := opt.Optimize(query)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Optimizer error: %v", err))
		return TestResultFail
	}

	// Execute query
	exec := executor.NewExecutor(r.store)
	result, err := exec.Execute(plan)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Execution error: %v", err))
		return TestResultFail
	}

	// Format results in the requested format
	var actualOutput []byte
	switch format {
	case "csv":
		if selectResult, ok := result.(*executor.SelectResult); ok {
			actualOutput, err = results.FormatSelectResultsCSV(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			actualOutput, err = results.FormatAskResultCSV(askResult)
		} else {
			r.recordError(test, fmt.Sprintf("Unsupported result type for CSV: %T", result))
			return TestResultFail
		}

	case "tsv":
		if selectResult, ok := result.(*executor.SelectResult); ok {
			actualOutput, err = results.FormatSelectResultsTSV(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			actualOutput, err = results.FormatAskResultTSV(askResult)
		} else {
			r.recordError(test, fmt.Sprintf("Unsupported result type for TSV: %T", result))
			return TestResultFail
		}

	case "json":
		if selectResult, ok := result.(*executor.SelectResult); ok {
			actualOutput, err = results.FormatSelectResultsJSON(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			actualOutput, err = results.FormatAskResultJSON(askResult)
		} else {
			r.recordError(test, fmt.Sprintf("Unsupported result type for JSON: %T", result))
			return TestResultFail
		}

	default:
		r.recordError(test, fmt.Sprintf("Unknown format: %s", format))
		return TestResultError
	}

	if err != nil {
		r.recordError(test, fmt.Sprintf("Format error: %v", err))
		return TestResultFail
	}

	// Load expected results
	if test.Result == "" {
		r.recordError(test, "No result file specified")
		return TestResultError
	}

	resultPath := manifest.ResolveFile(test.Result)
	expectedOutput, err := os.ReadFile(resultPath) // #nosec G304 - test suite legitimately reads test result files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read expected result file: %v", err))
		return TestResultError
	}

	// Compare outputs (normalize line endings and whitespace)
	if !compareOutputs(string(actualOutput), string(expectedOutput)) {
		r.recordError(test, fmt.Sprintf("Output mismatch\nExpected:\n%s\n\nActual:\n%s", string(expectedOutput), string(actualOutput)))
		return TestResultFail
	}

	return TestResultPass
}

// compareOutputs compares two output strings, normalizing line endings and trailing whitespace
func compareOutputs(actual, expected string) bool {
	// Normalize line endings
	actual = strings.ReplaceAll(actual, "\r\n", "\n")
	expected = strings.ReplaceAll(expected, "\r\n", "\n")

	// Split into lines and compare
	actualLines := strings.Split(strings.TrimSpace(actual), "\n")
	expectedLines := strings.Split(strings.TrimSpace(expected), "\n")

	if len(actualLines) != len(expectedLines) {
		return false
	}

	for i := range actualLines {
		// Trim trailing whitespace from each line
		actualLine := strings.TrimRight(actualLines[i], " \t")
		expectedLine := strings.TrimRight(expectedLines[i], " \t")

		if actualLine != expectedLine {
			return false
		}
	}

	return true
}

// runRDFPositiveSyntaxTest verifies an RDF document parses successfully
func (r *TestRunner) runRDFPositiveSyntaxTest(manifest *TestManifest, test *TestCase, format string) TestResult {
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	dataFile := manifest.ResolveFile(test.Action)
	dataBytes, err := os.ReadFile(dataFile) // #nosec G304 - test suite legitimately reads test data files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read data file: %v", err))
		return TestResultError
	}

	// Try to parse the RDF data
	_, err = r.parseRDFData(string(dataBytes), format, dataFile)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Parser error: %v", err))
		return TestResultFail
	}

	return TestResultPass
}

// runRDFNegativeSyntaxTest verifies an RDF document fails to parse
func (r *TestRunner) runRDFNegativeSyntaxTest(manifest *TestManifest, test *TestCase, format string) TestResult {
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	dataFile := manifest.ResolveFile(test.Action)
	dataBytes, err := os.ReadFile(dataFile) // #nosec G304 - test suite legitimately reads test data files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read data file: %v", err))
		return TestResultError
	}

	// Try to parse the RDF data - it should fail
	_, err = r.parseRDFData(string(dataBytes), format, dataFile)
	if err == nil {
		r.recordError(test, "Data parsed successfully but should have failed")
		return TestResultFail
	}

	// Expected to fail, so this is a pass
	return TestResultPass
}

// runRDFEvalTest parses RDF data and compares with expected triples
func (r *TestRunner) runRDFEvalTest(manifest *TestManifest, test *TestCase, format string) TestResult {
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	// Read and parse input RDF data
	dataFile := manifest.ResolveFile(test.Action)
	dataBytes, err := os.ReadFile(dataFile) // #nosec G304 - test suite legitimately reads test data files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read data file: %v", err))
		return TestResultError
	}

	actualTriples, err := r.parseRDFData(string(dataBytes), format, dataFile)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Parser error: %v", err))
		return TestResultFail
	}

	// Load expected triples from result file
	if test.Result == "" {
		r.recordError(test, "No result file specified")
		return TestResultError
	}

	resultFile := manifest.ResolveFile(test.Result)
	resultBytes, err := os.ReadFile(resultFile) // #nosec G304 - test suite legitimately reads test result files
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read result file: %v", err))
		return TestResultError
	}

	// Expected results are in N-Triples or N-Quads format
	expectedTriples, err := r.parseRDFData(string(resultBytes), "ntriples", "")
	if err != nil {
		// Try N-Quads format if N-Triples fails
		expectedTriples, err = r.parseRDFData(string(resultBytes), "nquads", "")
		if err != nil {
			r.recordError(test, fmt.Sprintf("Failed to parse expected results: %v", err))
			return TestResultError
		}
	}

	// Compare triples (order-independent, blank node isomorphism)
	if !r.compareTriples(expectedTriples, actualTriples) {
		r.recordError(test, fmt.Sprintf("Triples mismatch: expected %d triples, got %d triples", len(expectedTriples), len(actualTriples)))
		return TestResultFail
	}

	return TestResultPass
}

// parseRDFData parses RDF data in the specified format
func (r *TestRunner) parseRDFData(data string, format string, filePath string) ([]*rdf.Triple, error) {
	switch format {
	case "turtle":
		parser := rdf.NewTurtleParser(data)
		// Set base URI from file path if provided
		if filePath != "" {
			baseURI := r.filePathToURI(filePath)
			parser.SetBaseURI(baseURI)
		}
		return parser.Parse()
	case "ntriples":
		parser := rdf.NewNTriplesParser(data) // Use strict N-Triples parser
		return parser.Parse()
	case "nquads":
		parser := rdf.NewNQuadsParser(data)
		quads, err := parser.Parse()
		if err != nil {
			return nil, err
		}
		// Convert quads to triples (ignore graph)
		triples := make([]*rdf.Triple, len(quads))
		for i, quad := range quads {
			triples[i] = rdf.NewTriple(quad.Subject, quad.Predicate, quad.Object)
		}
		return triples, nil
	case "trig":
		parser := rdf.NewTriGParser(data)
		// Set base URI from file path if provided
		if filePath != "" {
			baseURI := r.filePathToURI(filePath)
			parser.SetBaseURI(baseURI)
		}
		quads, err := parser.Parse()
		if err != nil {
			return nil, err
		}
		// Convert quads to triples (ignore graph)
		triples := make([]*rdf.Triple, len(quads))
		for i, quad := range quads {
			triples[i] = rdf.NewTriple(quad.Subject, quad.Predicate, quad.Object)
		}
		return triples, nil
	case "rdfxml":
		parser := rdf.NewRDFXMLParser()

		// Set base URI from file path if provided
		if filePath != "" {
			// Convert file path to URI
			baseURI := r.filePathToURI(filePath)
			parser.SetBaseURI(baseURI)
		}

		reader := strings.NewReader(data)
		quads, err := parser.Parse(reader)
		if err != nil {
			return nil, err
		}
		// Convert quads to triples (ignore graph)
		triples := make([]*rdf.Triple, len(quads))
		for i, quad := range quads {
			triples[i] = rdf.NewTriple(quad.Subject, quad.Predicate, quad.Object)
		}
		return triples, nil
	case "jsonld":
		parser := rdf.NewJSONLDParser()
		reader := strings.NewReader(data)
		quads, err := parser.Parse(reader)
		if err != nil {
			return nil, err
		}
		// Convert quads to triples (ignore graph)
		triples := make([]*rdf.Triple, len(quads))
		for i, quad := range quads {
			triples[i] = rdf.NewTriple(quad.Subject, quad.Predicate, quad.Object)
		}
		return triples, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}
