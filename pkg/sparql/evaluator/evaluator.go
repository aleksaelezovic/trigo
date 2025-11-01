package evaluator

import (
	"fmt"

	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// Evaluator evaluates SPARQL expressions against bindings
type Evaluator struct {
	// Could add store reference here if needed for certain operations
}

// NewEvaluator creates a new expression evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate evaluates an expression against a binding and returns the result term
// Returns (result, error) where error is nil on success
// If the expression cannot be evaluated (type error, unbound variable, etc.), returns an error
func (e *Evaluator) Evaluate(expr parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if expr == nil {
		return nil, fmt.Errorf("cannot evaluate nil expression")
	}

	switch ex := expr.(type) {
	case *parser.BinaryExpression:
		return e.evaluateBinaryExpression(ex, binding)
	case *parser.UnaryExpression:
		return e.evaluateUnaryExpression(ex, binding)
	case *parser.VariableExpression:
		return e.evaluateVariableExpression(ex, binding)
	case *parser.LiteralExpression:
		return e.evaluateLiteralExpression(ex, binding)
	case *parser.FunctionCallExpression:
		return e.evaluateFunctionCall(ex, binding)
	case *parser.ExistsExpression:
		return e.evaluateExistsExpression(ex, binding)
	case *parser.InExpression:
		return e.evaluateInExpression(ex, binding)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// evaluateVariableExpression evaluates a variable reference
func (e *Evaluator) evaluateVariableExpression(expr *parser.VariableExpression, binding *store.Binding) (rdf.Term, error) {
	if expr.Variable == nil {
		return nil, fmt.Errorf("variable expression has nil variable")
	}

	// Special case for COUNT(*) which uses variable name "*"
	if expr.Variable.Name == "*" {
		return nil, fmt.Errorf("* is not a valid variable reference in expressions")
	}

	// Look up variable in binding
	value, exists := binding.Vars[expr.Variable.Name]
	if !exists {
		return nil, fmt.Errorf("unbound variable: ?%s", expr.Variable.Name)
	}

	return value, nil
}

// evaluateLiteralExpression evaluates a literal constant
func (e *Evaluator) evaluateLiteralExpression(expr *parser.LiteralExpression, binding *store.Binding) (rdf.Term, error) {
	if expr.Literal == nil {
		return nil, fmt.Errorf("literal expression has nil literal")
	}
	return expr.Literal, nil
}

// evaluateExistsExpression evaluates EXISTS or NOT EXISTS
func (e *Evaluator) evaluateExistsExpression(expr *parser.ExistsExpression, binding *store.Binding) (rdf.Term, error) {
	// TODO: Implement EXISTS/NOT EXISTS evaluation
	// This requires executing the graph pattern against the store with the current binding
	// and checking if any results are returned.
	// For now, return an error to indicate it's not yet implemented.
	return nil, fmt.Errorf("EXISTS/NOT EXISTS evaluation not yet implemented")
}

// evaluateInExpression evaluates IN or NOT IN operator
// x IN (e1, e2, ...) is equivalent to (x = e1) || (x = e2) || ...
// x NOT IN (e1, e2, ...) is equivalent to !((x = e1) || (x = e2) || ...)
func (e *Evaluator) evaluateInExpression(expr *parser.InExpression, binding *store.Binding) (rdf.Term, error) {
	// Evaluate the left-hand expression
	leftValue, err := e.Evaluate(expr.Expression, binding)
	if err != nil {
		return nil, err
	}

	// Check if leftValue equals any of the values in the list
	found := false
	for _, valueExpr := range expr.Values {
		rightValue, err := e.Evaluate(valueExpr, binding)
		if err != nil {
			// If evaluation fails for any value, skip it (SPARQL semantics)
			continue
		}

		// Check equality
		if leftValue.Equals(rightValue) {
			found = true
			break
		}
	}

	// Apply NOT if needed
	if expr.Not {
		return rdf.NewBooleanLiteral(!found), nil
	}
	return rdf.NewBooleanLiteral(found), nil
}
