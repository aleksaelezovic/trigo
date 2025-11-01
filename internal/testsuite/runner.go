package testsuite

import (
	"fmt"
	"os"
	"strings"

	"github.com/aleksaelezovic/trigo/internal/encoding"
	"github.com/aleksaelezovic/trigo/internal/sparql/executor"
	"github.com/aleksaelezovic/trigo/internal/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/internal/sparql/parser"
	"github.com/aleksaelezovic/trigo/internal/sparqlxml"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/internal/turtle"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

// TestRunner runs W3C SPARQL test suite tests
type TestRunner struct {
	store    *store.TripleStore
	stats    *TestStats
}

// TestStats tracks test execution statistics
type TestStats struct {
	Total    int
	Passed   int
	Failed   int
	Skipped  int
	Errors   []TestError
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
	case TestTypePositiveSyntax, TestTypePositiveSyntax11:
		return r.runPositiveSyntaxTest(manifest, test)
	case TestTypeNegativeSyntax, TestTypeNegativeSyntax11:
		return r.runNegativeSyntaxTest(manifest, test)
	case TestTypeQueryEvaluation:
		return r.runQueryEvaluationTest(manifest, test)
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

	// Convert results to bindings (only handle SELECT queries for now)
	selectResult, ok := result.(*executor.SelectResult)
	if !ok {
		r.recordError(test, fmt.Sprintf("Only SELECT queries supported for now, got: %T", result))
		return TestResultFail
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
	if !sparqlxml.CompareResults(expectedBindings, actualBindings) {
		r.recordError(test, fmt.Sprintf("Results mismatch: expected %d bindings, got %d bindings", len(expectedBindings), len(actualBindings)))
		return TestResultFail
	}

	return TestResultPass
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
		turtleParser := turtle.NewParser(string(dataBytes))
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
	results, err := sparqlxml.ParseXMLResults(resultFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML results: %w", err)
	}

	return results.ToBindings()
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
