package evaluator

import (
	"fmt"
	"math"
	"strconv"

	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// evaluateBinaryExpression evaluates binary operations
func (e *Evaluator) evaluateBinaryExpression(expr *parser.BinaryExpression, binding *store.Binding) (rdf.Term, error) {
	// Evaluate left and right operands
	left, err := e.Evaluate(expr.Left, binding)
	if err != nil {
		return nil, err
	}

	right, err := e.Evaluate(expr.Right, binding)
	if err != nil {
		return nil, err
	}

	switch expr.Operator {
	// Logical operators
	case parser.OpAnd:
		return e.evaluateAnd(left, right)
	case parser.OpOr:
		return e.evaluateOr(left, right)

	// Comparison operators
	case parser.OpEqual:
		return e.evaluateEqual(left, right)
	case parser.OpNotEqual:
		return e.evaluateNotEqual(left, right)
	case parser.OpLessThan:
		return e.evaluateLessThan(left, right)
	case parser.OpLessThanOrEqual:
		return e.evaluateLessThanOrEqual(left, right)
	case parser.OpGreaterThan:
		return e.evaluateGreaterThan(left, right)
	case parser.OpGreaterThanOrEqual:
		return e.evaluateGreaterThanOrEqual(left, right)

	// Arithmetic operators
	case parser.OpAdd:
		return e.evaluateAdd(left, right)
	case parser.OpSubtract:
		return e.evaluateSubtract(left, right)
	case parser.OpMultiply:
		return e.evaluateMultiply(left, right)
	case parser.OpDivide:
		return e.evaluateDivide(left, right)

	default:
		return nil, fmt.Errorf("unsupported binary operator: %v", expr.Operator)
	}
}

// evaluateUnaryExpression evaluates unary operations
func (e *Evaluator) evaluateUnaryExpression(expr *parser.UnaryExpression, binding *store.Binding) (rdf.Term, error) {
	operand, err := e.Evaluate(expr.Operand, binding)
	if err != nil {
		return nil, err
	}

	switch expr.Operator {
	case parser.OpNot:
		return e.evaluateNot(operand)
	default:
		return nil, fmt.Errorf("unsupported unary operator: %v", expr.Operator)
	}
}

// Logical operators

func (e *Evaluator) evaluateAnd(left, right rdf.Term) (rdf.Term, error) {
	leftEBV, err := e.effectiveBooleanValue(left)
	if err != nil {
		return nil, err
	}

	// Short-circuit: if left is false, return false without evaluating right
	if !leftEBV {
		return rdf.NewBooleanLiteral(false), nil
	}

	rightEBV, err := e.effectiveBooleanValue(right)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(leftEBV && rightEBV), nil
}

func (e *Evaluator) evaluateOr(left, right rdf.Term) (rdf.Term, error) {
	leftEBV, err := e.effectiveBooleanValue(left)
	if err != nil {
		// In SPARQL, if left is error but right is true, return true
		rightEBV, rightErr := e.effectiveBooleanValue(right)
		if rightErr == nil && rightEBV {
			return rdf.NewBooleanLiteral(true), nil
		}
		return nil, err
	}

	// Short-circuit: if left is true, return true
	if leftEBV {
		return rdf.NewBooleanLiteral(true), nil
	}

	rightEBV, err := e.effectiveBooleanValue(right)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(leftEBV || rightEBV), nil
}

func (e *Evaluator) evaluateNot(operand rdf.Term) (rdf.Term, error) {
	ebv, err := e.effectiveBooleanValue(operand)
	if err != nil {
		return nil, err
	}
	return rdf.NewBooleanLiteral(!ebv), nil
}

// effectiveBooleanValue computes the EBV of a term according to SPARQL spec
func (e *Evaluator) effectiveBooleanValue(term rdf.Term) (bool, error) {
	if term == nil {
		return false, fmt.Errorf("cannot compute EBV of nil term")
	}

	switch t := term.(type) {
	case *rdf.Literal:
		// Boolean literals
		if t.Datatype != nil && t.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#boolean" {
			return t.Value == "true" || t.Value == "1", nil
		}

		// Numeric literals: false if zero, true otherwise
		if t.Datatype != nil {
			switch t.Datatype.IRI {
			case "http://www.w3.org/2001/XMLSchema#integer",
				"http://www.w3.org/2001/XMLSchema#int",
				"http://www.w3.org/2001/XMLSchema#long":
				val, err := strconv.ParseInt(t.Value, 10, 64)
				if err != nil {
					return false, fmt.Errorf("invalid integer literal: %w", err)
				}
				return val != 0, nil

			case "http://www.w3.org/2001/XMLSchema#double",
				"http://www.w3.org/2001/XMLSchema#float",
				"http://www.w3.org/2001/XMLSchema#decimal":
				val, err := strconv.ParseFloat(t.Value, 64)
				if err != nil {
					return false, fmt.Errorf("invalid numeric literal: %w", err)
				}
				return val != 0 && !math.IsNaN(val), nil
			}
		}

		// String literals: false if empty, true otherwise
		if t.Datatype == nil || t.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#string" {
			return t.Value != "", nil
		}

		// Other literals: error
		return false, fmt.Errorf("cannot compute EBV of literal with datatype %s", t.Datatype.IRI)

	default:
		// IRIs, blank nodes, etc.: error
		return false, fmt.Errorf("cannot compute EBV of non-literal term")
	}
}

// Comparison operators

func (e *Evaluator) evaluateEqual(left, right rdf.Term) (rdf.Term, error) {
	// Use RDF term equality
	result := left.Equals(right)
	return rdf.NewBooleanLiteral(result), nil
}

func (e *Evaluator) evaluateNotEqual(left, right rdf.Term) (rdf.Term, error) {
	result := !left.Equals(right)
	return rdf.NewBooleanLiteral(result), nil
}

func (e *Evaluator) evaluateLessThan(left, right rdf.Term) (rdf.Term, error) {
	cmp, err := e.compareTerms(left, right)
	if err != nil {
		return nil, err
	}
	return rdf.NewBooleanLiteral(cmp < 0), nil
}

func (e *Evaluator) evaluateLessThanOrEqual(left, right rdf.Term) (rdf.Term, error) {
	cmp, err := e.compareTerms(left, right)
	if err != nil {
		return nil, err
	}
	return rdf.NewBooleanLiteral(cmp <= 0), nil
}

func (e *Evaluator) evaluateGreaterThan(left, right rdf.Term) (rdf.Term, error) {
	cmp, err := e.compareTerms(left, right)
	if err != nil {
		return nil, err
	}
	return rdf.NewBooleanLiteral(cmp > 0), nil
}

func (e *Evaluator) evaluateGreaterThanOrEqual(left, right rdf.Term) (rdf.Term, error) {
	cmp, err := e.compareTerms(left, right)
	if err != nil {
		return nil, err
	}
	return rdf.NewBooleanLiteral(cmp >= 0), nil
}

// compareTerms compares two terms for ordering
// Returns: -1 if left < right, 0 if left == right, 1 if left > right
func (e *Evaluator) compareTerms(left, right rdf.Term) (int, error) {
	// Try numeric comparison first
	leftNum, leftIsNum := e.extractNumeric(left)
	rightNum, rightIsNum := e.extractNumeric(right)

	if leftIsNum && rightIsNum {
		if leftNum < rightNum {
			return -1, nil
		} else if leftNum > rightNum {
			return 1, nil
		}
		return 0, nil
	}

	// Try string comparison
	leftStr := left.String()
	rightStr := right.String()

	if leftStr < rightStr {
		return -1, nil
	} else if leftStr > rightStr {
		return 1, nil
	}
	return 0, nil
}

// Arithmetic operators

func (e *Evaluator) evaluateAdd(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.extractNumeric(left)
	rightVal, rightOk := e.extractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot add non-numeric terms")
	}

	result := leftVal + rightVal
	return e.createNumericLiteral(result, left, right), nil
}

func (e *Evaluator) evaluateSubtract(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.extractNumeric(left)
	rightVal, rightOk := e.extractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot subtract non-numeric terms")
	}

	result := leftVal - rightVal
	return e.createNumericLiteral(result, left, right), nil
}

func (e *Evaluator) evaluateMultiply(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.extractNumeric(left)
	rightVal, rightOk := e.extractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot multiply non-numeric terms")
	}

	result := leftVal * rightVal
	return e.createNumericLiteral(result, left, right), nil
}

func (e *Evaluator) evaluateDivide(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.extractNumeric(left)
	rightVal, rightOk := e.extractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot divide non-numeric terms")
	}

	if rightVal == 0 {
		return nil, fmt.Errorf("division by zero")
	}

	result := leftVal / rightVal
	return e.createNumericLiteral(result, left, right), nil
}

// Helper functions

// extractNumeric extracts a numeric value from a literal
func (e *Evaluator) extractNumeric(term rdf.Term) (float64, bool) {
	lit, ok := term.(*rdf.Literal)
	if !ok {
		return 0, false
	}

	if lit.Datatype == nil {
		return 0, false
	}

	var val float64
	var err error

	switch lit.Datatype.IRI {
	case "http://www.w3.org/2001/XMLSchema#integer",
		"http://www.w3.org/2001/XMLSchema#int",
		"http://www.w3.org/2001/XMLSchema#long":
		intVal, err := strconv.ParseInt(lit.Value, 10, 64)
		if err != nil {
			return 0, false
		}
		val = float64(intVal)

	case "http://www.w3.org/2001/XMLSchema#double",
		"http://www.w3.org/2001/XMLSchema#float",
		"http://www.w3.org/2001/XMLSchema#decimal":
		val, err = strconv.ParseFloat(lit.Value, 64)
		if err != nil {
			return 0, false
		}

	default:
		return 0, false
	}

	return val, true
}

// createNumericLiteral creates a numeric literal from a float64 value
// Tries to preserve the type of the input literals
func (e *Evaluator) createNumericLiteral(value float64, left, right rdf.Term) rdf.Term {
	// Check if result is actually an integer
	if value == math.Floor(value) && !math.IsInf(value, 0) {
		// Both inputs are integers, return integer
		leftLit, leftOk := left.(*rdf.Literal)
		rightLit, rightOk := right.(*rdf.Literal)

		if leftOk && rightOk &&
			leftLit.Datatype != nil && rightLit.Datatype != nil &&
			(leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#integer" ||
				leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#int") &&
			(rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#integer" ||
				rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#int") {
			return rdf.NewIntegerLiteral(int64(value))
		}
	}

	// Otherwise return double
	return rdf.NewDoubleLiteral(value)
}
