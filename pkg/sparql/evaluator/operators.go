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
	// Special handling for logical operators to support short-circuit evaluation
	// This is critical for SPARQL patterns like: !bound(?x) || ?x = 5
	// where evaluating the right side would error if ?x is unbound
	if expr.Operator == parser.OpOr {
		// Evaluate left operand
		left, leftErr := e.Evaluate(expr.Left, binding)

		// If left is true, short-circuit (don't evaluate right)
		if leftErr == nil {
			leftEBV, err := e.EffectiveBooleanValue(left)
			if err == nil && leftEBV {
				return rdf.NewBooleanLiteral(true), nil
			}
		}

		// Either left is false/error, so we need to evaluate right
		right, rightErr := e.Evaluate(expr.Right, binding)

		// If we have both values, use the standard OR logic
		if leftErr == nil && rightErr == nil {
			return e.evaluateOr(left, right)
		}

		// SPARQL OR error semantics:
		// - If right is true, return true (even if left errored)
		// - If both error, return error
		if rightErr == nil {
			rightEBV, err := e.EffectiveBooleanValue(right)
			if err == nil && rightEBV {
				return rdf.NewBooleanLiteral(true), nil
			}
		}

		// Return left error if it exists, otherwise right error
		if leftErr != nil {
			return nil, leftErr
		}
		return nil, rightErr
	}

	if expr.Operator == parser.OpAnd {
		// Evaluate left operand
		left, leftErr := e.Evaluate(expr.Left, binding)

		// If left has error or is false, short-circuit
		if leftErr == nil {
			leftEBV, err := e.EffectiveBooleanValue(left)
			if err == nil && !leftEBV {
				return rdf.NewBooleanLiteral(false), nil
			}
		}

		// Left is true or error, evaluate right
		right, rightErr := e.Evaluate(expr.Right, binding)

		// If we have both values, use standard AND logic
		if leftErr == nil && rightErr == nil {
			return e.evaluateAnd(left, right)
		}

		// SPARQL AND error semantics:
		// - If either side errors, return error (unless one side is false)
		if leftErr != nil {
			return nil, leftErr
		}
		return nil, rightErr
	}

	// For all other operators, evaluate both operands first
	left, err := e.Evaluate(expr.Left, binding)
	if err != nil {
		return nil, err
	}

	right, err := e.Evaluate(expr.Right, binding)
	if err != nil {
		return nil, err
	}

	switch expr.Operator {
	// Logical operators (already handled above, but keep for clarity)
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
	leftEBV, err := e.EffectiveBooleanValue(left)
	if err != nil {
		return nil, err
	}

	// Short-circuit: if left is false, return false without evaluating right
	if !leftEBV {
		return rdf.NewBooleanLiteral(false), nil
	}

	rightEBV, err := e.EffectiveBooleanValue(right)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(leftEBV && rightEBV), nil
}

func (e *Evaluator) evaluateOr(left, right rdf.Term) (rdf.Term, error) {
	leftEBV, err := e.EffectiveBooleanValue(left)
	if err != nil {
		// In SPARQL, if left is error but right is true, return true
		rightEBV, rightErr := e.EffectiveBooleanValue(right)
		if rightErr == nil && rightEBV {
			return rdf.NewBooleanLiteral(true), nil
		}
		return nil, err
	}

	// Short-circuit: if left is true, return true
	if leftEBV {
		return rdf.NewBooleanLiteral(true), nil
	}

	rightEBV, err := e.EffectiveBooleanValue(right)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(leftEBV || rightEBV), nil
}

func (e *Evaluator) evaluateNot(operand rdf.Term) (rdf.Term, error) {
	ebv, err := e.EffectiveBooleanValue(operand)
	if err != nil {
		return nil, err
	}
	return rdf.NewBooleanLiteral(!ebv), nil
}

// EffectiveBooleanValue computes the EBV of a term according to SPARQL spec
func (e *Evaluator) EffectiveBooleanValue(term rdf.Term) (bool, error) {
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
	// Uncomment for debugging:
	// fmt.Printf("DEBUG sparqlEquals: left=%v (type=%T), right=%v (type=%T)\n", left, left, right, right)

	// If both are literals, try value-based comparison
	leftLit, leftIsLit := left.(*rdf.Literal)
	rightLit, rightIsLit := right.(*rdf.Literal)

	if leftIsLit && rightIsLit {
		// Check if both are the same term (same datatype IRI and lexical value)
		// This includes the case of identical invalid literals
		if leftLit.Datatype != nil && rightLit.Datatype != nil {
			if leftLit.Datatype.IRI == rightLit.Datatype.IRI && leftLit.Value == rightLit.Value {
				// Identical terms are equal, even if invalid
				return true, nil
			}
		}

		// For different terms with invalid numeric literals:
		// SPARQL spec allows implementations to handle this as an extension point
		// We'll treat them lexically - if one is invalid, error when comparing to valid numerics
		// but allow lexical comparison between two invalid numerics with same datatype
		leftInvalid := leftLit.Datatype != nil && e.isNumericDatatype(leftLit.Datatype.IRI)
		if leftInvalid {
			if _, ok := e.ExtractNumeric(left); !ok {
				leftInvalid = true
			} else {
				leftInvalid = false
			}
		}

		rightInvalid := rightLit.Datatype != nil && e.isNumericDatatype(rightLit.Datatype.IRI)
		if rightInvalid {
			if _, ok := e.ExtractNumeric(right); !ok {
				rightInvalid = true
			} else {
				rightInvalid = false
			}
		}

		// If both have same numeric datatype and both are invalid with different values, error
		// (same datatype + same value was already handled at line 317)
		if leftInvalid && rightInvalid {
			// Both invalid numerics - error
			return false, fmt.Errorf("cannot compare two invalid numeric literals")
		}

		// If one is invalid numeric and the other is not a numeric type:
		// - Can compare with lang-tagged literals (return false, not equal)
		// - Cannot compare with plain literals or other known datatypes (error)
		if leftInvalid {
			// Left is invalid numeric.
			if rightLit.Datatype == nil && rightLit.Language != "" {
				// Right is lang-tagged - these are incomparable types, return not equal
				return false, nil
			}
			if rightLit.Datatype == nil || !e.isNumericDatatype(rightLit.Datatype.IRI) {
				// Right is plain literal or non-numeric datatype - error
				return false, fmt.Errorf("cannot compare invalid numeric with non-numeric literal")
			}
			// Both are numeric types, one invalid - error
			return false, fmt.Errorf("cannot compare invalid and valid numeric literals")
		}
		if rightInvalid {
			// Right is invalid numeric.
			if leftLit.Datatype == nil && leftLit.Language != "" {
				// Left is lang-tagged - these are incomparable types, return not equal
				return false, nil
			}
			if leftLit.Datatype == nil || !e.isNumericDatatype(leftLit.Datatype.IRI) {
				// Left is plain literal or non-numeric datatype - error
				return false, fmt.Errorf("cannot compare invalid numeric with non-numeric literal")
			}
			// Both are numeric types, one invalid - error
			return false, fmt.Errorf("cannot compare valid and invalid numeric literals")
		}
		// Both valid, continue with normal comparison below

		// Check for unknown datatypes (same behavior as invalid numerics)
		leftUnknown := leftLit.Datatype != nil && !e.isKnownDatatype(leftLit.Datatype.IRI)
		rightUnknown := rightLit.Datatype != nil && !e.isKnownDatatype(rightLit.Datatype.IRI)

		if leftUnknown || rightUnknown {
			// At least one has unknown datatype
			// Unknown datatypes can only be compared with lang-tagged literals, not with plain or known datatypes
			if leftUnknown {
				if rightLit.Datatype == nil && rightLit.Language != "" {
					// Right is lang-tagged - incomparable types, return not equal
					return false, nil
				}
				// Right is plain or known datatype - error
				return false, fmt.Errorf("cannot compare unknown datatype with other literals")
			}
			if rightUnknown {
				if leftLit.Datatype == nil && leftLit.Language != "" {
					// Left is lang-tagged - incomparable types, return not equal
					return false, nil
				}
				// Left is plain or known datatype - error
				return false, fmt.Errorf("cannot compare unknown datatype with other literals")
			}
		}

		// Try numeric comparison first
		leftNum, leftIsNum := e.ExtractNumeric(left)
		rightNum, rightIsNum := e.ExtractNumeric(right)

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
			// Special handling for date/dateTime - use semantic comparison with timezone normalization
			if leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#dateTime" &&
				rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#dateTime" {
				leftTime, err1 := e.parseDateTimeValue(leftLit.Value)
				rightTime, err2 := e.parseDateTimeValue(rightLit.Value)
				if err1 != nil || err2 != nil {
					// Invalid datetime values
					return false, fmt.Errorf("cannot compare invalid dateTime values")
				}
				return leftTime == rightTime, nil
			}

			if leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#date" &&
				rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#date" {
				leftDate, leftHasTz, err1 := e.parseDateValue(leftLit.Value)
				rightDate, rightHasTz, err2 := e.parseDateValue(rightLit.Value)
				if err1 != nil || err2 != nil {
					// Invalid date values
					return false, fmt.Errorf("cannot compare invalid date values")
				}
				// Check if dates are equal first (UTC-normalized comparison)
				datesEqual := leftDate == rightDate
				// Per SPARQL spec: dates with different timezone presence can be unequal,
				// but error if they would be equal with different TZ presence (ambiguous equality)
				if datesEqual && leftHasTz != rightHasTz {
					return false, fmt.Errorf("cannot determine equality of dates with different timezone presence")
				}
				return datesEqual, nil
			}

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
	// DEBUG: Log all comparisons
	// fmt.Printf("DEBUG compareTerms: left=%v (type=%T), right=%v (type=%T)\n", left, left, right, right)

	// Get literals if both are literals
	leftLit, leftIsLit := left.(*rdf.Literal)
	rightLit, rightIsLit := right.(*rdf.Literal)

	// Cannot compare different term types (IRI vs literal, blank vs literal, etc.)
	if !leftIsLit && !rightIsLit {
		// Both are non-literals (IRI or blank node) - cannot use ordering operators
		return 0, fmt.Errorf("cannot compare non-literal terms")
	}
	if leftIsLit != rightIsLit {
		// One is literal, other isn't - cannot compare
		return 0, fmt.Errorf("cannot compare literal and non-literal terms")
	}

	// Try numeric comparison first if both are numeric literals
	if leftIsLit && rightIsLit {
		leftNum, leftIsNum := e.ExtractNumeric(left)
		rightNum, rightIsNum := e.ExtractNumeric(right)

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
			// Special handling for date/dateTime - use semantic comparison with timezone normalization
			if leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#dateTime" &&
				rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#dateTime" {
				leftTime, err1 := e.parseDateTimeValue(leftLit.Value)
				rightTime, err2 := e.parseDateTimeValue(rightLit.Value)
				if err1 != nil || err2 != nil {
					return 0, fmt.Errorf("cannot compare invalid dateTime values")
				}
				if leftTime < rightTime {
					return -1, nil
				} else if leftTime > rightTime {
					return 1, nil
				}
				return 0, nil
			}

			if leftLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#date" &&
				rightLit.Datatype.IRI == "http://www.w3.org/2001/XMLSchema#date" {
				leftDate, _, err1 := e.parseDateValue(leftLit.Value)
				rightDate, _, err2 := e.parseDateValue(rightLit.Value)
				if err1 != nil || err2 != nil {
					return 0, fmt.Errorf("cannot compare invalid date values")
				}
				// For ordering operations (<, >, <=, >=), allow comparison even with different TZ presence
				// The values are normalized to UTC for comparison
				if leftDate < rightDate {
					return -1, nil
				} else if leftDate > rightDate {
					return 1, nil
				}
				return 0, nil
			}

			if leftLit.Datatype.IRI != rightLit.Datatype.IRI {
				// Incompatible datatypes
				return 0, fmt.Errorf("cannot compare literals with different datatypes: %s and %s",
					leftLit.Datatype.IRI, rightLit.Datatype.IRI)
			}
			// Same datatype - but can only compare if it's a known datatype
			if !e.isKnownDatatype(leftLit.Datatype.IRI) {
				return 0, fmt.Errorf("cannot compare literals with unknown datatype: %s", leftLit.Datatype.IRI)
			}
			// Same known datatype, compare lexical forms
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

		// One has datatype, other doesn't - cannot compare
		return 0, fmt.Errorf("cannot compare typed and untyped literals")
	}

	// Not both literals - cannot compare
	return 0, fmt.Errorf("cannot compare non-literal terms")
}

// Arithmetic operators

func (e *Evaluator) evaluateAdd(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.ExtractNumeric(left)
	rightVal, rightOk := e.ExtractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot add non-numeric terms")
	}

	result := leftVal + rightVal
	return e.createNumericLiteral(result, left, right), nil
}

func (e *Evaluator) evaluateSubtract(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.ExtractNumeric(left)
	rightVal, rightOk := e.ExtractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot subtract non-numeric terms")
	}

	result := leftVal - rightVal
	return e.createNumericLiteral(result, left, right), nil
}

func (e *Evaluator) evaluateMultiply(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.ExtractNumeric(left)
	rightVal, rightOk := e.ExtractNumeric(right)

	if !leftOk || !rightOk {
		return nil, fmt.Errorf("cannot multiply non-numeric terms")
	}

	result := leftVal * rightVal
	return e.createNumericLiteral(result, left, right), nil
}

func (e *Evaluator) evaluateDivide(left, right rdf.Term) (rdf.Term, error) {
	leftVal, leftOk := e.ExtractNumeric(left)
	rightVal, rightOk := e.ExtractNumeric(right)

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

// ExtractNumeric extracts a numeric value from a literal
// Recognizes all XSD numeric types per SPARQL spec
// Exported for use in DISTINCT deduplication
func (e *Evaluator) ExtractNumeric(term rdf.Term) (float64, bool) {
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

// isNumericDatatype checks if a datatype IRI represents a numeric type
func (e *Evaluator) isNumericDatatype(iri string) bool {
	switch iri {
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
		"http://www.w3.org/2001/XMLSchema#unsignedByte",
		"http://www.w3.org/2001/XMLSchema#double",
		"http://www.w3.org/2001/XMLSchema#float",
		"http://www.w3.org/2001/XMLSchema#decimal":
		return true
	}
	return false
}

// parseDateTimeValue parses an xsd:dateTime value and normalizes to UTC
// Handles various datetime formats and special cases like 24:00:00
func (e *Evaluator) parseDateTimeValue(value string) (int64, error) {
	// Handle 24:00:00 edge case (represents midnight of next day)
	// Replace "T24:00:00" with "T00:00:00" and add 1 day after parsing
	add24Hours := false
	if strings.Contains(value, "T24:00:00") {
		value = strings.Replace(value, "T24:00:00", "T00:00:00", 1)
		add24Hours = true
	}

	// Try parsing various formats
	var parsedTime int64
	var err error

	// RFC3339 with timezone (most common)
	if strings.Contains(value, "T") {
		// Try with 'T' separator
		layouts := []string{
			"2006-01-02T15:04:05.999999999Z07:00", // With fractional seconds and timezone
			"2006-01-02T15:04:05Z07:00",           // Without fractional seconds
			"2006-01-02T15:04:05.999999999",       // With fractional seconds, no timezone
			"2006-01-02T15:04:05",                 // Without fractional seconds, no timezone
		}

		for _, layout := range layouts {
			parsedTime, err = parseTimeToUnixNano(value, layout)
			if err == nil {
				break
			}
		}
	}

	if err != nil {
		return 0, fmt.Errorf("invalid dateTime format: %s", value)
	}

	// Add 24 hours if we had 24:00:00
	if add24Hours {
		parsedTime += 24 * 60 * 60 * 1_000_000_000 // 24 hours in nanoseconds
	}

	return parsedTime, nil
}

// parseDateValue parses an xsd:date value
// Returns: (unixNanos, hasExplicitTimezone, error)
// Dates without timezone have an implicit timezone and cannot be compared with explicit timezone dates
func (e *Evaluator) parseDateValue(value string) (int64, bool, error) {
	// Check if date has explicit timezone
	hasTimezone := strings.HasSuffix(value, "Z") ||
		(strings.Contains(value, "+") || strings.LastIndex(value, "-") > 8)

	parsedTime, err := parseTimeToUnixNano(value, "")
	if err != nil {
		return 0, false, fmt.Errorf("invalid date format: %s", value)
	}

	return parsedTime, hasTimezone, nil
}

// parseTimeToUnixNano is a helper that parses time and returns Unix nanoseconds
// Uses custom parsing to avoid Go's time package limitations
func parseTimeToUnixNano(value string, layout string) (int64, error) {
	// For simple comparison, we can parse the components directly
	// This is more reliable than Go's time.Parse for XSD datetime edge cases

	// Check for timezone
	tzOffset := int64(0)
	dateTimePart := value

	// Extract timezone if present
	if strings.Contains(value, "+") || strings.LastIndex(value, "-") > 8 {
		// Has explicit timezone
		tzIdx := strings.LastIndexAny(value, "+-")
		if tzIdx > 0 {
			tzStr := value[tzIdx:]
			dateTimePart = value[:tzIdx]

			// Parse timezone offset (e.g., "+05:30" or "-08:00")
			sign := int64(1)
			if tzStr[0] == '-' {
				sign = -1
			}
			tzStr = tzStr[1:] // Remove sign

			var tzHours, tzMinutes int64
			_, err := fmt.Sscanf(tzStr, "%d:%d", &tzHours, &tzMinutes)
			if err != nil {
				return 0, fmt.Errorf("invalid timezone format: %s", tzStr)
			}

			tzOffset = sign * (tzHours*60 + tzMinutes) * 60 * 1_000_000_000 // Convert to nanoseconds
		}
	} else if strings.HasSuffix(value, "Z") {
		// UTC timezone (Z)
		dateTimePart = strings.TrimSuffix(value, "Z")
		tzOffset = 0
	}

	// Parse the date/datetime components
	var year, month, day, hour, minute, second int64
	var fracSeconds float64

	if strings.Contains(dateTimePart, "T") {
		// DateTime
		parts := strings.Split(dateTimePart, "T")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid datetime format")
		}

		// Parse date part
		_, err := fmt.Sscanf(parts[0], "%d-%d-%d", &year, &month, &day)
		if err != nil {
			return 0, err
		}

		// Parse time part
		if strings.Contains(parts[1], ".") {
			// With fractional seconds
			timeParts := strings.Split(parts[1], ".")
			_, err = fmt.Sscanf(timeParts[0], "%d:%d:%d", &hour, &minute, &second)
			if err != nil {
				return 0, err
			}
			_, err = fmt.Sscanf("0."+timeParts[1], "%f", &fracSeconds)
			if err != nil {
				return 0, err
			}
		} else {
			// Without fractional seconds
			_, err = fmt.Sscanf(parts[1], "%d:%d:%d", &hour, &minute, &second)
			if err != nil {
				return 0, err
			}
		}
	} else {
		// Date only
		_, err := fmt.Sscanf(dateTimePart, "%d-%d-%d", &year, &month, &day)
		if err != nil {
			return 0, err
		}
		hour, minute, second = 0, 0, 0
	}

	// Convert to Unix timestamp (nanoseconds since epoch)
	// Simple calculation for dates after 1970
	// Days since epoch
	daysSinceEpoch := int64(0)

	// Add years
	for y := int64(1970); y < year; y++ {
		if isLeapYear(y) {
			daysSinceEpoch += 366
		} else {
			daysSinceEpoch += 365
		}
	}

	// Add months
	daysInMonth := []int64{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if isLeapYear(year) {
		daysInMonth[1] = 29
	}
	for m := int64(1); m < month; m++ {
		daysSinceEpoch += daysInMonth[m-1]
	}

	// Add days
	daysSinceEpoch += day - 1

	// Convert to nanoseconds
	nanos := daysSinceEpoch * 24 * 60 * 60 * 1_000_000_000
	nanos += hour * 60 * 60 * 1_000_000_000
	nanos += minute * 60 * 1_000_000_000
	nanos += second * 1_000_000_000
	nanos += int64(fracSeconds * 1_000_000_000)

	// Adjust for timezone (subtract offset to normalize to UTC)
	nanos -= tzOffset

	return nanos, nil
}

// isLeapYear checks if a year is a leap year
func isLeapYear(year int64) bool {
	return (year%4 == 0 && year%100 != 0) || (year%400 == 0)
}
