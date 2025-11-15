package executor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/evaluator"
	"github.com/aleksaelezovic/trigo/pkg/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

// Executor executes SPARQL queries using the Volcano iterator model
type Executor struct {
	store *store.TripleStore
}

// NewExecutor creates a new query executor
func NewExecutor(store *store.TripleStore) *Executor {
	return &Executor{
		store: store,
	}
}

// Execute executes an optimized query
func (e *Executor) Execute(query *optimizer.OptimizedQuery) (QueryResult, error) {
	switch query.Original.QueryType {
	case parser.QueryTypeSelect:
		return e.executeSelect(query)
	case parser.QueryTypeAsk:
		return e.executeAsk(query)
	case parser.QueryTypeConstruct:
		return e.executeConstruct(query)
	case parser.QueryTypeDescribe:
		return e.executeDescribe(query)
	default:
		return nil, fmt.Errorf("unsupported query type")
	}
}

// QueryResult represents the result of a query
type QueryResult interface {
	resultType()
}

// SelectResult represents the result of a SELECT query
type SelectResult struct {
	Variables []*parser.Variable
	Bindings  []*store.Binding
}

func (r *SelectResult) resultType() {}

// AskResult represents the result of an ASK query
type AskResult struct {
	Result bool
}

func (r *AskResult) resultType() {}

// ConstructResult represents the result of a CONSTRUCT query
type ConstructResult struct {
	Triples []*Triple
}

// Triple represents an RDF triple (subject, predicate, object)
type Triple struct {
	Subject   Term
	Predicate Term
	Object    Term
}

// Term represents an RDF term (for CONSTRUCT results)
type Term struct {
	Type  string // "iri", "blank", "literal"
	Value string
}

func (r *ConstructResult) resultType() {}

// executeSelect executes a SELECT query
func (e *Executor) executeSelect(query *optimizer.OptimizedQuery) (*SelectResult, error) {
	// Create iterator from plan
	iter, err := e.createIterator(query.Plan)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	// Collect all bindings
	var bindings []*store.Binding
	for iter.Next() {
		binding := iter.Binding()
		// Clone to avoid mutation
		bindings = append(bindings, binding.Clone())
	}

	// Determine variables list
	variables := query.Original.Select.Variables
	if variables == nil {
		// SELECT * - extract variables from WHERE clause in order they appear
		variables = extractVariablesFromGraphPattern(query.Original.Select.Where)
	}

	// Apply DISTINCT if specified
	if query.Original.Select.Distinct {
		bindings = applyDistinct(bindings)
	}

	// Apply REDUCED if specified (relaxed de-duplication)
	if query.Original.Select.Reduced {
		bindings = applyReduced(bindings)
	}

	return &SelectResult{
		Variables: variables,
		Bindings:  bindings,
	}, nil
}

// applyDistinct removes duplicate bindings
func applyDistinct(bindings []*store.Binding) []*store.Binding {
	if len(bindings) == 0 {
		return bindings
	}

	seen := make(map[string]bool)
	var unique []*store.Binding

	for _, binding := range bindings {
		// Create a signature for this binding
		sig := bindingSignature(binding)
		if !seen[sig] {
			seen[sig] = true
			unique = append(unique, binding)
		}
	}

	return unique
}

// applyReduced is a hint that duplicates MAY be removed but are not required to be removed.
// Per SPARQL spec, REDUCED permits but does not require elimination of duplicates.
// For simplicity and correctness, we keep all duplicates (no-op).
func applyReduced(bindings []*store.Binding) []*store.Binding {
	// REDUCED is a hint to the query engine that it MAY eliminate duplicates
	// but is not required to. To ensure test compatibility, we don't remove any.
	return bindings
}

// bindingSignature creates a unique string representation of a binding
func bindingSignature(binding *store.Binding) string {
	var parts []string
	for varName, term := range binding.Vars {
		parts = append(parts, varName+"="+termSignature(term))
	}
	// Sort to ensure consistent ordering
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// termSignature creates a unique string representation of an RDF term
func termSignature(term rdf.Term) string {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return "iri:" + t.IRI
	case *rdf.BlankNode:
		return "blank:" + t.ID
	case *rdf.Literal:
		sig := "lit:" + t.Value
		if t.Language != "" {
			sig += "@" + t.Language
		}
		if t.Datatype != nil {
			sig += "^^" + t.Datatype.IRI
		}
		return sig
	default:
		return "unknown:" + fmt.Sprintf("%v", term)
	}
}

// executeAsk executes an ASK query
func (e *Executor) executeAsk(query *optimizer.OptimizedQuery) (*AskResult, error) {
	// Create iterator from plan
	iter, err := e.createIterator(query.Plan)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	// Check if there's at least one result
	result := iter.Next()

	return &AskResult{Result: result}, nil
}

// executeConstruct executes a CONSTRUCT query
func (e *Executor) executeConstruct(query *optimizer.OptimizedQuery) (*ConstructResult, error) {
	// Get the template from the construct plan
	constructPlan, ok := query.Plan.(*optimizer.ConstructPlan)
	if !ok {
		return nil, fmt.Errorf("expected ConstructPlan")
	}

	// Create iterator from the input plan (WHERE clause)
	iter, err := e.createIterator(constructPlan.Input)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	// Collect triples by instantiating template for each binding
	var triples []*Triple
	seenTriples := make(map[string]bool) // For deduplication

	for iter.Next() {
		binding := iter.Binding()

		// Instantiate each triple pattern in the template
		for _, pattern := range constructPlan.Template {
			triple, err := e.instantiateTriplePattern(pattern, binding)
			if err != nil {
				// Skip triples that can't be instantiated (e.g., unbound variables)
				continue
			}

			// Deduplicate triples
			key := fmt.Sprintf("%s|%s|%s", triple.Subject.Value, triple.Predicate.Value, triple.Object.Value)
			if !seenTriples[key] {
				seenTriples[key] = true
				triples = append(triples, triple)
			}
		}
	}

	return &ConstructResult{Triples: triples}, nil
}

// executeDescribe executes a DESCRIBE query
func (e *Executor) executeDescribe(query *optimizer.OptimizedQuery) (*ConstructResult, error) {
	// Get the describe plan
	describePlan, ok := query.Plan.(*optimizer.DescribePlan)
	if !ok {
		return nil, fmt.Errorf("expected DescribePlan")
	}

	// Collect resources to describe
	var resourcesToDescribe []rdf.Term

	if describePlan.Input != nil {
		// Execute WHERE clause to find resources dynamically
		iter, err := e.createIterator(describePlan.Input)
		if err != nil {
			return nil, err
		}
		defer iter.Close()

		// Collect all IRIs from bindings
		seen := make(map[string]bool)
		for iter.Next() {
			binding := iter.Binding()
			// Add all bound IRIs (named nodes) to resources
			for _, term := range binding.Vars {
				if namedNode, ok := term.(*rdf.NamedNode); ok {
					key := namedNode.IRI
					if !seen[key] {
						seen[key] = true
						resourcesToDescribe = append(resourcesToDescribe, namedNode)
					}
				}
			}
		}
	} else {
		// Use resources directly from DESCRIBE clause
		for _, resource := range describePlan.Resources {
			resourcesToDescribe = append(resourcesToDescribe, resource)
		}
	}

	// For each resource, get all triples where it's the subject (CBD - Concise Bounded Description)
	var triples []*Triple
	seenTriples := make(map[string]bool)

	for _, resource := range resourcesToDescribe {
		// Query pattern: <resource> ?p ?o
		pattern := &store.Pattern{
			Subject:   resource,
			Predicate: &store.Variable{Name: "p"},
			Object:    &store.Variable{Name: "o"},
			Graph:     &store.Variable{Name: "g"},
		}

		iter, err := e.store.Query(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to query store for resource %s: %w", resource.String(), err)
		}

		for iter.Next() {
			quad, err := iter.Quad()
			if err != nil {
				if closeErr := iter.Close(); closeErr != nil {
					return nil, fmt.Errorf("error closing iterator: %w (after quad error: %v)", closeErr, err)
				}
				return nil, err
			}

			// Convert to executor.Triple
			triple := &Triple{
				Subject:   Term{Type: "iri", Value: quad.Subject.String()},
				Predicate: Term{Type: "iri", Value: quad.Predicate.String()},
				Object:    e.rdfTermToExecutorTerm(quad.Object),
			}

			// Deduplicate triples
			key := fmt.Sprintf("%s|%s|%s", triple.Subject.Value, triple.Predicate.Value, triple.Object.Value)
			if !seenTriples[key] {
				seenTriples[key] = true
				triples = append(triples, triple)
			}
		}
		if err := iter.Close(); err != nil {
			return nil, fmt.Errorf("error closing iterator: %w", err)
		}
	}

	return &ConstructResult{Triples: triples}, nil
}

// rdfTermToExecutorTerm converts an rdf.Term to executor.Term
func (e *Executor) rdfTermToExecutorTerm(term rdf.Term) Term {
	switch t := term.(type) {
	case *rdf.NamedNode:
		return Term{Type: "iri", Value: t.IRI}
	case *rdf.BlankNode:
		return Term{Type: "blank", Value: t.ID}
	case *rdf.Literal:
		return Term{Type: "literal", Value: t.Value}
	default:
		return Term{Type: "literal", Value: term.String()}
	}
}

// instantiateTriplePattern creates a triple from a pattern and binding
func (e *Executor) instantiateTriplePattern(pattern *parser.TriplePattern, binding *store.Binding) (*Triple, error) {
	subject, err := e.instantiateTerm(pattern.Subject, binding)
	if err != nil {
		return nil, err
	}

	predicate, err := e.instantiateTerm(pattern.Predicate, binding)
	if err != nil {
		return nil, err
	}

	object, err := e.instantiateTerm(pattern.Object, binding)
	if err != nil {
		return nil, err
	}

	return &Triple{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
	}, nil
}

// instantiateTerm converts a TermOrVariable to a concrete Term using bindings
func (e *Executor) instantiateTerm(termOrVar parser.TermOrVariable, binding *store.Binding) (Term, error) {
	if termOrVar.IsVariable() {
		// Look up variable in binding
		value, found := binding.Vars[termOrVar.Variable.Name]
		if !found {
			return Term{}, fmt.Errorf("unbound variable: %s", termOrVar.Variable.Name)
		}
		return e.rdfTermToExecutorTerm(value), nil
	}

	// It's a constant term
	return e.rdfTermToExecutorTerm(termOrVar.Term), nil
}

// createIterator creates an iterator from a query plan
func (e *Executor) createIterator(plan optimizer.QueryPlan) (store.BindingIterator, error) {
	switch p := plan.(type) {
	case *optimizer.ScanPlan:
		return e.createScanIterator(p)
	case *optimizer.JoinPlan:
		return e.createJoinIterator(p)
	case *optimizer.FilterPlan:
		return e.createFilterIterator(p)
	case *optimizer.ProjectionPlan:
		return e.createProjectionIterator(p)
	case *optimizer.LimitPlan:
		return e.createLimitIterator(p)
	case *optimizer.OffsetPlan:
		return e.createOffsetIterator(p)
	case *optimizer.DistinctPlan:
		return e.createDistinctIterator(p)
	case *optimizer.GraphPlan:
		return e.createGraphIterator(p)
	case *optimizer.BindPlan:
		return e.createBindIterator(p)
	case *optimizer.OptionalPlan:
		return e.createOptionalIterator(p)
	case *optimizer.UnionPlan:
		return e.createUnionIterator(p)
	case *optimizer.MinusPlan:
		return e.createMinusIterator(p)
	case *optimizer.OrderByPlan:
		return e.createOrderByIterator(p)
	default:
		return nil, fmt.Errorf("unsupported plan type: %T", plan)
	}
}

// createScanIterator creates an iterator for scanning a triple pattern
func (e *Executor) createScanIterator(plan *optimizer.ScanPlan) (store.BindingIterator, error) {
	// Convert parser triple pattern to store pattern
	pattern := &store.Pattern{
		Subject:   e.convertTermOrVariable(plan.Pattern.Subject),
		Predicate: e.convertTermOrVariable(plan.Pattern.Predicate),
		Object:    e.convertTermOrVariable(plan.Pattern.Object),
	}

	// Execute pattern query
	quadIter, err := e.store.Query(pattern)
	if err != nil {
		return nil, err
	}

	return &scanIterator{
		quadIter: quadIter,
		pattern:  plan.Pattern,
		binding:  store.NewBinding(),
	}, nil
}

// createJoinIterator creates an iterator for join operations
func (e *Executor) createJoinIterator(plan *optimizer.JoinPlan) (store.BindingIterator, error) {
	left, err := e.createIterator(plan.Left)
	if err != nil {
		return nil, err
	}

	switch plan.Type {
	case optimizer.JoinTypeNestedLoop:
		return &nestedLoopJoinIterator{
			left:         left,
			rightPlan:    plan.Right,
			executor:     e,
			currentLeft:  nil,
			currentRight: nil,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported join type: %v", plan.Type)
	}
}

// createFilterIterator creates an iterator for filter operations
func (e *Executor) createFilterIterator(plan *optimizer.FilterPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &filterIterator{
		input:     input,
		filter:    plan.Filter,
		evaluator: evaluator.NewEvaluator(),
	}, nil
}

// createProjectionIterator creates an iterator for projection operations
func (e *Executor) createProjectionIterator(plan *optimizer.ProjectionPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &projectionIterator{
		input:     input,
		variables: plan.Variables,
	}, nil
}

// createLimitIterator creates an iterator for LIMIT operations
func (e *Executor) createLimitIterator(plan *optimizer.LimitPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &limitIterator{
		input: input,
		limit: plan.Limit,
		count: 0,
	}, nil
}

// createOffsetIterator creates an iterator for OFFSET operations
func (e *Executor) createOffsetIterator(plan *optimizer.OffsetPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &offsetIterator{
		input:   input,
		offset:  plan.Offset,
		skipped: 0,
	}, nil
}

// createDistinctIterator creates an iterator for DISTINCT operations
func (e *Executor) createDistinctIterator(plan *optimizer.DistinctPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &distinctIterator{
		input: input,
		seen:  make(map[string]bool),
	}, nil
}

// convertTermOrVariable converts a parser term/variable to store format
func (e *Executor) convertTermOrVariable(tov parser.TermOrVariable) any {
	if tov.IsVariable() {
		return store.NewVariable(tov.Variable.Name)
	}
	return tov.Term
}

// scanIterator implements BindingIterator for scanning
type scanIterator struct {
	quadIter store.QuadIterator
	pattern  *parser.TriplePattern
	binding  *store.Binding
}

func (it *scanIterator) Next() bool {
	for {
		if !it.quadIter.Next() {
			return false
		}

		quad, err := it.quadIter.Quad()
		if err != nil {
			return false
		}

		// Bind variables, checking for repeated variables
		it.binding = store.NewBinding()
		valid := true

		// Bind subject
		if it.pattern.Subject.IsVariable() {
			varName := it.pattern.Subject.Variable.Name
			it.binding.Vars[varName] = quad.Subject
		}

		// Bind predicate (check if variable already bound from subject)
		if it.pattern.Predicate.IsVariable() {
			varName := it.pattern.Predicate.Variable.Name
			if existingValue, exists := it.binding.Vars[varName]; exists {
				// Variable already bound - check if values match
				if !existingValue.Equals(quad.Predicate) {
					valid = false
				}
			} else {
				it.binding.Vars[varName] = quad.Predicate
			}
		}

		// Bind object (check if variable already bound from subject or predicate)
		if valid && it.pattern.Object.IsVariable() {
			varName := it.pattern.Object.Variable.Name
			if existingValue, exists := it.binding.Vars[varName]; exists {
				// Variable already bound - check if values match
				if !existingValue.Equals(quad.Object) {
					valid = false
				}
			} else {
				it.binding.Vars[varName] = quad.Object
			}
		}

		// If all variable constraints are satisfied, return this binding
		if valid {
			return true
		}
		// Otherwise, continue to next quad
	}
}

func (it *scanIterator) Binding() *store.Binding {
	return it.binding
}

func (it *scanIterator) Close() error {
	return it.quadIter.Close()
}

// nestedLoopJoinIterator implements nested loop join
type nestedLoopJoinIterator struct {
	left         store.BindingIterator
	rightPlan    optimizer.QueryPlan
	executor     *Executor
	currentLeft  *store.Binding
	currentRight store.BindingIterator
	result       *store.Binding
}

func (it *nestedLoopJoinIterator) Next() bool {
	for {
		// If we have a right iterator, try to get next from it
		if it.currentRight != nil {
			if it.currentRight.Next() {
				rightBinding := it.currentRight.Binding()

				// Merge bindings
				merged := it.mergeBindings(it.currentLeft, rightBinding)
				if merged != nil {
					it.result = merged
					return true
				}
				continue
			}
			// Right exhausted, close it
			_ = it.currentRight.Close() // #nosec G104 - close error doesn't affect iteration logic
			it.currentRight = nil
		}

		// Get next from left
		if !it.left.Next() {
			return false
		}

		it.currentLeft = it.left.Binding()

		// Create new right iterator (with current left binding applied)
		rightIter, err := it.executor.createIterator(it.rightPlan)
		if err != nil {
			return false
		}
		it.currentRight = rightIter
	}
}

func (it *nestedLoopJoinIterator) Binding() *store.Binding {
	return it.result
}

func (it *nestedLoopJoinIterator) Close() error {
	if it.currentRight != nil {
		_ = it.currentRight.Close() // #nosec G104 - right close error less critical than left close error
	}
	return it.left.Close()
}

// mergeBindings merges two bindings, returns nil if incompatible
func (it *nestedLoopJoinIterator) mergeBindings(left, right *store.Binding) *store.Binding {
	result := left.Clone()

	for varName, term := range right.Vars {
		if existingTerm, exists := result.Vars[varName]; exists {
			// Check compatibility
			if !existingTerm.Equals(term) {
				return nil
			}
		} else {
			result.Vars[varName] = term
		}
	}

	return result
}

// filterIterator implements filter operations
type filterIterator struct {
	input     store.BindingIterator
	filter    *parser.Filter
	evaluator *evaluator.Evaluator
}

func (it *filterIterator) Next() bool {
	for it.input.Next() {
		binding := it.input.Binding()

		// If no expression, pass through (shouldn't happen)
		if it.filter.Expression == nil {
			return true
		}

		// Evaluate the filter expression
		result, err := it.evaluator.Evaluate(it.filter.Expression, binding)
		if err != nil {
			// Expression evaluation error - filter out this binding
			continue
		}

		// Check effective boolean value
		lit, ok := result.(*rdf.Literal)
		if !ok {
			// Non-literal result - filter out
			continue
		}

		// Check if it's a boolean literal with value true
		if lit.Datatype != nil && lit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#boolean" {
			if lit.Value == "true" || lit.Value == "1" {
				return true
			}
		}

		// Not true - continue to next binding
	}
	return false
}

func (it *filterIterator) Binding() *store.Binding {
	return it.input.Binding()
}

func (it *filterIterator) Close() error {
	return it.input.Close()
}

// projectionIterator implements projection operations
type projectionIterator struct {
	input     store.BindingIterator
	variables []*parser.Variable
}

func (it *projectionIterator) Next() bool {
	return it.input.Next()
}

func (it *projectionIterator) Binding() *store.Binding {
	if it.variables == nil {
		// SELECT *
		return it.input.Binding()
	}

	// Project only selected variables
	binding := store.NewBinding()
	inputBinding := it.input.Binding()

	for _, variable := range it.variables {
		if term, exists := inputBinding.Vars[variable.Name]; exists {
			binding.Vars[variable.Name] = term
		}
	}

	return binding
}

func (it *projectionIterator) Close() error {
	return it.input.Close()
}

// limitIterator implements LIMIT operations
type limitIterator struct {
	input store.BindingIterator
	limit int
	count int
}

func (it *limitIterator) Next() bool {
	if it.count >= it.limit {
		return false
	}

	if it.input.Next() {
		it.count++
		return true
	}

	return false
}

func (it *limitIterator) Binding() *store.Binding {
	return it.input.Binding()
}

func (it *limitIterator) Close() error {
	return it.input.Close()
}

// offsetIterator implements OFFSET operations
type offsetIterator struct {
	input   store.BindingIterator
	offset  int
	skipped int
}

func (it *offsetIterator) Next() bool {
	// Skip initial rows
	for it.skipped < it.offset {
		if !it.input.Next() {
			return false
		}
		it.skipped++
	}

	return it.input.Next()
}

func (it *offsetIterator) Binding() *store.Binding {
	return it.input.Binding()
}

func (it *offsetIterator) Close() error {
	return it.input.Close()
}

// createGraphIterator creates an iterator for a GRAPH pattern
func (e *Executor) createGraphIterator(plan *optimizer.GraphPlan) (store.BindingIterator, error) {
	// The GRAPH pattern wraps the inner plan and constrains all scans to a specific graph
	// We need to wrap this by creating a modified executor that adds graph constraints

	// Create a graph-aware executor wrapper
	graphExec := &graphExecutor{
		base:  e,
		graph: plan.Graph,
	}

	// Execute the inner plan with the graph constraint
	return graphExec.createIterator(plan.Input)
}

// graphExecutor wraps an executor and adds graph constraints to all scans
type graphExecutor struct {
	base  *Executor
	graph *parser.GraphTerm
}

func (ge *graphExecutor) createIterator(plan optimizer.QueryPlan) (store.BindingIterator, error) {
	switch p := plan.(type) {
	case *optimizer.ScanPlan:
		return ge.createGraphScanIterator(p)
	case *optimizer.JoinPlan:
		// For joins, create an iterator with graph-constrained left side
		// The right side will be created on-demand during iteration
		left, err := ge.createIterator(p.Left)
		if err != nil {
			return nil, err
		}
		// Create a join iterator that uses the graph executor for right side too
		return &graphJoinIterator{
			left:      left,
			rightPlan: p.Right,
			graphExec: ge,
		}, nil
	default:
		// For other operators, delegate to base executor
		return ge.base.createIterator(plan)
	}
}

func (ge *graphExecutor) createGraphScanIterator(plan *optimizer.ScanPlan) (store.BindingIterator, error) {
	// Convert parser triple pattern to store pattern with graph constraint
	pattern := &store.Pattern{
		Subject:   ge.base.convertTermOrVariable(plan.Pattern.Subject),
		Predicate: ge.base.convertTermOrVariable(plan.Pattern.Predicate),
		Object:    ge.base.convertTermOrVariable(plan.Pattern.Object),
		Graph:     ge.convertGraphTerm(ge.graph),
	}

	// Execute pattern query
	quadIter, err := ge.base.store.Query(pattern)
	if err != nil {
		return nil, err
	}

	return &scanIterator{
		quadIter: quadIter,
		pattern:  plan.Pattern,
		binding:  store.NewBinding(),
	}, nil
}

func (ge *graphExecutor) convertGraphTerm(graphTerm *parser.GraphTerm) any {
	if graphTerm.Variable != nil {
		return &store.Variable{Name: graphTerm.Variable.Name}
	}
	return graphTerm.IRI
}

// graphJoinIterator implements nested loop join for GRAPH patterns
type graphJoinIterator struct {
	left         store.BindingIterator
	rightPlan    optimizer.QueryPlan
	graphExec    *graphExecutor
	currentLeft  *store.Binding
	currentRight store.BindingIterator
	result       *store.Binding
}

func (it *graphJoinIterator) Next() bool {
	for {
		// If we have a right iterator, try to get next from it
		if it.currentRight != nil {
			if it.currentRight.Next() {
				rightBinding := it.currentRight.Binding()

				// Merge bindings
				merged := it.mergeBindings(it.currentLeft, rightBinding)
				if merged != nil {
					it.result = merged
					return true
				}
				continue
			}
			// Right exhausted, close it
			_ = it.currentRight.Close() // #nosec G104 - close error doesn't affect iteration logic
			it.currentRight = nil
		}

		// Get next from left
		if !it.left.Next() {
			return false
		}

		it.currentLeft = it.left.Binding()

		// Create new right iterator using graph executor (with graph constraints)
		rightIter, err := it.graphExec.createIterator(it.rightPlan)
		if err != nil {
			return false
		}
		it.currentRight = rightIter
	}
}

func (it *graphJoinIterator) Binding() *store.Binding {
	return it.result
}

func (it *graphJoinIterator) Close() error {
	if it.currentRight != nil {
		_ = it.currentRight.Close() // #nosec G104 - right close error less critical than left close error
	}
	return it.left.Close()
}

// mergeBindings merges two bindings, returns nil if incompatible
func (it *graphJoinIterator) mergeBindings(left, right *store.Binding) *store.Binding {
	result := left.Clone()

	for varName, term := range right.Vars {
		if existingTerm, exists := result.Vars[varName]; exists {
			// Check compatibility
			if !existingTerm.Equals(term) {
				return nil
			}
		} else {
			result.Vars[varName] = term
		}
	}

	return result
}

// distinctIterator implements DISTINCT operations
type distinctIterator struct {
	input store.BindingIterator
	seen  map[string]bool
}

func (it *distinctIterator) Next() bool {
	for it.input.Next() {
		binding := it.input.Binding()
		key := it.bindingKey(binding)

		if !it.seen[key] {
			it.seen[key] = true
			return true
		}
	}
	return false
}

func (it *distinctIterator) Binding() *store.Binding {
	return it.input.Binding()
}

func (it *distinctIterator) Close() error {
	return it.input.Close()
}

// bindingKey creates a unique key for a binding
func (it *distinctIterator) bindingKey(binding *store.Binding) string {
	// Simple string concatenation for now
	// TODO: Implement better hashing
	key := ""
	for varName, term := range binding.Vars {
		key += varName + "=" + term.String() + ";"
	}
	return key
}

// createBindIterator creates an iterator for BIND operations
func (e *Executor) createBindIterator(plan *optimizer.BindPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &bindIterator{
		input:      input,
		expression: plan.Expression,
		variable:   plan.Variable,
		evaluator:  evaluator.NewEvaluator(),
	}, nil
}

// bindIterator implements BIND operations (variable assignment)
type bindIterator struct {
	input      store.BindingIterator
	expression parser.Expression
	variable   *parser.Variable
	evaluator  *evaluator.Evaluator
}

func (it *bindIterator) Next() bool {
	return it.input.Next()
}

func (it *bindIterator) Binding() *store.Binding {
	inputBinding := it.input.Binding()

	// Evaluate the expression
	result, err := it.evaluator.Evaluate(it.expression, inputBinding)
	if err != nil {
		// If evaluation fails, skip this binding by continuing without adding the variable
		// In SPARQL, BIND failures cause the solution to be dropped
		// However, we can't drop it here (we're in Binding() not Next())
		// So we return the input binding unchanged
		// TODO: Consider adding error handling in Next() instead
		return inputBinding
	}

	// Clone the input binding to avoid modifying it
	extendedBinding := inputBinding.Clone()

	// Add the result to the extended binding
	extendedBinding.Vars[it.variable.Name] = result

	return extendedBinding
}

func (it *bindIterator) Close() error {
	return it.input.Close()
}

// createOptionalIterator creates an iterator for OPTIONAL operations (left outer join)
func (e *Executor) createOptionalIterator(plan *optimizer.OptionalPlan) (store.BindingIterator, error) {
	left, err := e.createIterator(plan.Left)
	if err != nil {
		return nil, err
	}

	return &optionalIterator{
		left:         left,
		rightPlan:    plan.Right,
		executor:     e,
		currentLeft:  nil,
		currentRight: nil,
		hasMatch:     false,
	}, nil
}

// optionalIterator implements OPTIONAL patterns (left outer join)
type optionalIterator struct {
	left         store.BindingIterator
	rightPlan    optimizer.QueryPlan
	executor     *Executor
	currentLeft  *store.Binding
	currentRight store.BindingIterator
	result       *store.Binding
	hasMatch     bool
}

func (it *optionalIterator) Next() bool {
	for {
		// If we have a right iterator, try to get next from it
		if it.currentRight != nil {
			if it.currentRight.Next() {
				rightBinding := it.currentRight.Binding()

				// Try to merge bindings
				merged := it.mergeBindings(it.currentLeft, rightBinding)
				if merged != nil {
					it.hasMatch = true
					it.result = merged
					return true
				}
				continue
			}
			// Right exhausted
			_ = it.currentRight.Close() // #nosec G104 - close error doesn't affect iteration logic
			it.currentRight = nil

			// If no match was found, return the left binding alone
			if !it.hasMatch {
				it.result = it.currentLeft
				return true
			}
		}

		// Get next from left
		if !it.left.Next() {
			return false
		}

		it.currentLeft = it.left.Binding()
		it.hasMatch = false

		// Create new right iterator
		rightIter, err := it.executor.createIterator(it.rightPlan)
		if err != nil {
			// If right fails, still return left binding (OPTIONAL semantics)
			it.result = it.currentLeft
			return true
		}
		it.currentRight = rightIter
	}
}

func (it *optionalIterator) Binding() *store.Binding {
	return it.result
}

func (it *optionalIterator) Close() error {
	if it.currentRight != nil {
		_ = it.currentRight.Close() // #nosec G104 - right close error less critical than left close error
	}
	return it.left.Close()
}

// mergeBindings merges two bindings, returns nil if incompatible
func (it *optionalIterator) mergeBindings(left, right *store.Binding) *store.Binding {
	result := left.Clone()

	for varName, term := range right.Vars {
		if existingTerm, exists := result.Vars[varName]; exists {
			// Check compatibility
			if !existingTerm.Equals(term) {
				return nil
			}
		} else {
			result.Vars[varName] = term
		}
	}

	return result
}

// createUnionIterator creates an iterator for UNION operations (alternation)
func (e *Executor) createUnionIterator(plan *optimizer.UnionPlan) (store.BindingIterator, error) {
	left, err := e.createIterator(plan.Left)
	if err != nil {
		return nil, err
	}

	right, err := e.createIterator(plan.Right)
	if err != nil {
		_ = left.Close() // #nosec G104 - cleanup on error
		return nil, err
	}

	return &unionIterator{
		left:     left,
		right:    right,
		leftDone: false,
	}, nil
}

// unionIterator implements UNION patterns (alternation)
type unionIterator struct {
	left     store.BindingIterator
	right    store.BindingIterator
	leftDone bool
}

func (it *unionIterator) Next() bool {
	// First exhaust the left side
	if !it.leftDone {
		if it.left.Next() {
			return true
		}
		it.leftDone = true
	}

	// Then process the right side
	return it.right.Next()
}

func (it *unionIterator) Binding() *store.Binding {
	if !it.leftDone {
		return it.left.Binding()
	}
	return it.right.Binding()
}

func (it *unionIterator) Close() error {
	_ = it.left.Close() // #nosec G104 - left close error less critical than right close error
	return it.right.Close()
}

// createMinusIterator creates an iterator for MINUS operations (set difference)
func (e *Executor) createMinusIterator(plan *optimizer.MinusPlan) (store.BindingIterator, error) {
	left, err := e.createIterator(plan.Left)
	if err != nil {
		return nil, err
	}

	return &minusIterator{
		left:      left,
		rightPlan: plan.Right,
		executor:  e,
	}, nil
}

// minusIterator implements MINUS patterns (set difference)
type minusIterator struct {
	left      store.BindingIterator
	rightPlan optimizer.QueryPlan
	executor  *Executor
}

func (it *minusIterator) Next() bool {
	for it.left.Next() {
		leftBinding := it.left.Binding()

		// Check if this binding is compatible with any right binding
		rightIter, err := it.executor.createIterator(it.rightPlan)
		if err != nil {
			// If right fails, return left binding (MINUS semantics)
			return true
		}

		hasMatch := false
		for rightIter.Next() {
			rightBinding := rightIter.Binding()

			// Check if bindings are compatible (share common variables with same values)
			if it.isCompatible(leftBinding, rightBinding) {
				hasMatch = true
				break
			}
		}
		_ = rightIter.Close() // #nosec G104 - close error doesn't affect iteration logic

		// Only return the binding if there was no match (MINUS semantics)
		if !hasMatch {
			return true
		}
	}

	return false
}

func (it *minusIterator) Binding() *store.Binding {
	return it.left.Binding()
}

func (it *minusIterator) Close() error {
	return it.left.Close()
}

// isCompatible checks if two bindings are compatible (no conflicting variable values)
func (it *minusIterator) isCompatible(left, right *store.Binding) bool {
	for varName, leftTerm := range left.Vars {
		if rightTerm, exists := right.Vars[varName]; exists {
			if !leftTerm.Equals(rightTerm) {
				return false
			}
		}
	}
	return true
}

// createOrderByIterator creates an iterator for ORDER BY operations
func (e *Executor) createOrderByIterator(plan *optimizer.OrderByPlan) (store.BindingIterator, error) {
	input, err := e.createIterator(plan.Input)
	if err != nil {
		return nil, err
	}

	return &orderByIterator{
		input:   input,
		orderBy: plan.OrderBy,
	}, nil
}

// orderByIterator implements ORDER BY operations
type orderByIterator struct {
	input       store.BindingIterator
	orderBy     []*parser.OrderCondition
	bindings    []*store.Binding
	position    int
	initialized bool
}

func (it *orderByIterator) Next() bool {
	// Materialize and sort all bindings on first call
	if !it.initialized {
		it.initialized = true

		// Collect all bindings
		for it.input.Next() {
			binding := it.input.Binding()
			it.bindings = append(it.bindings, binding.Clone())
		}

		// Sort bindings according to ORDER BY conditions
		it.sortBindings()
	}

	if it.position >= len(it.bindings) {
		return false
	}

	it.position++
	return true
}

func (it *orderByIterator) Binding() *store.Binding {
	if it.position > 0 && it.position <= len(it.bindings) {
		return it.bindings[it.position-1]
	}
	return store.NewBinding()
}

func (it *orderByIterator) Close() error {
	return it.input.Close()
}

// sortBindings sorts the bindings according to ORDER BY conditions
func (it *orderByIterator) sortBindings() {
	if len(it.orderBy) == 0 {
		return
	}

	// Use a simple bubble sort with comparison function
	// For better performance, could use sort.Slice with a custom Less function
	for i := 0; i < len(it.bindings); i++ {
		for j := i + 1; j < len(it.bindings); j++ {
			if it.shouldSwap(it.bindings[i], it.bindings[j]) {
				it.bindings[i], it.bindings[j] = it.bindings[j], it.bindings[i]
			}
		}
	}
}

// shouldSwap returns true if binding a should come after binding b
func (it *orderByIterator) shouldSwap(a, b *store.Binding) bool {
	// Compare based on each ORDER BY condition in order
	for _, condition := range it.orderBy {
		cmp := it.compareByCondition(a, b, condition)

		if cmp != 0 {
			// If descending, reverse the comparison
			if !condition.Ascending {
				cmp = -cmp
			}
			return cmp > 0
		}
		// If equal, continue to next condition
	}
	return false
}

// compareByCondition compares two bindings based on a single order condition
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func (it *orderByIterator) compareByCondition(a, b *store.Binding, condition *parser.OrderCondition) int {
	// For now, only handle simple variable expressions
	// TODO: Evaluate full expressions once expression evaluator is implemented

	varExpr, ok := condition.Expression.(*parser.VariableExpression)
	if !ok {
		// Can't evaluate complex expressions yet, treat as equal
		return 0
	}

	varName := varExpr.Variable.Name

	aVal, aExists := a.Vars[varName]
	bVal, bExists := b.Vars[varName]

	// Handle missing values (unbound variables)
	if !aExists && !bExists {
		return 0
	}
	if !aExists {
		return -1 // Treat unbound as less than any value
	}
	if !bExists {
		return 1
	}

	// Compare the terms
	return it.compareTerms(aVal, bVal)
}

// compareTerms compares two RDF terms
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func (it *orderByIterator) compareTerms(a, b rdf.Term) int {
	// Use string comparison for now
	// TODO: Implement proper SPARQL ordering rules
	aStr := a.String()
	bStr := b.String()

	if aStr < bStr {
		return -1
	}
	if aStr > bStr {
		return 1
	}
	return 0
}

// extractVariablesFromGraphPattern extracts all variables from a graph pattern
// in the order they first appear. This is used for SELECT * queries to determine
// the column order in result sets.
func extractVariablesFromGraphPattern(pattern *parser.GraphPattern) []*parser.Variable {
	if pattern == nil {
		return nil
	}

	seen := make(map[string]bool)
	var variables []*parser.Variable

	// Helper to add variable if not seen
	addVar := func(v *parser.Variable) {
		if v != nil && !seen[v.Name] {
			seen[v.Name] = true
			variables = append(variables, v)
		}
	}

	// Helper to extract variables from TermOrVariable
	extractFromTerm := func(t parser.TermOrVariable) {
		addVar(t.Variable)
	}

	// Process patterns in order
	var processPattern func(*parser.GraphPattern)
	processPattern = func(p *parser.GraphPattern) {
		if p == nil {
			return
		}

		// Process elements in order (preserves query text order)
		for _, elem := range p.Elements {
			if elem.Triple != nil {
				extractFromTerm(elem.Triple.Subject)
				extractFromTerm(elem.Triple.Predicate)
				extractFromTerm(elem.Triple.Object)
			}
			if elem.Bind != nil {
				addVar(elem.Bind.Variable)
			}
			// Filters don't introduce new variables
		}

		// Also process legacy Patterns array (for backward compatibility)
		for _, triple := range p.Patterns {
			extractFromTerm(triple.Subject)
			extractFromTerm(triple.Predicate)
			extractFromTerm(triple.Object)
		}

		// Process BIND expressions (legacy)
		for _, bind := range p.Binds {
			addVar(bind.Variable)
		}

		// Recursively process child patterns (UNION, OPTIONAL, etc.)
		for _, child := range p.Children {
			processPattern(child)
		}
	}

	processPattern(pattern)
	return variables
}
