package evaluator

import (
	"fmt"
	"math"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// evaluateFunctionCall evaluates a function call expression
func (e *Evaluator) evaluateFunctionCall(expr *parser.FunctionCallExpression, binding *store.Binding) (rdf.Term, error) {
	funcName := strings.ToUpper(expr.Function)

	switch funcName {
	// Type checking functions
	case "BOUND":
		return e.evaluateBound(expr.Arguments, binding)
	case "ISIRI", "ISURI":
		return e.evaluateIsIRI(expr.Arguments, binding)
	case "ISBLANK":
		return e.evaluateIsBlank(expr.Arguments, binding)
	case "ISLITERAL":
		return e.evaluateIsLiteral(expr.Arguments, binding)
	case "ISNUMERIC":
		return e.evaluateIsNumeric(expr.Arguments, binding)

	// Value extraction functions
	case "STR":
		return e.evaluateStr(expr.Arguments, binding)
	case "LANG":
		return e.evaluateLang(expr.Arguments, binding)
	case "DATATYPE":
		return e.evaluateDatatype(expr.Arguments, binding)

	// String functions
	case "STRLEN":
		return e.evaluateStrLen(expr.Arguments, binding)
	case "SUBSTR":
		return e.evaluateSubStr(expr.Arguments, binding)
	case "UCASE":
		return e.evaluateUCase(expr.Arguments, binding)
	case "LCASE":
		return e.evaluateLCase(expr.Arguments, binding)
	case "CONCAT":
		return e.evaluateConcat(expr.Arguments, binding)
	case "CONTAINS":
		return e.evaluateContains(expr.Arguments, binding)
	case "STRSTARTS":
		return e.evaluateStrStarts(expr.Arguments, binding)
	case "STRENDS":
		return e.evaluateStrEnds(expr.Arguments, binding)

	// Numeric functions
	case "ABS":
		return e.evaluateAbs(expr.Arguments, binding)
	case "CEIL":
		return e.evaluateCeil(expr.Arguments, binding)
	case "FLOOR":
		return e.evaluateFloor(expr.Arguments, binding)
	case "ROUND":
		return e.evaluateRound(expr.Arguments, binding)

	default:
		return nil, fmt.Errorf("unsupported function: %s", funcName)
	}
}

// Type checking functions

func (e *Evaluator) evaluateBound(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("BOUND requires exactly 1 argument")
	}

	// BOUND is special - it doesn't evaluate the argument, just checks if the variable is bound
	varExpr, ok := args[0].(*parser.VariableExpression)
	if !ok {
		return nil, fmt.Errorf("BOUND requires a variable argument")
	}

	_, exists := binding.Vars[varExpr.Variable.Name]
	return rdf.NewBooleanLiteral(exists), nil
}

func (e *Evaluator) evaluateIsIRI(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isIRI requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isIRI := term.(*rdf.NamedNode)
	return rdf.NewBooleanLiteral(isIRI), nil
}

func (e *Evaluator) evaluateIsBlank(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isBlank requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isBlank := term.(*rdf.BlankNode)
	return rdf.NewBooleanLiteral(isBlank), nil
}

func (e *Evaluator) evaluateIsLiteral(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isLiteral requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isLiteral := term.(*rdf.Literal)
	return rdf.NewBooleanLiteral(isLiteral), nil
}

func (e *Evaluator) evaluateIsNumeric(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isNumeric requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isNumeric := e.extractNumeric(term)
	return rdf.NewBooleanLiteral(isNumeric), nil
}

// Value extraction functions

func (e *Evaluator) evaluateStr(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("STR requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	switch t := term.(type) {
	case *rdf.NamedNode:
		return rdf.NewLiteral(t.IRI), nil
	case *rdf.Literal:
		return rdf.NewLiteral(t.Value), nil
	case *rdf.BlankNode:
		return nil, fmt.Errorf("STR cannot be applied to blank nodes")
	default:
		return nil, fmt.Errorf("STR: unsupported term type")
	}
}

func (e *Evaluator) evaluateLang(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("LANG requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	lit, ok := term.(*rdf.Literal)
	if !ok {
		return rdf.NewLiteral(""), nil
	}

	return rdf.NewLiteral(lit.Language), nil
}

func (e *Evaluator) evaluateDatatype(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("DATATYPE requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	lit, ok := term.(*rdf.Literal)
	if !ok {
		return nil, fmt.Errorf("DATATYPE can only be applied to literals")
	}

	if lit.Datatype != nil {
		return lit.Datatype, nil
	}

	// Default to xsd:string if no datatype
	return rdf.NewNamedNode("http://www.w3.org/2001/XMLSchema#string"), nil
}

// String functions

func (e *Evaluator) evaluateStrLen(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("STRLEN requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	return rdf.NewIntegerLiteral(int64(len(str))), nil
}

func (e *Evaluator) evaluateSubStr(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("SUBSTR requires 2 or 3 arguments")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	startTerm, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	start, ok := e.extractNumeric(startTerm)
	if !ok {
		return nil, fmt.Errorf("SUBSTR start position must be numeric")
	}

	// SPARQL uses 1-based indexing
	startIdx := int(start) - 1
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(str) {
		return rdf.NewLiteral(""), nil
	}

	if len(args) == 3 {
		lengthTerm, err := e.Evaluate(args[2], binding)
		if err != nil {
			return nil, err
		}

		length, ok := e.extractNumeric(lengthTerm)
		if !ok {
			return nil, fmt.Errorf("SUBSTR length must be numeric")
		}

		endIdx := startIdx + int(length)
		if endIdx > len(str) {
			endIdx = len(str)
		}

		return rdf.NewLiteral(str[startIdx:endIdx]), nil
	}

	return rdf.NewLiteral(str[startIdx:]), nil
}

func (e *Evaluator) evaluateUCase(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("UCASE requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	return rdf.NewLiteral(strings.ToUpper(str)), nil
}

func (e *Evaluator) evaluateLCase(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("LCASE requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	return rdf.NewLiteral(strings.ToLower(str)), nil
}

func (e *Evaluator) evaluateConcat(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) == 0 {
		return rdf.NewLiteral(""), nil
	}

	var result strings.Builder
	for _, arg := range args {
		term, err := e.Evaluate(arg, binding)
		if err != nil {
			return nil, err
		}

		str, err := e.extractString(term)
		if err != nil {
			return nil, err
		}

		result.WriteString(str)
	}

	return rdf.NewLiteral(result.String()), nil
}

func (e *Evaluator) evaluateContains(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("CONTAINS requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	str1, err := e.extractString(term1)
	if err != nil {
		return nil, err
	}

	str2, err := e.extractString(term2)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(strings.Contains(str1, str2)), nil
}

func (e *Evaluator) evaluateStrStarts(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("STRSTARTS requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	str1, err := e.extractString(term1)
	if err != nil {
		return nil, err
	}

	str2, err := e.extractString(term2)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(strings.HasPrefix(str1, str2)), nil
}

func (e *Evaluator) evaluateStrEnds(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("STRENDS requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	str1, err := e.extractString(term1)
	if err != nil {
		return nil, err
	}

	str2, err := e.extractString(term2)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(strings.HasSuffix(str1, str2)), nil
}

// Numeric functions

func (e *Evaluator) evaluateAbs(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ABS requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("ABS requires numeric argument")
	}

	return e.createNumericLiteral(math.Abs(val), term, term), nil
}

func (e *Evaluator) evaluateCeil(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("CEIL requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("CEIL requires numeric argument")
	}

	return rdf.NewIntegerLiteral(int64(math.Ceil(val))), nil
}

func (e *Evaluator) evaluateFloor(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("FLOOR requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("FLOOR requires numeric argument")
	}

	return rdf.NewIntegerLiteral(int64(math.Floor(val))), nil
}

func (e *Evaluator) evaluateRound(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ROUND requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("ROUND requires numeric argument")
	}

	return rdf.NewIntegerLiteral(int64(math.Round(val))), nil
}

// Helper function

func (e *Evaluator) extractString(term rdf.Term) (string, error) {
	switch t := term.(type) {
	case *rdf.Literal:
		return t.Value, nil
	case *rdf.NamedNode:
		return t.IRI, nil
	default:
		return "", fmt.Errorf("cannot extract string from term type: %T", term)
	}
}
