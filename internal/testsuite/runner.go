package testsuite

import (
	"fmt"
	"os"
	"strings"

	"github.com/aleksaelezovic/trigo/internal/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/internal/sparql/parser"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/internal/store"
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
		store: store.NewTripleStore(storage),
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
	queryBytes, err := os.ReadFile(queryFile)
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
	queryBytes, err := os.ReadFile(queryFile)
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
	// TODO: Implement full evaluation test
	// This would require:
	// 1. Loading data files into the store
	// 2. Executing the query
	// 3. Comparing results with expected output
	// 4. Handling different result formats (XML, JSON, etc.)

	// For now, just try to parse and optimize
	if test.Action == "" {
		r.recordError(test, "No action file specified")
		return TestResultError
	}

	queryFile := manifest.ResolveFile(test.Action)
	queryBytes, err := os.ReadFile(queryFile)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Failed to read query file: %v", err))
		return TestResultError
	}

	// Parse
	p := parser.NewParser(string(queryBytes))
	query, err := p.Parse()
	if err != nil {
		r.recordError(test, fmt.Sprintf("Parser error: %v", err))
		return TestResultFail
	}

	// Optimize
	count, _ := r.store.Count()
	stats := &optimizer.Statistics{TotalTriples: count}
	opt := optimizer.NewOptimizer(stats)
	_, err = opt.Optimize(query)
	if err != nil {
		r.recordError(test, fmt.Sprintf("Optimizer error: %v", err))
		return TestResultFail
	}

	// TODO: Execute and compare results
	// For now, skip these tests
	return TestResultSkip
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
