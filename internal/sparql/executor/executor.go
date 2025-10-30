package executor

import (
	"fmt"

	"github.com/aleksaelezovic/trigo/internal/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/internal/sparql/parser"
	"github.com/aleksaelezovic/trigo/internal/store"
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

	return &SelectResult{
		Variables: query.Original.Select.Variables,
		Bindings:  bindings,
	}, nil
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
		input:  input,
		filter: plan.Filter,
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
func (e *Executor) convertTermOrVariable(tov parser.TermOrVariable) interface{} {
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
	if !it.quadIter.Next() {
		return false
	}

	quad, err := it.quadIter.Quad()
	if err != nil {
		return false
	}

	// Bind variables
	it.binding = store.NewBinding()

	if it.pattern.Subject.IsVariable() {
		it.binding.Vars[it.pattern.Subject.Variable.Name] = quad.Subject
	}
	if it.pattern.Predicate.IsVariable() {
		it.binding.Vars[it.pattern.Predicate.Variable.Name] = quad.Predicate
	}
	if it.pattern.Object.IsVariable() {
		it.binding.Vars[it.pattern.Object.Variable.Name] = quad.Object
	}

	return true
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
	input  store.BindingIterator
	filter *parser.Filter
}

func (it *filterIterator) Next() bool {
	for it.input.Next() {
		// TODO: Evaluate filter expression
		// For now, pass through all bindings
		return true
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
