package evaluator

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
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
	// SPARQL equality with value-based comparison for compatible types
	result, err := e.sparqlEquals(left, right)
	if err != nil {
		// If comparison is undefined (incompatible types), return false
		return rdf.NewBooleanLiteral(false), nil
	}
	return rdf.NewBooleanLiteral(result), nil
}

func (e *Evaluator) evaluateNotEqual(left, right rdf.Term) (rdf.Term, error) {
	// SPARQL inequality with value-based comparison for compatible types
	result, err := e.sparqlEquals(left, right)
	if err != nil {
		// Propagate error - incompatible types cause filter to fail
		return nil, err
	}
	return rdf.NewBooleanLiteral(!result), nil
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

// sparqlEquals implements SPARQL equality semantics
// Returns true if terms are equal, false if not equal, error if incompatible
func (e *Evaluator) sparqlEquals(left, right rdf.Term) (bool, error) {
	// DEBUG: Print what we're comparing
	// fmt.Printf("DEBUG sparqlEquals: left=%v (type=%T), right=%v (type=%T)\n", left, left, right, right)

	// If both are literals, try value-based comparison
	leftLit, leftIsLit := left.(*rdf.Literal)
	rightLit, rightIsLit := right.(*rdf.Literal)

	if leftIsLit && rightIsLit {
		// Try numeric comparison first
		leftNum, leftIsNum := e.extractNumeric(left)
		rightNum, rightIsNum := e.extractNumeric(right)

		if leftIsNum && rightIsNum {
			// Numeric equality: "1"^^xsd:integer == "01"^^xsd:integer
			return leftNum == rightNum, nil
		}

		// If one is numeric and the other isn't, error
		if leftIsNum != rightIsNum {
			return false, fmt.Errorf("cannot compare numeric and non-numeric values")
		}

		// Try simple literal comparison (same datatype and value)
		// This handles strings, booleans, dates, etc.
		if leftLit.Datatype != nil && rightLit.Datatype != nil {
			// Check if datatypes are known (XSD types)
			leftKnown := e.isKnownDatatype(leftLit.Datatype.IRI)
			rightKnown := e.isKnownDatatype(rightLit.Datatype.IRI)

			if leftLit.Datatype.IRI == rightLit.Datatype.IRI {
				// Same datatype IRI
				if !leftKnown && leftLit.Value != rightLit.Value {
					// Unknown datatype with different lexical forms - cannot determine if values differ
					return false, fmt.Errorf("cannot compare unknown datatype values")
				}
				// Same datatype, same lexical form (or known datatype) - compare lexical forms
				return leftLit.Value == rightLit.Value, nil
			}

			// Different datatypes
			if !leftKnown || !rightKnown {
				// At least one unknown datatype - cannot compare
				return false, fmt.Errorf("cannot compare unknown datatypes")
			}
			// Both known but different (non-numeric) - not equal
			return false, nil
		}

		// Plain literals (no datatype, no language tag)
		if leftLit.Datatype == nil && rightLit.Datatype == nil {
			// Both plain literals
			if leftLit.Language == "" && rightLit.Language == "" {
				// Both are simple literals (no lang tag) - compare values
				return leftLit.Value == rightLit.Value, nil
			}
			// At least one has a language tag - compare with case-insensitive lang tags
			// RFC 5646: language tags are case-insensitive
			return strings.EqualFold(leftLit.Language, rightLit.Language) && leftLit.Value == rightLit.Value, nil
		}

		// SPARQL 1.0 special case: plain literal (no lang tag) equals xsd:string
		// Per SPARQL spec, simple literals are equivalent to xsd:string for equality
		if (leftLit.Datatype == nil && leftLit.Language == "" &&
			rightLit.Datatype != nil && rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#string") ||
			(rightLit.Datatype == nil && rightLit.Language == "" &&
				leftLit.Datatype != nil && leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#string") {
			return leftLit.Value == rightLit.Value, nil
		}

		// One has datatype/lang tag, other doesn't - not equal
		return false, nil
	}

	// For non-literals (IRIs, blank nodes), use RDF term equality
	return left.Equals(right), nil
}

// compareTerms compares two terms for ordering
// Returns: -1 if left < right, 0 if left == right, 1 if left > right
func (e *Evaluator) compareTerms(left, right rdf.Term) (int, error) {
	// Get literals if both are literals
	leftLit, leftIsLit := left.(*rdf.Literal)
	rightLit, rightIsLit := right.(*rdf.Literal)

	// Try numeric comparison first if both are numeric literals
	if leftIsLit && rightIsLit {
		leftNum, leftIsNum := e.extractNumeric(left)
		rightNum, rightIsNum := e.extractNumeric(right)

		if leftIsNum && rightIsNum {
			// Numeric comparison
			if leftNum < rightNum {
				return -1, nil
			} else if leftNum > rightNum {
				return 1, nil
			}
			return 0, nil
		}

		// Both literals but not numeric - try string/datetime comparison
		// Must have compatible datatypes
		if leftLit.Datatype != nil && rightLit.Datatype != nil {
			if leftLit.Datatype.IRI != rightLit.Datatype.IRI {
				// Incompatible datatypes
				return 0, fmt.Errorf("cannot compare literals with different datatypes: %s and %s",
					leftLit.Datatype.IRI, rightLit.Datatype.IRI)
			}
			// Same datatype, compare lexical forms
			if leftLit.Value < rightLit.Value {
				return -1, nil
			} else if leftLit.Value > rightLit.Value {
				return 1, nil
			}
			return 0, nil
		}

		// Plain literals - compare by value and language tag
		if leftLit.Datatype == nil && rightLit.Datatype == nil {
			if leftLit.Language != rightLit.Language {
				return 0, fmt.Errorf("cannot compare literals with different language tags")
			}
			if leftLit.Value < rightLit.Value {
				return -1, nil
			} else if leftLit.Value > rightLit.Value {
				return 1, nil
			}
			return 0, nil
		}

		// One has datatype, other doesn't
		return 0, fmt.Errorf("cannot compare typed and untyped literals")
	}

	// Not both literals - cannot compare
	return 0, fmt.Errorf("cannot compare non-literal terms")
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
// Recognizes all XSD numeric types per SPARQL spec
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
	// Integer types (all subtypes of xsd:integer)
	case "http://www.w3.org/2001/XMLSchema#integer",
		"http://www.w3.org/2001/XMLSchema#int",
		"http://www.w3.org/2001/XMLSchema#long",
		"http://www.w3.org/2001/XMLSchema#short",
		"http://www.w3.org/2001/XMLSchema#byte",
		"http://www.w3.org/2001/XMLSchema#nonPositiveInteger",
		"http://www.w3.org/2001/XMLSchema#negativeInteger",
		"http://www.w3.org/2001/XMLSchema#nonNegativeInteger",
		"http://www.w3.org/2001/XMLSchema#positiveInteger",
		"http://www.w3.org/2001/XMLSchema#unsignedLong",
		"http://www.w3.org/2001/XMLSchema#unsignedInt",
		"http://www.w3.org/2001/XMLSchema#unsignedShort",
		"http://www.w3.org/2001/XMLSchema#unsignedByte":
		intVal, err := strconv.ParseInt(lit.Value, 10, 64)
		if err != nil {
			return 0, false
		}
		val = float64(intVal)

	// Floating point and decimal types
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
// Implements SPARQL type promotion rules for arithmetic operations
func (e *Evaluator) createNumericLiteral(value float64, left, right rdf.Term) rdf.Term {
	leftLit, leftOk := left.(*rdf.Literal)
	rightLit, rightOk := right.(*rdf.Literal)

	if !leftOk || !rightOk || leftLit.Datatype == nil || rightLit.Datatype == nil {
		// Fallback to double if we can't determine types
		return rdf.NewDoubleLiteral(value)
	}

	// Get promoted type according to SPARQL type promotion rules
	promotedType := promoteNumericTypes(leftLit.Datatype.IRI, rightLit.Datatype.IRI)

	// Create result with promoted type
	switch promotedType {
	case "http://www.w3.org/2001/XMLSchema#integer":
		// All integer subtypes promote to xsd:integer
		if value == math.Floor(value) && !math.IsInf(value, 0) {
			return rdf.NewIntegerLiteral(int64(value))
		}
		// If result is not an integer (e.g., division), promote to decimal
		return rdf.NewLiteralWithDatatype(fmt.Sprintf("%g", value), rdf.XSDDecimal)

	case "http://www.w3.org/2001/XMLSchema#decimal":
		return rdf.NewLiteralWithDatatype(fmt.Sprintf("%g", value), rdf.XSDDecimal)

	case "http://www.w3.org/2001/XMLSchema#float":
		return rdf.NewLiteralWithDatatype(fmt.Sprintf("%e", float32(value)), rdf.XSDFloat)

	case "http://www.w3.org/2001/XMLSchema#double":
		return rdf.NewDoubleLiteral(value)

	default:
		return rdf.NewDoubleLiteral(value)
	}
}

// promoteNumericTypes returns the promoted type for two XSD numeric types
// Implements SPARQL 1.0 type promotion rules
func promoteNumericTypes(leftType, rightType string) string {
	// Type promotion hierarchy (higher number = wider type)
	typeRank := map[string]int{
		// Integer types all rank 1 (promote to xsd:integer)
		"http://www.w3.org/2001/XMLSchema#integer":            1,
		"http://www.w3.org/2001/XMLSchema#int":                1,
		"http://www.w3.org/2001/XMLSchema#long":               1,
		"http://www.w3.org/2001/XMLSchema#short":              1,
		"http://www.w3.org/2001/XMLSchema#byte":               1,
		"http://www.w3.org/2001/XMLSchema#nonPositiveInteger": 1,
		"http://www.w3.org/2001/XMLSchema#negativeInteger":    1,
		"http://www.w3.org/2001/XMLSchema#nonNegativeInteger": 1,
		"http://www.w3.org/2001/XMLSchema#positiveInteger":    1,
		"http://www.w3.org/2001/XMLSchema#unsignedLong":       1,
		"http://www.w3.org/2001/XMLSchema#unsignedInt":        1,
		"http://www.w3.org/2001/XMLSchema#unsignedShort":      1,
		"http://www.w3.org/2001/XMLSchema#unsignedByte":       1,
		// Decimal
		"http://www.w3.org/2001/XMLSchema#decimal": 2,
		// Float
		"http://www.w3.org/2001/XMLSchema#float": 3,
		// Double (widest)
		"http://www.w3.org/2001/XMLSchema#double": 4,
	}

	leftRank, leftExists := typeRank[leftType]
	rightRank, rightExists := typeRank[rightType]

	if !leftExists || !rightExists {
		// Unknown type, default to double
		return "http://www.w3.org/2001/XMLSchema#double"
	}

	// Promote to wider type
	maxRank := leftRank
	if rightRank > maxRank {
		maxRank = rightRank
	}

	// Return the promoted type
	switch maxRank {
	case 1:
		return "http://www.w3.org/2001/XMLSchema#integer"
	case 2:
		return "http://www.w3.org/2001/XMLSchema#decimal"
	case 3:
		return "http://www.w3.org/2001/XMLSchema#float"
	case 4:
		return "http://www.w3.org/2001/XMLSchema#double"
	default:
		return "http://www.w3.org/2001/XMLSchema#double"
	}
}

// isKnownDatatype checks if a datatype IRI is a known XSD type
func (e *Evaluator) isKnownDatatype(iri string) bool {
	// SPARQL 1.0 recognizes XSD datatypes and rdf:langString
	return strings.HasPrefix(iri, "http://www.w3.org/2001/XMLSchema#") ||
		iri == "http://www.w3.org/1999/02/22-rdf-syntax-ns#langString"
}
